package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
)

// GenerateAgentSecret creates a high-entropy per-computer credential and its
// non-reversible database representation. The plaintext is returned only once.
func GenerateAgentSecret() (string, string, error) {
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", err
	}
	secret := base64.RawURLEncoding.EncodeToString(secretBytes)
	return secret, HashAgentSecret(secret), nil
}

func HashAgentSecret(secret string) string {
	digest := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(digest[:])
}

func VerifyAgentSecret(secret, expectedHash string) bool {
	if len(secret) < 32 || len(secret) > 256 || len(expectedHash) != sha256.Size*2 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(HashAgentSecret(secret)), []byte(expectedHash)) == 1
}
