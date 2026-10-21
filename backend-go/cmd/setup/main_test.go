package main

import (
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestLANIPv4sSkipsLoopbackLinkLocalAndIPv6(t *testing.T) {
	addrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)},
		&net.IPNet{IP: net.ParseIP("169.254.3.4"), Mask: net.CIDRMask(16, 32)},
		&net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)},
		&net.IPNet{IP: net.ParseIP("192.168.1.20"), Mask: net.CIDRMask(24, 32)},
	}
	ips := lanIPv4s(addrs)
	if len(ips) != 1 || ips[0].String() != "192.168.1.20" {
		t.Fatalf("got %v", ips)
	}
}

func TestCertificateCoversNameAndLANIP(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "tls"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := createCertificate(root, "labaccess-server", []net.IP{net.ParseIP("192.168.1.20").To4()}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "tls", "server.pem"))
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(data)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"labaccess-server", "192.168.1.20", "localhost", "127.0.0.1"} {
		if err := cert.VerifyHostname(host); err != nil {
			t.Errorf("%s: %v", host, err)
		}
	}
}
