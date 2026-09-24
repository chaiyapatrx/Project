package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/gorilla/websocket"
)

type Config struct {
	ServerURL           string `json:"server_url"`
	MachineName         string `json:"machine_name"`
	PingIntervalSeconds int    `json:"ping_interval_seconds"`
	AgentSecret         string `json:"agent_secret"`
}

type CommandMessage struct {
	Action    string                 `json:"action"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Signature string                 `json:"signature"`
}

// maxCommandAgeSeconds bounds how old a signed command may be, mitigating replay.
const maxCommandAgeSeconds = 30

var (
	modUser32                = syscall.NewLazyDLL("user32.dll")
	procMessageBoxW          = modUser32.NewProc("MessageBoxW")
	procRegisterClassExW     = modUser32.NewProc("RegisterClassExW")
	procCreateWindowExW      = modUser32.NewProc("CreateWindowExW")
	procGetSystemMetrics     = modUser32.NewProc("GetSystemMetrics")
	procPostMessageW         = modUser32.NewProc("PostMessageW")
	procShowWindow           = modUser32.NewProc("ShowWindow")
	procSetWindowPos         = modUser32.NewProc("SetWindowPos")
	procSetForegroundWindow  = modUser32.NewProc("SetForegroundWindow")
	procInvalidateRect       = modUser32.NewProc("InvalidateRect")
	procBeginPaint           = modUser32.NewProc("BeginPaint")
	procEndPaint             = modUser32.NewProc("EndPaint")
	procGetClientRect        = modUser32.NewProc("GetClientRect")
	procGetMessageW          = modUser32.NewProc("GetMessageW")
	procTranslateMessage     = modUser32.NewProc("TranslateMessage")
	procDispatchMessageW     = modUser32.NewProc("DispatchMessageW")
	procDefWindowProcW       = modUser32.NewProc("DefWindowProcW")
	procGetStockObject       = syscall.NewLazyDLL("gdi32.dll").NewProc("GetStockObject")
	procFillRect             = syscall.NewLazyDLL("user32.dll").NewProc("FillRect")
	procSetTextColor         = syscall.NewLazyDLL("gdi32.dll").NewProc("SetTextColor")
	procSetBkMode            = syscall.NewLazyDLL("gdi32.dll").NewProc("SetBkMode")
	procSelectObject         = syscall.NewLazyDLL("gdi32.dll").NewProc("SelectObject")
	procDrawTextW            = syscall.NewLazyDLL("user32.dll").NewProc("DrawTextW")
	procCreateFontW          = syscall.NewLazyDLL("gdi32.dll").NewProc("CreateFontW")
	procGetModuleHandleW     = syscall.NewLazyDLL("kernel32.dll").NewProc("GetModuleHandleW")
	stationLockOverlayWindow uintptr
	stationLockOverlayProc   uintptr
	stationLockOverlayFont   uintptr
	stationLockOverlayText   []uint16
)

const (
	wmAppStationLock = 0x8001
	wmClose          = 0x0010
	wmPaint          = 0x000F
	wmEraseBkgnd     = 0x0014
	wmSysCommand     = 0x0112
	scClose          = 0xF060

	wsPopup        = 0x80000000
	wsExTopmost    = 0x00000008
	wsExToolWindow = 0x00000080

	swHide = 0
	swShow = 5

	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
	swpShowWindow = 0x0040

	dtCenter   = 0x00000001
	dtVCenter  = 0x00000004
	dtWordWrap = 0x00000010
)

type stationLockWndClass struct {
	Size        uint32
	Style       uint32
	WndProc     uintptr
	ClassExtra  int32
	WindowExtra int32
	Instance    syscall.Handle
	Icon        syscall.Handle
	Cursor      syscall.Handle
	Background  syscall.Handle
	MenuName    *uint16
	ClassName   *uint16
	IconSmall   syscall.Handle
}

type stationLockRect struct {
	Left, Top, Right, Bottom int32
}

type stationLockPaint struct {
	DC        syscall.Handle
	Erase     int32
	Paint     stationLockRect
	Restore   int32
	IncUpdate int32
	Reserved  [32]byte
}

type stationLockPoint struct {
	X, Y int32
}

type stationLockMessage struct {
	Window  syscall.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Point   stationLockPoint
	Private uint32
}

func main() {
	log.Println("==================================================")
	log.Println(" AUCC High-Performance Client Agent (Go)")
	log.Println("==================================================")

	if err := startStationLockOverlay(); err != nil {
		log.Fatalf("[Agent] Could not start station lock overlay: %v", err)
	}

	cfg := loadConfig()
	if strings.TrimSpace(cfg.AgentSecret) == "" || strings.TrimSpace(cfg.MachineName) == "" {
		log.Fatal("[Agent] config.json must contain machine_name and agent_secret")
	}
	hwid := getHWID()

	log.Printf("[Agent] Machine Name : %s", cfg.MachineName)
	log.Printf("[Agent] Hardware ID  : %s", hwid)
	log.Printf("[Agent] Target Server: %s", cfg.ServerURL)
	log.Println("[Agent] Starting WebSocket agent engine...")

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	// Keep-alive connection loop with exponential backoff
	backoff := 1 * time.Second
	for {
		connected, err := runAgentSession(cfg, hwid, interrupt)
		if err == nil {
			// Normal clean exit
			log.Println("[Agent] Agent exited cleanly.")
			return
		}
		if connected {
			backoff = time.Second
		}

		log.Printf("[Agent] Connection error: %v. Reconnecting in %v...", err, backoff)
		time.Sleep(backoff)
		backoff *= 2
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
	}
}

func runAgentSession(cfg *Config, hwid string, interrupt chan os.Signal) (bool, error) {
	u, err := url.Parse(cfg.ServerURL)
	if err != nil {
		return false, fmt.Errorf("invalid server URL: %w", err)
	}
	if (u.Scheme != "ws" && u.Scheme != "wss") || u.Host == "" {
		return false, fmt.Errorf("server_url must be a ws:// or wss:// URL")
	}
	if u.Scheme == "ws" && !isLoopbackHost(u.Hostname()) {
		return false, fmt.Errorf("unencrypted ws:// is allowed only for localhost; use wss:// for remote servers")
	}

	q := u.Query()
	q.Set("name", cfg.MachineName)
	q.Set("hwid", hwid)
	u.RawQuery = q.Encode()

	requestHeader := http.Header{}
	requestHeader.Set("X-Agent-Secret", cfg.AgentSecret)

	log.Printf("[WS] Connecting to %s...", u.String())
	c, _, err := websocket.DefaultDialer.Dial(u.String(), requestHeader)
	if err != nil {
		return false, err
	}
	defer c.Close()

	log.Println("[WS] Connected to Server successfully!")

	done := make(chan struct{})

	// Read loop: receives real-time commands
	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Printf("[WS] Read error: %v", err)
				return
			}

			var cmd CommandMessage
			if err := json.Unmarshal(message, &cmd); err != nil {
				log.Printf("[WS] Malformed command: %s", string(message))
				continue
			}

			if !verifyCommand(cmd, cfg.AgentSecret) {
				log.Printf("[Security] Rejected unsigned/invalid/stale command: %s", cmd.Action)
				continue
			}

			handleServerCommand(cmd)
		}
	}()

	// Heartbeat / Ping ticker
	interval := cfg.PingIntervalSeconds
	if interval <= 0 {
		interval = 15
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return true, fmt.Errorf("connection closed by server")
		case <-ticker.C:
			pingPayload := map[string]string{
				"type": "PING",
				"time": time.Now().Format(time.RFC3339),
			}
			data, _ := json.Marshal(pingPayload)
			err := c.WriteMessage(websocket.TextMessage, data)
			if err != nil {
				log.Printf("[WS] Failed to write ping: %v", err)
				return true, err
			}
		case <-interrupt:
			log.Println("[Agent] Interrupt received. Disconnecting...")
			_ = c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Agent shutdown"))
			return true, nil
		}
	}
}

// verifyCommand validates the HMAC-SHA256 signature and freshness of a command.
// Commands are rejected when the shared agent secret is missing.
func verifyCommand(cmd CommandMessage, secret string) bool {
	if secret == "" {
		return false
	}
	if cmd.Signature == "" {
		return false
	}

	// Reject stale/replayed commands outside the freshness window.
	age := time.Now().Unix() - cmd.Timestamp
	if age < -maxCommandAgeSeconds || age > maxCommandAgeSeconds {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	canonical, err := json.Marshal(struct {
		Action    string                 `json:"action"`
		Timestamp int64                  `json:"timestamp"`
		Data      map[string]interface{} `json:"data"`
	}{cmd.Action, cmd.Timestamp, cmd.Data})
	if err != nil {
		return false
	}
	mac.Write(canonical)
	expected := hex.EncodeToString(mac.Sum(nil))

	// Constant-time comparison to avoid timing side channels.
	return hmac.Equal([]byte(expected), []byte(cmd.Signature))
}

func handleServerCommand(cmd CommandMessage) {
	log.Printf(">>> [Command Received] Action: %s | Data: %v", cmd.Action, cmd.Data)

	switch strings.ToUpper(cmd.Action) {
	case "LOCK":
		setStationLocked(true)
	case "UNLOCK":
		setStationLocked(false)
	case "SHUTDOWN":
		executeSystemShutdown()
	case "REBOOT":
		executeSystemReboot()
	case "MESSAGE", "NOTIFICATION":
		msg := "Notice from Lab Administrator"
		if text, ok := cmd.Data["message"].(string); ok && text != "" {
			msg = text
		}
		showWindowsAlert("Station Management Notice", msg)
	default:
		log.Printf("[Command] Unknown action: %s", cmd.Action)
	}
}

func startStationLockOverlay() error {
	ready := make(chan error, 1)
	go runStationLockOverlay(ready)
	return <-ready
}

func runStationLockOverlay(ready chan<- error) {
	runtime.LockOSThread()
	className, _ := syscall.UTF16PtrFromString("AUCCStationLockOverlay")
	windowTitle, _ := syscall.UTF16PtrFromString("Station locked")
	text, _ := syscall.UTF16FromString("This station is locked by the lab administrator.\nPlease contact staff to start a session.")
	stationLockOverlayText = text[:len(text)-1]
	stationLockOverlayProc = syscall.NewCallback(stationLockOverlayWindowProc)

	instance, _, callErr := procGetModuleHandleW.Call(0)
	if instance == 0 {
		ready <- windowsCallError("GetModuleHandleW", callErr)
		return
	}
	brush, _, _ := procGetStockObject.Call(4) // BLACK_BRUSH
	wndClass := stationLockWndClass{
		Size:       uint32(unsafe.Sizeof(stationLockWndClass{})),
		Style:      0x0003, // CS_HREDRAW | CS_VREDRAW
		WndProc:    stationLockOverlayProc,
		Instance:   syscall.Handle(instance),
		Background: syscall.Handle(brush),
		ClassName:  className,
	}
	if atom, _, callErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wndClass))); atom == 0 {
		ready <- windowsCallError("RegisterClassExW", callErr)
		return
	}

	fontName, _ := syscall.UTF16PtrFromString("Segoe UI")
	stationLockOverlayFont, _, _ = procCreateFontW.Call(36, 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 4, 0, uintptr(unsafe.Pointer(fontName)))
	if stationLockOverlayFont == 0 {
		stationLockOverlayFont, _, _ = procGetStockObject.Call(17) // DEFAULT_GUI_FONT
	}

	x := int32(systemMetric(76))
	y := int32(systemMetric(77))
	width := int32(systemMetric(78))
	height := int32(systemMetric(79))
	if width <= 0 || height <= 0 {
		width = int32(systemMetric(0))
		height = int32(systemMetric(1))
		x, y = 0, 0
	}
	window, _, callErr := procCreateWindowExW.Call(
		wsExTopmost|wsExToolWindow,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowTitle)),
		wsPopup,
		uintptr(x), uintptr(y), uintptr(width), uintptr(height),
		0, 0, instance, 0,
	)
	if window == 0 {
		ready <- windowsCallError("CreateWindowExW", callErr)
		return
	}
	stationLockOverlayWindow = window
	ready <- nil

	var message stationLockMessage
	for {
		result, _, callErr := procGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) == 0 {
			return
		}
		if int32(result) == -1 {
			log.Printf("[Overlay] GetMessageW failed: %v", callErr)
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
}

func systemMetric(index int32) uintptr {
	value, _, _ := procGetSystemMetrics.Call(uintptr(index))
	return value
}

func setStationLocked(locked bool) {
	if stationLockOverlayWindow == 0 {
		log.Println("[Overlay] Ignoring lock command because the overlay is unavailable.")
		return
	}
	state := uintptr(0)
	if locked {
		state = 1
	}
	if result, _, callErr := procPostMessageW.Call(stationLockOverlayWindow, wmAppStationLock, state, 0); result == 0 {
		log.Printf("[Overlay] Could not update lock state: %v", windowsCallError("PostMessageW", callErr))
	}
}

func stationLockOverlayWindowProc(window, message, wParam, lParam uintptr) uintptr {
	switch uint32(message) {
	case wmAppStationLock:
		if wParam == 0 {
			procShowWindow.Call(window, swHide)
			return 0
		}
		procShowWindow.Call(window, swShow)
		procSetWindowPos.Call(window, ^uintptr(0), 0, 0, 0, 0, swpNoMove|swpNoSize|swpShowWindow)
		procSetForegroundWindow.Call(window)
		procInvalidateRect.Call(window, 0, 1)
		return 0
	case wmClose:
		return 0
	case wmSysCommand:
		if wParam&0xFFF0 == scClose {
			return 0
		}
	case wmEraseBkgnd:
		return 1
	case wmPaint:
		var paint stationLockPaint
		dc, _, _ := procBeginPaint.Call(window, uintptr(unsafe.Pointer(&paint)))
		if dc == 0 {
			return 0
		}
		var rect stationLockRect
		procGetClientRect.Call(window, uintptr(unsafe.Pointer(&rect)))
		brush, _, _ := procGetStockObject.Call(4)
		procFillRect.Call(dc, uintptr(unsafe.Pointer(&rect)), brush)
		font, _, _ := procSelectObject.Call(dc, stationLockOverlayFont)
		procSetTextColor.Call(dc, 0x00FFFFFF)
		procSetBkMode.Call(dc, 1) // TRANSPARENT
		procDrawTextW.Call(dc, uintptr(unsafe.Pointer(&stationLockOverlayText[0])), uintptr(len(stationLockOverlayText)), uintptr(unsafe.Pointer(&rect)), dtCenter|dtVCenter|dtWordWrap)
		procSelectObject.Call(dc, font)
		procEndPaint.Call(window, uintptr(unsafe.Pointer(&paint)))
		return 0
	}
	result, _, _ := procDefWindowProcW.Call(window, message, wParam, lParam)
	return result
}

func windowsCallError(api string, callErr error) error {
	if callErr != nil && callErr != syscall.Errno(0) {
		return fmt.Errorf("%s: %w", api, callErr)
	}
	return fmt.Errorf("%s failed", api)
}

func executeSystemShutdown() {
	if runtime.GOOS == "windows" {
		log.Println("[OS] Shutting down Windows machine...")
		_ = exec.Command("shutdown", "/s", "/t", "0").Run()
	}
}

func executeSystemReboot() {
	if runtime.GOOS == "windows" {
		log.Println("[OS] Rebooting Windows machine...")
		_ = exec.Command("shutdown", "/r", "/t", "0").Run()
	}
}

func showWindowsAlert(title, message string) {
	if runtime.GOOS == "windows" {
		go func() {
			titlePtr, _ := syscall.UTF16PtrFromString(title)
			msgPtr, _ := syscall.UTF16PtrFromString(message)
			// MB_OK (0x0) | MB_ICONINFORMATION (0x40) | MB_SYSTEMMODAL (0x1000)
			procMessageBoxW.Call(0, uintptr(unsafe.Pointer(msgPtr)), uintptr(unsafe.Pointer(titlePtr)), 0x40|0x1000)
		}()
	} else {
		log.Printf("[ALERT] %s: %s", title, message)
	}
}

func getHWID() string {
	// Derive stable HWID from first physical non-loopback MAC address
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 {
				mac := iface.HardwareAddr.String()
				if mac != "" {
					return strings.ToUpper(strings.ReplaceAll(mac, ":", "-"))
				}
			}
		}
	}
	hostname, _ := os.Hostname()
	return "HWID-" + hostname
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func loadConfig() *Config {
	exePath, err := os.Executable()
	var configPath string
	if err == nil {
		configPath = filepath.Join(filepath.Dir(exePath), "config.json")
	} else {
		configPath = "config.json"
	}

	cfg := &Config{
		ServerURL:           "ws://localhost:8000/api/ws/agent",
		MachineName:         "COM-01",
		PingIntervalSeconds: 15,
	}

	data, err := os.ReadFile(configPath)
	if err == nil {
		_ = json.Unmarshal(data, cfg)
	} else {
		// Try fallback to local config.json
		data, err = os.ReadFile("config.json")
		if err == nil {
			_ = json.Unmarshal(data, cfg)
		}
	}

	return cfg
}
