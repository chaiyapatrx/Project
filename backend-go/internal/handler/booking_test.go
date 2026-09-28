package handler

import (
	"testing"

	"station-backend/internal/hub"
)

func TestClassifyBookingCommitRecovery(t *testing.T) {
	tests := []struct {
		name           string
		bookingStatus  string
		computerStatus string
		activeBookings int
		want           bookingCommitRecovery
	}{
		{name: "persisted booking", bookingStatus: "active", computerStatus: "in_use", activeBookings: 1, want: bookingCommitPersisted},
		{name: "no active booking", bookingStatus: "cancelled", computerStatus: "available", want: bookingCommitNoActive},
		{name: "newer active booking", bookingStatus: "cancelled", computerStatus: "in_use", activeBookings: 1, want: bookingCommitOtherActive},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyBookingCommitRecovery(tt.bookingStatus, tt.computerStatus, tt.activeBookings); got != tt.want {
				t.Fatalf("classifyBookingCommitRecovery() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCanRestoreClearedHubBooking(t *testing.T) {
	tests := []struct {
		name  string
		state hub.ComputerRuntimeState
		want  bool
	}{
		{name: "cleared state", state: hub.ComputerRuntimeState{Status: "available"}, want: true},
		{name: "active walk-in session", state: hub.ComputerRuntimeState{Status: "available", CurrentUsageLogID: 7}},
		{name: "new booking", state: hub.ComputerRuntimeState{Status: "in_use", CurrentBookingID: 43}},
		{name: "changed status", state: hub.ComputerRuntimeState{Status: "maintenance"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canRestoreClearedHubBooking(&tt.state); got != tt.want {
				t.Fatalf("canRestoreClearedHubBooking() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCancellationCommitRecovery(t *testing.T) {
	if !cancelCommitRequiresDisconnect("active") {
		t.Fatal("active booking commit error must disconnect the agent before resync")
	}
	if cancelCommitRequiresDisconnect("pending") {
		t.Fatal("cancelling a pending booking should not disconnect a sessionless agent")
	}
	if !cancellationCommitPersisted("cancelled") {
		t.Fatal("cancelled booking should confirm the commit")
	}
	if cancellationCommitPersisted("active") {
		t.Fatal("active booking should indicate the cancellation did not persist")
	}
	tests := []struct {
		name           string
		bookingStatus  string
		computerStatus string
		activeBookings int
		activeUsage    int
		want           bool
	}{
		{name: "clear cancelled orphan", bookingStatus: "cancelled", computerStatus: "available", want: true},
		{name: "preserve newer active booking", bookingStatus: "cancelled", computerStatus: "in_use", activeBookings: 1},
		{name: "preserve active walk-in", bookingStatus: "cancelled", computerStatus: "in_use", activeUsage: 1},
		{name: "preserve rollback", bookingStatus: "active", computerStatus: "in_use", activeBookings: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canClearCancelledBookingHub(tt.bookingStatus, tt.computerStatus, tt.activeBookings, tt.activeUsage); got != tt.want {
				t.Fatalf("canClearCancelledBookingHub() = %v, want %v", got, tt.want)
			}
		})
	}
}
