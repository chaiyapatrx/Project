package hub

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestSignedCommandsHaveUniqueAuthenticatedNonces(t *testing.T) {
	secret := "test-command-secret-not-a-production-credential"
	original := map[string]interface{}{"reason": "test"}
	first := signCommand(secret, "LOCK", 123, original)
	second := signCommand(secret, "LOCK", 123, original)
	if first == nil || second == nil || first["signature"] == second["signature"] {
		t.Fatal("identical commands in one second must have distinct signed nonces")
	}
	if _, mutated := original["command_nonce"]; mutated {
		t.Fatal("signCommand modified caller data")
	}
	data := first["data"].(map[string]interface{})
	nonce, ok := data["command_nonce"].(string)
	if !ok || len(nonce) != 32 {
		t.Fatal("command nonce must contain 16 random bytes encoded as hex")
	}
	canonical := func() string {
		encoded, err := json.Marshal(struct {
			Action    string                 `json:"action"`
			Timestamp int64                  `json:"timestamp"`
			Data      map[string]interface{} `json:"data"`
		}{"LOCK", 123, data})
		if err != nil {
			t.Fatal(err)
		}
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(encoded)
		return hex.EncodeToString(mac.Sum(nil))
	}
	if canonical() != first["signature"] {
		t.Fatal("legacy command envelope did not verify the signed nonce")
	}
	data["command_nonce"] = "tampered"
	if canonical() == first["signature"] {
		t.Fatal("nonce tampering did not invalidate the signature")
	}
	if signCommand("", "LOCK", 123, nil) != nil || signCommand(secret, "LOCK", 123, map[string]interface{}{"invalid": make(chan int)}) != nil {
		t.Fatal("signing failed to reject missing credentials or unserializable data")
	}
}

func TestMatchesBookingID(t *testing.T) {
	tests := []struct {
		name           string
		currentBooking int
		bookingID      int
		want           bool
	}{
		{name: "same booking", currentBooking: 42, bookingID: 42, want: true},
		{name: "newer booking", currentBooking: 43, bookingID: 42},
		{name: "missing booking ID", currentBooking: 0, bookingID: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchesBookingID(tt.currentBooking, tt.bookingID); got != tt.want {
				t.Fatalf("MatchesBookingID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCurrentBookingIDIsHiddenFromJSON(t *testing.T) {
	encoded, err := json.Marshal(ComputerRuntimeState{CurrentBookingID: 42})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "CurrentBookingID") || strings.Contains(string(encoded), "current_booking_id") {
		t.Fatalf("booking ID leaked in Hub JSON: %s", encoded)
	}
}

func TestLockAuthDataUsesStationReservation(t *testing.T) {
	original := map[string]interface{}{"reason": "staff_lock"}
	if got := lockAuthData("LOCK", original, true)["auth_mode"]; got != "access_code" {
		t.Fatalf("reserved station auth mode = %v, want access_code", got)
	}
	if _, mutated := original["auth_mode"]; mutated {
		t.Fatal("lockAuthData mutated caller data")
	}
	if got := lockAuthData("LOCK", nil, false)["auth_mode"]; got != "account" {
		t.Fatalf("unreserved station auth mode = %v, want account", got)
	}
	if got := lockAuthData("LOCK", map[string]interface{}{"auth_mode": "access_code"}, false)["auth_mode"]; got != "access_code" {
		t.Fatalf("explicit lock auth mode = %v, want access_code", got)
	}
}
