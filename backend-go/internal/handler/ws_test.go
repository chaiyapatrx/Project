package handler

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"station-backend/internal/hub"
)

func TestDecideAgentSync(t *testing.T) {
	tests := []struct {
		name             string
		status           string
		hasActiveBooking bool
		hasActiveUsage   bool
		wantCommand      string
		wantAuthMode     string
		wantRepair       bool
	}{
		{name: "active booking requires code", status: "in_use", hasActiveBooking: true, wantCommand: "LOCK", wantAuthMode: "access_code"},
		{name: "active walk-in requires account", status: "in_use", hasActiveUsage: true, wantCommand: "LOCK", wantAuthMode: "account"},
		{name: "stale station is repaired", status: "in_use", wantCommand: "LOCK", wantAuthMode: "account", wantRepair: true},
		{name: "available station requires account", status: "available", wantCommand: "LOCK", wantAuthMode: "account"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decideAgentSync(tt.status, tt.hasActiveBooking, tt.hasActiveUsage)
			if got.command != tt.wantCommand || got.authMode != tt.wantAuthMode || got.repairStaleState != tt.wantRepair {
				t.Fatalf("decideAgentSync() = %+v, want command %q mode %q repair %v", got, tt.wantCommand, tt.wantAuthMode, tt.wantRepair)
			}
		})
	}
}

func TestRestoredWalkInSessionEnd(t *testing.T) {
	if got := restoredWalkInSessionEnd(sql.NullTime{}); !got.IsZero() {
		t.Fatalf("NULL walk-in expiry should be immediately expired, got %v", got)
	}
	want := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	if got := restoredWalkInSessionEnd(sql.NullTime{Time: want, Valid: true}); !got.Equal(want) {
		t.Fatalf("valid walk-in expiry = %v, want %v", got, want)
	}
}

func TestMatchesAgentHubSnapshot(t *testing.T) {
	userID := 7
	changedUserID := 8
	userName := "student"
	sessionEnd := time.Now()
	original := &hub.ComputerRuntimeState{
		Status:           "in_use",
		CurrentBookingID: 42,
		CurrentUserID:    &userID,
		CurrentUserName:  &userName,
		SessionEndsAt:    &sessionEnd,
	}
	snapshot := snapshotAgentHubState(original)
	tests := []struct {
		name  string
		state hub.ComputerRuntimeState
		want  bool
	}{
		{name: "same snapshot", state: *original, want: true},
		{name: "newer booking", state: hub.ComputerRuntimeState{Status: "in_use", CurrentBookingID: 43}},
		{name: "changed session", state: hub.ComputerRuntimeState{Status: "in_use", CurrentBookingID: 42, CurrentUserID: &changedUserID, CurrentUserName: &userName, SessionEndsAt: &sessionEnd}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesAgentHubSnapshot(&tt.state, snapshot); got != tt.want {
				t.Fatalf("matchesAgentHubSnapshot() = %v, want %v", got, tt.want)
			}
		})
	}

	legacy := &hub.ComputerRuntimeState{Status: "in_use"}
	legacySnapshot := snapshotAgentHubState(legacy)
	if !matchesAgentHubSnapshot(legacy, legacySnapshot) {
		t.Fatal("unchanged legacy Hub state did not match its snapshot")
	}
	legacy.Status = "available"
	if matchesAgentHubSnapshot(legacy, legacySnapshot) {
		t.Fatal("changed legacy Hub state matched its old snapshot")
	}
}

func TestHasStaleAgentHubSession(t *testing.T) {
	bookingID := 12
	usageID := 4
	tests := []struct {
		name   string
		state  *hub.ComputerRuntimeState
		status string
		want   bool
	}{
		{name: "missing state", status: "available"},
		{name: "clean state", state: &hub.ComputerRuntimeState{Status: "available"}, status: "available"},
		{name: "status differs", state: &hub.ComputerRuntimeState{Status: "in_use"}, status: "available", want: true},
		{name: "stale booking ID", state: &hub.ComputerRuntimeState{Status: "available", CurrentBookingID: bookingID}, status: "available", want: true},
		{name: "stale usage ID", state: &hub.ComputerRuntimeState{Status: "available", CurrentUsageLogID: usageID}, status: "available", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasStaleAgentHubSession(tt.state, tt.status); got != tt.want {
				t.Fatalf("hasStaleAgentHubSession() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStartAgentPingWritesPingAndStops(t *testing.T) {
	pong := make(chan struct{}, 1)
	pingReady := make(chan struct{})
	serverDone := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(serverDone)
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetPongHandler(func(string) error {
			select {
			case pong <- struct{}{}:
			default:
			}
			return nil
		})
		stopPinging := startAgentPing(conn, 10*time.Millisecond)
		defer stopPinging()
		close(pingReady)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(strings.Replace(server.URL, "http://", "ws://", 1), nil)
	if err != nil {
		t.Fatal(err)
	}
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	select {
	case <-pingReady:
	case <-time.After(time.Second):
		t.Fatal("agent ping loop did not start")
	}
	select {
	case <-pong:
	case <-time.After(time.Second):
		t.Fatal("agent ping did not receive a pong")
	}
	_ = conn.Close()
	select {
	case <-readerDone:
	case <-time.After(time.Second):
		t.Fatal("agent reader did not stop after connection close")
	}
	select {
	case <-serverDone:
	case <-time.After(time.Second):
		t.Fatal("agent ping loop did not stop after handler exit")
	}
}
