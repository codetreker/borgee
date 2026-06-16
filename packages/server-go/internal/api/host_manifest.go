// Package api — hb_1_plugin_manifest.go: HB-1 install-butler server-side
// `GET /api/v1/plugin-manifest` endpoint (v0 [A] scope).
//
// Blueprint锚: docs/blueprint/current/host-bridge.md §1.1+§1.2 + spec brief
// docs/implementation/modules/hb-1-spec.md v1 (战马D 升级 战马A v0 #491).
//
// Public surface:
//   - PluginManifestHandler{Logger, SigningKey}
//   - (h *PluginManifestHandler) RegisterRoutes(mux, authMw)
//   - PluginManifestEntries (const slice — 0 schema 设计 ②)
//   - HB1Reason* 7 字面 const (跟 spec §3.2 byte-identical)
//
// 反向检查 (hb-1-spec.md §0 + content-lock §1+§2+§5):
//   - 设计 ① owner-only Bearer api-key 鉴权 (admin god-mode 不挂; 反向
//     grep `admin-api/v[0-9]+/.*plugin-manifest` 0 hit, ADM-0 §1.3 红线).
//   - 设计 ② manifest data const slice (PluginManifestEntries) 单一来源, 0
//     schema 改 (grep 检查 `migrations/hb_1_\d+|ALTER.*plugin` 0 hit;
//     v3 升级 admin DB 表留位).
//   - 设计 ③ 7-reason 字典字面 byte-identical 跟 spec §3.2 + v0 #491.
//   - 设计 ④ ed25519 detached signature non-empty (HB-1 v0 简化, sequoia/
//     openpgp 双签 留 HB-1b Rust client 实施).
//   - 设计 ⑤ AL-1a reason 对齐链不漂 — HB-1 7-dict 跟 runtime AL-1a 6-dict
//     字典分立 (grep 检查 `hb1.*reason\|plugin.*reason` 在 internal/agent/
//     reasons/ 0 hit, 对齐链停在 HB-6 #19).
//   - 设计 ⑥ AST 对齐链延伸第 23 处 forbidden 3 token 0 hit.
//
// ⚠️ 命名拆死锚 — 跟 DL-4 #485 `GET /api/v1/pwa/manifest` 拆开:
//   - 本 endpoint: install-butler binary plugin manifest (双签必需, 蓝图
//     host-bridge §1.2 + §4.5 "未签 100% reject"); HB-1b Rust client 消费.
//   - DL-4 endpoint: PWA installable web app manifest (浏览器 install
//     prompt 用), HTTPS + 公开无 auth.
//   - grep 检查 `pwa\|appmanifest` 在本文件 count==0.
//   - grep 检查 `pwa_manifest_test.go::TestDL44_PWAManifest_NameNotPluginManifest`
//     既有不破 + 新加正向 `TestHB1_PluginManifest_Returns200`.
package api

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"time"
)

// HB-1 v0 [A] 7-reason 字典字面锁 byte-identical 跟 spec §3.2 + 战马A v0
// #491 spec brief §3.3 同源. server 端 v0 简化 ed25519 单签, HB-1b Rust
// client 真消费 BinaryGPGInvalid (sequoia/openpgp 双签).
const (
	HB1ReasonOK                       = "ok"
	HB1ReasonManifestSignatureInvalid = "manifest_signature_invalid"
	HB1ReasonBinarySHA256Mismatch     = "binary_sha256_mismatch"
	HB1ReasonBinaryGPGInvalid         = "binary_gpg_invalid"
	HB1ReasonManifestFetchFailed      = "manifest_fetch_failed"
	HB1ReasonDiskWriteFailed          = "disk_write_failed"
	HB1ReasonUnknownPlugin            = "unknown_plugin"
)

