package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                     int
	Host                     string
	LogLevelStr              string
	NodeEnv                  string
	CORSOrigin               string
	DatabasePath             string
	UploadDir                string
	WorkspaceDir             string
	ClientDist               string
	JWTSecret                string
	SQLiteMaxOpenConns       int
	SQLiteTxLock             string
	DisableBackgroundWorkers bool
	DevAuthBypass            bool
	AdminUser                string
	AdminPassword            string

	// PublicHelperOrigin overrides the scheme+host used in the
	// `install_command` field returned by POST /api/v1/helper/enrollments.
	// 默认 (空) 时 handler 走老路径: 从 r.Host / X-Forwarded-* 推 origin,
	// 与 prod 现有行为一致 (backward compat). 设置时 (e.g.
	// `ws://borgee-server:4900` for the docker dev-stack, or
	// `wss://borgee.codetrek.cn` behind a reverse proxy whose Host header
	// 不是 public URL) handler 直接用这个值作为 --server 的 origin,
	// 解 Host != public URL 场景下印的 install 命令不可达的问题. 见 #1052.
	PublicHelperOrigin string

	// Rate limit token-bucket params. 单位 req/s (秒).
	// auth: path 前缀 /api/v1/auth/ 或 /admin-api/auth/, per-IP, 防爆破
	// user: 登录态 (UserFromContext 拿到 user), per-user_id
	// anon: 其它 (未登录非 auth 端点), per-IP
	RateLimitAuthPerSec int
	RateLimitAuthBurst  int
	RateLimitUserPerSec int
	RateLimitUserBurst  int
	RateLimitAnonPerSec int
	RateLimitAnonBurst  int
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

		PublicHelperOrigin: envStr("BORGEE_PUBLIC_HELPER_ORIGIN", ""),

		RateLimitAuthPerSec: envInt("RATE_LIMIT_AUTH_PER_SEC", 5),
		RateLimitAuthBurst:  envInt("RATE_LIMIT_AUTH_BURST", 15),
		RateLimitUserPerSec: envInt("RATE_LIMIT_USER_PER_SEC", 20),
		RateLimitUserBurst:  envInt("RATE_LIMIT_USER_BURST", 60),
		RateLimitAnonPerSec: envInt("RATE_LIMIT_ANON_PER_SEC", 100),
		RateLimitAnonBurst:  envInt("RATE_LIMIT_ANON_BURST", 300),
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
	// #1052: BORGEE_PUBLIC_HELPER_ORIGIN, when set, must be a `ws://` or
	// `wss://` origin (scheme + host[:port], no trailing slash / path).
	// 反 silent footgun: e.g. `https://borgee.example.com` 会被 borgee install
	// 当成 http origin upgrade 失败. 空 (未设) 跳过校验, 走 r.Host 老路径.
	if v := strings.TrimSpace(c.PublicHelperOrigin); v != "" {
		if !(strings.HasPrefix(v, "ws://") || strings.HasPrefix(v, "wss://")) {
			return fmt.Errorf("BORGEE_PUBLIC_HELPER_ORIGIN must start with ws:// or wss:// (got %q)", v)
		}
		if strings.HasSuffix(v, "/") || strings.Contains(strings.TrimPrefix(strings.TrimPrefix(v, "wss://"), "ws://"), "/") {
			return fmt.Errorf("BORGEE_PUBLIC_HELPER_ORIGIN must be scheme+host[:port] only, no path (got %q)", v)
		}
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
