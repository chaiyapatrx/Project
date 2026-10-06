package main

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAgentUpdateManifest(t *testing.T) {
	var status atomic.Int32
	status.Store(http.StatusNoContent)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agent/update" || r.Header.Get("X-Agent-Version") != agentVersion || r.Header.Get("X-Agent-Secret") != "station-secret" {
			t.Errorf("incorrect update request: %s", r.URL.Path)
		}
		w.WriteHeader(int(status.Load()))
		if status.Load() == http.StatusOK {
			_, _ = w.Write([]byte(`{"version":"0.3.0","sha256":"invalid","file_size":2048,"can_update":true}`))
		}
	}))
	defer server.Close()
	caPath := filepath.Join(t.TempDir(), "server-ca.pem")
	if err := os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{ServerURL: strings.Replace(server.URL, "https://", "wss://", 1) + "/api/ws/agent", ServerCAFile: caPath, MachineName: "LAB-01", AgentSecret: "station-secret"}
	started, err := checkForAgentUpdate(context.Background(), cfg, "HWID-01")
	if err != nil || started {
		t.Fatalf("no release: started=%v err=%v", started, err)
	}
	status.Store(http.StatusOK)
	started, err = checkForAgentUpdate(context.Background(), cfg, "HWID-01")
	if err == nil || started {
		t.Fatalf("invalid release: started=%v err=%v", started, err)
	}
}

func TestAgentUpdateRejectsPlaintextLoopbackBeforeSendingSecret(t *testing.T) {
	var requested atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested.Store(true)
	}))
	defer server.Close()
	cfg := &Config{ServerURL: strings.Replace(server.URL, "http://", "ws://", 1) + "/api/ws/agent", AgentSecret: "must-not-leak"}
	if started, err := checkForAgentUpdate(context.Background(), cfg, "HWID-01"); started || err == nil {
		t.Fatalf("plaintext update accepted: started=%v err=%v", started, err)
	}
	if requested.Load() {
		t.Fatal("Agent contacted plaintext update endpoint")
	}
}
