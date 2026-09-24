package database

import (
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
	"station-backend/internal/config"
)

var DB *sqlx.DB

func InitDB(cfg *config.Config) *sqlx.DB {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.DBUser, cfg.DBPass, cfg.DBHost, cfg.DBPort, cfg.DBName)

	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		log.Fatalf("[Database] Failed to connect to MySQL: %v", err)
	}

	// Production connection pool configuration
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Fatalf("[Database] MySQL Ping failed: %v", err)
	}

	log.Println("[Database] Connected to MySQL successfully (Pool: Max 50 conns)")
	DB = db

	// Seed the initial Super Admin from configuration (never a hardcoded default)
	ensureSuperAdmin(db, cfg)

	return db
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
