# schema_migrations 框架 — 现状

> Phase 0 / INFRA-1a 引入。Blueprint: data-layer §3.2 forward-only versioned migrations。

## 1. 两套并行机制 (v0 过渡期)

server-go 启动时按以下顺序跑数据库初始化:

```
store.Open(cfg.DatabasePath)        # 打开 sqlite + WAL + FK ON
store.Migrate()                     # 旧的 big-bang: createSchema + applyColumnMigrations + backfill*
migrations.Default(db).Run(0)       # INFRA-1a: 版本化迁移引擎, 跑所有 Pending
```

**为什么并行**: v0 不删旧 schema, 但 Phase 1+ 所有新 schema 改动都进 `internal/migrations/registry.go` 的 `All` 列表, 不再继续往 `createSchema` 里塞 DDL。这给了 v1 切换时一个清晰的"形迁分裂点"。

## 2. 表结构

```sql
CREATE TABLE schema_migrations (
  version    INTEGER PRIMARY KEY,
  applied_at INTEGER NOT NULL,
  name       TEXT NOT NULL
);
```

每条已 apply 的迁移留一行。Engine 启动时读这张表算出 Pending。

## 3. 编写约束

- `Version` 严格递增正整数, 不可重用 / 不可重排。
- 一旦 migration 进 main, **body 不可再编辑**, 改 schema = 追加新 migration。
- 没有 `Down()`。v0 出错"删库重建"; v1 靠 backup restore (见 README §阶段策略)。
- 每条 migration 跑在独立 transaction 内。失败回滚, **不会**写 `schema_migrations` 行。

## 4. CLI

```
borgee-migrate up                # 跑全部 pending
borgee-migrate up --target 5     # 跑到 version 5 为止
borgee-migrate status            # applied vs pending
```

代码: `cmd/migrate/main.go`。

## 5. Phase 0 验收 (G0.1)

- 数据契约: `schema_migrations(version INT PK, applied_at INT, name TEXT)` 存在。
- E2E: 跑一条 `_migrations_marker` 假迁移 (registry.go version=1), `schema_migrations` 多 1 行。
- 单测: `internal/migrations/migrations_test.go` (≥80%, 覆盖 ascending / 幂等 / target / rollback / 重复版本 / 校验)。
- Seed 契约: `internal/migrations/testdata/infra-1a/seed.sql` (Phase 0 留空, Phase 1+ 按需填)。

## 6. 与旧 Store.Migrate() 的迁移路径

Phase 1 CM-1 (organizations 表) **必须** 走新引擎, 不进 `createSchema`。`Store.Migrate()` 内部 backfill 函数在 v1 切换前评估迁出。

## 7. 已注册迁移清单

