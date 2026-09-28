package handler

import (
	"database/sql"
	"testing"

	"station-backend/internal/auth"
)

func TestValidAccessCode(t *testing.T) {
	for _, tc := range []struct {
		code string
		want bool
	}{
		{code: "000042", want: true},
		{code: "123456", want: true},
		{code: "12345"},
		{code: "1234567"},
		{code: "12a456"},
		{code: "１２３４５６"},
	} {
		if got := validAccessCode(tc.code); got != tc.want {
			t.Errorf("validAccessCode(%q) = %v, want %v", tc.code, got, tc.want)
		}
	}
}

func TestVerifyStationSecret(t *testing.T) {
	secret, hash, err := auth.GenerateAgentSecret()
	if err != nil {
		t.Fatal(err)
	}
	if !verifyStationSecret(secret, sql.NullString{String: hash, Valid: true}, "", false) {
		t.Fatal("valid stored station secret rejected")
	}
	if verifyStationSecret("wrong-station-secret-with-sufficient-length", sql.NullString{String: hash, Valid: true}, "", false) {
		t.Fatal("invalid stored station secret accepted")
	}
	if !verifyStationSecret(secret, sql.NullString{}, secret, true) {
		t.Fatal("enabled legacy station secret rejected")
	}
	if verifyStationSecret(secret, sql.NullString{}, secret, false) {
		t.Fatal("disabled legacy station secret accepted")
	}
}
