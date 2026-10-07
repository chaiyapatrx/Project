package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
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
	ServerCAFile        string `json:"server_ca_file"`
	MachineName         string `json:"machine_name"`
	PingIntervalSeconds int    `json:"ping_interval_seconds"`
	AgentSecret         string `json:"agent_secret"`
	AutoEnroll          bool   `json:"auto_enroll"`
	EnrollmentToken     string `json:"enrollment_token,omitempty"`
	enrollmentApproved  bool
}

var agentVersion = "1.0.0"

var (
	agentCommandReplayMu   sync.Mutex
	agentCommandSignatures = make(map[string]int64)
)

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
	agentWriteTimeout     = 5 * time.Second
	maxAgentCommandBytes  = 64 << 10
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
	procIsDialogMessageW     = modUser32.NewProc("IsDialogMessageW")
	procTranslateMessage     = modUser32.NewProc("TranslateMessage")
	procDispatchMessageW     = modUser32.NewProc("DispatchMessageW")
	procDefWindowProcW       = modUser32.NewProc("DefWindowProcW")
	procGetStockObject       = syscall.NewLazyDLL("gdi32.dll").NewProc("GetStockObject")
	procFillRect             = syscall.NewLazyDLL("user32.dll").NewProc("FillRect")
	procSetTextColor         = syscall.NewLazyDLL("gdi32.dll").NewProc("SetTextColor")
	procSetBkColor           = syscall.NewLazyDLL("gdi32.dll").NewProc("SetBkColor")
	procSetBkMode            = syscall.NewLazyDLL("gdi32.dll").NewProc("SetBkMode")
	procSelectObject         = syscall.NewLazyDLL("gdi32.dll").NewProc("SelectObject")
	procCreateSolidBrush     = syscall.NewLazyDLL("gdi32.dll").NewProc("CreateSolidBrush")
	procDeleteObject         = syscall.NewLazyDLL("gdi32.dll").NewProc("DeleteObject")
	procRoundRect            = syscall.NewLazyDLL("gdi32.dll").NewProc("RoundRect")
	procEllipse              = syscall.NewLazyDLL("gdi32.dll").NewProc("Ellipse")
	procCreateRoundRectRgn   = syscall.NewLazyDLL("gdi32.dll").NewProc("CreateRoundRectRgn")
	procSetWindowRgn         = modUser32.NewProc("SetWindowRgn")
	procDrawTextW            = syscall.NewLazyDLL("user32.dll").NewProc("DrawTextW")
	procCreateFontW          = syscall.NewLazyDLL("gdi32.dll").NewProc("CreateFontW")
	procGetModuleHandleW     = syscall.NewLazyDLL("kernel32.dll").NewProc("GetModuleHandleW")
	stationLockOverlayWindow uintptr
	stationLockOverlayProc   uintptr
	stationLockOverlayFont   uintptr
	stationAuthFont          uintptr
	stationLabelFont         uintptr
	stationButtonFont        uintptr
	stationInputBrush        uintptr
	stationCardBrush         uintptr
	stationPageBrush         uintptr
	stationAgentConfig       *Config
	stationHWID              string
	stationAuthMode          string
	stationTitleControl      uintptr
	stationSubtitleControl   uintptr
	stationStatusControl     uintptr
	stationUsernameControl   uintptr
	stationPasswordControl   uintptr
	stationAccessCodeControl uintptr
	stationAccountLabel      uintptr
	stationPasswordLabel     uintptr
	stationCodeLabel         uintptr
	stationSubmitControl     uintptr
	stationUsernameRect      stationLockRect
	stationPasswordRect      stationLockRect
	stationAccessCodeRect    stationLockRect
	stationSubmitRect        stationLockRect
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
	wmCtlColorEdit   = 0x0133
	wmDrawItem       = 0x002B
	wmKeyDown        = 0x0100
	wmClose          = 0x0010
	wmPaint          = 0x000F
	wmEraseBkgnd     = 0x0014
	wmSysCommand     = 0x0112
	scClose          = 0xF060

	wsPopup           = 0x80000000
	wsChild           = 0x40000000
	wsVisible         = 0x10000000
	wsTabStop         = 0x00010000
	wsBorder          = 0x00800000
	esAutoHScroll     = 0x00000080
	esPassword        = 0x00000020
	bsDefault         = 0x00000001
	bsOwnerDraw       = 0x0000000B
	ssCenter          = 0x00000001
	wsExTopmost       = 0x00000008
	wsExToolWindow    = 0x00000080
	wsExControlParent = 0x00010000

	swHide = 0
	swShow = 5

	swpNoSize            = 0x0001
	swpNoMove            = 0x0002
	swpShowWindow        = 0x0040
	wmSetFont            = 0x0030
	bnClicked            = 0
	stationLoginButtonID = 1001
	vkReturn             = 0x0D
	emSetCueBanner       = 0x1501
	emSetMargins         = 0x00D3
	ecLeftMargin         = 0x0001
	ecRightMargin        = 0x0002
	authModeConnecting   = 0
	authModeAccount      = 1
	authModeAccessCode   = 2

	dtCenter     = 0x00000001
	dtVCenter    = 0x00000004
	dtWordWrap   = 0x00000010
	dtSingleLine = 0x00000020
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

