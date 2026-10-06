package database

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
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
	driverConfig.Timeout = 5 * time.Second
	driverConfig.ReadTimeout = 15 * time.Second
	driverConfig.WriteTimeout = 15 * time.Second

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
		detail := err.Error()
		if cfg.DBPass != "" {
			detail = strings.ReplaceAll(detail, cfg.DBPass, "[redacted]")
		}
		log.Fatalf("[Database] Failed to connect to MySQL: %s", detail)
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
	if err := ensureSuperAdmin(db, cfg); err != nil {
		log.Fatalf("[Database] Initial admin bootstrap failed: %s", err)
	}

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

func ensureSuperAdmin(db *sqlx.DB, cfg *config.Config) error {
	if !cfg.BootstrapAdmin {
		return nil
	}
	defer func() { cfg.SAdminPassword = "" }()
	if err := config.ValidateBootstrapCredentials(cfg.SAdminUsername, cfg.SAdminPassword); err != nil {
		return err
	}
	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("cannot start bootstrap transaction")
	}
	defer tx.Rollback()
	// The unique settings key serializes bootstrap even under READ COMMITTED.
	// Keep the marker after account deletion so restarting cannot recreate an admin.
	if _, err := tx.Exec("INSERT INTO system_settings (setting_key, setting_value) VALUES ('admin_bootstrap_completed', 'false') ON DUPLICATE KEY UPDATE setting_key = setting_key"); err != nil {
		return fmt.Errorf("cannot lock bootstrap marker")
	}
	var completed string
	if err := tx.Get(&completed, "SELECT setting_value FROM system_settings WHERE setting_key = 'admin_bootstrap_completed' FOR UPDATE"); err != nil {
		return fmt.Errorf("cannot read bootstrap marker")
	}
	if completed == "true" {
		return nil
	}
	if completed != "false" {
		return fmt.Errorf("invalid bootstrap marker")
	}
	var adminID int
	err = tx.Get(&adminID, "SELECT id FROM users WHERE role = 'admin' ORDER BY id LIMIT 1 FOR UPDATE")
	if err == nil {
		if _, err := tx.Exec("UPDATE system_settings SET setting_value = 'true' WHERE setting_key = 'admin_bootstrap_completed'"); err != nil {
			return fmt.Errorf("cannot complete bootstrap marker")
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("cannot commit bootstrap marker")
		}
		log.Println("[Database] An administrator already exists; bootstrap skipped. Remove bootstrap credentials from configuration.")
		return nil
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("cannot verify existing administrators")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(cfg.SAdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("cannot hash bootstrap password")
	}
	_, err = tx.Exec(`
		INSERT INTO users (username, password_hash, full_name, role, department, user_type, is_active)
		VALUES (?, ?, 'Super Administrator', 'admin', 'System Control', 'staff', 1)`,
		cfg.SAdminUsername, string(hashed),
	)
	if err != nil {
		return fmt.Errorf("cannot insert initial administrator")
	}
	if _, err := tx.Exec("UPDATE system_settings SET setting_value = 'true' WHERE setting_key = 'admin_bootstrap_completed'"); err != nil {
		return fmt.Errorf("cannot complete bootstrap marker")
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("cannot commit initial administrator")
	}
	log.Println("[Database] Initial administrator created. Disable BOOTSTRAP_ADMIN and remove SADMIN credentials before exposing the service.")
	return nil
}
