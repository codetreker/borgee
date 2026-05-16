package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port               int
	Host               string
	LogLevelStr        string
	NodeEnv            string
	CORSOrigin         string
	DatabasePath       string
	UploadDir          string
	WorkspaceDir       string
	ClientDist         string
	JWTSecret          string
	SQLiteMaxOpenConns int
	SQLiteTxLock       string
	DevAuthBypass      bool
	AdminUser          string
	AdminPassword      string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:               envInt("PORT", 4900),
		Host:               envStr("HOST", "0.0.0.0"),
		LogLevelStr:        envStr("LOG_LEVEL", "info"),
		NodeEnv:            envStr("NODE_ENV", ""),
		CORSOrigin:         envStr("CORS_ORIGIN", ""),
		DatabasePath:       envStr("DATABASE_PATH", "data/collab.db"),
		UploadDir:          envStr("UPLOAD_DIR", "data/uploads"),
		WorkspaceDir:       envStr("WORKSPACE_DIR", "data/workspaces"),
		ClientDist:         envStr("CLIENT_DIST", "packages/client/dist"),
		JWTSecret:          envStr("JWT_SECRET", ""),
		SQLiteMaxOpenConns: envInt("SQLITE_MAX_OPEN_CONNS", 0),
		SQLiteTxLock:       envStr("SQLITE_TXLOCK", ""),
		DevAuthBypass:      envBool("DEV_AUTH_BYPASS", false),
		AdminUser:          envStr("ADMIN_USER", ""),
		AdminPassword:      envStr("ADMIN_PASSWORD", ""),
	}

	if cfg.JWTSecret == "" && cfg.IsDevelopment() {
		cfg.JWTSecret = "dev-secret"
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) IsDevelopment() bool {
	return c.NodeEnv == "development"
}

func (c *Config) Validate() error {
	if !c.IsDevelopment() && c.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required in production")
	}
	// no-hardcoded-domain milestone: CORS_ORIGIN is required in non-dev.
	// 反 silent prod default `https://borgee.codetrek.cn` 烧 fork / staging /
	// testing / on-prem 部署. Pattern same as #635 admin-password panic-on-missing
	// (review checklist 1.A bootstrap fail-loud).
	if !c.IsDevelopment() && c.CORSOrigin == "" {
		return fmt.Errorf("CORS_ORIGIN env required (e.g. https://your-deploy-host.example.com)")
	}
	return nil
}

func (c *Config) LogLevel() slog.Level {
	switch strings.ToLower(c.LogLevelStr) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}
