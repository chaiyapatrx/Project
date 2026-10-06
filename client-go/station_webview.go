package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	webview2 "github.com/jchv/go-webview2"
)

//go:embed station_login.html
var stationLoginHTML string

var (
	stationWebView             webview2.WebView
	stationWebViewOriginalProc uintptr
	procSetWindowLongPtrW      = modUser32.NewProc("SetWindowLongPtrW")
	procCallWindowProcW        = modUser32.NewProc("CallWindowProcW")
)

const (
	gwlStyle        = ^uintptr(15) // -16
	gwlExStyle      = ^uintptr(19) // -20
	gwlpWndProc     = ^uintptr(3)  // -4
	swpFrameChanged = 0x0020
)

type stationWebLoginRequest struct {
	Mode       string `json:"mode"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	AccessCode string `json:"access_code"`
}

func startStationLockOverlay() error {
	ready := make(chan error, 1)
	go runStationWebOverlay(ready)
	if err := <-ready; err != nil {
		log.Printf("[Overlay] Web UI unavailable, using native UI: %v", err)
		return startNativeStationLockOverlay()
	}
	return nil
}

func runStationWebOverlay(ready chan<- error) {
	runtime.LockOSThread()
	dataPath := filepath.Join(os.Getenv("LOCALAPPDATA"), "AUCC Agent", "WebView2")
	view := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		DataPath:  dataPath,
		WindowOptions: webview2.WindowOptions{
			Title:  "AUCC Station Access",
			Width:  uint(systemMetric(78)),
			Height: uint(systemMetric(79)),
		},
	})
	if view == nil {
		ready <- fmt.Errorf("Microsoft Edge WebView2 Runtime is not installed")
		return
	}
	defer view.Destroy()
	if err := view.Bind("stationLogin", submitStationWebLogin); err != nil {
		ready <- err
		return
	}

	stationWebView = view
	stationLockOverlayWindow = uintptr(view.Window())
	stationWebViewOriginalProc, _, _ = procSetWindowLongPtrW.Call(
		stationLockOverlayWindow,
		gwlpWndProc,
		syscall.NewCallback(stationWebWindowProc),
	)
	procSetWindowLongPtrW.Call(stationLockOverlayWindow, gwlStyle, wsPopup)
	procSetWindowLongPtrW.Call(stationLockOverlayWindow, gwlExStyle, wsExTopmost|wsExToolWindow)

	x := int32(systemMetric(76))
	y := int32(systemMetric(77))
	width := int32(systemMetric(78))
	height := int32(systemMetric(79))
	if width <= 0 || height <= 0 {
		x, y = 0, 0
		width = int32(systemMetric(0))
		height = int32(systemMetric(1))
	}
	procSetWindowPos.Call(
		stationLockOverlayWindow,
		^uintptr(0),
		uintptr(x), uintptr(y), uintptr(width), uintptr(height),
		swpShowWindow|swpFrameChanged,
	)
	view.SetHtml(stationLoginHTML)
	ready <- nil
	view.Run()
}

func stationWebWindowProc(window, message, wParam, lParam uintptr) uintptr {
	if uint32(message) == wmClose || uint32(message) == wmSysCommand && wParam&0xFFF0 == scClose {
		return 0
	}
	result, _, _ := procCallWindowProcW.Call(stationWebViewOriginalProc, window, message, wParam, lParam)
	return result
}

func setStationWebAuthMode(mode string) {
	encoded, _ := json.Marshal(mode)
	stationWebView.Dispatch(func() {
		stationAuthMode = mode
		stationWebView.Eval("window.stationSetMode(" + string(encoded) + ")")
	})
}

func setStationWebLocked(locked bool) {
	stationWebView.Dispatch(func() {
		if !locked {
			stationWebView.Eval("window.stationReset()")
			procShowWindow.Call(stationLockOverlayWindow, swHide)
			return
		}
		procShowWindow.Call(stationLockOverlayWindow, swShow)
		procSetWindowPos.Call(stationLockOverlayWindow, ^uintptr(0), 0, 0, 0, 0, swpNoMove|swpNoSize|swpShowWindow)
		procSetForegroundWindow.Call(stationLockOverlayWindow)
		stationWebView.Eval("window.stationFocus()")
	})
}

func submitStationWebLogin(input stationWebLoginRequest) (map[string]string, error) {
	if stationLoginPending {
		return map[string]string{"status": "pending"}, nil
	}
	if input.Mode != stationAuthMode {
		return nil, fmt.Errorf("authentication mode changed; try again")
	}

	request := map[string]string{"mode": input.Mode}
	switch input.Mode {
	case "account":
		request["username"] = strings.TrimSpace(input.Username)
		request["password"] = input.Password
		if request["username"] == "" || request["password"] == "" {
			return nil, fmt.Errorf("enter both username and password")
		}
	case "access_code":
		request["access_code"] = strings.TrimSpace(input.AccessCode)
		if !validClientAccessCode(request["access_code"]) {
			return nil, fmt.Errorf("enter the six-digit booking code")
		}
	default:
		return nil, fmt.Errorf("station is still connecting")
	}
	if stationAgentConfig == nil {
		return nil, fmt.Errorf("agent configuration is not ready")
	}

	stationLoginPending = true
	config := stationAgentConfig
	hwid := stationHWID
	go func() {
		err := authenticateStation(config, hwid, request)
		if err != nil {
			log.Printf("[Overlay] Login failed: %v", err)
		}
		stationWebView.Dispatch(func() {
			stationLoginPending = false
			message := "Login accepted. Opening station..."
			if err != nil {
				message = "Login failed. Check the details or connection, then try again."
			}
			encoded, _ := json.Marshal(message)
			stationWebView.Eval(fmt.Sprintf("window.stationLoginResult(%t,%s)", err == nil, encoded))
		})
	}()
	return map[string]string{"status": "pending"}, nil
}
