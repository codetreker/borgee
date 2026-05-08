# Acceptance Template — HB-3: host_grants schema SSOT + 情境化授权弹窗 UX

> 蓝图: `host-bridge.md` §1.3 (情境化授权 4 类 + 弹窗 UX 字面) + §1.5 release gate 第 5 行 (撤销 grant → daemon < 100ms 拒绝) + §2 信任五支柱第 3 条 (可审计日志 schema 锁定)
> Spec: `docs/implementation/modules/hb-3-spec.md` (战马A v0 9b97809, 3 立场 + 3 拆段 + 6 grep 反查 + 6 反约束)
> Stance: `docs/qa/hb-3-stance-checklist.md` (战马A v0, 3 立场 + 5 蓝图边界)
> 文案锁: `docs/qa/hb-3-content-lock.md` (战马A v0, 弹窗三按钮 DOM 字面 + 反向同义词禁词)
> 拆 PR: **HB-3 整 milestone 一 PR** (新协议 #479): `feat/hb-3` 三段一次合
> Owner: 战马A (实施) / 飞马 review / 烈马 验收

## 验收清单

### §1 HB-3.1 — schema migration v=26 + REST CRUD endpoints

> 锚: spec §1 BPP-3.1 + stance §1+§2+§3 守 + content-lock §1.①+§1.②

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 host_grants 表 9 列 (id PK/user_id NOT NULL/agent_id NULL/grant_type CHECK 4-enum/scope TEXT JSON/ttl_kind CHECK 2-enum/granted_at NOT NULL/expires_at NULL/revoked_at NULL) + idx_user_id + idx_agent_id; v=26 顺延 al_1_4 #492 v=25 | unit (migration test + schema 反断) | 战马A / 烈马 | `internal/migrations/hb_3_1_host_grants_test.go::TestHB3_1_TableShape` (9 列 + CHECK constraint + 2 idx) |
| 1.2 grant_type 4-enum byte-identical 跟蓝图 §1.3 字面 (install/exec/filesystem/network); ttl_kind 2-enum (one_shot/always) | unit (CHECK reject 枚举外值) | 战马A / 烈马 | `TestHB3_1_GrantTypeEnum_Reject` (插 'admin'/'sudo' 等枚举外值 → CHECK fail) |
| 1.3 REST GET/POST/DELETE `/api/v1/host-grants` (owner-only ACL anchor #360 同模式) + cross-user 403 reject | E2E (real REST + token) | 战马A / 烈马 | `internal/api/host_grants_test.go::TestHB3_REST_OwnerOnly_403` (cross-user POST/DELETE → 403; same-user 200) |
| 1.4 撤销 grant **< 100ms** (REST DELETE → revoked_at NOT NULL + daemon 每次 IPC 重查, 不缓存); HB-4 §1.5 release gate 第 5 行 真测 | scenario test (clock + daemon mock SELECT) | 战马A / 烈马 / 野马 | `host_grants_test.go::TestHB3_RevokeWithin100ms` (mock daemon SELECT, DELETE → next read returns revoked_at) |
| 1.5 audit log 5 字段 byte-identical 跟 BPP-4 dead-letter + HB-1/HB-2 audit; reflect 锁 | unit (reflect + cross-file grep) | 战马A / 飞马 / 烈马 | `host_grants_test.go::TestHB3_AuditSchema5FieldsByteIdentical` (struct 字段名 + JSON tag 跟 BPP-4 DeadLetterAuditEntry 同源) |
| 1.6 反约束 — 不复用 AP-1 user_permissions schema (字典分立) | CI grep | 飞马 / 烈马 | 反向 grep `host_grants.*JOIN.*user_permissions\|grants.*INSERT.*user_permissions` count==0 |
| 1.7 反约束 — best-effort 立场承袭 BPP-4/5 (AST scan grant-queue 类 0 hit) | unit (AST scan, 跟 BPP-4 dead_letter_test 同模式) | 飞马 / 烈马 | `host_grants_test.go::TestHB3_NoGrantQueueInAPIPackage` (AST forbidden tokens: pendingGrants/grantQueue/deadLetterGrants) |

### §2 HB-3.2 — daemon 读路径合约 (HB-2 spec §3.2 cross-ref 锁)

> 锚: spec §1 HB-3.2 (无 server-go 代码改, 仅 contract 锁); 跨 PR drift 防御跟 DL-4↔HB-1 anchor 8a35589 同模式

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 HB-2 spec §3.2 既有合约 byte-identical 跟 HB-3.1 schema (`scope, ttl_until` SELECT 字段) — daemon 不另起 mapper | doc cross-ref | 战马A / 飞马 | `docs/implementation/modules/hb-2-spec.md` §3.2 SELECT SQL 字面 == hb_3_1_host_grants migration 字面 (改 = 改两处) |
| 2.2 daemon 路径反向断言 — HB-2 Rust crate `packages/host-bridge/` 反向 grep `host_grants.*INSERT\|host_grants.*UPDATE` count==0 (待 HB-2 真实施时落地; 本 PR 仅锁 contract) | CI grep (deferred) | 飞马 / 烈马 | HB-2 实施 PR 加 CI lint, 跨 PR drift 守 |

### §3 HB-3.3 — client SPA HostGrantsPanel + e2e + closure

> 锚: spec §1 HB-3.3 + content-lock §1+§2 弹窗三按钮字面

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 弹窗三按钮 DOM data-action byte-identical 跟 content-lock §1 (`deny`/`grant_one_shot`/`grant_always`); 弹窗 title + body 字面 byte-identical 跟蓝图 §1.3 | client vitest + e2e | 战马A / 野马 / 烈马 | `packages/client/src/__tests__/hb-3-content-lock.test.ts` (DOM attr 反断 + 同义词 0 hit) |
| 3.2 e2e: 弹窗触发 → 选 "仅这一次" → grants insert ttl_kind='one_shot' + expires_at=now+1h; 选 "始终允许" → ttl_kind='always' + expires_at NULL | E2E (Playwright) | 战马A / 烈马 | `packages/e2e/tests/hb-3-grants.spec.ts::test_one_shot_vs_always` |
| 3.3 e2e: 撤销 grant → daemon read 反断 < 100ms (跟 §1.4 真测同链; client SPA 触发 DELETE + 立即触发 daemon mock IPC 验 revoked_at) | E2E + daemon mock | 战马A / 烈马 / 野马 | `hb-3-grants.spec.ts::test_revoke_within_100ms` |
| 3.4 REG-HB3-001..009 (1 schema + 7 server REST + 3 client e2e — 部分合并) 全 🟢 + acceptance template 翻 ✅ + PROGRESS [x] | docs flip + CI | 飞马 / 烈马 | `docs/qa/regression-registry.md` REG-HB3-* + `docs/implementation/PROGRESS.md` HB-3 [x] |

## 边界 (跟其他 milestone 关系)

| Milestone | 关系 | 字面承袭 |
|---|---|---|
| HB-1 #491 spec | install audit log 5 字段同源 (待 HB-1 真实施落地) | actor/action/target/when/scope byte-identical |
| HB-2 #491 spec §3.2 | daemon 读路径合约 cross-ref (read-only consumer) | SQL SELECT 字段字面同源 |
| BPP-4 #499 | dead-letter audit schema 5 字段同源 (HB-3 是第 4 处单测锁链) | `bpp.frame_dropped_plugin_offline` audit struct 字面 |
| BPP-5 #501 | best-effort 立场 + AST scan 锁链延伸 (forbidden tokens 加 pendingGrants/grantQueue/deadLetterGrants) | TestBPP4_NoRetryQueueInBPPPackage + TestBPP5_NoReconnectQueueInBPPPackage 锁链同源 |
| AP-1 #493 user_permissions | **字典分立** (host vs runtime), 不复用 schema | grant_type 4-enum 跟 permission 4 域字面集不交 |
| HB-4 ⭐ release gate | 第 4 行 audit schema 锁 + 第 5 行 撤销 < 100ms 真测 | gate 数字 byte-identical |
| ADM-0 §1.3 | admin god-mode 不入 grant (用户主权) | 字面立场反断 |

## 退出条件

- §1 schema+REST (7) + §2 daemon contract (2) + §3 client+e2e (4) **全 🟢**
- audit schema 跨四 milestone (HB-1 + HB-2 + BPP-4 + HB-3) byte-identical 不漂 (改 = 改四处单测锁)
- AST scan `pendingGrants/grantQueue/deadLetterGrants` 0 hit (BPP-4 + BPP-5 best-effort 锁链延伸第 3 处)
- 撤销 grant < 100ms 真测 PASS (HB-4 §1.5 release gate 第 5 行)
- REG-HB3-001..009 全 🟢 + PROGRESS HB-3 [x]
