package auth

import "testing"

func TestGenerateAndVerifyAgentSecret(t *testing.T) {
	secret, hash, err := GenerateAgentSecret()
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyAgentSecret(secret, hash) {
		t.Fatal("generated agent secret did not verify")
	}
	if VerifyAgentSecret(secret+"x", hash) || VerifyAgentSecret("short", hash) {
		t.Fatal("invalid agent secret was accepted")
	}
}
