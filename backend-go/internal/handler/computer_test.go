package handler

import "testing"

func TestValidAgentCommand(t *testing.T) {
	for _, command := range []string{"LOCK", "UNLOCK", "REBOOT", "SHUTDOWN", "MESSAGE", "NOTIFICATION"} {
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
