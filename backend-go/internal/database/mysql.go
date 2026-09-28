package database

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
	"station-backend/internal/config"
)

var DB *sqlx.DB

func InitDB(cfg *config.Config) *sqlx.DB {
	driverConfig := mysql.NewConfig()
	driverConfig.User = cfg.DBUser
	driverConfig.Passwd = cfg.DBPass
	driverConfig.Net = "tcp"
	driverConfig.Addr = net.JoinHostPort(cfg.DBHost, cfg.DBPort)
	driverConfig.DBName = cfg.DBName
	driverConfig.Params = map[string]string{"charset": "utf8mb4"}
	driverConfig.ParseTime = true
	driverConfig.Loc = time.Local

	tlsConfig, useTLS, err := dbTLSConfig(cfg.DBHost, cfg.DBTLSCAFile)
	if err != nil {
		log.Fatal("[Database] Failed to configure verified TLS for remote MySQL")
	}
	if useTLS {
		if err := mysql.RegisterTLSConfig("verified-db", tlsConfig); err != nil {
			log.Fatal("[Database] Failed to register verified TLS for remote MySQL")
		}
		driverConfig.TLSConfig = "verified-db"
	}

	db, err := sqlx.Connect("mysql", driverConfig.FormatDSN())
	if err != nil {
		log.Fatal("[Database] Failed to connect to MySQL")
	}

	// Production connection pool configuration
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Fatal("[Database] MySQL Ping failed")
	}

	log.Println("[Database] Connected to MySQL successfully (Pool: Max 50 conns)")
	DB = db

	// Seed the initial Super Admin from configuration (never a hardcoded default)
	ensureSuperAdmin(db, cfg)

	return db
}

func dbTLSConfig(host, caFile string) (*tls.Config, bool, error) {
	cleanHost := strings.Trim(host, "[]")
	ip := net.ParseIP(cleanHost)
	if strings.EqualFold(cleanHost, "localhost") || (ip != nil && ip.IsLoopback()) {
		return nil, false, nil
	}

	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		return nil, false, fmt.Errorf("system certificate roots unavailable")
	}
	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, false, fmt.Errorf("unable to read database CA file")
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, false, fmt.Errorf("database CA file contains no valid certificates")
		}
	}

	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    roots,
		ServerName: cleanHost,
	}, true, nil
}

func ensureSuperAdmin(db *sqlx.DB, cfg *config.Config) {
	// If no super-admin credentials are configured, do not seed one. This avoids
	// ever creating an account with a weak/default password.
	if cfg.SAdminUsername == "" || cfg.SAdminPassword == "" {
		log.Println("[Database] SADMIN_USERNAME/SADMIN_PASSWORD not set; skipping super admin seed.")
		return
	}
	if len(cfg.SAdminPassword) < 12 || len(cfg.SAdminPassword) > 72 {
		log.Printf("[Database] Super admin password must be 12-72 bytes; skipping initial seed.")
		return
	}

	// Check if the super admin already exists; if so, leave it untouched.
	var count int
	_ = db.Get(&count, "SELECT COUNT(*) FROM users WHERE username = ?", cfg.SAdminUsername)
	if count > 0 {
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(cfg.SAdminPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[Database] Failed to hash super admin password: %v", err)
		return
	}

	_, err = db.Exec(`
		INSERT INTO users (username, password_hash, full_name, role, department, user_type, is_active)
		VALUES (?, ?, 'Super Administrator', 'admin', 'System Control', 'staff', 1)`,
		cfg.SAdminUsername, string(hashed),
	)
	if err != nil {
		log.Printf("[Database] Failed to seed super admin: %v", err)
		return
	}
	log.Printf("[Database] Initial Super Admin %q created from configuration.", cfg.SAdminUsername)
}
