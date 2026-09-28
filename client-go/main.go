package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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
	"sync"
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
	AutoEnroll          bool   `json:"auto_enroll"`
	enrollmentApproved  bool
}

var agentVersion = "1.0.0"

type CommandMessage struct {
	Action    string                 `json:"action"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Signature string                 `json:"signature"`
}

// maxCommandAgeSeconds bounds how old a signed command may be, mitigating replay.
const maxCommandAgeSeconds = 30

const maxPingIntervalSeconds = 30

const (
	agentReadTimeout      = 60 * time.Second
	agentPongWriteTimeout = 2 * time.Second
)

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
	procGetWindowTextW       = modUser32.NewProc("GetWindowTextW")
	procSetWindowTextW       = modUser32.NewProc("SetWindowTextW")
	procEnableWindow         = modUser32.NewProc("EnableWindow")
	procSetFocus             = modUser32.NewProc("SetFocus")
	procSendMessageW         = modUser32.NewProc("SendMessageW")
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
	stationAuthFont          uintptr
	stationAgentConfig       *Config
	stationHWID              string
	stationAuthMode          string
	stationTitleControl      uintptr
	stationStatusControl     uintptr
	stationUsernameControl   uintptr
	stationPasswordControl   uintptr
	stationAccessCodeControl uintptr
	stationAccountLabel      uintptr
	stationPasswordLabel     uintptr
	stationCodeLabel         uintptr
	stationSubmitControl     uintptr
	stationLoginPending      bool
	stationLoginResultMu     sync.Mutex
	stationLoginResult       string
)

const (
	wmAppStationLock = 0x8001
	wmAppAuthMode    = 0x8002
	wmAppLoginResult = 0x8003
	wmCreate         = 0x0001
	wmCommand        = 0x0111
	wmCtlColorStatic = 0x0138
	wmClose          = 0x0010
	wmPaint          = 0x000F
	wmEraseBkgnd     = 0x0014
	wmSysCommand     = 0x0112
	scClose          = 0xF060

	wsPopup        = 0x80000000
	wsChild        = 0x40000000
	wsVisible      = 0x10000000
	wsTabStop      = 0x00010000
	wsBorder       = 0x00800000
	esAutoHScroll  = 0x00000080
	esPassword     = 0x00000020
	bsDefault      = 0x00000001
	ssCenter       = 0x00000001
	wsExTopmost    = 0x00000008
	wsExToolWindow = 0x00000080

	swHide = 0
	swShow = 5

	swpNoSize            = 0x0001
	swpNoMove            = 0x0002
	swpShowWindow        = 0x0040
	wmSetFont            = 0x0030
	bnClicked            = 0
	stationLoginButtonID = 1001
	authModeConnecting   = 0
	authModeAccount      = 1
	authModeAccessCode   = 2

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
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(agentVersion)
		return
	}
	log.Println("==================================================")
	log.Println(" AUCC High-Performance Client Agent (Go)")
	log.Println("==================================================")
	if len(os.Args) > 1 && os.Args[1] == "--enroll" {
		cfg, err := loadConfig()
		if err != nil {
			log.Printf("[Agent] Enrollment setup failed: %v", err)
			return
		}
		if err := enrollAgent(context.Background(), cfg, getHWID()); err != nil {
			log.Printf("[Agent] Enrollment will retry when the agent starts: %v", err)
		}
		return
	}

	if err := startStationLockOverlay(); err != nil {
		log.Fatalf("[Agent] Could not start station lock overlay: %v", err)
	}
	setStationAuthMode("connecting")
	setStationLocked(true)

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("[Agent] Could not load config: %v", err)
	}
	if strings.TrimSpace(cfg.AgentSecret) == "" || strings.TrimSpace(cfg.MachineName) == "" {
		log.Fatal("[Agent] config.json must contain machine_name and agent_secret, or enable auto_enroll")
	}
	hwid := getHWID()
	stationAgentConfig = cfg
	stationHWID = hwid

	log.Printf("[Agent] Machine Name : %s", cfg.MachineName)
	log.Printf("[Agent] Hardware ID  : %s", hwid)
	log.Printf("[Agent] Target Server: %s", cfg.ServerURL)
	log.Println("[Agent] Starting WebSocket agent engine...")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Keep-alive connection loop with exponential backoff
	backoff := 1 * time.Second
	for {
		connected, err := runAgentSession(ctx, cfg, hwid)
		if err == nil {
			// Normal clean exit
			log.Println("[Agent] Agent exited cleanly.")
			return
		}
		if connected {
			backoff = time.Second
		}

		log.Printf("[Agent] Connection error: %v. Reconnecting in %v...", err, backoff)
		if !waitForReconnect(ctx, backoff) {
			log.Println("[Agent] Interrupt received. Exiting...")
			return
		}
		backoff *= 2
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
	}
}

func waitForReconnect(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func runAgentSession(ctx context.Context, cfg *Config, hwid string) (bool, error) {
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
	if cfg.AutoEnroll && !cfg.enrollmentApproved {
		if err := enrollAgent(ctx, cfg, hwid); err != nil {
			return false, err
		}
		cfg.enrollmentApproved = true
	}

	q := u.Query()
	q.Set("name", cfg.MachineName)
	q.Set("hwid", hwid)
	u.RawQuery = q.Encode()

	requestHeader := http.Header{}
	requestHeader.Set("X-Agent-Secret", cfg.AgentSecret)

	log.Printf("[WS] Connecting to %s...", u.String())
	c, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), requestHeader)
	if err != nil {
		return false, err
	}
	defer func() {
		_ = c.Close()
		setStationAuthMode("connecting")
		setStationLocked(true)
	}()
	if err := setupAgentKeepalive(c, agentReadTimeout); err != nil {
		return true, fmt.Errorf("set WebSocket read deadline: %w", err)
	}

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
	ticker := time.NewTicker(heartbeatInterval(cfg.PingIntervalSeconds))
	defer ticker.Stop()
	updateTicker := time.NewTicker(time.Minute)
	defer updateTicker.Stop()
	updateNow := time.NewTimer(time.Second)
	defer updateNow.Stop()

	for {
		select {
		case <-updateNow.C:
			if started, err := checkForAgentUpdate(ctx, cfg, hwid); err != nil {
				log.Printf("[Update] Check failed: %v", err)
			} else if started {
				return true, nil
			}
		case <-updateTicker.C:
			if started, err := checkForAgentUpdate(ctx, cfg, hwid); err != nil {
				log.Printf("[Update] Check failed: %v", err)
			} else if started {
				return true, nil
			}
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
		case <-ctx.Done():
			log.Println("[Agent] Interrupt received. Disconnecting...")
			_ = c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Agent shutdown"))
			return true, nil
		}
	}
}

func heartbeatInterval(seconds int) time.Duration {
	if seconds <= 0 {
		seconds = 15
	}
	if seconds > maxPingIntervalSeconds {
		seconds = maxPingIntervalSeconds
	}
	return time.Duration(seconds) * time.Second
}

func setupAgentKeepalive(c *websocket.Conn, readTimeout time.Duration) error {
	if err := c.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		return err
	}
	c.SetPingHandler(func(appData string) error {
		if err := c.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
			return err
		}
		return c.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(agentPongWriteTimeout))
	})
	return nil
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
		mode := "account"
		if requestedMode, ok := cmd.Data["auth_mode"].(string); ok {
			mode = requestedMode
		}
		setStationAuthMode(mode)
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
	if err := createStationAuthControls(window, width, height, instance); err != nil {
		ready <- err
		return
	}
	applyStationAuthMode("connecting")
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

func createStationAuthControls(parent uintptr, width, height int32, instance uintptr) error {
	centerX, centerY := width/2, height/2
	stationAuthFont, _, _ = procCreateFontW.Call(20, 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 4, 0, uintptr(unsafe.Pointer(mustUTF16("Segoe UI"))))
	if stationAuthFont == 0 {
		stationAuthFont, _, _ = procGetStockObject.Call(17)
	}

	var err error
	stationTitleControl, err = createStationControl(parent, instance, "STATIC", "AUCC Station Login", wsChild|wsVisible|ssCenter, 0, 0, centerX-300, centerY-205, 600, 52)
	if err != nil {
		return err
	}
	procSendMessageW.Call(stationTitleControl, wmSetFont, stationLockOverlayFont, 1)

	stationAccountLabel, err = createStationControl(parent, instance, "STATIC", "Username", wsChild|wsVisible, 0, 0, centerX-180, centerY-126, 360, 26)
	if err != nil {
		return err
	}
	stationUsernameControl, err = createStationControl(parent, instance, "EDIT", "", wsChild|wsVisible|wsTabStop|wsBorder|esAutoHScroll, 0, 0, centerX-180, centerY-100, 360, 36)
	if err != nil {
		return err
	}
	stationPasswordLabel, err = createStationControl(parent, instance, "STATIC", "Password", wsChild|wsVisible, 0, 0, centerX-180, centerY-52, 360, 26)
	if err != nil {
		return err
	}
	stationPasswordControl, err = createStationControl(parent, instance, "EDIT", "", wsChild|wsVisible|wsTabStop|wsBorder|esAutoHScroll|esPassword, 0, 0, centerX-180, centerY-26, 360, 36)
	if err != nil {
		return err
	}
	stationCodeLabel, err = createStationControl(parent, instance, "STATIC", "Booking access code", wsChild|wsVisible, 0, 0, centerX-180, centerY-74, 360, 26)
	if err != nil {
		return err
	}
	stationAccessCodeControl, err = createStationControl(parent, instance, "EDIT", "", wsChild|wsVisible|wsTabStop|wsBorder|esAutoHScroll, 0, 0, centerX-120, centerY-48, 240, 38)
	if err != nil {
		return err
	}
	stationSubmitControl, err = createStationControl(parent, instance, "BUTTON", "Sign in", wsChild|wsVisible|wsTabStop|bsDefault, 0, stationLoginButtonID, centerX-80, centerY+48, 160, 42)
	if err != nil {
		return err
	}
	stationStatusControl, err = createStationControl(parent, instance, "STATIC", "", wsChild|wsVisible|ssCenter, 0, 0, centerX-300, centerY+104, 600, 40)
	if err != nil {
		return err
	}

	for _, control := range []uintptr{stationAccountLabel, stationUsernameControl, stationPasswordLabel, stationPasswordControl, stationCodeLabel, stationAccessCodeControl, stationSubmitControl, stationStatusControl} {
		procSendMessageW.Call(control, wmSetFont, stationAuthFont, 1)
	}
	return nil
}

func createStationControl(parent, instance uintptr, className, title string, style, extendedStyle, controlID uintptr, x, y, width, height int32) (uintptr, error) {
	class, err := syscall.UTF16PtrFromString(className)
	if err != nil {
		return 0, err
	}
	text, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return 0, err
	}
	window, _, callErr := procCreateWindowExW.Call(
		extendedStyle,
		uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(text)), style,
		uintptr(x), uintptr(y), uintptr(width), uintptr(height), parent, controlID, instance, 0,
	)
	if window == 0 {
		return 0, windowsCallError("CreateWindowExW control", callErr)
	}
	return window, nil
}

func mustUTF16(value string) *uint16 {
	ptr, err := syscall.UTF16PtrFromString(value)
	if err != nil {
		return nil
	}
	return ptr
}

func setStationAuthMode(mode string) {
	if stationLockOverlayWindow == 0 {
		return
	}
	code := uintptr(authModeConnecting)
	switch mode {
	case "account":
		code = authModeAccount
	case "access_code":
		code = authModeAccessCode
	}
	if result, _, callErr := procPostMessageW.Call(stationLockOverlayWindow, wmAppAuthMode, code, 0); result == 0 {
		log.Printf("[Overlay] Could not update authentication mode: %v", windowsCallError("PostMessageW", callErr))
	}
}

func applyStationAuthMode(mode string) {
	previousMode := stationAuthMode
	stationAuthMode = mode
	if previousMode != mode {
		if mode != "account" {
			setStationControlText(stationUsernameControl, "")
			setStationControlText(stationPasswordControl, "")
		}
		if mode != "access_code" {
			setStationControlText(stationAccessCodeControl, "")
		}
	}
	setControlVisible(stationAccountLabel, mode == "account")
	setControlVisible(stationUsernameControl, mode == "account")
	setControlVisible(stationPasswordLabel, mode == "account")
	setControlVisible(stationPasswordControl, mode == "account")
	setControlVisible(stationCodeLabel, mode == "access_code")
	setControlVisible(stationAccessCodeControl, mode == "access_code")
	setControlVisible(stationSubmitControl, mode == "account" || mode == "access_code")
	if shouldEnableStationLoginButton(mode, stationLoginPending) {
		procEnableWindow.Call(stationSubmitControl, 1)
	} else {
		procEnableWindow.Call(stationSubmitControl, 0)
	}
	setStationControlText(stationSubmitControl, "Sign in")
	setStationControlText(stationStatusControl, "Connecting to booking server...")
	if mode == "account" {
		setStationControlText(stationTitleControl, "Sign in to this station")
		setStationControlText(stationSubmitControl, "Sign in")
		setStationControlText(stationStatusControl, "No active reservation. Use your username and password.")
		procSetFocus.Call(stationUsernameControl)
	} else if mode == "access_code" {
		setStationControlText(stationTitleControl, "This station is reserved")
		setStationControlText(stationSubmitControl, "Enter station")
		setStationControlText(stationStatusControl, "Enter the six-digit code for this station.")
		procSetFocus.Call(stationAccessCodeControl)
	} else {
		setStationControlText(stationTitleControl, "AUCC Station Login")
	}
}

func shouldEnableStationLoginButton(mode string, pending bool) bool {
	return !pending && (mode == "account" || mode == "access_code")
}

func setControlVisible(control uintptr, visible bool) {
	if control == 0 {
		return
	}
	state := uintptr(swHide)
	if visible {
		state = swShow
	}
	procShowWindow.Call(control, state)
}

func setStationControlText(control uintptr, value string) {
	if control == 0 {
		return
	}
	text, err := syscall.UTF16PtrFromString(value)
	if err == nil {
		procSetWindowTextW.Call(control, uintptr(unsafe.Pointer(text)))
	}
}

func stationControlText(control uintptr) string {
	if control == 0 {
		return ""
	}
	buffer := make([]uint16, 256)
	procGetWindowTextW.Call(control, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	return syscall.UTF16ToString(buffer)
}

func submitStationLogin() {
	if stationLoginPending || (stationAuthMode != "account" && stationAuthMode != "access_code") {
		return
	}
	request := map[string]string{"mode": stationAuthMode}
	if stationAuthMode == "account" {
		request["username"] = strings.TrimSpace(stationControlText(stationUsernameControl))
		request["password"] = stationControlText(stationPasswordControl)
		if request["username"] == "" || request["password"] == "" {
			setStationControlText(stationStatusControl, "Enter both username and password.")
			return
		}
	} else {
		request["access_code"] = stationControlText(stationAccessCodeControl)
		if !validClientAccessCode(request["access_code"]) {
			setStationControlText(stationStatusControl, "Enter the six-digit booking code.")
			return
		}
	}
	if stationAgentConfig == nil {
		setStationControlText(stationStatusControl, "Agent configuration is not ready.")
		return
	}
	stationLoginPending = true
	procEnableWindow.Call(stationSubmitControl, 0)
	setStationControlText(stationStatusControl, "Checking login...")
	config := stationAgentConfig
	hwid := stationHWID
	go func() {
		err := authenticateStation(config, hwid, request)
		message := ""
		if err != nil {
			message = "Login failed. Check details or connection, then try again."
		}
		stationLoginResultMu.Lock()
		stationLoginResult = message
		stationLoginResultMu.Unlock()
		code := uintptr(0)
		if err != nil {
			code = 1
		}
		procPostMessageW.Call(stationLockOverlayWindow, wmAppLoginResult, code, 0)
	}()
}

func validClientAccessCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, digit := range code {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

func authenticateStation(config *Config, hwid string, login map[string]string) error {
	endpoint, err := stationLoginURL(config.ServerURL)
	if err != nil {
		return err
	}
	body, err := json.Marshal(login)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Computer-Name", config.MachineName)
	request.Header.Set("X-Computer-HWID", hwid)
	request.Header.Set("X-Agent-Secret", config.AgentSecret)
	response, err := (&http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("station login rejected with HTTP %d", response.StatusCode)
	}
	return nil
}

func stationLoginURL(serverURL string) (string, error) {
	return stationAPIURL(serverURL, "/api/agent/login")
}

func stationAPIURL(serverURL, path string) (string, error) {
	endpoint, err := url.Parse(serverURL)
	if err != nil || endpoint.Host == "" {
		return "", fmt.Errorf("invalid server URL")
	}
	switch endpoint.Scheme {
	case "ws":
		if !isLoopbackHost(endpoint.Hostname()) {
			return "", fmt.Errorf("unencrypted ws:// is allowed only for localhost; use wss:// for remote servers")
		}
		endpoint.Scheme = "http"
	case "wss":
		endpoint.Scheme = "https"
	default:
		return "", fmt.Errorf("server URL must use ws:// or wss://")
	}
	endpoint.Path = path
	endpoint.RawPath = ""
	endpoint.RawQuery = ""
	endpoint.Fragment = ""
	return endpoint.String(), nil
}

func enrollAgent(ctx context.Context, cfg *Config, hwid string) error {
	endpoint, err := stationAPIURL(cfg.ServerURL, "/api/agent/enroll")
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]string{"name": cfg.MachineName, "hwid": hwid, "secret": cfg.AgentSecret})
	if err != nil {
		return err
	}
	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusAccepted:
		return fmt.Errorf("waiting for administrator to approve station %s", cfg.MachineName)
	case http.StatusConflict:
		return fmt.Errorf("station %s conflicts with an existing registration; contact administrator", cfg.MachineName)
	default:
		return fmt.Errorf("station enrollment failed with HTTP %d", response.StatusCode)
	}
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
			setStationControlText(stationUsernameControl, "")
			setStationControlText(stationPasswordControl, "")
			setStationControlText(stationAccessCodeControl, "")
			procShowWindow.Call(window, swHide)
			return 0
		}
		procShowWindow.Call(window, swShow)
		procSetWindowPos.Call(window, ^uintptr(0), 0, 0, 0, 0, swpNoMove|swpNoSize|swpShowWindow)
		procSetForegroundWindow.Call(window)
		if stationAuthMode == "account" {
			procSetFocus.Call(stationUsernameControl)
		} else if stationAuthMode == "access_code" {
			procSetFocus.Call(stationAccessCodeControl)
		}
		procInvalidateRect.Call(window, 0, 1)
		return 0
	case wmAppAuthMode:
		switch wParam {
		case authModeAccount:
			applyStationAuthMode("account")
		case authModeAccessCode:
			applyStationAuthMode("access_code")
		default:
			applyStationAuthMode("connecting")
		}
		return 0
	case wmAppLoginResult:
		stationLoginPending = false
		stationLoginResultMu.Lock()
		result := stationLoginResult
		stationLoginResultMu.Unlock()
		if wParam == 0 {
			setStationControlText(stationStatusControl, "Login accepted. Opening station...")
			return 0
		}
		setStationControlText(stationPasswordControl, "")
		setStationControlText(stationAccessCodeControl, "")
		procEnableWindow.Call(stationSubmitControl, 1)
		if result == "" {
			result = "Login failed. Check details or connection, then try again."
		}
		setStationControlText(stationStatusControl, result)
		if stationAuthMode == "account" {
			procSetFocus.Call(stationPasswordControl)
		} else if stationAuthMode == "access_code" {
			procSetFocus.Call(stationAccessCodeControl)
		}
		return 0
	case wmCommand:
		if wParam&0xFFFF == stationLoginButtonID && (wParam>>16)&0xFFFF == bnClicked {
			submitStationLogin()
			return 0
		}
	case wmCtlColorStatic:
		procSetTextColor.Call(wParam, 0x00FFFFFF)
		procSetBkMode.Call(wParam, 1)
		brush, _, _ := procGetStockObject.Call(4)
		return brush
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

func loadConfig() (*Config, error) {
	cfg := &Config{
		ServerURL:           "ws://localhost:8000/api/ws/agent",
		PingIntervalSeconds: 15,
	}

	var paths []string
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		paths = append(paths, filepath.Join(localAppData, "AUCC Agent", "config.json"))
	}
	if exePath, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exePath), "config.json"))
	}
	paths = append(paths, "config.json")
	configPath := paths[0]
	for _, configPath := range paths {
		data, err := os.ReadFile(configPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", configPath, err)
		}
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse %s: %w", configPath, err)
		}
		if cfg.AutoEnroll && cfg.AgentSecret == "" {
			return completeAutoEnrollmentConfig(cfg, configPath)
		}
		break
	}
	if cfg.AutoEnroll && cfg.AgentSecret == "" {
		return completeAutoEnrollmentConfig(cfg, configPath)
	}
	if cfg.AgentSecret == "" {
		cfg.AutoEnroll = true
		return completeAutoEnrollmentConfig(cfg, configPath)
	}
	return cfg, nil
}

func completeAutoEnrollmentConfig(cfg *Config, path string) (*Config, error) {
	if cfg.MachineName == "" {
		name, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("read computer name: %w", err)
		}
		cfg.MachineName = name
	}
	if len(cfg.MachineName) > 50 {
		return nil, fmt.Errorf("computer name exceeds 50 bytes")
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate station credential: %w", err)
	}
	cfg.AgentSecret = base64.RawURLEncoding.EncodeToString(secret)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create station settings folder: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "config-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("create station settings: %w", err)
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return nil, err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return nil, fmt.Errorf("save station settings: %w", err)
	}
	return cfg, nil
}
