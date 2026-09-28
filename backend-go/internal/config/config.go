package config

import (
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                    string
	ListenAddr              string
	TLSCertFile             string
	TLSKeyFile              string
	DBHost                  string
	DBPort                  string
	DBUser                  string
	DBPass                  string
	DBName                  string
	DBTLSCAFile             string
	JWTSecret               string
	JWTExpiresIn            int
	AgentSecret             string
	AgentReleaseDir         string
	AllowedOrigins          []string
	TrustedProxies          []string
	SAdminUsername          string
	SAdminPassword          string
	AllowLegacyAgents       bool
	SelfRegistrationEnabled bool
	CookieSecure            bool
	CookieSameSite          string
	CookieDomain            string
}

func LoadConfig() *Config {
	// Try loading .env from current directory or parent directory
	if err := godotenv.Load(".env"); err != nil {
		if err := godotenv.Load("../.env"); err != nil {
			log.Println("[Config] No .env file found, using environment variables")
		}
	}

	port := getEnv("PORT", "8000")
	tlsCert := getEnv("TLS_CERT_FILE", "")
	tlsKey := getEnv("TLS_KEY_FILE", "")
	if (tlsCert == "") != (tlsKey == "") {
		log.Fatal("[Config] TLS_CERT_FILE and TLS_KEY_FILE must be set together")
	}
	listenAddr := getEnv("HTTP_ADDR", "")
	if listenAddr == "" {
		if tlsCert != "" {
			listenAddr = ":" + port
		} else {
			listenAddr = net.JoinHostPort("127.0.0.1", port)
		}
	}
	if tlsCert == "" && !isLoopbackAddress(listenAddr) {
		log.Fatal("[Config] Plain HTTP must bind to loopback; configure TLS or use a local HTTPS reverse proxy")
	}
	exp, err := strconv.Atoi(getEnv("JWT_EXPIRATION_MINUTES", "1440"))
	if err != nil || exp <= 0 {
		log.Fatal("[Config] JWT_EXPIRATION_MINUTES must be a positive integer")
	}
	cookieSecure := strings.EqualFold(getEnv("COOKIE_SECURE", "true"), "true")
	cookieSameSite := strings.ToLower(getEnv("COOKIE_SAMESITE", "lax"))
	if cookieSameSite == "none" && !cookieSecure {
		log.Fatal("[Config] COOKIE_SAMESITE=none requires COOKIE_SECURE=true")
	}

	jwtSecret := mustGetEnv("JWT_SECRET")
	if len(jwtSecret) < 32 {
		log.Fatal("[Config] JWT_SECRET must contain at least 32 bytes")
	}
	agentSecret := getEnv("AGENT_SECRET", "")
	allowLegacyAgents, err := strconv.ParseBool(getEnv("ALLOW_LEGACY_AGENTS", strconv.FormatBool(agentSecret != "")))
	if err != nil {
		log.Fatal("[Config] ALLOW_LEGACY_AGENTS must be true or false")
	}
	if allowLegacyAgents && (len(agentSecret) < 32 || len(agentSecret) > 256) {
		log.Fatal("[Config] ALLOW_LEGACY_AGENTS requires an AGENT_SECRET of 32-256 bytes")
	}
	registrationEnabled, err := strconv.ParseBool(getEnv("SELF_REGISTRATION_ENABLED", "false"))
	if err != nil {
		log.Fatal("[Config] SELF_REGISTRATION_ENABLED must be true or false")
	}
	adminName := getEnv("SADMIN_USERNAME", "")
	adminPassword := getEnv("SADMIN_PASSWORD", "")
	if (adminName == "") != (adminPassword == "") {
		log.Fatal("[Config] Set both SADMIN_USERNAME and SADMIN_PASSWORD, or neither")
	}
	if adminName != "" && (len(adminName) > 50 || len(adminPassword) < 12 || len(adminPassword) > 72) {
		log.Fatal("[Config] Initial admin username must be at most 50 bytes and password 12-72 bytes")
	}

	return &Config{
		Port:            port,
		ListenAddr:      listenAddr,
		TLSCertFile:     tlsCert,
		TLSKeyFile:      tlsKey,
		DBHost:          getEnv("DB_HOST", "127.0.0.1"),
		DBPort:          getEnv("DB_PORT", "3306"),
		DBUser:          mustGetEnv("DB_USER"),
		DBPass:          mustGetEnv("DB_PASS"),
		DBName:          mustGetEnv("DB_NAME"),
		DBTLSCAFile:     getEnv("DB_TLS_CA_FILE", ""),
		JWTSecret:       jwtSecret,
		JWTExpiresIn:    exp,
		AgentSecret:     agentSecret,
		AgentReleaseDir: getEnv("AGENT_RELEASE_DIR", "agent-releases"),
		AllowedOrigins: splitAndTrim(getEnv("ALLOWED_ORIGINS",
			"http://localhost:5173,http://127.0.0.1:5173,http://localhost:3000")),
		TrustedProxies:          splitAndTrim(getEnv("TRUSTED_PROXIES", "")),
		SAdminUsername:          adminName,
		SAdminPassword:          adminPassword,
		AllowLegacyAgents:       allowLegacyAgents,
		SelfRegistrationEnabled: registrationEnabled,
		CookieSecure:            cookieSecure,
		CookieSameSite:          cookieSameSite,
		CookieDomain:            getEnv("COOKIE_DOMAIN", ""),
	}
}

func isLoopbackAddress(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// splitAndTrim splits a comma-separated env value into a cleaned slice,
// dropping empty entries.
func splitAndTrim(csv string) []string {
	if csv == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

// mustGetEnv returns the value of a required environment variable.
// It terminates the process if the variable is unset or empty, preventing
// the server from ever falling back to insecure hardcoded secrets.
func mustGetEnv(key string) string {
	val, ok := os.LookupEnv(key)
	if !ok || val == "" {
		log.Fatalf("[Config] FATAL: required environment variable %q is not set. "+
			"Refusing to start with insecure defaults. Set it in .env or the environment.", key)
	}
	return val
}
