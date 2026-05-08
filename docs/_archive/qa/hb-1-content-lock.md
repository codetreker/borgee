# HB-1 content lock — server endpoint manifest shape + 7-reason 字典 (战马D v1)

战马D · 2026-04-30 · server-side `GET /api/v1/plugin-manifest` response
shape + 7-reason 字典 byte-identical 锁. **关联**: spec `hb-1-spec.md` v1
+ stance + acceptance. 跟战马A v0 #491 spec §3.2+§3.3 byte-identical 同源.

## §1 server endpoint response shape (byte-identical 跟 spec §3.1)

```json
{
  "manifest_version": 1,
  "issued_at": 1735689600000,
  "expires_at": 1735776000000,
  "signature": "<base64 ed25519 detached signature over canonical JSON>",
  "plugins": [
    {
      "id": "openclaw",
      "version": "1.0.0",
      "binary_url": "https://cdn.borgee.io/plugins/openclaw-1.0.0-linux-x64",
      "sha256": "<hex>",
      "signature": "<base64 ed25519 detached signature over binary>",
      "platforms": ["linux-x64", "darwin-x64", "darwin-arm64"]
    }
  ]
}
```

**字面锁** (vitest / Go unit 反向 grep 守):
- top-level keys byte-identical: `manifest_version` / `issued_at` /
  `expires_at` / `signature` / `plugins`
- per-plugin entry keys byte-identical: `id` / `version` / `binary_url` /
  `sha256` / `signature` / `platforms`
- `manifest_version` 字面 = 1 (int, 不字符串)
- `signature` 字段名 byte-identical (跟 战马A v0 #491 spec §3.2 `gpg_signature`
  字面分歧 — v1 简化为 `signature` 字面统一: ed25519 detached signature
  over canonical JSON / binary; HB-1b Rust client 消费同字段名)
- 反 reject 字段名漂: `gpg_signature` / `pgpSig` / `digest` / `hash` (drift
  反向 grep 守门)

## §2 7-reason 字典字面锁 (byte-identical 跟战马A v0 #491 spec §3.3)

```go
const (
    HB1ReasonOK                       = "ok"
    HB1ReasonManifestSignatureInvalid = "manifest_signature_invalid"
    HB1ReasonBinarySHA256Mismatch     = "binary_sha256_mismatch"
    HB1ReasonBinaryGPGInvalid         = "binary_gpg_invalid"
    HB1ReasonManifestFetchFailed      = "manifest_fetch_failed"
    HB1ReasonDiskWriteFailed          = "disk_write_failed"
    HB1ReasonUnknownPlugin            = "unknown_plugin"
)
```

**字面锁** (Go unit 反向 grep 守):
- 7 const 字面 byte-identical 跟 spec brief v0 #491 + v1 §3.2 同源
- `binary_gpg_invalid` 字面保留 (HB-1b Rust client 走 sequoia/openpgp 时
  消费此字面 — server 端 v0 简化 ed25519 不消费此 reason, HB-1b 真触发)
- 反 reject 同义词漂: `signature_bad` / `sha256_bad` / `manifest_bad` (字
  面单源, drift 反向 grep 守门)

## §3 manifest data const slice (0 schema 立场 ②)

```go
// PluginManifestEntries — HB-1 v0 hardcoded plugin manifest. v3 升级走
// admin DB 表留位 (跟 RT-4 / DM-9 0-schema 模式同精神). 改 = 改一处 +
// 反向 grep `migrations/hb_1_\d+|ALTER.*plugin` 0 hit 守门.
var PluginManifestEntries = []PluginEntry{
    {
        ID:          "openclaw",
        Version:     "1.0.0",
        BinaryURL:   "https://cdn.borgee.io/plugins/openclaw-1.0.0-linux-x64",
        SHA256:      "0000000000000000000000000000000000000000000000000000000000000000",
        Signature:   "", // populated at server startup via signing private key
        Platforms:   []string{"linux-x64", "darwin-x64", "darwin-arm64"},
    },
}
```

**字面锁**:
- v0 单 plugin (openclaw 占位) — v3 升级时加 admin UI / DB 表
- `BinaryURL` https-only (反向 reject http://, file://, data:) — 跟 CV-3.2
  ValidateImageLinkURL 同精神
- `SHA256` 64-char hex (v0 占位 0); 真 binary 上 CDN 后填
- `Signature` server 启动时 ed25519 签 binary 后填 (private key from env)

## §4 ed25519 signing 立场 (HB-1 v0 简化, sequoia/openpgp 留 HB-1b)

```go
// SignManifest — HB-1 v0 ed25519 detached signature over canonical JSON
// (sort keys + no whitespace). HB-1b Rust client 走 sequoia/openpgp 双签
// (binary_gpg_invalid reason 字面届时消费). v0 简化为 ed25519 单签.
func SignManifest(payload []byte, privKey ed25519.PrivateKey) []byte {
    return ed25519.Sign(privKey, payload)
}
```

**反约束**:
- v0 ed25519 单签 (server 端 holds 私钥) — 简化 boot
- HB-1b Rust client 双签 (manifest GPG + binary GPG) — sequoia/openpgp 留位
- canonical JSON 序列化锁 (sort keys + no whitespace) — 反向 reject
  pretty-print drift

## §5 admin god-mode 不挂 (反向 grep 守门)

`mux.Handle("(POST|DELETE|PATCH|PUT) /admin-api/v[0-9]+/.*plugin-manifest"`
反向 grep 0 hit (单测 filepath.Walk + regex 守门).

admin 看 audit log (`/admin-api/v[0-9]+/audit-log` 既有) 可见 `plugin_manifest_
fetch` 事件, 不直接改 manifest entries. v3 升级 admin UI 编辑表时, 走 admin
god-mode mw + admin_actions 5 字段 audit (跟 ADM-2.1 同模式).

## §6 cross-milestone 锚链 (DL-4 拆死 + HB-1b 留位)

- ✅ `pwa_manifest_test.go::TestDL44_PWAManifest_NameNotPluginManifest` 既
  有反向锚不破 (DL-4 PWA endpoint 不 squat HB-1 字面)
- ✅ 新加正向 `TestHB1_PluginManifest_Returns200` (HB-1 真挂, GET 真返 200)
- ✅ HB-1b Rust client 消费 §1 manifest shape byte-identical
- ✅ HB-4 ⭐ release gate 第 3 行 (签名校验失败率) + 第 4 行 (audit log
  schema) byte-identical 跟此 §1+§2

## §7 错码字面单源 (跟 AP-1 / CHN-13 const 同模式)

```go
const (
    HB1ErrCodeUnauthorized          = "hb1.unauthorized"
    HB1ErrCodeManifestNotFound      = "hb1.manifest_not_found"
    HB1ErrCodeSignatureFailed       = "hb1.signature_failed"
)
```

drift 守门: handler hardcoded strings byte-identical (单测 substring
asserts).
