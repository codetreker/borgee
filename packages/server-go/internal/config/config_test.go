package config

import (
	"os"
	"strings"
	"testing"
)

func TestConfigIsDevelopment(t *testing.T) {
	cfg := &Config{NodeEnv: "development"}
	if !cfg.IsDevelopment() {
		t.Fatal("expected development")
	}

	cfg2 := &Config{NodeEnv: "production"}
	if cfg2.IsDevelopment() {
		t.Fatal("expected not development")
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := &Config{NodeEnv: "production", JWTSecret: ""}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing JWT secret in production")
	}

	cfg2 := &Config{NodeEnv: "production", JWTSecret: "secret", CORSOrigin: "https://example.com"}
	if err := cfg2.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg3 := &Config{NodeEnv: "development", JWTSecret: ""}
	if err := cfg3.Validate(); err != nil {
		t.Fatalf("unexpected error in dev: %v", err)
	}

	// REG-NHD-002 — no-hardcoded-domain: CORS_ORIGIN required in production
	// (反 silent prod default `https://borgee.codetrek.cn`).
	cfg4 := &Config{NodeEnv: "production", JWTSecret: "secret", CORSOrigin: ""}
	if err := cfg4.Validate(); err == nil {
		t.Fatal("expected error for missing CORS_ORIGIN in production")
	} else if !strings.Contains(err.Error(), "CORS_ORIGIN env required") {
		t.Errorf("error msg should mention CORS_ORIGIN env required; got %v", err)
	}

	// REG-NHD-002b — dev path: CORS_ORIGIN empty is fine.
	cfg5 := &Config{NodeEnv: "development", JWTSecret: "", CORSOrigin: ""}
	if err := cfg5.Validate(); err != nil {
		t.Fatalf("dev should allow empty CORS_ORIGIN; got %v", err)
	}
}

func TestConfigLogLevel(t *testing.T) {
	tests := []struct {
		level    string
		expected string
	}{
		{"debug", "DEBUG"},
		{"warn", "WARN"},
		{"warning", "WARN"},
		{"error", "ERROR"},
		{"info", "INFO"},
		{"", "INFO"},
	}
	for _, tt := range tests {
		cfg := &Config{LogLevelStr: tt.level}
		got := cfg.LogLevel().String()
		if got != tt.expected {
			t.Errorf("LogLevel(%q) = %s, want %s", tt.level, got, tt.expected)
		}
	}
}

func TestConfigLoad(t *testing.T) {
	os.Setenv("NODE_ENV", "development")
	os.Setenv("PORT", "5000")
	os.Setenv("ADMIN_USER", "root")
	os.Setenv("SQLITE_TXLOCK", "immediate")
	t.Cleanup(func() {
		os.Unsetenv("NODE_ENV")
		os.Unsetenv("PORT")
		os.Unsetenv("ADMIN_USER")
		os.Unsetenv("SQLITE_TXLOCK")
	})

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 5000 {
		t.Fatalf("expected port 5000, got %d", cfg.Port)
	}
	if cfg.JWTSecret != "dev-secret" {
		t.Fatalf("expected dev-secret, got %s", cfg.JWTSecret)
	}
	if cfg.AdminUser != "root" {
		t.Fatalf("expected admin user root, got %s", cfg.AdminUser)
	}
	if cfg.SQLiteTxLock != "immediate" {
		t.Fatalf("expected sqlite txlock immediate, got %q", cfg.SQLiteTxLock)
	}
}

func TestEnvHelpers(t *testing.T) {
	os.Setenv("TEST_STR", "hello")
	os.Setenv("TEST_INT", "42")
	os.Setenv("TEST_BOOL", "true")
	t.Cleanup(func() {
		os.Unsetenv("TEST_STR")
		os.Unsetenv("TEST_INT")
		os.Unsetenv("TEST_BOOL")
	})

	if envStr("TEST_STR", "def") != "hello" {
		t.Fatal("expected hello")
	}
	if envStr("MISSING", "def") != "def" {
		t.Fatal("expected def")
	}

	if envInt("TEST_INT", 0) != 42 {
		t.Fatal("expected 42")
	}
	if envInt("MISSING", 99) != 99 {
		t.Fatal("expected 99")
	}

	if !envBool("TEST_BOOL", false) {
		t.Fatal("expected true")
	}
	if envBool("MISSING", false) {
		t.Fatal("expected false")
	}
}
