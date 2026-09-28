package hub

import (
	"encoding/json"
	"strings"
	"testing"
)

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
