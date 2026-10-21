package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestAgentRejectsOversizedCommands(t *testing.T) {
	client, server := newWebSocketPair(t)
	if err := setupAgentKeepalive(client, time.Second); err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.WriteMessage(websocket.TextMessage, make([]byte, maxAgentCommandBytes+1)) }()
	if _, _, err := client.ReadMessage(); err != websocket.ErrReadLimit {
		t.Fatalf("oversized command error = %v, want %v", err, websocket.ErrReadLimit)
	}
}

func TestVerifyCommandRejectsTamperingAndStaleCommands(t *testing.T) {
	secret := "test-agent-secret"
	command := CommandMessage{Action: "LOCK", Timestamp: time.Now().Unix(), Data: map[string]interface{}{"auth_mode": "account"}}
	sign := func(command *CommandMessage) {
		canonical, err := json.Marshal(struct {
			Action    string                 `json:"action"`
			Timestamp int64                  `json:"timestamp"`
			Data      map[string]interface{} `json:"data"`
		}{command.Action, command.Timestamp, command.Data})
		if err != nil {
			t.Fatal(err)
		}
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(canonical)
		command.Signature = hex.EncodeToString(mac.Sum(nil))
	}
	sign(&command)
	if !verifyCommand(command, secret) {
		t.Fatal("valid command rejected")
	}
	command.Action = "UNLOCK"
	if verifyCommand(command, secret) {
		t.Fatal("tampered command accepted")
	}
	command.Timestamp = time.Now().Unix() - maxCommandAgeSeconds - 5
	sign(&command)
	if verifyCommand(command, secret) {
		t.Fatal("stale command accepted")
	}
	command.Timestamp = time.Now().Unix() + maxCommandAgeSeconds + 5
	sign(&command)
	if verifyCommand(command, secret) {
		t.Fatal("future command accepted")
	}
	if verifyCommand(command, "") {
		t.Fatal("empty secret accepted")
	}
	command.Timestamp = time.Now().Unix()
	command.Data["command_nonce"] = "first-command-" + time.Now().String()
	sign(&command)
	if !acceptCommand(command, secret) || acceptCommand(command, secret) {
		t.Fatal("command must execute once across reconnects")
	}
	command.Data["command_nonce"] = "second-command-" + time.Now().String()
	if verifyCommand(command, secret) {
		t.Fatal("unsigned nonce tampering accepted")
	}
	sign(&command)
	if !acceptCommand(command, secret) {
		t.Fatal("different nonce for identical action rejected")
	}
}

func TestHeartbeatIntervalClampsToThirtySeconds(t *testing.T) {
	if got := heartbeatInterval(60); got != 30*time.Second {
		t.Fatalf("heartbeatInterval(60) = %v, want %v", got, 30*time.Second)
	}
	if got := heartbeatInterval(0); got != 15*time.Second {
		t.Fatalf("heartbeatInterval(0) = %v, want %v", got, 15*time.Second)
	}
}