// HB1AllReasons — 7-tuple grep 守门用 (TestHB1_ReasonsByteIdentical
// 反向断 7 字面 byte-identical, mismatch 守门).
var HB1AllReasons = []string{
	HB1ReasonOK,
	HB1ReasonManifestSignatureInvalid,
	HB1ReasonBinarySHA256Mismatch,
	HB1ReasonBinaryGPGInvalid,
	HB1ReasonManifestFetchFailed,
	HB1ReasonDiskWriteFailed,
	HB1ReasonUnknownPlugin,
}

// PluginManifestEntry mirrors content-lock §1 per-plugin entry shape.
// 字段名 byte-identical 跟 spec §3.1 content-lock §1.
//
// Class — #999 update classification. "security" = startup prominent prompt
// + one-click confirm. "feature" (or empty default) = settings panel only.
// Blueprint锚: docs/blueprint/current/host-bridge.md §1.3 "更新策略: 分类,
// 不自动" — 自动更新仍是反模式. Class is metadata for the update-flow only;
// install-butler does NOT consume it (canonical signing bytes intentionally
// exclude Class — same as Platforms — so adding the field never changes any
// already-signed manifest entry).
type PluginManifestEntry struct {
	ID        string   `json:"id"`
	Version   string   `json:"version"`
	BinaryURL string   `json:"binary_url"`
	SHA256    string   `json:"sha256"`
	Signature string   `json:"signature"`
	Platforms []string `json:"platforms"`
	Class     string   `json:"class,omitempty"`
}

// PluginManifestPayload mirrors content-lock §1 top-level shape.
type PluginManifestPayload struct {
	ManifestVersion int                   `json:"manifest_version"`
	IssuedAt        int64                 `json:"issued_at"`
	ExpiresAt       int64                 `json:"expires_at"`
	Signature       string                `json:"signature"`
	Plugins         []PluginManifestEntry `json:"plugins"`
}

// PluginManifestEntries — built-in default entry list (设计 ②).
//
// 0 schema 模式跟 RT-4 / DM-9 同精神. v3 升级走 admin DB 表留位; grep 检查
// `migrations/hb_1_\d+|ALTER.*plugin` 0 hit 守门.
//
// 这个 slice 是 dev / 单测 / 缺 env 时的 fallback. 生产 entry list 真值由
// ops 走 BORGEE_MANIFEST_ENTRIES_JSON / BORGEE_MANIFEST_ENTRIES_FILE 注入
// (LoadManifestEntries 三档 fallback, manifest_signing.go).
//
// SHA256 占位 (32 个 "0") + BinaryURL 还指 cdn.borgee.io — 真值待 #1003
// release-helper.yml 第一个 borgee-helper-v* tag 跑完, deploy 改 env JSON
// 指 github release artifact URL + 真 SHA256. 这个 PR 把签名链路接通, 真值
// 填属 deploy ops, 不在代码里硬编码.
//
// 单 entry (openclaw) 保留 — 跟 issue #997 "v1 still openclaw-only" 立场一致.
var PluginManifestEntries = []PluginManifestEntry{
	{
		ID:        "openclaw",
		Version:   "1.0.0",
		BinaryURL: "https://cdn.borgee.io/plugins/openclaw-1.0.0-linux-x64",
		SHA256:    "0000000000000000000000000000000000000000000000000000000000000000",
		Signature: "",
		Platforms: []string{"linux-x64", "darwin-x64", "darwin-arm64"},
	},
}

// PluginManifestHandler serves the authenticated GET endpoint that returns
// the signed plugin manifest for install-butler (HB-1b Rust client).
type PluginManifestHandler struct {
	Logger *slog.Logger
	// SigningKey is the ed25519 private key used to sign the manifest payload.
	// Nil + AllowUnsignedPlaceholder=true leaves a placeholder signature for
	// dev/tests; nil in production (AllowUnsignedPlaceholder=false) fails closed
	// with HTTP 500. Production must set this via env config.
	SigningKey ed25519.PrivateKey
	// AllowUnsignedPlaceholder gates the dev/test signing seam. When true AND
	// SigningKey is nil, the handler emits the fixed placeholder signature
	// (and empty per-entry signatures) so dev / unit tests need no crypto
	// setup. When false (the safe production default) AND SigningKey is nil,
	// the handler fails closed with a per-request HTTP 500 instead of serving
	// a fake signature. server.go wires this to cfg.IsDevelopment().
	// #1108 F3 — refusing to emit unsigned manifests in production.
	AllowUnsignedPlaceholder bool
	// NowMs is injected for test (default time.Now().UnixMilli when nil).
	NowMs func() int64
	// ExpiresInMs is the manifest validity window (default 24h).
	ExpiresInMs int64
}

