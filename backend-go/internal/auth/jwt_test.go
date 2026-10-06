package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"station-backend/internal/config"
	"station-backend/internal/models"
)

func TestPasswordComparisonRejectsBcryptTruncation(t *testing.T) {
	password := strings.Repeat("a", 72)
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPasswordHash(password, hash) {
		t.Fatal("valid 72-byte password rejected")
	}
	if CheckPasswordHash(password+"extra", hash) || CheckPasswordHash("", hash) {
		t.Fatal("invalid password accepted")
	}
}

func TestValidateTokenRequiresExpiryAndHS256(t *testing.T) {
	cfg := &config.Config{JWTSecret: "01234567890123456789012345678901", JWTExpiresIn: 1}
	valid, err := GenerateToken(&models.User{ID: 1, Username: "student", Role: "student"}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateToken(valid, cfg); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	for _, test := range []struct {
		name   string
		method jwt.SigningMethod
		claims jwt.RegisteredClaims
	}{
		{name: "missing expiry", method: jwt.SigningMethodHS256},
		{name: "expired", method: jwt.SigningMethodHS256, claims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute))}},
		{name: "wrong signing method", method: jwt.SigningMethodHS384, claims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))}},
	} {
		t.Run(test.name, func(t *testing.T) {
			token, err := jwt.NewWithClaims(test.method, &CustomClaims{UserID: 1, RegisteredClaims: test.claims}).SignedString([]byte(cfg.JWTSecret))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ValidateToken(token, cfg); err == nil {
				t.Fatal("invalid token accepted")
			}
		})
	}
}
