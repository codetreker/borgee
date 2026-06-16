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

	// TrustedProxyCount 控制 rate-limit client-IP 推导信任几跳反代.
	// 默认 0 (安全默认): 只用 RemoteAddr, X-Forwarded-For / X-Real-IP 完全不参与
	// → 攻击者无法靠伪造 header 旋转 per-IP 限速桶 key (#1108 F2).
	// N≥1: 信任 X-Forwarded-For 链最右 N 跳为可信代理, 取链中真实 client entry.
	// 各环境真实跳数由运营者在各自 .env 配 TRUSTED_PROXY_COUNT.
	TrustedProxyCount int
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

		RateLimitAuthPerSec: envInt("RATE_LIMIT_AUTH_PER_SEC", 5),
		RateLimitAuthBurst:  envInt("RATE_LIMIT_AUTH_BURST", 15),
		RateLimitUserPerSec: envInt("RATE_LIMIT_USER_PER_SEC", 20),
		RateLimitUserBurst:  envInt("RATE_LIMIT_USER_BURST", 60),
		RateLimitAnonPerSec: envInt("RATE_LIMIT_ANON_PER_SEC", 100),
		RateLimitAnonBurst:  envInt("RATE_LIMIT_ANON_BURST", 300),

		TrustedProxyCount: envInt("TRUSTED_PROXY_COUNT", 0),
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
