package config

import (
	"os"
	"strconv"
)

// Config holds all application configuration
type Config struct {
	// HTTP server ports
	ConsolePort int // SaaS Console (REST API)
	MCPPort     int // MCP Gateway (JSON-RPC over HTTP)

	// Database
	DBType string // sqlite (dev) | postgres (prod)
	DBDSN  string

	// JWT
	JWTSecret string
	JWTExpire int // hours

	// Default admin account
	BootstrapUsername string
	BootstrapPassword string

	// Encryption
	EncryptionKey string

	// Redis (optional, for rate-limiting and caching)
	RedisAddr string

	// Enable two-step confirmation handshake for high-risk tools
	EnableConfirmation bool
}

// Dev defaults (must be overridden in production; SecurityWarnings checks these).
const (
	defaultJWTSecret         = "open-agent-hub-dev-secret-please-change-in-production"
	defaultEncryptionKey     = "open-agent-hub-encryption-key-32"
	defaultBootstrapPassword = "admin123"
)

func Load() *Config {
	return &Config{
		ConsolePort: getEnvInt("CONSOLE_PORT", 8084),
		MCPPort:     getEnvInt("MCP_PORT", 8085),

		DBType: getEnv("DB_TYPE", "sqlite"),
		DBDSN:  getEnv("DB_DSN", "data/openagenthub.db"),

		JWTSecret: getEnv("JWT_SECRET", defaultJWTSecret),
		JWTExpire: getEnvInt("JWT_EXPIRE_HOURS", 24*7),

		BootstrapUsername: getEnv("BOOTSTRAP_USERNAME", "admin"),
		BootstrapPassword: getEnv("BOOTSTRAP_PASSWORD", defaultBootstrapPassword),

		EncryptionKey: getEnv("ENCRYPTION_KEY", defaultEncryptionKey),

		RedisAddr: getEnv("REDIS_ADDR", ""),

		EnableConfirmation: getEnvBool("ENABLE_CONFIRMATION", false),
	}
}

// SecurityWarnings returns sensitive config items still using dev defaults,
// for logging at startup. Production should override these via env vars.
func (c *Config) SecurityWarnings() []string {
	var w []string
	if c.JWTSecret == defaultJWTSecret {
		w = append(w, "JWT_SECRET is still the dev default; anyone can forge tokens — override in production")
	}
	if c.EncryptionKey == defaultEncryptionKey {
		w = append(w, "ENCRYPTION_KEY is still the dev default; stored credentials are not properly encrypted — override it")
	}
	if c.BootstrapPassword == defaultBootstrapPassword {
		w = append(w, "BOOTSTRAP_PASSWORD is still the default admin123; change the admin password immediately")
	}
	return w
}

func getEnvBool(k string, def bool) bool {
	if v := os.Getenv(k); v != "" {
		switch v {
		case "1", "true", "TRUE", "True", "yes", "on":
			return true
		case "0", "false", "FALSE", "False", "no", "off":
			return false
		}
	}
	return def
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getEnvInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