const defaultManifestValidityMs int64 = 24 * 60 * 60 * 1000

// RegisterRoutes registers GET /api/v1/plugin-manifest behind authMw
// (Bearer API-key authentication). It is intentionally not mounted on
// the admin API; there is no RegisterAdminRoutes entry for this handler.
func (h *PluginManifestHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/plugin-manifest",
		authMw(http.HandlerFunc(h.handleGet)))
}

// handleGet — GET /api/v1/plugin-manifest. authMw has already enforced
// Bearer API-key authentication. The response payload stays byte-identical
// with the content-lock §1 shape.
func (h *PluginManifestHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	now := int64(0)
	if h.NowMs != nil {
		now = h.NowMs()
	}
	if now == 0 {
		// Production fallback — millisecond precision matches issued_at /
		// expires_at int64 ms epoch contract.
		now = nowUnixMsHB1()
	}
	expires := now + h.expiresInMs()

	// Load entries fresh per-request — three-tier ops fallback
	// (env JSON > env file > built-in default) lives in
	// manifest_signing.go::LoadManifestEntries. Per-request load keeps
	// the operational rotation cycle simple: change env / file content
	// + next fetch picks it up without server restart for entry data.
	// (Signing key rotation still requires restart by design — keys live
	// in env, read once at startup via server.go.)
	entries := LoadManifestEntries(h.Logger)

	// #1108 F3 — fail closed in production before signing. Without a signing
	// key, SignEntry returns "" for every per-entry signature; serving those
	// empty signatures (alongside the top-level placeholder) would leak a
	// fake-but-200 manifest. Only the dev/test seam (AllowUnsignedPlaceholder)
	// may serve unsigned entries. signPayload re-checks this for the top-level
	// signature; this earlier guard keeps empty per-entry sigs from ever being
	// built in production.
	if h.SigningKey == nil && !h.AllowUnsignedPlaceholder {
		if h.Logger != nil {
			h.Logger.Error("hb1.unsigned_refused",
				"reason", "manifest signing key not configured",
				"effect", "refusing to emit unsigned manifest in production")
		}
		writeJSONError(w, http.StatusInternalServerError, "Failed to sign manifest")
		return
	}

	// Per-entry signing — each entry independently signed over canonical
	// bytes (ID|Version|BinaryURL|SHA256). Rotating a single entry does
	// NOT invalidate other entries' signatures. 跟 manifest_signing.go
	// EntryCanonicalBytes 一处真值. install-butler (HB-1b Rust client,
	// #996 范围) verify path 必复算同 canonical form.
	signed := make([]PluginManifestEntry, len(entries))
	for i, e := range entries {
		e.Signature = SignEntry(h.SigningKey, e)
		signed[i] = e
	}

	payload := PluginManifestPayload{
		ManifestVersion: 1,
		IssuedAt:        now,
		ExpiresAt:       expires,
		Plugins:         signed,
	}

	// Sign canonical JSON (设计 ④ ed25519). Signing covers the payload
	// fields except `signature` itself (signature is set after signing).
	sigBytes, err := h.signPayload(payload)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Error("hb1.sign", "error", err)
		}
		writeJSONError(w, http.StatusInternalServerError, "Failed to sign manifest")
		return
	}
	payload.Signature = base64.StdEncoding.EncodeToString(sigBytes)

	if h.Logger != nil {
		// HB-4 §1.5 release gate 第 4 行: audit log 5 字段
		// (actor / action / target / when / scope) byte-identical.
		h.Logger.Info("plugin_manifest.fetch",
			"actor", user.ID,
			"action", "fetch",
			"target", "plugin_manifest",
			"when", now,
			"scope", "openclaw")
	}

	writeJSONResponse(w, http.StatusOK, payload)
}