type stationDrawItem struct {
	CtlType, CtlID, ItemID, ItemAction, ItemState uint32
	Window, DC                                    uintptr
	Rect                                          stationLockRect
	ItemData                                      uintptr
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
	if err := ensureAgentAutoStart(); err != nil {
		log.Printf("[Agent] Could not repair Windows auto-start: %v", err)
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
		if backoff > 5*time.Second {
			backoff = 5 * time.Second
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
	if (u.Scheme != "ws" && u.Scheme != "wss") || u.Host == "" || u.User != nil {
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
	tlsConfig, err := agentTLSConfig(cfg)
	if err != nil {
		return false, err
	}
	dialer := *websocket.DefaultDialer
	dialer.TLSClientConfig = tlsConfig
	dialer.HandshakeTimeout = 5 * time.Second
	c, _, err := dialer.DialContext(ctx, u.String(), requestHeader)
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

			if !acceptCommand(cmd, cfg.AgentSecret) {
				log.Printf("[Security] Rejected unsigned/invalid/stale command: %s", cmd.Action)
				continue
			}

			handleServerCommand(cmd)
		}
	}()

	// Heartbeat / Ping ticker
	ticker := time.NewTicker(heartbeatInterval(cfg.PingIntervalSeconds))
	defer ticker.Stop()
	updateTicker := time.NewTicker(15 * time.Second)
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
			if err := c.SetWriteDeadline(time.Now().Add(agentWriteTimeout)); err != nil {
				return true, err
			}
			err := c.WriteMessage(websocket.TextMessage, data)
			if err != nil {
				log.Printf("[WS] Failed to write ping: %v", err)
				return true, err
			}
		case <-ctx.Done():
			log.Println("[Agent] Interrupt received. Disconnecting...")
			_ = c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Agent shutdown"), time.Now().Add(agentWriteTimeout))
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
	c.SetReadLimit(maxAgentCommandBytes)
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
	now := time.Now().Unix()
	if cmd.Timestamp < now-maxCommandAgeSeconds || cmd.Timestamp > now+maxCommandAgeSeconds {
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

// Keep replay protection across WebSocket reconnects. Backend command nonces
// distinguish legitimate identical actions issued within the same second.
func acceptCommand(cmd CommandMessage, secret string) bool {
	if !verifyCommand(cmd, secret) {
		return false
	}
	agentCommandReplayMu.Lock()
	defer agentCommandReplayMu.Unlock()
	now := time.Now().Unix()
	for signature, expires := range agentCommandSignatures {
		if expires < now {
			delete(agentCommandSignatures, signature)
		}
	}
	if _, seen := agentCommandSignatures[cmd.Signature]; seen {
		return false
	}
	agentCommandSignatures[cmd.Signature] = cmd.Timestamp + maxCommandAgeSeconds
	return true
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

func startNativeStationLockOverlay() error {
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
	stationPageBrush, _, _ = procCreateSolidBrush.Call(stationRGB(247, 245, 252))
	stationCardBrush, _, _ = procCreateSolidBrush.Call(stationRGB(255, 255, 255))
	stationInputBrush, _, _ = procCreateSolidBrush.Call(stationRGB(250, 249, 255))
	if stationPageBrush == 0 || stationCardBrush == 0 || stationInputBrush == 0 {
		ready <- fmt.Errorf("could not create station login colors")
		return
	}
	defer procDeleteObject.Call(stationPageBrush)
	defer procDeleteObject.Call(stationCardBrush)
	defer procDeleteObject.Call(stationInputBrush)
	wndClass := stationLockWndClass{
		Size:       uint32(unsafe.Sizeof(stationLockWndClass{})),
		Style:      0x0003, // CS_HREDRAW | CS_VREDRAW
		WndProc:    stationLockOverlayProc,
		Instance:   syscall.Handle(instance),
		Background: syscall.Handle(stationPageBrush),
		ClassName:  className,
	}
	if atom, _, callErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wndClass))); atom == 0 {
		ready <- windowsCallError("RegisterClassExW", callErr)
		return
	}

	fontName, _ := syscall.UTF16PtrFromString("Segoe UI")
	stationLockOverlayFont, _, _ = procCreateFontW.Call(36, 0, 0, 0, 700, 0, 0, 0, 1, 0, 0, 4, 0, uintptr(unsafe.Pointer(fontName)))
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
		wsExTopmost|wsExToolWindow|wsExControlParent,
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
		if message.Message == wmKeyDown && message.WParam == vkReturn {
			submitStationLogin()
			continue
		}
		if handled, _, _ := procIsDialogMessageW.Call(stationLockOverlayWindow, uintptr(unsafe.Pointer(&message))); handled != 0 {
			continue
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
}

func systemMetric(index int32) uintptr {
	value, _, _ := procGetSystemMetrics.Call(uintptr(index))
	return value
}

func stationRGB(red, green, blue byte) uintptr {
	return uintptr(red) | uintptr(green)<<8 | uintptr(blue)<<16
}

func stationLoginCard(width, height int32) (x, y, cardWidth, cardHeight, leftWidth int32) {
	cardWidth, cardHeight = width-40, height-40
	if cardWidth > 1100 {
		cardWidth = 1100
	}
	if cardHeight > 620 {
		cardHeight = 620
	}
	if cardWidth >= 900 {
		leftWidth = cardWidth / 2
	}
	return (width - cardWidth) / 2, (height - cardHeight) / 2, cardWidth, cardHeight, leftWidth
}

func stationFillRect(dc uintptr, rect stationLockRect, color uintptr) {
	brush, _, _ := procCreateSolidBrush.Call(color)
	if brush == 0 {
		return
	}
	procFillRect.Call(dc, uintptr(unsafe.Pointer(&rect)), brush)
	procDeleteObject.Call(brush)
}

func stationRoundRect(dc uintptr, rect stationLockRect, radius int32, color uintptr) {
	brush, _, _ := procCreateSolidBrush.Call(color)
	if brush == 0 {
		return
	}
	pen, _, _ := procGetStockObject.Call(8) // NULL_PEN
	oldBrush, _, _ := procSelectObject.Call(dc, brush)
	oldPen, _, _ := procSelectObject.Call(dc, pen)
	procRoundRect.Call(dc, uintptr(rect.Left), uintptr(rect.Top), uintptr(rect.Right), uintptr(rect.Bottom), uintptr(radius), uintptr(radius))
	procSelectObject.Call(dc, oldPen)
	procSelectObject.Call(dc, oldBrush)
	procDeleteObject.Call(brush)
}

func stationEllipse(dc uintptr, rect stationLockRect, color uintptr) {
	brush, _, _ := procCreateSolidBrush.Call(color)
	if brush == 0 {
		return
	}
	pen, _, _ := procGetStockObject.Call(8) // NULL_PEN
	oldBrush, _, _ := procSelectObject.Call(dc, brush)
	oldPen, _, _ := procSelectObject.Call(dc, pen)
	procEllipse.Call(dc, uintptr(rect.Left), uintptr(rect.Top), uintptr(rect.Right), uintptr(rect.Bottom))
	procSelectObject.Call(dc, oldPen)
	procSelectObject.Call(dc, oldBrush)
	procDeleteObject.Call(brush)
}

func stationDrawText(dc uintptr, value string, rect stationLockRect, font, color, flags uintptr) {
	text, err := syscall.UTF16PtrFromString(value)
	if err != nil {
		return
	}
	oldFont, _, _ := procSelectObject.Call(dc, font)
	procSetTextColor.Call(dc, color)
	procSetBkMode.Call(dc, 1) // TRANSPARENT
	procDrawTextW.Call(dc, uintptr(unsafe.Pointer(text)), ^uintptr(0), uintptr(unsafe.Pointer(&rect)), flags)
	procSelectObject.Call(dc, oldFont)
}

func stationDrawInputFrame(dc uintptr, rect stationLockRect) {
	stationRoundRect(dc, stationLockRect{rect.Left + 2, rect.Top + 5, rect.Right + 2, rect.Bottom + 5}, 30, stationRGB(232, 224, 246))
	stationRoundRect(dc, rect, 30, stationRGB(221, 211, 241))
	stationRoundRect(dc, stationLockRect{rect.Left + 1, rect.Top + 1, rect.Right - 1, rect.Bottom - 1}, 28, stationRGB(250, 249, 255))
}

func paintStationLogin(dc uintptr, width, height int32) {
	page := stationLockRect{Right: width, Bottom: height}
	procFillRect.Call(dc, uintptr(unsafe.Pointer(&page)), stationPageBrush)
	stationEllipse(dc, stationLockRect{0, 0, 460, 460}, stationRGB(244, 238, 255))
	stationEllipse(dc, stationLockRect{width - 520, height - 420, width, height}, stationRGB(238, 244, 255))
	x, y, cardWidth, cardHeight, leftWidth := stationLoginCard(width, height)
	stationRoundRect(dc, stationLockRect{x + 14, y + 20, x + cardWidth + 14, y + cardHeight + 20}, 48, stationRGB(220, 210, 238))
	stationRoundRect(dc, stationLockRect{x + 6, y + 10, x + cardWidth + 6, y + cardHeight + 10}, 46, stationRGB(234, 228, 246))
	stationRoundRect(dc, stationLockRect{x, y, x + cardWidth, y + cardHeight}, 44, stationRGB(255, 255, 255))
	if leftWidth == 0 {
		stationRoundRect(dc, stationLockRect{x + 30, y + 24, x + 92, y + 62}, 14, stationRGB(124, 58, 237))
		stationDrawText(dc, "AUCC", stationLockRect{x + 37, y + 29, x + 85, y + 58}, stationLabelFont, stationRGB(255, 255, 255), dtCenter|dtVCenter|dtSingleLine)
	} else {
		stationRoundRect(dc, stationLockRect{x, y, x + leftWidth + 24, y + cardHeight}, 44, stationRGB(76, 29, 149))
		stationEllipse(dc, stationLockRect{x + leftWidth - 230, y, x + leftWidth + 20, y + 250}, stationRGB(124, 58, 237))
		stationEllipse(dc, stationLockRect{x, y + cardHeight - 250, x + 230, y + cardHeight}, stationRGB(91, 33, 182))
		stationEllipse(dc, stationLockRect{x + 70, y + 150, x + 300, y + 380}, stationRGB(109, 40, 217))
		stationFillRect(dc, stationLockRect{x + leftWidth, y, x + leftWidth + 24, y + cardHeight}, stationRGB(255, 255, 255))
		stationRoundRect(dc, stationLockRect{x + 42, y + 38, x + 200, y + 82}, 22, stationRGB(139, 92, 246))
		stationDrawText(dc, "STATION ACCESS", stationLockRect{x + 54, y + 44, x + 188, y + 76}, stationLabelFont, stationRGB(255, 255, 255), dtCenter|dtVCenter|dtSingleLine)
		stationDrawText(dc, "Computer Lab\nAccess", stationLockRect{x + 46, y + cardHeight/2 - 56, x + leftWidth - 45, y + cardHeight/2 + 62}, stationLockOverlayFont, stationRGB(255, 255, 255), dtWordWrap)
		stationDrawText(dc, "Sign in to reserve a station and manage your lab sessions.", stationLockRect{x + 46, y + cardHeight - 112, x + leftWidth - 45, y + cardHeight - 42}, stationAuthFont, stationRGB(238, 226, 255), dtWordWrap)
	}
	if stationAuthMode == "account" {
		stationDrawInputFrame(dc, stationUsernameRect)
		stationDrawInputFrame(dc, stationPasswordRect)
	} else if stationAuthMode == "access_code" {
		stationDrawInputFrame(dc, stationAccessCodeRect)
	}
	if stationAuthMode == "account" || stationAuthMode == "access_code" {
		stationRoundRect(dc, stationLockRect{stationSubmitRect.Left + 2, stationSubmitRect.Top + 7, stationSubmitRect.Right + 2, stationSubmitRect.Bottom + 7}, 30, stationRGB(218, 203, 244))
	}
}

func createStationAuthControls(parent uintptr, width, height int32, instance uintptr) error {
	cardX, cardY, cardWidth, cardHeight, leftWidth := stationLoginCard(width, height)
	rightX, rightWidth := cardX+leftWidth, cardWidth-leftWidth
	margin := int32(56)
	if rightWidth < 500 {
		margin = 32
	}
	inputX, inputWidth := rightX+margin, rightWidth-2*margin
	titleY, firstLabelY, inputHeight := cardY+72, cardY+210, int32(54)
	if cardHeight < 520 {
		titleY, firstLabelY, inputHeight = cardY+46, cardY+148, 38
	}
	firstInputY := firstLabelY + 27
	secondLabelY := firstInputY + inputHeight + 24
	secondInputY := secondLabelY + 27
	buttonY := cardY + cardHeight - 132
	if cardHeight < 520 {
		buttonY = cardY + cardHeight - 116
	}
	stationUsernameRect = stationLockRect{inputX, firstInputY, inputX + inputWidth, firstInputY + inputHeight}
	stationPasswordRect = stationLockRect{inputX, secondInputY, inputX + inputWidth, secondInputY + inputHeight}
	stationAccessCodeRect = stationUsernameRect
	stationSubmitRect = stationLockRect{inputX, buttonY, inputX + inputWidth, buttonY + 50}
	const inputInset = int32(3)

	stationAuthFont, _, _ = procCreateFontW.Call(20, 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 4, 0, uintptr(unsafe.Pointer(mustUTF16("Segoe UI"))))
	if stationAuthFont == 0 {
		stationAuthFont, _, _ = procGetStockObject.Call(17)
	}
	stationLabelFont, _, _ = procCreateFontW.Call(16, 0, 0, 0, 700, 0, 0, 0, 1, 0, 0, 4, 0, uintptr(unsafe.Pointer(mustUTF16("Segoe UI"))))
	if stationLabelFont == 0 {
		stationLabelFont = stationAuthFont
	}
	stationButtonFont, _, _ = procCreateFontW.Call(19, 0, 0, 0, 700, 0, 0, 0, 1, 0, 0, 4, 0, uintptr(unsafe.Pointer(mustUTF16("Segoe UI"))))
	if stationButtonFont == 0 {
		stationButtonFont = stationAuthFont
	}

	var err error
	stationTitleControl, err = createStationControl(parent, instance, "STATIC", "AUCC Station Login", wsChild|wsVisible, 0, 0, inputX, titleY, inputWidth, 52)
	if err != nil {
		return err
	}
	procSendMessageW.Call(stationTitleControl, wmSetFont, stationLockOverlayFont, 1)
	stationSubtitleControl, err = createStationControl(parent, instance, "STATIC", "Use your account or booking code to continue.", wsChild|wsVisible, 0, 0, inputX, titleY+56, inputWidth, 40)
	if err != nil {
		return err
	}

	stationAccountLabel, err = createStationControl(parent, instance, "STATIC", "IDENTITY", wsChild|wsVisible, 0, 0, inputX, firstLabelY, inputWidth, 24)
	if err != nil {
		return err
	}
	stationUsernameControl, err = createStationControl(parent, instance, "EDIT", "", wsChild|wsVisible|wsTabStop|esAutoHScroll, 0, 0, inputX+inputInset, firstInputY+inputInset, inputWidth-2*inputInset, inputHeight-2*inputInset)
	if err != nil {
		return err
	}
	stationPasswordLabel, err = createStationControl(parent, instance, "STATIC", "PASSWORD", wsChild|wsVisible, 0, 0, inputX, secondLabelY, inputWidth, 24)
	if err != nil {
		return err
	}
	stationPasswordControl, err = createStationControl(parent, instance, "EDIT", "", wsChild|wsVisible|wsTabStop|esAutoHScroll|esPassword, 0, 0, inputX+inputInset, secondInputY+inputInset, inputWidth-2*inputInset, inputHeight-2*inputInset)
	if err != nil {
		return err
	}
	stationCodeLabel, err = createStationControl(parent, instance, "STATIC", "BOOKING ACCESS CODE", wsChild|wsVisible, 0, 0, inputX, firstLabelY, inputWidth, 24)
	if err != nil {
		return err
	}
	stationAccessCodeControl, err = createStationControl(parent, instance, "EDIT", "", wsChild|wsVisible|wsTabStop|esAutoHScroll, 0, 0, inputX+inputInset, firstInputY+inputInset, inputWidth-2*inputInset, inputHeight-2*inputInset)
	if err != nil {
		return err
	}
	stationSubmitControl, err = createStationControl(parent, instance, "BUTTON", "Sign in", wsChild|wsVisible|wsTabStop|bsOwnerDraw, 0, stationLoginButtonID, inputX, buttonY, inputWidth, 50)
	if err != nil {
		return err
	}
	stationStatusControl, err = createStationControl(parent, instance, "STATIC", "", wsChild|wsVisible|ssCenter, 0, 0, inputX, buttonY+62, inputWidth, 52)
	if err != nil {
		return err
	}

	for _, control := range []uintptr{stationSubtitleControl, stationUsernameControl, stationPasswordControl, stationAccessCodeControl, stationStatusControl} {
		procSendMessageW.Call(control, wmSetFont, stationAuthFont, 1)
	}
	setStationCueBanner(stationUsernameControl, "Username")
	setStationCueBanner(stationPasswordControl, "Password")
	setStationCueBanner(stationAccessCodeControl, "6-digit access code")
	for _, control := range []uintptr{stationUsernameControl, stationPasswordControl, stationAccessCodeControl} {
		procSendMessageW.Call(control, emSetMargins, ecLeftMargin|ecRightMargin, 18|18<<16)
		region, _, _ := procCreateRoundRectRgn.Call(0, 0, uintptr(inputWidth-2*inputInset), uintptr(inputHeight-2*inputInset), 26, 26)
		if region != 0 {
			procSetWindowRgn.Call(control, region, 1)
		}
	}
	buttonRegion, _, _ := procCreateRoundRectRgn.Call(0, 0, uintptr(inputWidth), 50, 30, 30)
	if buttonRegion != 0 {
		procSetWindowRgn.Call(stationSubmitControl, buttonRegion, 1)
	}
	for _, control := range []uintptr{stationAccountLabel, stationPasswordLabel, stationCodeLabel} {
		procSendMessageW.Call(control, wmSetFont, stationLabelFont, 1)
	}
	return nil
}

func setStationCueBanner(control uintptr, value string) {
	if control == 0 {
		return
	}
	text, err := syscall.UTF16PtrFromString(value)
	if err == nil {
		procSendMessageW.Call(control, emSetCueBanner, 0, uintptr(unsafe.Pointer(text)))
	}
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
	if stationWebView != nil {
		setStationWebAuthMode(mode)
		return
	}
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
		setStationControlText(stationTitleControl, "System Access")
		setStationControlText(stationSubtitleControl, "Enter your account credentials to continue.")
		setStationControlText(stationSubmitControl, "Sign in")
		setStationControlText(stationStatusControl, "No active reservation. Use your username and password.")
		procSetFocus.Call(stationUsernameControl)
	} else if mode == "access_code" {
		setStationControlText(stationTitleControl, "Reservation Access")
		setStationControlText(stationSubtitleControl, "This station is reserved for a booking.")
		setStationControlText(stationSubmitControl, "Enter station")
		setStationControlText(stationStatusControl, "Enter the six-digit code for this station.")
		procSetFocus.Call(stationAccessCodeControl)
	} else {
		setStationControlText(stationTitleControl, "Connecting...")
		setStationControlText(stationSubtitleControl, "Waiting for the station server.")
	}
	procInvalidateRect.Call(stationLockOverlayWindow, 0, 1)
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
	client, err := agentHTTPClient(config, 10*time.Second)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
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

func agentTLSConfig(cfg *Config) (*tls.Config, error) {
	if cfg.ServerCAFile == "" {
		return nil, nil
	}
	pem, err := os.ReadFile(cfg.ServerCAFile)
	if err != nil {
		return nil, fmt.Errorf("read server certificate: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("server certificate is not valid PEM")
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}, nil
}

func agentHTTPClient(cfg *Config, timeout time.Duration) (*http.Client, error) {
	tlsConfig, err := agentTLSConfig(cfg)
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	return &http.Client{
		Transport:     transport,
		Timeout:       timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}, nil
}

func stationAPIURL(serverURL, path string) (string, error) {
	endpoint, err := url.Parse(serverURL)
	if err != nil || endpoint.Host == "" || endpoint.User != nil {
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
	body, err := json.Marshal(map[string]string{"name": cfg.MachineName, "hwid": hwid, "secret": cfg.AgentSecret, "enrollment_token": cfg.EnrollmentToken})
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
	client, err := agentHTTPClient(cfg, 10*time.Second)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
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
	if stationWebView != nil {
		setStationWebLocked(locked)
		return
	}
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
		color := stationRGB(100, 116, 139)
		if lParam == stationTitleControl {
			color = stationRGB(15, 23, 42)
		} else if lParam == stationAccountLabel || lParam == stationPasswordLabel || lParam == stationCodeLabel {
			color = stationRGB(109, 40, 217)
		} else if lParam == stationStatusControl {
			status := strings.ToLower(stationControlText(stationStatusControl))
			if strings.Contains(status, "failed") || strings.Contains(status, "enter both") || strings.Contains(status, "not ready") {
				color = stationRGB(190, 24, 93)
			}
		}
		procSetTextColor.Call(wParam, color)
		procSetBkColor.Call(wParam, stationRGB(255, 255, 255))
		return stationCardBrush
	case wmCtlColorEdit:
		procSetTextColor.Call(wParam, stationRGB(15, 23, 42))
		procSetBkColor.Call(wParam, stationRGB(250, 249, 255))
		return stationInputBrush
	case wmDrawItem:
		if lParam == 0 {
			return 0
		}
		item := (*stationDrawItem)(unsafe.Pointer(lParam))
		if item.CtlID == stationLoginButtonID {
			color := stationRGB(124, 58, 237)
			if item.ItemState&4 != 0 { // ODS_DISABLED
				color = stationRGB(184, 166, 216)
			} else if item.ItemState&1 != 0 { // ODS_SELECTED
				color = stationRGB(109, 40, 217)
			}
			stationRoundRect(item.DC, item.Rect, 30, color)
			stationDrawText(item.DC, stationControlText(stationSubmitControl), item.Rect, stationButtonFont, stationRGB(255, 255, 255), dtCenter|dtVCenter|dtSingleLine)
			return 1
		}
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
		paintStationLogin(dc, rect.Right-rect.Left, rect.Bottom-rect.Top)
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
		if err := protectStationConfig(configPath); err != nil {
			return nil, fmt.Errorf("protect station credential file: %w", err)
		}
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse %s: %w", configPath, err)
		}
		if cfg.ServerCAFile == "" {
			if exePath, err := os.Executable(); err == nil {
				caPath := filepath.Join(filepath.Dir(exePath), "server-ca.pem")
				if _, err := os.Stat(caPath); err == nil {
					cfg.ServerCAFile = caPath
				}
			}
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
	if err := protectStationConfig(tmp.Name()); err != nil {
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
