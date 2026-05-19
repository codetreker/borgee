// Package api — manifest_signing.go: HB-1 install-butler manifest
// ed25519 signing helpers (#997 follow-up to #1003 release pipeline).
//
// Blueprint锚: docs/blueprint/current/host-bridge.md §1.2 + §4.5
// "只安装 Borgee 签名 manifest 内列出的 runtime; 每个二进制走
// SHA256 + GPG 双校验. 未签 100% reject".
//
// Operational contract (跟 docs/current/host-bridge/manifest-signing.md
// 一处真值, 同步改):
//
//	env BORGEE_MANIFEST_SIGNING_KEY = base64(ed25519 seed, 32 字节)
//	env BORGEE_MANIFEST_ENTRIES_JSON = full JSON array (entry list inline)
//	env BORGEE_MANIFEST_ENTRIES_FILE = path to JSON file containing entry list
//
// 三档 fallback (entry list): env JSON > env file > 内置 default slice.
// 私钥 unset 时 fall-soft: 不签 (per-entry Signature 留空 + 顶层
// signature 走 test placeholder), 日志 warn 一次, 不 panic.
// 这个 fall-soft 是 dev 友好 (本机起 server 不必生 key), 生产由监控
// 抓 warn 行强制配齐.
//
// Per-entry signing — canonical form:
//
//	ID + "|" + Version + "|" + BinaryURL + "|" + SHA256
//
// 用单字符 "|" 分隔, byte-concat 后整 ed25519.Sign. 每个 entry 独立
// 签 — 单 entry 旋转 / SHA 改不破其他 entry 签. install-butler
// (HB-1b Rust client, #996 范围) 必须以同 canonical form 复算 verify.
package api

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
)

// 三个 env var 名 — operational contract 真值, 改名同时改
// docs/current/host-bridge/manifest-signing.md.
const (
	EnvManifestSigningKey  = "BORGEE_MANIFEST_SIGNING_KEY"
	EnvManifestEntriesJSON = "BORGEE_MANIFEST_ENTRIES_JSON"
	EnvManifestEntriesFile = "BORGEE_MANIFEST_ENTRIES_FILE"
)

// entrySigSeparator — canonical form 字段分隔符 byte-identical. install-butler
// 复算 verify 必须用同字符. ASCII "|" (0x7C) 在 URL / sha256 hex / 版本号
// 字面里都不出现, 不会跟 entry 内字面撞.
const entrySigSeparator = "|"

// LoadSigningKey reads BORGEE_MANIFEST_SIGNING_KEY (base64 ed25519 seed,
// 32 bytes after decode). Returns:
//
//   - (nil, nil) — env unset (dev fall-soft, caller leaves Signature empty)
//   - (nil, err) — env set but malformed (caller logs + falls back to dev mode)
//   - (priv, nil) — env set + valid
//
// 不 panic — 启动期不破 server (manifest endpoint 是一个可选功能, 不应阻塞
// 整 server 启动. 真值缺失监控通过日志 warn 抓).
func LoadSigningKey(logger *slog.Logger) (ed25519.PrivateKey, error) {
	raw := os.Getenv(EnvManifestSigningKey)
	if raw == "" {
		if logger != nil {
			logger.Warn("manifest_signing.key_unset",
				"env", EnvManifestSigningKey,
				"effect", "manifest entries served unsigned (Signature=\"\"); install-butler will reject in production")
		}
		return nil, nil
	}
	seed, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("manifest signing key base64 decode: %w", err)
	}
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("manifest signing key seed length: got %d, want %d", len(seed), ed25519.SeedSize)
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

// EntryCanonicalBytes returns the byte sequence that gets signed for a single
// manifest entry. install-butler (HB-1b Rust client, #996) MUST reproduce
// this exact byte sequence to verify. Documented in code + ops doc.
//
// Canonical form: ID + "|" + Version + "|" + BinaryURL + "|" + SHA256
// Platforms field is intentionally excluded from per-entry signature —
// platforms is metadata for client filtering, not security-relevant.
// (Top-level payload signature still covers the whole entry incl. platforms.)
func EntryCanonicalBytes(e PluginManifestEntry) []byte {
	return []byte(e.ID + entrySigSeparator + e.Version + entrySigSeparator + e.BinaryURL + entrySigSeparator + e.SHA256)
}

// SignEntry computes the per-entry ed25519 signature and returns it
// base64-encoded (StdEncoding, matching top-level payload signature
// convention). Returns "" when key is nil (dev fall-soft path).
func SignEntry(key ed25519.PrivateKey, e PluginManifestEntry) string {
	if key == nil {
		return ""
	}
	sig := ed25519.Sign(key, EntryCanonicalBytes(e))
	return base64.StdEncoding.EncodeToString(sig)
}

// VerifyEntry decodes a base64-encoded entry signature and verifies it
// against the canonical entry bytes. Returns true iff the signature matches.
// Used by tests in this repo + documented as the byte-identical algorithm
// that install-butler must implement on the Rust client.
func VerifyEntry(pub ed25519.PublicKey, e PluginManifestEntry, sigB64 string) bool {
	if pub == nil || sigB64 == "" {
		return false
	}
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return false
	}
	return ed25519.Verify(pub, EntryCanonicalBytes(e), sig)
}

// LoadManifestEntries returns the manifest entry list with three-tier
// fallback: env JSON > env file > built-in default slice
// (PluginManifestEntries).
//
// On malformed env / unreadable file, logs error and falls back to default
// (fail-soft — keeps endpoint serving so client side never sees an HTTP
// 500 due to ops config typo).
func LoadManifestEntries(logger *slog.Logger) []PluginManifestEntry {
	if raw := os.Getenv(EnvManifestEntriesJSON); raw != "" {
		entries, err := parseEntriesJSON([]byte(raw))
		if err == nil {
			return entries
		}
		if logger != nil {
			logger.Error("manifest_signing.entries_env_invalid",
				"env", EnvManifestEntriesJSON, "error", err.Error())
		}
	}
	if path := os.Getenv(EnvManifestEntriesFile); path != "" {
		data, err := os.ReadFile(path) // #nosec G304 — ops-config path
		if err == nil {
			entries, perr := parseEntriesJSON(data)
			if perr == nil {
				return entries
			}
			if logger != nil {
				logger.Error("manifest_signing.entries_file_parse",
					"env", EnvManifestEntriesFile, "path", path, "error", perr.Error())
			}
		} else if logger != nil {
			logger.Error("manifest_signing.entries_file_read",
				"env", EnvManifestEntriesFile, "path", path, "error", err.Error())
		}
	}
	return PluginManifestEntries
}

func parseEntriesJSON(data []byte) ([]PluginManifestEntry, error) {
	var entries []PluginManifestEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, errors.New("entry list is empty")
	}
	for i, e := range entries {
		if e.ID == "" || e.Version == "" || e.BinaryURL == "" {
			return nil, fmt.Errorf("entry[%d] missing required field (id/version/binary_url)", i)
		}
	}
	return entries, nil
}
