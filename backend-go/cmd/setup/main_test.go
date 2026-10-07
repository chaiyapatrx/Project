package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joho/godotenv"
)

func TestPrepareAndRepairPreserveCredentials(t *testing.T) {
	root := t.TempDir()
	if err := prepare(root, "LAB-SERVER"); err != nil {
		t.Fatal(err)
	}
	values, err := godotenv.Read(filepath.Join(root, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if values["DB_PORT"] != "3308" || len(values["AGENT_ENROLLMENT_TOKEN"]) != 43 || values["COOKIE_SECURE"] != "true" {
		t.Fatal("unsafe generated configuration")
	}
	certPEM, _ := os.ReadFile(filepath.Join(root, "tls", "server.pem"))
	block, _ := pem.Decode(certPEM)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	caPEM, _ := os.ReadFile(filepath.Join(root, "tls", "ca.pem"))
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		t.Fatal("CA missing")
	}
	for _, host := range []string{"LAB-SERVER", "localhost", "127.0.0.1"} {
		if _, err := cert.Verify(x509.VerifyOptions{Roots: roots, DNSName: host}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := cert.Verify(x509.VerifyOptions{Roots: roots, DNSName: "other-server"}); err == nil {
		t.Fatal("wrong server certificate accepted")
	}
	if _, err := tls.LoadX509KeyPair(filepath.Join(root, "tls", "server.pem"), filepath.Join(root, "tls", "server.key")); err != nil {
		t.Fatal(err)
	}
	deployment, _ := os.ReadFile(filepath.Join(root, "client-package", "deployment.ini"))
	if !strings.Contains(string(deployment), values["AGENT_ENROLLMENT_TOKEN"]) || strings.Contains(string(deployment), values["DB_PASS"]) {
		t.Fatal("invalid private client package")
	}
	if _, err := os.Stat(filepath.Join(root, "tls", "ca.key")); !os.IsNotExist(err) {
		t.Fatal("CA private key persisted")
	}
	if err := finish(root); err != nil {
		t.Fatal(err)
	}
	if err := prepare(root, "NEW-NAME"); err != nil {
		t.Fatal(err)
	}
	after, _ := godotenv.Read(filepath.Join(root, ".env"))
	if after["DB_PASS"] != values["DB_PASS"] || after["JWT_SECRET"] != values["JWT_SECRET"] || after["BOOTSTRAP_ADMIN"] != "false" || after["SADMIN_PASSWORD"] != "" {
		t.Fatal("repair changed credentials or restored bootstrap")
	}
	if _, err := os.Stat(filepath.Join(root, "database-init.sql")); !os.IsNotExist(err) {
		t.Fatal("bootstrap SQL not removed")
	}
}

func TestPrepareRejectsInjectionAndUnmanagedInstallation(t *testing.T) {
	for _, host := range []string{"", "server\nEnrollmentToken=x", "https://server", "server:8000", "server name"} {
		if err := prepare(t.TempDir(), host); err == nil {
			t.Fatalf("accepted invalid host %q", host)
		}
	}
	root := t.TempDir()
	path := filepath.Join(root, ".env")
	if err := os.WriteFile(path, []byte("DB_PASS=keep-this\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := prepare(root, "server"); err == nil {
		t.Fatal("overwrote unmanaged deployment")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "DB_PASS=keep-this\n" {
		t.Fatal("modified unmanaged credentials")
	}
	withData := t.TempDir()
	if err := os.MkdirAll(filepath.Join(withData, "data", "mysql"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := prepare(withData, "server"); err == nil {
		t.Fatal("reinitialized database data with missing configuration")
	}
}
