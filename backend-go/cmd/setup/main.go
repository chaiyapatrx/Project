// AUCCSetup prepares a private, self-contained Windows server installation.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "Usage: AUCCSetup prepare|finish ROOT HOST")
		os.Exit(1)
	}
	root, err := filepath.Abs(os.Args[2])
	if err == nil {
		switch os.Args[1] {
		case "prepare":
			err = prepare(root, os.Args[3])
		case "finish":
			err = finish(root)
		default:
			err = fmt.Errorf("unknown setup operation")
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func randomSecret() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

// This hash is used only by the bundled MariaDB mysql_native_password plugin.
func mariaPassword(password string) string {
	first := sha1.Sum([]byte(password))
	second := sha1.Sum(first[:])
	return "*" + strings.ToUpper(hex.EncodeToString(second[:]))
}

func prepare(root, host string) error {
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]{0,252}$`).MatchString(host) {
		return fmt.Errorf("server name must be a DNS name or IPv4 address")
	}
	host = strings.ToLower(host)
	envPath := filepath.Join(root, ".env")
	if _, err := os.Stat(envPath); err == nil {
		// Existing credentials and data are never replaced on repair/reinstall.
		values, err := godotenv.Read(envPath)
		if err != nil {
			return err
		}
		if values["HTTP_ADDR"] != "0.0.0.0:8000" || values["DB_PORT"] != "3308" || values["DB_USER"] != "aucc_app" || values["DB_NAME"] != "aucc" {
			return fmt.Errorf("this folder contains an unmanaged installation; use a separate folder")
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, "data", "mysql")); err == nil {
		return fmt.Errorf("database data exists without its configuration; restore the configuration before running Setup")
	} else if !os.IsNotExist(err) {
		return err
	}
	for _, dir := range []string{"tls", "data", "client-package", "agent-releases"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
			return err
		}
	}
	secrets := make([]string, 5)
	for i := range secrets {
		var err error
		secrets[i], err = randomSecret()
		if err != nil {
			return err
		}
	}
	if err := createCertificate(root, host); err != nil {
		return err
	}
	url := "https://" + host + ":8000"
	files := map[string]string{
		"database-admin.txt":            "MariaDB root (localhost only): " + secrets[0] + "\r\n",
		"admin-login.txt":               "AUCC URL: " + url + "\r\nUsername: admin\r\nPassword: " + secrets[3] + "\r\nChange this password after signing in.\r\n",
		"AUCC.url":                      "[InternetShortcut]\r\nURL=" + url + "\r\n",
		"client-package/deployment.ini": "[AUCC]\r\nServerURL=wss://" + host + ":8000/api/ws/agent\r\nEnrollmentToken=" + secrets[4] + "\r\n",
		"my.ini":                        "[mysqld]\r\nbasedir=" + filepath.ToSlash(filepath.Join(root, "mariadb")) + "\r\ndatadir=" + filepath.ToSlash(filepath.Join(root, "data")) + "\r\nport=3308\r\nbind-address=127.0.0.1\r\ncharacter-set-server=utf8mb4\r\ncollation-server=utf8mb4_unicode_ci\r\nskip-name-resolve\r\n",
		"database-init.sql": "CREATE DATABASE IF NOT EXISTS aucc CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;\n" +
			"CREATE USER IF NOT EXISTS 'aucc_app'@'127.0.0.1' IDENTIFIED VIA mysql_native_password USING '" + mariaPassword(secrets[1]) + "';\n" +
			"GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, INDEX, REFERENCES ON aucc.* TO 'aucc_app'@'127.0.0.1';\n" +
			"CREATE USER IF NOT EXISTS 'root'@'127.0.0.1' IDENTIFIED VIA mysql_native_password USING '" + mariaPassword(secrets[0]) + "';\n" +
			"ALTER USER 'root'@'127.0.0.1' IDENTIFIED VIA mysql_native_password USING '" + mariaPassword(secrets[0]) + "';\n" +
			"GRANT ALL PRIVILEGES ON *.* TO 'root'@'127.0.0.1' WITH GRANT OPTION;\n" +
			"DELETE FROM mysql.global_priv WHERE User = '' OR (User = 'root' AND Host <> '127.0.0.1');\n" +
			"FLUSH PRIVILEGES;\n" +
			"DROP DATABASE IF EXISTS test;\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			return err
		}
	}
	ca, err := os.ReadFile(filepath.Join(root, "tls", "ca.pem"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "client-package", "server-ca.pem"), ca, 0600); err != nil {
		return err
	}
	// Write the marker last: failed preparation can be retried before DB initialization.
	return godotenv.Write(map[string]string{
		"DB_HOST": "127.0.0.1", "DB_PORT": "3308", "DB_USER": "aucc_app", "DB_PASS": secrets[1], "DB_NAME": "aucc",
		"JWT_SECRET": secrets[2], "BOOTSTRAP_ADMIN": "true", "SADMIN_USERNAME": "admin", "SADMIN_PASSWORD": secrets[3],
		"HTTP_ADDR": "0.0.0.0:8000", "TLS_CERT_FILE": filepath.ToSlash(filepath.Join(root, "tls", "server.pem")),
		"TLS_KEY_FILE": filepath.ToSlash(filepath.Join(root, "tls", "server.key")), "COOKIE_SECURE": "true", "COOKIE_SAMESITE": "lax",
		"ALLOWED_ORIGINS": url + ",https://localhost:8000", "FRONTEND_DIST_DIR": "../frontend/dist",
		"AGENT_RELEASE_DIR": filepath.ToSlash(filepath.Join(root, "agent-releases")), "AGENT_ENROLLMENT_TOKEN": secrets[4],
		"SELF_REGISTRATION_ENABLED": "false", "ALLOW_LEGACY_AGENTS": "false",
	}, envPath)
}

func finish(root string) error {
	path := filepath.Join(root, ".env")
	values, err := godotenv.Read(path)
	if err != nil {
		return err
	}
	values["BOOTSTRAP_ADMIN"] = "false"
	delete(values, "SADMIN_USERNAME")
	delete(values, "SADMIN_PASSWORD")
	if err := godotenv.Write(values, path); err != nil {
		return err
	}
	err = os.Remove(filepath.Join(root, "database-init.sql"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func createCertificate(root, host string) error {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial := func() *big.Int { n, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128)); return n }
	now := time.Now()
	ca := &x509.Certificate{SerialNumber: serial(), Subject: pkix.Name{CommonName: "AUCC " + host + " Installation CA"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(10, 0, 0), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		return err
	}
	// Use the parsed CA so its generated SubjectKeyId becomes the leaf's AKI.
	ca, err = x509.ParseCertificate(caDER)
	if err != nil {
		return err
	}
	server := &x509.Certificate{SerialNumber: serial(), Subject: pkix.Name{CommonName: host},
		NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(5, 0, 0), BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	if ip := net.ParseIP(host); ip != nil {
		server.IPAddresses = append(server.IPAddresses, ip)
	} else {
		server.DNSNames = append(server.DNSNames, host)
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, server, ca, &serverKey.PublicKey, caKey)
	if err != nil {
		return err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(serverKey)
	if err != nil {
		return err
	}
	for name, block := range map[string]*pem.Block{
		"ca.pem": {Type: "CERTIFICATE", Bytes: caDER}, "server.pem": {Type: "CERTIFICATE", Bytes: serverDER},
		"server.key": {Type: "PRIVATE KEY", Bytes: keyDER},
	} {
		if err := os.WriteFile(filepath.Join(root, "tls", name), pem.EncodeToMemory(block), 0600); err != nil {
			return err
		}
	}
	// The CA private key is intentionally not persisted or distributed.
	_, err = tls.LoadX509KeyPair(filepath.Join(root, "tls", "server.pem"), filepath.Join(root, "tls", "server.key"))
	return err
}
