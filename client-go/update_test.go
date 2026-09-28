package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAgentUpdateManifest(t *testing.T) {
	var status atomic.Int32
	status.Store(http.StatusNoContent)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agent/update" || r.Header.Get("X-Agent-Version") != agentVersion || r.Header.Get("X-Agent-Secret") != "station-secret" {
			t.Errorf("incorrect update request: %s", r.URL.Path)
		}
		w.WriteHeader(int(status.Load()))
		if status.Load() == http.StatusOK {
			_, _ = w.Write([]byte(`{"version":"0.3.0","sha256":"invalid","file_size":2048,"can_update":true}`))
		}
	}))
	defer server.Close()
	cfg := &Config{ServerURL: strings.Replace(server.URL, "http://", "ws://", 1) + "/api/ws/agent", MachineName: "LAB-01", AgentSecret: "station-secret"}
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
