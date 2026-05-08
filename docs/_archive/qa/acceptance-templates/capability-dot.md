# Acceptance Template — CAPABILITY-DOT (14 const snake_case → dot-notation 兑现蓝图字面)

> Spec brief `capability-dot-spec.md` (飞马 v0). Owner: 战马 实施 / 飞马 review / 烈马 验收.
>
> **CAPABILITY-DOT 范围**: 14 capability const 字符串值 snake_case → dot-notation 跟蓝图 auth-permissions.md `<domain>.<verb>` 风格 byte-identical 兑现. 0 schema column 名改 + 0 endpoint URL 改. AP-2 v1 #620 LABEL_MAP / capability-bundles.ts / content-lock byte-identical 跟随 rekey + AP-4-enum #591 reflect-lint byte-identical 不破. **audit-反转**: 旧 acceptance 草稿写 CapabilityDot UI 视觉层 (跟 spec drift), 反转为字面 rename 兑现 (跟 RT-3 / DL-3 / AP-2 / WIRE-1 audit-反转 同精神).

## 验收清单

### §1 行为不变量 (server const + DB backfill)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 `internal/auth/capabilities.go` 14 const 字符串值 byte-identical 跟蓝图 §1 字面 (channel.read / channel.write / artifact.commit / user.mention / dm.read / channel.manage_members / etc) | unit + grep | `abac_test.go::TestCapabilities_WhitelistByteIdentical` + `capabilities_lint_test.go::TestAP_ALL_OrderedByteIdentical` PASS |
| 1.2 `migrations/capabilities_dot_notation_backfill.go` v=48 14 行 per-token UPDATE 真挂 + idempotent (反复跑不破, hasColumns guard) | unit | migration registry 含 v=48 + 真跑 |
| 1.3 AP-4-enum reflect-lint byte-identical 不破 (TestAP_ALL_OrderedByteIdentical / TestAP_reflect_lint_NoOrphanConst / TestAP_IsValidCapability_TruthTable) | unit | `auth/capabilities_lint_test.go` 全 PASS |

### §2 数据契约 (0 schema column 改 + 0 endpoint URL 改)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 0 schema column rename / 0 column add — `git diff main -- packages/server-go/internal/migrations/` 反向 grep `ALTER TABLE.*capability\|RENAME COLUMN` 0 hit (user_permissions.capability TEXT 字段名不动) | git diff + grep | 反向 grep 0 hit |
| 2.2 0 endpoint URL 改 / 0 routes.go 改 — `git diff main -- packages/server-go/internal/api/server.go packages/server-go/internal/server/server.go` mux.Handle/`POST /api/v1/`/`GET /api/v1/` 0 行改 | git diff | 0 行 ✅ |

### §3 反向 grep 锚 (snake_case 14 字面 0 hit + dot-notation 14 字面 ≥1 hit)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 snake_case 14 capability 字面在 `auth/capabilities.go` + `lib/capabilities.ts` + `content-lock.md` 0 hit (除 migration backfill SQL mapping table — 用于真值 UPDATE) | grep | reverse grep PASS |
| 3.2 dot-notation 14 字面在 server const + client CAPABILITY_TOKENS + content-lock §1.1 ≥1 hit per token | grep | grep PASS |
| 3.3 admin_actions.action 枚举 (delete_channel/change_role/...) NOT in scope — 独立 enum 字段 (admin_actions 表 CHECK), 不动 | grep | admin_actions.action 字面保留 |
| 3.4 BPP semantic ops (mention_user/read_artifact/...) NOT in scope — bpp/dispatcher.go 独立 enum, 不动 | grep | dispatcher.go 字面保留 |

### §4 closure (REG + cov gate + 跨 milestone 锁)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 4.1 既有全包 unit + vitest 全绿不破 + post-#621 haystack gate 三轨过 | full test + CI | go test ./internal/auth ./internal/api ./internal/migrations 全 PASS + vitest AP-2 28 PASS |
| 4.2 立场承袭蓝图 `<domain>.<verb>` 字面 + AP-2 v1 #620 LABEL_MAP byte-identical 跟随 rekey + AP-4-enum #591 reflect-lint 不破 | inspect | spec §0 + content-lock §1.1 byte-identical |

## REG-CAPDOT-* (initial ⚪→🟢 post-impl)

- REG-CAPDOT-001 🟢 14 const 字符串值 byte-identical 跟蓝图 (`channel.read` / `channel.write` / `artifact.commit` / `user.mention` / `dm.read` / `channel.manage_members` / etc 14 字面)
- REG-CAPDOT-002 🟢 migration v=48 capabilities_dot_notation_backfill 14 行 per-token UPDATE + hasColumns idempotent guard
- REG-CAPDOT-003 🟢 AP-2 v1 #620 LABEL_MAP / CAPABILITY_TOKENS / capability-bundles.ts / content-lock §1.1 byte-identical 跟随 rekey
- REG-CAPDOT-004 🟢 AP-4-enum #591 reflect-lint byte-identical 不破 (TestAP_ALL_OrderedByteIdentical + TestAP_IsValidCapability_TruthTable PASS)
- REG-CAPDOT-005 🟢 0 schema column rename + 0 endpoint URL 改 + 0 routes.go 改 (git diff 反向断言)
- REG-CAPDOT-006 🟢 立场承袭蓝图字面单源 + 跟 NAMING-1 / REFACTOR-2 一次做干净铁律 + audit-反转 (旧 CapabilityDot UI 视觉层 → 字面 rename 兑现)

## 退出条件

- §1 (3) + §2 (2) + §3 (4) + §4 (2) 全绿 — 一票否决
- 14 字面 byte-identical + DB backfill idempotent + 0 schema column 改
- AP-2 v1 #620 LABEL_MAP / capability-bundles.ts / content-lock byte-identical 跟随
- 登记 REG-CAPDOT-001..006

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-05-01 | 烈马 | v0 — acceptance template (CapabilityDot UI 视觉层, drift from spec) |
| 2026-05-01 | 战马 | v1 audit-反转 — acceptance scope 反转为 14 const snake_case → dot-notation rename 兑现 (跟 spec brief §0+§1+§2 byte-identical, 跟 RT-3 / DL-3 / AP-2 / WIRE-1 audit-反转 同精神). REG-CAPDOT-001..006 ⚪→🟢. |