func TestAgentHTTPClientTrustsOnlyConfiguredCertificate(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	caPath := filepath.Join(t.TempDir(), "server-ca.pem")
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	if err := os.WriteFile(caPath, certificate, 0600); err != nil {
		t.Fatal(err)
	}
	client, err := agentHTTPClient(&Config{ServerCAFile: caPath}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if _, err := client.Get(strings.Replace(server.URL, "127.0.0.1", "localhost", 1)); err == nil {
		t.Fatal("certificate accepted for a different host")
	}
}

func TestWaitForReconnectStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan bool, 1)
	go func() { done <- waitForReconnect(ctx, time.Hour) }()
	cancel()

	select {
	case reconnected := <-done:
		if reconnected {
			t.Fatal("waitForReconnect returned true after cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("waitForReconnect did not stop after cancellation")
	}
}

func TestAgentPingRefreshesReadDeadlineAndRepliesWithPong(t *testing.T) {
	client, server := newWebSocketPair(t)
	if err := setupAgentKeepalive(client, 10*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := client.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	pongs := make(chan string, 1)
	server.SetPongHandler(func(appData string) error {
		pongs <- appData
		return nil
	})
	go func() {
		for {
			if _, _, err := server.ReadMessage(); err != nil {
				return
			}
		}
	}()

	type readResult struct {
		message string
		err     error
	}
	read := make(chan readResult, 1)
	go func() {
		_, message, err := client.ReadMessage()
		read <- readResult{string(message), err}
	}()
	if err := server.WriteControl(websocket.PingMessage, []byte("probe"), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	select {
	case pong := <-pongs:
		if pong != "probe" {
			t.Fatalf("Pong payload = %q, want %q", pong, "probe")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("client did not reply to server ping")
	}

	time.Sleep(1100 * time.Millisecond)
	if err := server.WriteMessage(websocket.TextMessage, []byte("still-connected")); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-read:
		if result.err != nil {
			t.Fatalf("client read after ping: %v", result.err)
		}
		if result.message != "still-connected" {
			t.Fatalf("client message = %q, want %q", result.message, "still-connected")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("client read did not resume after ping")
	}
}

func TestStationLoginURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "ws://127.0.0.1:8000/api/ws/agent", want: "http://127.0.0.1:8000/api/agent/login"},
		{input: "wss://labaccess.example/api/ws/agent?region=1", want: "https://labaccess.example/api/agent/login"},
	}
	for _, tt := range tests {
		got, err := stationLoginURL(tt.input)
		if err != nil || got != tt.want {
			t.Errorf("stationLoginURL(%q) = %q, %v; want %q", tt.input, got, err, tt.want)
		}
	}
	if _, err := stationLoginURL("ftp://labaccess.example/agent"); err == nil {
		t.Fatal("stationLoginURL accepted an unsupported protocol")
	}
	if _, err := stationLoginURL("wss://user:password@labaccess.example/api/ws/agent"); err == nil {
		t.Fatal("stationLoginURL accepted credentials in server URL")
	}
}

func TestAutoEnrollmentConfigPersistsCredential(t *testing.T) {
	localAppData := t.TempDir()
	t.Setenv("LOCALAPPDATA", localAppData)
	path := filepath.Join(localAppData, "LabAccess Agent", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"server_url":"ws://localhost:8000/api/ws/agent","auto_enroll":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	first, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if first.MachineName == "" || len(first.AgentSecret) != 43 {
		t.Fatalf("auto enrollment identity incomplete: name=%q secret length=%d", first.MachineName, len(first.AgentSecret))
	}
	second, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if second.MachineName != first.MachineName || second.AgentSecret != first.AgentSecret {
		t.Fatal("station credential changed after config reload")
	}
	if first.HWID == "" || second.HWID != first.HWID {
		t.Fatalf("hardware ID not persisted: first=%q second=%q", first.HWID, second.HWID)
	}
}

func TestLegacyConfigFreezesHardwareID(t *testing.T) {
	localAppData := t.TempDir()
	t.Setenv("LOCALAPPDATA", localAppData)
	path := filepath.Join(localAppData, "LabAccess Agent", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	legacy := `{"server_url":"ws://localhost:8000/api/ws/agent","machine_name":"LAB-01","agent_secret":"` + strings.Repeat("a", 43) + `"}`
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got := stationHardwareID(cfg); got != getHWID() {
		t.Fatalf("legacy station must keep its MAC-based ID, got %q", got)
	}
	reloaded, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.HWID != cfg.HWID || reloaded.AgentSecret != cfg.AgentSecret {
		t.Fatal("legacy hardware ID was not saved with the existing credential")
	}
}

func TestEnrollAgentWaitsForApproval(t *testing.T) {
	var approved atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agent/enroll" || r.Method != http.MethodPost {
			t.Errorf("unexpected enrollment request: %s %s", r.Method, r.URL.Path)
		}
		var identity map[string]string
		if err := json.NewDecoder(r.Body).Decode(&identity); err != nil {
			t.Error(err)
		}
		if identity["name"] != "LAB-01" || identity["hwid"] != "HWID-01" || identity["secret"] != "secret" || identity["enrollment_token"] != "private-package-token" {
			t.Errorf("incorrect enrollment identity: %v", identity)
		}
		if approved.Load() {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusAccepted)
		}
	}))
	defer server.Close()
	cfg := &Config{ServerURL: strings.Replace(server.URL, "http://", "ws://", 1) + "/api/ws/agent", MachineName: "LAB-01", AgentSecret: "secret", EnrollmentToken: "private-package-token"}
	if err := enrollAgent(context.Background(), cfg, "HWID-01"); err == nil || !strings.Contains(err.Error(), "waiting for administrator") {
		t.Fatalf("pending enrollment returned %v", err)
	}
	approved.Store(true)
	if err := enrollAgent(context.Background(), cfg, "HWID-01"); err != nil {
		t.Fatalf("approved enrollment returned %v", err)
	}
}