| version | name | 来源 | 备注 |
|---|---|---|---|
| 1 | `infra_1a_dummy_marker` | Phase 0 / INFRA-1a | 建 `_migrations_marker(version, note)`, 端到端验证引擎。 |
| 2 | `cm_1_1_organizations` | Phase 1 / CM-1.1 | 见下节。 |
| 3 | `cm_4_0_agent_invitations` | Phase 2 / CM-4.0 | `agent_invitations` 表 + 状态机, 见 internal/store/agent_invitation.go。 |
| 4 | `adm_0_1_admins` | Phase 2 / ADM-0.1 | 见下节 (admin 独立表 + env bootstrap)。 |
| 5 | `adm_0_2_admin_sessions` | Phase 2 / ADM-0.2 | 见下节 (server-side admin session token 表)。 |
| 7 | `cm_onboarding_welcome` | Phase 2 / CM-onboarding | 见下节 (messages.quick_action + system user seed + #welcome backfill)。 |
| 8 | `ap_0_bis_message_read` | Phase 2 / AP-0-bis | 见下节 (legacy agent message.read 默认权限回填)。 |
| 9 | `cm_3_org_id_backfill` | Phase 1 / CM-3 后置 | 4 资源表 org_id 回填 (PR #208)。 |
| 10 | `adm_0_3_users_role_collapse` | Phase 2 / ADM-0.3 | `users.role` enum 收 → {member, agent} + 4 步 admin backfill 顺序锁。 |

### 7.1 v2 — `cm_1_1_organizations`

Blueprint: concept-model.md §1.1 + §2 (1 person = 1 org, UI 永久不暴露; 数据层 org first-class)。

DDL:

```sql
CREATE TABLE IF NOT EXISTS organizations (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  created_at INTEGER NOT NULL
);

ALTER TABLE users           ADD COLUMN org_id TEXT NOT NULL DEFAULT '';
ALTER TABLE channels        ADD COLUMN org_id TEXT NOT NULL DEFAULT '';
ALTER TABLE messages        ADD COLUMN org_id TEXT NOT NULL DEFAULT '';
ALTER TABLE workspace_files ADD COLUMN org_id TEXT NOT NULL DEFAULT '';
ALTER TABLE remote_nodes    ADD COLUMN org_id TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_users_org_id           ON users(org_id);
CREATE INDEX IF NOT EXISTS idx_channels_org_id        ON channels(org_id);
CREATE INDEX IF NOT EXISTS idx_messages_org_id        ON messages(org_id);
CREATE INDEX IF NOT EXISTS idx_workspace_files_org_id ON workspace_files(org_id);
CREATE INDEX IF NOT EXISTS idx_remote_nodes_org_id    ON remote_nodes(org_id);
```

**v0 stance — `NOT NULL DEFAULT ''`**: 现有行直接拿到空串, 不做 backfill。CM-1.2 会让注册流程开始写真值; v0 dev DB 可"删库重建", 不再回填历史空串行。这条 v0 债已在 `docs/implementation/README.md` audit 表登记 (`organizations 表` + `users.org_id NOT NULL`)。

**v1 切换路径**: 单独 PR — 先一次性 backfill `org_id`, 再追加新 migration 把 default 拿掉 (forward-only, 不改 v2 body)。

**资源表覆盖**: CM-3 将开始基于 `org_id` 直查这五张表 (channels / messages / workspace_files / remote_nodes), 故索引在 CM-1.1 一并落地, 避免 Phase 2 切流时再补索引。`users.org_id` 用于 CM-1.3 admin stats GROUP BY 与 CM-1.2 注册流程 owner→org 关联。

### 7.2 v4 — `adm_0_1_admins`

Blueprint: admin-model.md §1.2 (B env bootstrap, 无 promote) + §3 (admins 独立表)。R3 PR #188 / implementation R3 PR #189 锁定。

DDL (4 字段, 不准多, 锁死):

```sql
CREATE TABLE IF NOT EXISTS admins (
  id            TEXT PRIMARY KEY,
  login         TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at    INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_admins_login ON admins(login);
```

**红线 (review checklist §ADM-0.1)**: `admins` 表**不准**多 `org_id` / `role` / `is_admin` / `email` 字段。admin 不在任何 org, 不通过 promote 产生 (与 user/org 模型完全分裂)。

**Bootstrap 路径**: `cmd/collab/main.go` 启动时调 `internal/admin.Bootstrap(db)`, 读 env `BORGEE_ADMIN_LOGIN` / `BORGEE_ADMIN_PASSWORD_HASH` (字面锁), 走 `INSERT … ON CONFLICT(login) DO NOTHING` 落第一个 admin。env 缺任一 → fail-loud panic。bcrypt cost < 10 / 非 bcrypt 串 → fail-loud panic。

**Cookie name (字面锁)**: `borgee_admin_session` — 见 `internal/admin/auth.go::CookieName` const。改名 = 红线。

**Auth path 隔离**: `internal/admin/` 包**禁止 import** `internal/auth/`。grep 命中 0, 单测 (`internal/admin/auth_test.go::TestLogin_1E_ConstantTimeCompare`) 自动 enforce。

**双轨并存 (ADM-0.1 阶段)**: 老的 `users.role='admin'` 仍可经由 `internal/api/admin_auth.go` (旧 `borgee_admin_token` cookie) 走 `/admin-api/v1/*`。新路径走 `/admin-api/auth/login` + `borgee_admin_session` cookie。两路 cookie 名互不识别, ADM-0.2 切 cookie 时单边收尾。

### 7.3 v5 — `adm_0_2_admin_sessions`

Blueprint: review checklist §ADM-0.2 §1 (cookie 值不能是 admin id, 必须服务端反查的不可猜 token)。

DDL:

```sql
CREATE TABLE IF NOT EXISTS admin_sessions (
  token      TEXT PRIMARY KEY,
  admin_id   TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_admin_sessions_admin_id  ON admin_sessions(admin_id);
CREATE INDEX IF NOT EXISTS idx_admin_sessions_expires_at ON admin_sessions(expires_at);
```

**红线**:
- `token` = 32 字节 `crypto/rand` hex (64 hex chars), **不可**是 admin id / email / login / 任何可枚举值。
- `internal/admin/auth.go::ResolveSession` 是 cookie → admin 的唯一通路。改字段名 = 红线。
- ADM-0.2 同时砍掉 `RegisterAppRoutes` (旧 `/api/v1/admin/*` god-mode 挂载) 与 `auth.RequirePermission` 里的 `users.role == "admin"` 短路 — 双轨彻底分离。

**ADM-0.2 收尾后还活着的 admin code**:
`internal/admin/{auth.go, middleware.go, handlers.go, bootstrap.go}` + 本迁移。`internal/api/admin_auth.go` / `admin_e2e_test.go` 被删除, `internal/api/admin.go::RegisterAppRoutes` 移除。

> v=6 originally reserved for ADM-0.3 — slot skipped after CM-onboarding (v=7) / AP-0-bis (v=8) / CM-3 (v=9) landed sequentially. ADM-0.3 took v=10 to keep the registry strictly increasing.

### 7.4 v7 — `cm_onboarding_welcome`

Blueprint: concept-model.md §10 + onboarding-journey.md §4 (野马 v1, R2 merged).

DDL + seed + backfill (跑在单事务):

```sql
ALTER TABLE messages ADD COLUMN quick_action TEXT;  -- JSON {kind,label,action}, NULL by default

-- system user seed (FK target for sender_id='system'); idempotent
INSERT OR IGNORE INTO users (id, display_name, role, created_at, disabled, require_mention, org_id)
VALUES ('system', 'System', 'system', strftime('%s','now')*1000, 1, 0, '');

-- per-user #welcome backfill (existing users 无 welcome → 建 channel + member + system message)
-- 实现见 internal/migrations/cm_onboarding_welcome.go (含 hasColumns 探针, 单测 scaffold 安全)
```

**红线**:
- `quick_action` JSON schema v0 锁 `{"kind":"button","label":string,"action":string}` — 改 schema = 追加 v=N+1 而非改 v=7。
- `system` user disabled=1, role='system' (非 'admin' 非 'agent') — 不会进 RequirePermission gate, 不计 admin stats。
- 后续 register / admin createUser 由 handler 自动调 `store.CreateWelcomeChannelForUser` 建 #welcome (见 api/auth.go + api/admin.go)。

**bug-030 后置 fix (PR #203 commit 22ed221)**: `ListChannelsWithUnread` WHERE 子句加 `c.type IN ('channel','system')` + `channel_members` LEFT JOIN gate, 确保本人能看 #welcome 但其他人看不到。回归 row REG-INV-003。

### 7.5 v8 — `ap_0_bis_message_read`

Blueprint: ap-0-bis.md §数据契约 + R3 Decision #1 (default capability set 锁 `[message.send, message.read]`).

DDL + backfill:

```sql
INSERT INTO user_permissions (user_id, permission, scope, granted_at)
SELECT u.id, 'message.read', '*', CAST(strftime('%s','now') AS INTEGER) * 1000
FROM users u
WHERE u.role = 'agent'
  AND u.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM user_permissions p
    WHERE p.user_id = u.id
      AND p.permission = 'message.read'
      AND (p.scope = '*' OR p.scope = '*')
  );
```

**红线**:
- 范围严格: `role='agent' AND deleted_at IS NULL` — 不污染 member / admin / 软删 agent (回归 row REG-AP0B-002 + 单测 `TestAP0Bis_SkipsNonAgentRoles` / `_SkipsSoftDeletedAgents`)。
- 幂等: `NOT EXISTS` 子句保护重复跑不再插 (`TestAP0Bis_Idempotent`); v0 forward-only 契约下替代 Down 回滚断言。
- 新 agent 走 `store.GrantDefaultPermissions` (queries.go:375), 默认两行 `(message.send, *) + (message.read, *)`; 历史 agent 走本迁移 backfill。两条路径合流 = 全 agent 默认含 `message.read`。

**配套测试 (生效证据)**:
- `internal/api/messages_perm_test.go::TestGetMessages_LegacyAgentNoReadPerm_403` — 反向 (SeedLegacyAgent 只授 message.send → GET messages 403)
- `internal/api/messages_perm_test.go::TestGetMessages_AgentWithReadPerm_200` — 正向 (默认 agent 含 read → 200)
- EXPLAIN: hot-path `WHERE user_id=?` 走 `idx_user_permissions_lookup`, 无 SCAN (见 docs/implementation/00-foundation/g1-audit.md §3.5)。

### 7.6 v9 — `cm_3_org_id_backfill`

Blueprint: `docs/qa/cm-3-resource-ownership-checklist.md` (PR #200, 野马). CM-1.1 (v=2) added the `org_id` columns + indexes; v=9 backfills legacy rows on 4 resource tables (`channels`, `messages`, `workspace_files`, `remote_nodes`) via `org_id <- users.org_id` joined on creator/sender/uploader FK. Idempotent + trimmed-schema tolerant via `hasTable`/`hasColumn` PRAGMA introspection. v0 leaves un-stampable rows at `org_id=''` (CrossOrg returns false → falls through to membership checks); v1 will hard-flip NOT NULL.

### 7.7 v10 — `adm_0_3_users_role_collapse`

Blueprint: `docs/implementation/modules/adm-0-review-checklist.md` §ADM-0.3. Collapses `users.role` enum to `{'member', 'agent'}` and seals user-rail / admin-rail split. **4-step admin backfill 顺序锁** in single transaction:
1. `INSERT INTO admins (login, …) SELECT … FROM users WHERE role='admin' ON CONFLICT(login) DO NOTHING`
2. `DELETE FROM sessions WHERE user_id IN (SELECT id FROM users WHERE role='admin')`
3. `DELETE FROM user_permissions WHERE user_id IN (…)` (ADM-0.2 wildcard sweep)
4. `DELETE FROM users WHERE role='admin'`

SQLite cannot `ADD CHECK` post-create — covered by data-invariant + 5 reverse-assertion tests (§3.A–§3.G); v1 hard-flip via `CREATE+RENAME` deferred. Post-migration, admin authority lives exclusively on `/admin-api/*` behind `borgee_admin_session`; AP-0 default `(*, *)` carries human-member authority on the user rail.

### 7.8 ADM-0.3 红线

- **4 步顺序锁** (单事务, 顺序不可调): admins INSERT → sessions DELETE → user_permissions DELETE → users DELETE。任意 step 失败整条回滚, 不得插入"半 admin"中间态。
- **ON CONFLICT(login) DO NOTHING**: bootstrap (`internal/admin/bootstrap.go`) 已先建过相同 login 的 admin → 迁移不得覆盖, 跳过即可。改成 DO UPDATE = 红线。
- **`users.role` enum 收紧**: 字面只剩 `{'member', 'agent'}`。v0 用数据 invariant + 反向单测兜; **v1 hard-flip via `CREATE TABLE ... CHECK + RENAME`** 留 backlog (SQLite 不支持 post-create ADD CONSTRAINT)。
- **Regression registry**: `internal/testutil/regression_suite/` 已注册 5 反向断言 (§3.A–§3.G), 任意未来迁移破坏 admin 拆表 invariant 立即红。

### 7.9 v11 — `chn_1_1_channels_org_scoped`

Blueprint: `docs/blueprint/current/channel-model.md` §1.1 (Channel = 协作场) + §2 关键不变量 (Channel 跨 org 共享 / 创建者归属) + `concept-model.md` §1.2 (agent = 同事, 默认沉默)。Phase 3 第一波, PR 拆分文档 #265。

**What changes**:
1. **Pre-flight dup detection**: `SELECT COUNT(*) GROUP BY (org_id, name) HAVING cnt > 1`. Cross-org historic dup → hard-fail with row list, **no auto-rename** (CHN-1 spec: 历史行人工 audit, 防丢历史)。
2. **channels rebuild**: drop inline `UNIQUE(name)` via `CREATE channels_new + COPY + DROP + RENAME` (SQLite 不支持 DROP CONSTRAINT)。captured user indexes via `sqlite_master.sql IS NOT NULL` reapplied (autoindex 不回填)。
3. `channels.archived_at INTEGER NULL` 加列 (蓝图反约束: archive 不删)。
4. `CREATE UNIQUE INDEX idx_channels_org_id_name ON channels(org_id, name) WHERE deleted_at IS NULL` — 跨 org 同名合法, 同 org 同名拒, 软删行不占名。
5. `channel_members.silent INTEGER NOT NULL DEFAULT 0` + `channel_members.org_id_at_join TEXT NOT NULL DEFAULT ''` (audit snapshot)。
6. **Backfill**: `silent = 1 WHERE user_id IN (SELECT id FROM users WHERE role='agent')` — agent 默认沉默 (蓝图 §1.2 立场); `org_id_at_join = users.org_id` snapshot for audit-only join queries.
7. `CREATE INDEX idx_channel_members_org_at_join`。

**红线**:
- **不自动 rename 历史 dup**: pre-flight 失败硬停 + 报 row 列出。任何"自动 rename + 加后缀"补丁 = 红线 (野马 R2 立场: 历史名是用户认知锚, 自动改名 = 丢历史)。
- **rebuild 步骤完整性**: PRAGMA 抓 cols + indexes → COPY → DROP → RENAME → reapply。autoindex (来自 inline UNIQUE) 不回填; 用户索引 (idx_channels_org_id, idx_channels_position, idx_channels_group) 必须保留。
- **silent=0 default**: column-level default 是 0 (人), backfill UPDATE 把 agent 行翻 1。任何"channel 创建即给 silent flag" = 走代码路径不走迁移。

**Trimmed-schema tolerance**: `hasTable("channels"/"channel_members")` + `hasColumn` 双 guard, 与 cm_3 / cm_onboarding 同模式; 仅 channels(id) + created_by 的 trimmed scaffold (TestDefaultRegistryRunsClean 覆盖) 跳过 rebuild + 跳过 backfill。

**配套测试**:
- `chn_1_1_channels_org_scoped_test.go::TestCHN11_AddsArchivedAtAndSilentColumns` — schema 列 + 默认值断言
- `…::TestCHN11_DropsGlobalNameUniqueAndAddsPerOrgIndex` — 跨 org 同名 INSERT 合法 / 同 org dup INSERT 拒 / idx_channels_org_id 幸存
- `…::TestCHN11_HardFailsOnHistoricDuplicateNoAutoRename` — pre-flight 报 dup 硬停 + 不写 schema_migrations (PR #265 CHN-1.1 spec drift 行)
- `…::TestCHN11_BackfillsAgentSilentAndOrgIDAtJoin` — agent silent=1 / human silent=0 / org_id_at_join 双方 snapshot
- `…::TestCHN11_IsIdempotentOnRerun` + `_ToleratesTrimmedSchema`

## 8. Phase 3 + Phase 4 入口 migration sequence (v=13 ~ v=21)

> 跟 spec brief sequencing 锁字面延续: CV-1.1 v=13 / CV-2.1 v=14 / DM-2.1 v=15 / AL-4.1 v=16 / CV-3.1 v=17 / CV-4.1 v=18 / CHN-3.1 v=19 / **AL-2a.1 v=20** (Phase 4 起步) / **AL-1b.1 v=21** (本 PR). 真先到先拿 — CV-2.1 抢 v=14, DM-2.1 顺延 v=15, AL-4.1 顺延 v=16, CV-3.1 v=17, CV-4.1 v=18 ✅ (#405), CHN-3.1 v=19 ✅ (#410), AL-2a.1 v=20 ✅ (#447 Phase 4 第一段), AL-1b.1 v=21 ✅ (本 PR #453 Phase 4 第二段). CHN-4.1 不消费 v 号 (PR #411 client wiring 无 schema 改).

### 8.1 v=13 — `cv_1_1_artifacts` (CV-1.1, PR #334)

artifacts + artifact_versions 表已落, 详见 `data-model.md` (待补) + spec `cv-1-spec.md`。

### 8.2 v=14 — `cv_2_1_anchor_comments` (CV-2.1, PR #359)

CREATE TABLE artifact_anchors (range CHECK end>=start, immutable artifact_version_id FK) + anchor_comments (author_kind CHECK ('agent','human')) + 双索引 idx_anchors_artifact_version + idx_anchor_comments_anchor。**反约束**: anchors 表无 kind/cursor 列; anchor_comments 无 committer_kind 列 (锚点是评论作者非 commit 提交者, 立场 ① 字面拆开)。8 PASS test 锁 schema drift。

### 8.3 v=15 — `dm_2_1_message_mentions` (DM-2.1, PR #361)

CREATE TABLE message_mentions (`message_id` + `target_user_id` + `created_at` + UNIQUE pair) + idx_message_mentions_target_user_id (mention 路由热路径)。**反约束 7 列**: 无 cursor / fanout_to_owner_id / cc_owner_id / owner_id / target_kind / read_at / acknowledged_at — 蓝图 `concept-model.md §4` mention 永不抄送 owner 立场字面拆死。PK 全局单调 (cross-message interleave 测)。

### 8.4 v=16 — `al_4_1_agent_runtimes` (AL-4.1, PR #398)

CREATE TABLE agent_runtimes (`agent_id` UNIQUE + `process_kind` CHECK ('openclaw','hermes') + `status` CHECK ('registered','running','stopped','error') + `last_error_reason` 复用 AL-1a #249 6 reason 枚举字面) + idx_agent_runtimes_agent_id。**反约束**: 无 is_online (跟 AL-3 SessionsTracker 边界拆死, runtime status ≠ session presence); 无 llm_provider/api_key (Borgee 不调 LLM, 蓝图立场 #7)。

### 8.5 v=17 — `cv_3_1_artifact_kinds` (CV-3.1, PR #396)

ALTER TABLE artifacts: kind enum CHECK 扩 'code' / 'image_link' (CV-1.1 既有 'markdown' 不动); CHECK 反约束 reject 'pdf' / 'kanban' / 'mindmap' (蓝图 `canvas-vision.md §2` v1 不做字面禁)。**反约束**: 不裂 artifact_code / artifact_images 拆表 (立场 ① enum 扩不裂表, 避免 schema 爆炸 + RT-1 envelope cursor 失同步)。配套 server `POST /artifacts` 校验: kind=='code' 必含 metadata.language ∈ 11 项白名单 + kind=='image_link' 必含 metadata.kind ∈ ('image','link') + URL https only (反 javascript:/data:/http: 协议)。

### 8.6 v=18 — `cv_4_1_artifact_iterations` (CV-4.1, PR #405)

CREATE TABLE artifact_iterations (`id` PK + `artifact_id` FK + `requested_by` FK users + `intent_text` (隐私字段, admin god-mode 不返 raw) + `target_agent_id` FK users role='agent' + `state` CHECK 4 态 ('pending','running','completed','failed') byte-identical 跟 #380 文案锁 + `created_artifact_version_id` NULL FK artifact_versions (completed 态填; 立场 ② CV-1 commit 单源 atomic UPDATE) + `error_reason` NULL (复用 AL-1a #249 6 reason 枚举字面 + AL-4 stub fail-closed `runtime_not_registered`; schema 不装 CHECK enum, server 校验) + `created_at` + `completed_at` NULL) + idx_iterations_artifact_id_state (per-artifact 热路径) + idx_iterations_target_agent (agent 工作队列). **反约束** (cv-4-spec.md §0 + acceptance §4): 立场 ① 域隔离 — 不污染 messages 表加反指列, 不动 artifact_versions schema (反向 grep count==0); 立场 ② CV-1 commit 单源 — 不开 `POST /iterations/:id/commit` 旁路 (commit 走 `?iteration_id=` query atomic UPDATE); 立场 ③ server 不算 diff — 表无 `diff_blob`/`diff_lines` (jsdiff 仅 client); state CHECK 严格 reject 'starting'/'busy'/'unknown' 中间态; 不挂 `cursor` 列 (跟 RT-1 envelope cursor 拆死 — IterationStateChangedFrame 9 字段 cursor 是 frame 路径); 不挂 `retry_count` 列 (failed 态 owner 重新触发 = 新 iteration_id, 不复用 failed 行 — #380 ⑦ + #365 反约束 ②).

### 8.7 v=19 — `chn_3_1_user_channel_layout` (CHN-3.1, PR #410)

CREATE TABLE user_channel_layout (`user_id` FK users + `channel_id` FK channels + `collapsed` INTEGER NOT NULL DEFAULT 0 (BOOL 0/1, group 折叠态) + `position` REAL NOT NULL (单调小数 ordering, pin = MIN(已有) - 1.0) + `created_at` + `updated_at` (server stamp 跟 channel_groups #276 同模式) + PRIMARY KEY (user_id, channel_id) 复合 PK) + idx_user_channel_layout_user_id (本人 GET /me/layout 热路径). **反约束** (chn-3-spec.md §0 + 野马 #366 stance 7 立场 + #402 6 字面锁): 立场 ① 物理拆死 — 不动 channels / channel_groups schema (反向 grep `ALTER TABLE channels.*ADD.*collapsed` count==0); 立场 ② 个人偏好两维 — 仅 collapsed + position, 反向断言 hidden / muted / pinned / is_pinned / group_id 全无 (mute 走 Phase 5+, hide 留 v3+); 立场 ③ pin 走 position 单调小数 — 反向 grep `pinned BOOL` count==0 (避免 ORDER BY pinned DESC, position ASC 双源排序); 立场 ⑤ ADM-0 红线 — schema 不挂 admin god-mode 字段 (admin 不读业务数据); 立场 ⑥ ordering client 端 — 表无 cursor 列 (跟 al_3_1 / al_4_1 / cv_*_1 / dm_2_1 / cv_4_1 同模式); 立场 ⑦ lazy 清理不级联 — 表无 ON DELETE CASCADE (作者删 group → 个人 layout 行 lazy GC 90d cron, 不阻塞作者删 group 路径).

### 8.8 v=20 — `al_2a_1_agent_configs` (AL-2a.1, PR #447 Phase 4 第一段)

CREATE TABLE agent_configs (`agent_id` PK FK users (agent 行) + `schema_version` INTEGER NOT NULL (单调递增, 4.1.a 并发 update 末次胜出 + 严格递增 防丢失) + `blob` TEXT NOT NULL (JSON; 仅含蓝图 §1.4 "归 Borgee 管" 字段 — name / avatar / prompt / model / 能力开关 / 启用状态 / memory_ref) + `created_at` + `updated_at`) + idx_agent_configs_agent_id (PATCH/GET /api/v1/agents/:id/config 热路径). **反约束** (al-2a acceptance #264 + 蓝图 §1.4 SSOT + §1.5 BPP frame 反约束): runtime-only 字段 (api_key / temperature / token_limit / retry_policy) 不入 schema (blob TEXT JSON, runtime 校验 fail-closed 走 AL-2a.2 server REST API + 4.1.c reflect scan); 不挂 cursor 列 (蓝图 §1.5 BPP frame `agent_config_update` 留给 AL-2b 与 BPP-3 同合, AL-2a 走轮询 reload, 跟 al_3_1 / al_4_1 / cv_*_1 / dm_2_1 / cv_4_1 / chn_3_1 同模式); 不挂 org_id (跨 org 隔离走 server-side ACL, users.org_id 单源 CM-1 #176, schema 不重复持有避免双源); 不裂 multi-row by config_key (PK 单 agent_id, SSOT blob 整体替换 PATCH 语义 — 立场 #1.4); 表无 ON DELETE CASCADE (跟 chn_3_1 同模式, agent 删 → config 行 lazy GC 留 v3+ cron).

### 8.9 v=22 — `adm_2_1_admin_actions` (ADM-2.1, PR #470)

CREATE TABLE admin_actions (`id` PK + `actor_id` NOT NULL (FK admins.id 逻辑, ADM-0 红线 admin ∉ users 表) + `target_user_id` NOT NULL (FK users.id 逻辑, 蓝图 §1.4 红线 1 受影响者必有) + `action` TEXT NOT NULL CHECK 5 字面 byte-identical ('delete_channel','suspend_user','change_role','reset_password','start_impersonation') + `metadata` TEXT NOT NULL DEFAULT '' (JSON 上下文, 不挂 schema CHECK server 校验) + `created_at` INTEGER NOT NULL Unix ms) + idx_admin_actions_target_user_id_created_at (受影响者 GET /api/v1/me/admin-actions 热路径) + idx_admin_actions_actor_id_created_at (admin SPA GET /admin-api/v1/audit-log 热路径). AL-2b 是 BPP frame 无 migration 不占号, v=21 占 AL-1b.1 (#453), v=20 占 AL-2a.1 (#447). **反约束** (adm-2-spec.md §1 + acceptance §数据契约 + 蓝图 §1.4 红线 + §2 不变量): 立场 ① 每写必留痕 — admin SPA 任意写动作 INSERT 此表无白名单旁路 (ADM-2.2 server 路径反向 grep `skip_audit\|noAudit\|bypassAudit` count==0); 立场 ⑤ forward-only audit — 不挂 `updated_at` 列 (蓝图 §2 不变量 "Audit 100% 留痕" — 错误录入只能再写新行 action='correction'); 立场 ⑥ admin ∉ 业务路径 — actor_id FK admins.id 独立表 (反向 `actor_id.*users\b` count==0); 反约束列名: 不挂 `org_id` (受影响者 org 通过 users.org_id 派生不冗余存) / `session_id` (impersonate 走单独 impersonation_grants 表 ADM-2.2 v1 落) / `cursor` (跟 al_3_1 / al_4_1 / cv_*_1 / dm_2_1 / cv_4_1 / chn_3_1 / al_2a_1 同模式 RT-1 envelope cursor 不下沉 schema). action CHECK 严格 reject 同义词 (`remove_channel`/`ban_user`/`update_role`) + 大小写漂移 (`Delete_Channel`/`SUSPEND_USER`) + 字典外值 (`create_user`/`force_logout`) + 空字符串 (15 反约束值 PASS).

### 8.10 v=23 — `adm_2_2_impersonation_grants` (ADM-2.2, PR #484)

CREATE TABLE impersonation_grants (`id` PK UUID + `user_id` NOT NULL (FK users.id 逻辑, 业主自己 grant) + `granted_at` INTEGER NOT NULL (Unix ms) + `expires_at` INTEGER NOT NULL (granted_at + 24h, server 固定不接受 client 传) + `revoked_at` INTEGER NULL (业主主动撤销时 stamp; NULL 表示有效)) + idx_impersonation_grants_user_id_expires (ActiveGrant query 热路径). v=22 占 ADM-2.1 admin_actions, v=25 留下个 milestone. **反约束** (adm-2-spec.md §2.5 + acceptance §4.2 + stance §1 立场 ⑦): 蓝图 §3 字面 "由 user 创建": 此表 INSERT 路径仅从 user-rail 进入 (POST /api/v1/me/impersonation-grants 走 user cookie), admin SPA 不开授予自己 impersonate 路径 (反向 grep `force_impersonate\|admin_impersonate_self` count==0); 期限固定 24h (server 端 expires_at = granted_at + 24h, schema 不挂 CHECK 但 server 校验); revoked_at 是唯一允许的 UPDATE 路径 (forward-only 立场 ⑤ 例外 — 撤销是业主主动权); 反向列名: 不挂 `actor_id` (admin 不在此表, 蓝图 §3 字面 "admin 仅消费这条记录") / `cursor` (跟 al_3_1 / al_4_1 / cv_*_1 / dm_2_1 / cv_4_1 / chn_3_1 / al_2a_1 / adm_2_1 同模式 frame 路径不下沉 schema) / `token` / `session_token` / `api_key` / `duration` / `duration_hours`.
CREATE TABLE agent_configs (`agent_id` PK FK users (agent 行) + `schema_version` INTEGER NOT NULL (单调递增, 4.1.a 并发 update 末次胜出 + 严格递增 防丢失) + `blob` TEXT NOT NULL (JSON; 仅含蓝图 §1.4 "归 Borgee 管" 字段 — name / avatar / prompt / model / 能力开关 / 启用状态 / memory_ref) + `created_at` + `updated_at`) + idx_agent_configs_agent_id (PATCH/GET /api/v1/agents/:id/config 热路径). **反约束** (al-2a acceptance #264 + 蓝图 §1.4 SSOT + §1.5 BPP frame 反约束): runtime-only 字段 (api_key / temperature / token_limit / retry_policy) 不入 schema (blob TEXT JSON, runtime 校验 fail-closed 走 AL-2a.2 server REST API + 4.1.c reflect scan); 不挂 cursor 列 (蓝图 §1.5 BPP frame `agent_config_update` 留给 AL-2b 与 BPP-3 同合, AL-2a 走轮询 reload, 跟 al_3_1 / al_4_1 / cv_*_1 / dm_2_1 / cv_4_1 / chn_3_1 同模式); 不挂 org_id (跨 org 隔离走 server-side ACL, users.org_id 单源 CM-1 #176, schema 不重复持有避免双源); 不裂 multi-row by config_key (PK 单 agent_id, SSOT blob 整体替换 PATCH 语义 — 立场 #1.4); 表无 ON DELETE CASCADE (跟 chn_3_1 同模式, agent 删 → config 行 lazy GC 留 v3+ cron). 注: CHN-4.1 (PR #411 client wiring 双 tab) 不消费 v 号 — server endpoint 拼装层 GET /api/v1/channels/:id/scene 无 DB schema 改, sequencing 字面锁继续延续到 AL-1b.1 v=21.

### 8.9 v=21 — `al_1b_1_agent_status` (AL-1b.1, 本 PR Phase 4 第二段)

CREATE TABLE agent_status (`agent_id` PRIMARY KEY (1 row per agent, 立场 ① 拆三路径) + `state` NOT NULL CHECK ('busy','idle') 2 态 + `last_task_id` NULL (BPP frame 上行 task_id) + `last_task_started_at` NULL (Unix ms, busy 态填) + `last_task_finished_at` NULL (Unix ms, idle 态填) + `created_at` + `updated_at` NOT NULL) + idx_agent_status_state (busy 列表 lookup 热路径). **反约束** (al-1b-spec.md §0 + acceptance §1.5 + 立场 ①②): 立场 ① 拆三路径 — 表无 `is_online` / `presence` (跟 AL-3 presence_sessions 拆死, agent 在 task in-flight 但 hub 心跳超时是合法态) + 无 `last_error_reason` / `endpoint_url` / `process_kind` (跟 AL-4 agent_runtimes process-level 拆死, busy/idle 是 task-level); 立场 ② BPP 单源 反人工伪造 — 表无 `source` / `set_by` 列 (state machine 唯一 source = BPP `task_started`/`task_finished` frame, server 端 state machine 守, 不在 schema 层暴露); 不挂 `cursor` 列 (跟 RT-1 envelope cursor 拆死, 同 al_3_1 / al_4_1 / cv_*_1 / dm_2_1 模式); 不挂 ON DELETE CASCADE (蓝图 §2.3 字面 "保留状态历史" — agent 删后 status row 留账 admin 审计路径). 6 TestAL1B1_* 全 PASS (CreatesTable + NoDomainBleed 9 列反向 + AcceptsBusyIdleEnum 2 PASS+11 reject + HasIndex + NoCascadeDelete + Idempotent). 配套 server (本 PR 同 commit, AL-1b.2 #457 + AL-1b.3 #462 已 merged 进 #453 baseline): GET /api/v1/agents/:id/status 5-state 合并 (error > busy > idle > online > offline) + PATCH 405 admin god-mode reject + IdleThreshold = 5 * time.Minute reaper.

### 8.11 v=25 — `al_1_4_agent_state_log` (AL-1.4 wrapper milestone)

CREATE TABLE agent_state_log (`id` INTEGER PK AUTOINCREMENT (单调 history 序号) + `agent_id` NOT NULL (FK users.id role='agent' 逻辑) + `from_state` NOT NULL (前态; '' 表示首次, 跟 AL-1a 三态 'online'/'offline'/'error' + AL-1b 'busy'/'idle' 5 字面 union) + `to_state` NOT NULL (新态; 同上) + `reason` NOT NULL DEFAULT '' (failed → 复用 AL-1a 6 reason byte-identical: api_key_invalid / quota_exceeded / network_unreachable / runtime_crashed / runtime_timeout / unknown; 非 error 转移留空) + `task_id` NOT NULL DEFAULT '' (BPP-2.2 task lifecycle 触发的转移记 task_id; presence 触发留空) + `ts` NOT NULL Unix ms 转移发生时刻) + idx_agent_state_log_agent_id_ts (DESC, owner GET /api/v1/agents/:id/state-log 热路径). **反约束** (蓝图 §2.3 + AL-1a/AL-1b 立场承袭): 立场 ① forward-only audit log (蓝图 §2 不变量同精神 — 不挂 `updated_at` / `modified_at`, 错误录入只能再写新行); 立场 ② state machine 单源 (server-side ValidateTransition 走严格 graph, schema 不挂 from_state/to_state CHECK 让 AL-1a 三态 + AL-1b 5-state 复用同表无需 schema 改 — 反向不为每态裂表); 立场 ③ task-driven 优先 (BPP-2.2 task lifecycle 触发 busy/idle 转移时 task_id 必填; presence 触发 task_id 空); 立场 ④ reason 复用 AL-1a 6 字面 (改 = 改 8 处单测锁链 — #249 + #305 + #321 + #380 + #454 + #458 + #481 + 此); 反向列名: 不挂 `cursor` (跟 al_3_1 / al_4_1 / cv_*_1 / dm_2_1 / cv_4_1 / chn_3_1 / al_2a_1 / adm_2_1 / adm_2_2 同模式 frame 路径不下沉) / `org_id` (派生 users.org_id) / `state` 单字段 (语义在 to_state, 双字段引诱 redundant write) / `owner_id` (派生 users.owner_id). v=25 sequencing: ADM-2.1 v=22 / ADM-2.2 v=23 / **AL-1.4 v=25** (本 migration); v=25 留下个 milestone. 6 TestAL14_* 全 PASS (CreatesAgentStateLogTable + InsertAndAutoIncrement + NoDomainBleed 6 反向列 + HasIndex + AcceptsAL1aReasonValues 6 字面 + Idempotent).

### 8.12 v=43 — `adm_3_1_audit_events_rename` (ADM-3, PR #586)

`ALTER TABLE admin_actions RENAME TO audit_events` (SQLite 元数据 0 数据迁移 — 索引 + sparse `idx_admin_actions_archived_at WHERE archived_at IS NOT NULL` AL-7.1 + CHECK enum 12 字面全自动跟随) + `CREATE VIEW admin_actions AS SELECT * FROM audit_events` (alias backward compat, read-transparent) + 2 INSTEAD OF triggers: `admin_actions_insert` (路由 view INSERT → audit_events table; SQLite views 不可直接写) + `admin_actions_update` (路由 sweeper archive UPDATE archived_at → audit_events). **idempotent** (`hasTable("audit_events")` 短路重跑). **立场承袭 ADM-3 spec §0** (3 立场): ① 表名改 audit_events 单表跨全 actor type (admin / system sweeper / plugin lifecycle / 未来类型) — BPP-8 #532 5 plugin lifecycle 事件入此表名实相符; ② alias view backward compat — gorm `(AdminAction).TableName() == "admin_actions"` 战马原代码 0 改, 既有 ADM-2 / BPP-8 / AL-7 / AL-8 unit tests 全 PASS via alias view; ③ ADM-0 §1.3 红线扩展 — audit-forward-only 单表跨全 actor type, forward-only 不可 DELETE/UPDATE (sweeper 走 archived_at AL-7), admin 仍走 /admin-api/* 单独路径. **反约束** (spec §2 反向 grep 4 锚): `DELETE FROM audit_events|UPDATE audit_events` 0 hit (production *.go); `INSERT INTO admin_actions` production 0 hit (除 12-step rebuild 中间表 admin_actions_*_new); `/admin-api.*audit_events|admin.*god.*audit_events` 0 hit (admin god-mode 不挂直 endpoint); audit-forward-only 锁链 ADM-3 = 第 11 处 (ADM-2.1+AP-2+BPP-4+BPP-7+BPP-8+HB-3 v2+AL-7+AL-8+HB-5+CHN-5+ADM-3). v=43 sequencing: team-lead 占号 reservation (post BPP-8 v=31, CV-6 v=36, CHN-14 v=36 等). 6 TestADM31_* 全 PASS (AuditEventsTableExists + AdminActionsViewExists + ViewSelectRoundtrip + ViewInsertRoutedToTable + VersionIs43 + Idempotent).