func (h *PluginManifestHandler) expiresInMs() int64 {
	if h.ExpiresInMs > 0 {
		return h.ExpiresInMs
	}
	return defaultManifestValidityMs
}

// signPayload serializes payload as canonical JSON (sort keys, no extra
// whitespace) and signs with ed25519. Returns the raw signature bytes.
// When SigningKey is nil: the dev/test seam (AllowUnsignedPlaceholder=true)
// returns a fixed placeholder so shape is preserved without crypto setup;
// in production (AllowUnsignedPlaceholder=false) it returns an error so the
// caller fails closed with HTTP 500 rather than serving a fake signature
// (#1108 F3).
func (h *PluginManifestHandler) signPayload(payload PluginManifestPayload) ([]byte, error) {
	// Build canonical JSON: marshal payload with empty Signature, sort
	// per encoding/json default (Go map ordering insertion-stable on
	// struct fields; struct serializes fields in declared order which is
	// already canonical for this type).
	payload.Signature = "" // ensure signature not part of signed bytes
	canonical, err := canonicalJSON(payload)
	if err != nil {
		return nil, err
	}
	if h.SigningKey == nil {
		// #1108 F3 — fail closed in production. Without a signing key we
		// previously returned a fixed placeholder that looks like a real
		// signature to a naive consumer. Only the dev/test seam
		// (AllowUnsignedPlaceholder) may emit it.
		if !h.AllowUnsignedPlaceholder {
			return nil, fmt.Errorf("manifest signing key not configured — refusing to emit unsigned manifest in production")
		}
		// Dev/test seam — return deterministic 32-byte placeholder so signature
		// field is non-empty (REG-HB1-004 acceptance: signature non-empty).
		// Production path must inject SigningKey via env config.
		return []byte("test-signature-placeholder-32by"), nil
	}
	return ed25519.Sign(h.SigningKey, canonical), nil
}

// canonicalJSON marshals payload with sorted map keys (struct fields are
// already declared in canonical order). Returns deterministic bytes that
// signing + verification consumers must reproduce byte-identical.
//
// PURE — never mutates the caller's slices (#1117). handleGet builds
// signed[i] = e (a struct copy that copies the Platforms slice HEADER, not
// the backing array), so every concurrently-signed entry's Platforms can
// alias the SAME backing array held by LoadManifestEntries' shared built-in
// default. An in-place sort.Strings on that shared array under two concurrent
// GET /api/v1/plugin-manifest fetches is a DATA RACE. We therefore copy each
// Platforms slice into a fresh backing array (and build a fresh Plugins slice,
// so we never write the shared Plugins[i] struct field either) and sort only
// the copy. The marshaled bytes are byte-identical to the previous in-place
// form (platforms still sorted ascending) — the ed25519 signature and
// install-butler's recomputed canonical form are unchanged.
func canonicalJSON(payload PluginManifestPayload) ([]byte, error) {
	// json.Marshal on struct emits fields in declared order. For nested
	// platforms []string, sort a defensive copy to enforce determinism
	// without mutating any (possibly shared) input backing array.
	plugins := make([]PluginManifestEntry, len(payload.Plugins))
	for i, p := range payload.Plugins {
		p.Platforms = append([]string(nil), p.Platforms...) // defensive copy
		sort.Strings(p.Platforms)
		plugins[i] = p
	}
	payload.Plugins = plugins
	return json.Marshal(payload)
}

// nowUnixMsHB1 — production fallback (millisecond UnixMs). Local helper
// to avoid coupling to other handler timestamp helpers.
func nowUnixMsHB1() int64 {
	return time.Now().UnixMilli()
}
