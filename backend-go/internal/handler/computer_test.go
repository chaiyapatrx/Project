package handler

import (
	"strings"
	"testing"
)

func TestPrivatePackageEnrollmentToken(t *testing.T) {
	token := strings.Repeat("aB9_", 11)
	for _, test := range []struct {
		expected, supplied string
		want               bool
	}{
		{token, token, true}, {"", "", false}, {"", token, false},
		{token, "", false}, {token, token + "x", false}, {token, strings.Repeat("x", 257), false},
	} {
		if got := validEnrollmentToken(test.expected, test.supplied); got != test.want {
			t.Fatalf("enrollment token authorization = %v, want %v", got, test.want)
		}
	}
}

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
