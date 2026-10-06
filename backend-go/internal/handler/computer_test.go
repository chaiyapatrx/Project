package handler

import "testing"

func TestReleasedStationStatus(t *testing.T) {
	for _, status := range []string{"available", "maintenance", "disabled", "in_use"} {
		want := status
		if status == "in_use" {
			want = "available"
		}
		if got := releasedStationStatus(status); got != want {
			t.Errorf("releasedStationStatus(%q) = %q, want %q", status, got, want)
		}
	}
}

func TestValidAgentCommand(t *testing.T) {
	for _, command := range []string{"LOCK", "LOGOUT", "UNLOCK", "REBOOT", "SHUTDOWN", "MESSAGE", "NOTIFICATION"} {
		if !validAgentCommand(command) {
			t.Errorf("validAgentCommand(%q) = false", command)
		}
	}
	for _, command := range []string{"", "UPDATE", "DELETE", "LOCK ALL"} {
		if validAgentCommand(command) {
			t.Errorf("validAgentCommand(%q) = true", command)
		}
	}
}