func TestValidClientAccessCode(t *testing.T) {
	for code, want := range map[string]bool{"000001": true, "123456": true, "12345": false, "12a456": false, "１２３４５６": false} {
		if got := validClientAccessCode(code); got != want {
			t.Errorf("validClientAccessCode(%q) = %v, want %v", code, got, want)
		}
	}
}

func TestStationLoginButtonEnabledForNextLockedSession(t *testing.T) {
	tests := []struct {
		mode    string
		pending bool
		want    bool
	}{
		{mode: "account", want: true},
		{mode: "access_code", want: true},
		{mode: "connecting", want: false},
		{mode: "account", pending: true, want: false},
	}
	for _, tt := range tests {
		if got := shouldEnableStationLoginButton(tt.mode, tt.pending); got != tt.want {
			t.Errorf("shouldEnableStationLoginButton(%q, %v) = %v, want %v", tt.mode, tt.pending, got, tt.want)
		}
	}
}

func TestAuthenticateStationSendsBackendRequestContract(t *testing.T) {
	for _, login := range []map[string]string{
		{"mode": "account", "username": "pilot-user", "password": "test-password"},
		{"mode": "access_code", "access_code": "123456"},
	} {
		login := login
		t.Run(login["mode"], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/agent/login" {
					t.Errorf("request = %s %s, want POST /api/agent/login", r.Method, r.URL.Path)
				}
				if got := r.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
					t.Errorf("Content-Type = %q, want application/json", got)
				}
				if got := r.Header.Get("X-Computer-Name"); got != "VM-01" {
					t.Errorf("X-Computer-Name = %q, want VM-01", got)
				}
				if got := r.Header.Get("X-Computer-HWID"); got != "HWID-VM-01" {
					t.Errorf("X-Computer-HWID = %q, want HWID-VM-01", got)
				}
				if got := r.Header.Get("X-Agent-Secret"); got != "test-station-secret" {
					t.Errorf("X-Agent-Secret = %q, want test-station-secret", got)
				}
				var got map[string]string
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Errorf("decode login body: %v", err)
					return
				}
				if len(got) != len(login) {
					t.Errorf("login body = %#v, want %#v", got, login)
				}
				for key, want := range login {
					if got[key] != want {
						t.Errorf("login body[%q] = %q, want %q", key, got[key], want)
					}
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			config := &Config{
				ServerURL:   "ws" + strings.TrimPrefix(server.URL, "http") + "/api/ws/agent",
				MachineName: "VM-01",
				AgentSecret: "test-station-secret",
			}
			if err := authenticateStation(config, "HWID-VM-01", login); err != nil {
				t.Fatalf("authenticateStation() error = %v", err)
			}
		})
	}
}

func newWebSocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	connections := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err == nil {
			connections <- conn
		}
	}))
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	var peer *websocket.Conn
	select {
	case peer = <-connections:
	case <-time.After(5 * time.Second):
		_ = client.Close()
		server.Close()
		t.Fatal("server did not accept WebSocket connection")
	}
	t.Cleanup(func() {
		_ = client.Close()
		_ = peer.Close()
		server.Close()
	})
	return client, peer
}
