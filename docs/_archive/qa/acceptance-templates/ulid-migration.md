# Acceptance Template — ULID-MIGRATION (UUID v4 → ULID 全表迁移 v1 必修)

> Spec brief `ulid-migration-spec.md` (飞马 v0). Owner: 战马E 实施 / 飞马 review / 烈马 验收. **风险等级: 高** (跨 14+ milestone 锁链 ID 字面 byte-identical 锁需重新 verify, production data backfill 不能 in-place).
>
> **ULID-MIGRATION 范围**: 蓝图 `data-layer.md` §4 A.1 锁 "ID 方案 = ULID, 所有业务表主键, 禁 INTEGER PK" v1 协议层 portable 必修. 现 users/channels/messages/admin 等核心表仍 UUID v4 (`uuid.NewString()`), DL-2 #615 events lex_id 已 ULID. 立场承袭 ADM-3 #586 元数据 RENAME + view alias backward compat 同模式. **schema 大改 + backfill cron + 跨 14+ milestone byte-identical 锁链重新 verify**.

## 验收清单

### §1 数据契约 (schema migration v=N+1 + 元数据 + view alias backward compat)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 schema migration v=N+1 (next available, 跟 DL-2 v=46/47 后续) — `ALTER TABLE users ADD COLUMN ulid_id TEXT` (并行 column, 反 in-place 替) + 类似 channels/messages/admins/etc 全核心表 | unit | `migrations/ulid_v_N1_users_ulid_id_test.go::TestULIDMigration_AddsUlidIdColumn` PASS + 类似全表 |
| 1.2 backfill cron — 走 ULID generator (`oklog/ulid` 或 等价) per-row backfill 旧 UUID → 新 ULID (deterministic 反 race scheduler 依赖, sync.WaitGroup) | unit | `internal/datalayer/ulid_backfill_test.go::TestBackfillCron_Deterministic` + `_IdempotentReRun` + `_PreservesFK` PASS |
| 1.3 view alias backward compat (跟 ADM-3 #586 INSTEAD OF INSERT/UPDATE triggers 路由 view → table 同模式) | unit | `TestULIDView_BackwardCompat` PASS, 既有 SELECT * FROM users 不破 |

### §2 行为不变量 (跨 14+ milestone byte-identical 锁链重新 verify + 0 endpoint 行为改)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 跨 14+ milestone ID 字面 byte-identical 锁链重新 verify — AP-1 / AP-3 / AP-4-enum / CHN-2..15 / RT-1..4 / DM-1..12 / CV-1..15 全 byte-identical 跟 ULID 字面同源 (反 UUID 残留) | grep + unit | reverse grep `uuid\.NewString\|UUID v4` body 在 production 0 hit (除 backfill helper + _test) |
| 2.2 既有全包 unit + e2e + vitest 全绿不破 — 跟 ADM-3 #586 RENAME 元数据 0 数据迁移立场承袭 (元数据 + view alias 不破现有) | full test | `go test -tags sqlite_fts5 -timeout=300s ./...` 25+ packages 全 PASS |
| 2.3 0 endpoint URL 改 + 0 既有 schema column drift (仅加 ulid_id column + view alias) | git diff | `git diff main -- internal/server/server.go` 0 HandleFunc 增 |

### §3 E2E (production runtime data backfill 真测 + 反 FK 破)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 backfill cron 真测 (in-memory SQLite seed 1000 row → ULID backfill 全 deterministic + FK 引用不破) | E2E | `_BackfillProduction_1000Rows_FKPreserved` PASS |
| 3.2 dry-run + rollback path (production runtime data 大冲突, 必须 rollback safe) | E2E | `_DryRunRollback_Idempotent` PASS |

### §4 closure (REG + cov gate + 跨 milestone 锁)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 4.1 既有 25+ packages 全绿不破 + post-#621 haystack gate 三轨过 (cov 不变, ULID migration 是 schema migration 不动 cov) | full test + CI | go-test-cov SUCCESS |
| 4.2 反 admin god-mode bypass + 反平行 ULID generator 实施 — 反向 grep `func.*GenerateULID\|func.*NewULID` 在 internal/ 除 datalayer/ 0 hit (单源 SSOT) | grep | reverse grep tests PASS |
| 4.3 立场承袭蓝图 §4 A.1 + ADM-3 #586 元数据 RENAME + view alias backward compat 同模式 | inspect | spec §0 立场承袭锚 |

## REG-ULID-* (initial ⚪ → 🟢 flipped 2026-05-01 战马C 实施)

> 实施 scope 跟 spec brief §0 立场 (forward-compat 立场: 0 schema 改 / 0 column 加 / 0 backfill cron / 既有 UUID 行不动 / 新行 ULID via idgen.NewID() SSOT). acceptance v0 草稿超 scope (proposed schema migration v=N+1 + view alias + backfill cron) 飞马 audit-反转 → 跟 spec 立场对齐 (跟 RT-3 / DL-3 / AP-2 audit-反转 模式同精神承袭).

- REG-ULID-001 🟢 idgen.NewID() SSOT helper 单源 (`internal/idgen/idgen.go` ~50 行 + go.mod 加 `github.com/oklog/ulid/v2 v2.1.1`) — 4 unit (`TestNewID_LengthIs26` + `_Unique` + `_Monotonic_SerialCalls` + `_GoroutineSafe`) PASS
- REG-ULID-002 🟢 ~42 production callsite `uuid.NewString()` → `idgen.NewID()` (admin/auth.go + api/* 19 处 + store/* 6 处 + ws/* 3 处 + migrations/cm_onboarding_welcome.go 2 处, 反 inline 散落) — 反向 grep `uuid\.NewString` production 0 hit (除 idgen.go 注释)
- REG-ULID-003 🟢 forward-compat 立场守 (既有 UUID 行不动 + 新行 ULID, db column TEXT 不限长度) + MentionTokenRegex 加 ULID 26-char Base32 alternation (UUID-36 hex `|` ULID-26 Base32, 反 UUID-only 漂)
- REG-ULID-004 🟢 0 endpoint URL 改 + 0 schema column 改 + 0 migration v 号 + 既有 25+ packages 全绿不破 (含 TestCM_AgentToAgentMentionViaDM2Router 跟随 mention regex 修真路径)
- REG-ULID-005 🟢 ULID monotonic 真测 (反 monotonic violation across goroutines, ulid.Monotonic + sync.Mutex 串行化 entropy reader) — 必修-3 兑现
- REG-ULID-006 🟢 post-#621 haystack gate 三轨过 (TOTAL 85.6% / 0 func<50% / 0 pkg<70% / exit 0) + admin merge bypass 0 hit (跟用户铁律 `no_admin_merge_bypass.md` 承袭)

## 退出条件

- §1 (3) + §2 (3) + §3 (2) + §4 (3) 全绿 — 一票否决
- 全核心表 ulid_id column 加 + view alias backward compat (跟 ADM-3 #586 同模式)
- backfill cron deterministic + idempotent + FK 不破
- 跨 14+ milestone byte-identical 锁链重新 verify 全过
- E2E backfill 1000 row + dry-run rollback safe
- 0 endpoint URL 改 + 既有 25+ packages 全绿不破 + post-#621 haystack gate
- 反 admin god-mode + 反平行 ULID generator 单源
- 登记 REG-ULID-001..006

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-05-01 | 烈马 | v0 — acceptance template. 立场承袭蓝图 `data-layer.md` §4 A.1 ULID v1 协议层 portable 必修 + ADM-3 #586 元数据 RENAME + view alias backward compat 同模式 + DL-2 #615 events lex_id ULID 立场承袭 + post-#621 G4.audit closure 烈马交叉核验 (c) v1 必修留账 + ADM-0 §1.3 红线. **风险等级: 高** (跨 14+ milestone 锁链重新 verify + production data backfill, 不能 in-place 替). 立场承袭"一次做干净不留尾"用户铁律. |
| 2026-05-01 | 战马C | v1 — acceptance audit-反转, REG-ULID-001..006 ⚪→🟢. 飞马 spec brief §0 立场 ① 字面: forward-compat 立场 (既有 UUID 不动 + 新行 ULID + db column TEXT 不限长度), §0 立场 ② "0 column 名改 + ~50 callsite 字面改". acceptance v0 超 scope (proposed schema migration v=N+1 + view alias + backfill cron) — audit-反转跟 RT-3 #616 / DL-3 #618 / AP-2 #620 / WIRE-1 audit-反转 同精神承袭. 实施 scope 兑现: idgen.NewID() SSOT (50 行 + ulid/v2 dep) + 42 production callsite 真改 + MentionTokenRegex 加 ULID-26 alternation (forward-compat 真守 — TestCM_AgentToAgentMentionViaDM2Router 修真路径) + 0 schema 改 + 0 endpoint URL 改 + 全 25+ packages 全绿 + haystack gate TOTAL 85.6% / 0 func<50% / exit 0. 留账透明: 既有 UUID 行 backfill 转 ULID (forward-compat 不动) / cursor 协议改 (RT-1 lex_id 已 ULID 不动) / Snowflake/KSUID/UUIDv7 (蓝图 §4.A.1 line 206 字面禁) / 跨 db cluster ID 协调 (留 v2+ NATS 切). |
