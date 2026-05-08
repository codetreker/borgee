# HB-1 stance checklist (战马D v1)

战马D · 2026-04-30 · 立场守门 (3+3 边界). HB-1 v0 [A] server endpoint scope.
**关联**: spec `hb-1-spec.md` v1 + acceptance + content-lock.
**承袭**: 战马A v0 spec #491 锚 + DL-4 命名拆死锚 (`pwa_manifest_test.go`).

## §0 立场 3 项

- [x] **① server endpoint owner-only Bearer api-key** — `GET /api/v1/plugin-
  manifest` 走 Bearer `<user-api-key>` 鉴权 (反向断: no auth → 401);
  admin god-mode 不挂 (反向 grep `admin-api/v[0-9]+/.*plugin-manifest` 0
  hit, ADM-0 §1.3 红线 — admin 看 audit 不直接改 manifest).
- [x] **② manifest data const slice (0 schema)** — plugins 走 server 端
  const `PluginManifestEntries` 单源, 不另起 plugins 表 / migration. 反向
  grep `migrations/hb_1_\d+|ALTER.*plugin` 0 hit; 0 schema 模式跟 RT-4 /
  DM-9 同精神. v3 升级 admin DB 表留位.
- [x] **③ 7-reason 字典字面锁 byte-identical** — `HB1ReasonOK / HB1Reason
  ManifestSignatureInvalid / HB1ReasonBinarySHA256Mismatch /
  HB1ReasonBinaryGPGInvalid / HB1ReasonManifestFetchFailed /
  HB1ReasonDiskWriteFailed / HB1ReasonUnknownPlugin` byte-identical 跟
  spec brief v0 #491 §3.3 + v1 §3.2 同源.

## §0.边界 3 项

- [x] **④ ed25519 detached signature** — response.signature non-empty 单
  测 verify 通过 (HB-1 v0 [A] 简化, sequoia/openpgp 双签 留 HB-1b Rust
  client 实施). server 持 ed25519 私钥 (env var / config).
- [x] **⑤ AL-1a reason 锁链不漂** (停在 HB-6 #19) — HB-1 7-dict 跟
  runtime AL-1a 6-dict 拆死 (反向 grep `hb1.*reason\|plugin.*reason` 在
  internal/agent/reasons/ 0 hit; install path 7-dict 跟 runtime path
  6-dict 字典分立反约束 — spec §3.2 + v0 #491 spec §3.3 立场).
- [x] **⑥ AST 锁链延伸第 23 处** — forbidden 3 token (`pendingPluginManifest
  / pluginManifestQueue / deadLetterPluginManifest`) 在 internal/api 0 hit
  (跟 BPP-4..8 + HB-3 v2 + AL-7+AL-8 + HB-5 + CHN-5..14 + DM-6/7 + HB-6
  + RT-4 同模式 AST 锁链).

## §1 测试

- [x] REG-HB1-001 server endpoint Bearer api-key + 200 + shape (`TestHB1_
  PluginManifest_Returns200_WithShape` + `_Unauthorized_NoToken_401`).
- [x] REG-HB1-002 manifest data const slice 0 schema (`TestHB1_NoSchema
  Change` + `TestHB1_PluginEntriesConstNonEmpty`).
- [x] REG-HB1-003 7-reason 字典字面锁 (`TestHB1_ReasonsByteIdentical`
  反向断 7 const string 字面 byte-identical 跟 spec).
- [x] REG-HB1-004 ed25519 signature non-empty + verify 通过
  (`TestHB1_ManifestSignatureVerify`).
- [x] REG-HB1-005 admin god-mode 不挂 (`TestHB1_NoAdminPluginManifestPath`
  filepath.Walk + regex 反向 0 hit).
- [x] REG-HB1-006 AST 锁链延伸第 23 处 (`TestHB1_NoPluginManifestQueue` AST
  scan 3 forbidden 0 hit) + DL-4 命名拆死锚不破 (`TestDL44_PWAManifest_
  NameNotPluginManifest` 既有 + 转正向 `TestHB1_PluginManifest_Returns200`).

## §2 反约束 grep 锚

- 0 schema: `migrations/hb_1_\d+|ALTER.*plugin` 0 hit.
- admin god-mode 不挂: `admin-api/v[0-9]+/.*plugin-manifest` 0 hit.
- 7-reason 字面锁: const 字符串 byte-identical 跟 spec §3.2 + v0 #491.
- DL-4 命名拆死: `pwa_manifest_test.go::TestDL44_PWAManifest_NameNotPluginManifest`
  既有反向锚不破 (HB-1 endpoint 真返 200 不 404).
- AST 锁链延伸第 23 处: 3 forbidden token 0 hit.
- AL-1a 拆死: `hb1.*reason|plugin.*reason` 在 internal/agent/reasons/ 0 hit.
