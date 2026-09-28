package database

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDBTLSConfigRequiresVerifiedTLSForRemoteHosts(t *testing.T) {
	for _, host := range []string{"localhost", "127.0.0.1", "::1"} {
		cfg, useTLS, err := dbTLSConfig(host, "")
		if err != nil || useTLS || cfg != nil {
			t.Fatalf("dbTLSConfig(%q) = (%v, %v, %v), want local plaintext", host, cfg, useTLS, err)
		}
	}

	cfg, useTLS, err := dbTLSConfig("db.example.invalid", "")
	if err != nil {
		t.Fatal("remote TLS config failed")
	}
	if !useTLS || cfg == nil || cfg.MinVersion != tls.VersionTLS12 || cfg.ServerName != "db.example.invalid" || cfg.RootCAs == nil || cfg.InsecureSkipVerify {
		t.Fatalf("remote TLS config is not verified TLS 1.2+: %#v", cfg)
	}
}

func TestDBTLSConfigAddsCAAndRejectsInvalidCAFile(t *testing.T) {
	certPEM, rawSubject := makeTestRootCertificate(t)
	caFile := filepath.Join(t.TempDir(), "db-ca.pem")
	if err := os.WriteFile(caFile, certPEM, 0o600); err != nil {
		t.Fatal("write test CA")
	}
	cfg, useTLS, err := dbTLSConfig("db.example.invalid", caFile)
	if err != nil || !useTLS || cfg == nil {
		t.Fatal("valid CA file was rejected")
	}
	found := false
	for _, subject := range cfg.RootCAs.Subjects() {
		if bytes.Equal(subject, rawSubject) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("configured CA was not added to trusted roots")
	}

	badCAFile := filepath.Join(t.TempDir(), "bad-ca.pem")
	if err := os.WriteFile(badCAFile, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal("write invalid test CA")
	}
	if _, _, err := dbTLSConfig("db.example.invalid", badCAFile); err == nil {
		t.Fatal("invalid CA file was accepted")
	}
}

func makeTestRootCertificate(t *testing.T) ([]byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal("generate test CA key")
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test database CA"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal("create test CA certificate")
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal("parse test CA certificate")
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), cert.RawSubject
}
