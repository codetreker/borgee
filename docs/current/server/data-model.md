# Data Model — SQLite schema 与事件日志

代码位置：`packages/server-go/internal/store/`。
所有表定义在 `migrations.go`（幂等 `CREATE TABLE IF NOT EXISTS` + `ALTER TABLE ADD COLUMN` 守卫），ORM 用 GORM 但只是薄包装，没有自动迁移。

## 1. 表清单

| 表 | 关键列 | 备注 |
|----|--------|------|
| `users` | `id`, `display_name`, `role` (`member` / `agent` / `system`，**ADM-0.3 后** `admin` **不再**在此 enum), `email`（可空，部分唯一索引：`WHERE email IS NOT NULL`）, `password_hash`, `api_key` UNIQUE, `owner_id` FK→users, `disabled`, `deleted_at`, `org_id` (CM-1.1) | agent 行 `role="agent"` 且必有 `owner_id`；`role="system"` 用于 `sender_id='system'` 欢迎消息发送方 (CM-onboarding)；软删 |
| `admins` | `id`, `login` UNIQUE, `password_hash` (bcrypt), `created_at` | **ADM-0.1 (v=4)** — admin 独立子系统的凭证表；不准多 `org_id / role / is_admin / email` 字段。Bootstrap 由 `BORGEE_ADMIN_LOGIN` + `BORGEE_ADMIN_PASSWORD_HASH` env 注入 |
| `admin_sessions` | `token` PK (32B hex), `admin_id`, `created_at`, `expires_at` | **ADM-0.2 (v=5)** — `borgee_admin_session` cookie 反查表；token 不可猜，cookie 值不能是 admin id |
| `channels` | `id`, `name` (per-org UNIQUE via `idx_channels_org_id_name WHERE deleted_at IS NULL`, **CHN-1.1 v=11**), `type` (`channel` / `dm` / `system`), `visibility` (`public` / `private`), `topic`, `position` (LexoRank), `group_id` FK, `created_by`, `deleted_at`, `archived_at` (CHN-1.1, NULL=active), `org_id` | DM name = `dm:<uid_low>_<uid_high>`；`system` type 给 CM-onboarding `#welcome` 私属频道用; **跨 org 同名合法** (蓝图 channel-model §2) |
| `channel_groups` | `id`, `name`, `position`, `created_by` | 侧边栏分组 |
| `channel_members` | PK (`channel_id`, `user_id`), `joined_at`, `last_read_at`, `silent` (CHN-1.1, agent 行默认 1, 人 0), `org_id_at_join` (CHN-1.1, audit snapshot) | `last_read_at` 给未读计数用; `silent` 落蓝图 concept-model §1.2 "agent = 同事但默认不抢话" 的设计, owner 显式翻 0 才发言 |
| `messages` | `id`, `channel_id`, `sender_id`, `content`, `content_type` (默认 `text`), `reply_to_id`, `edited_at`, `deleted_at`, `edit_history` (DM-7.1 v=34, JSON 数组 `[{old_content, ts, reason}]`, NULL=无编辑), **`pinned_at`** (DM-10.1 v=45, INTEGER NULL Unix ms, NULL=未 pin, sparse partial idx `idx_messages_pinned_at(channel_id, pinned_at DESC) WHERE pinned_at IS NOT NULL` 现网零开销) | 软删. **DM-10**: `pinned_at` 仅 DM channel scope 写 (server `pin.dm_only_path` 反 non-DM); pin/unpin 走 `POST/DELETE /api/v1/channels/{channelId}/messages/{messageId}/pin` + `GET /api/v1/channels/{channelId}/messages/pinned`; channel-member ACL 复用 AP-4 #551 / AP-5 #555 同 helper; admin god-mode 不挂 (ADM-0 §1.3 红线; grep 检查 `admin.*pin.*messages` 在 admin*.go 0 hit). 反向约束: 不挂 `pinned_by`/`pin_reason` 列 (per-DM scope, 反 per-user pin 留 v2 跟 CHN-3.2 风格不同源); 不另起 `pinned_messages` 表 (单一来源 `messages.pinned_at` 列). |
| `mentions` | `id`, `message_id`, `user_id`, `channel_id` | `CreateMessageFull` 解析 `@name` 时回填 |
| `events` | `cursor` AUTOINC PK, `kind`, `channel_id` NOT NULL, `payload` (JSON text), `created_at` | **事件日志唯一来源**，下文详述 |
| `user_permissions` | (`user_id`, `permission`, `scope`) UNIQUE | scope 例如 `*` 或 `channel:<id>` |
| `invite_codes` | `code`, `created_by`, `expires_at`, `used_by`, `used_at` | 邀请制注册 |
| `message_reactions` | `id`, `message_id`, `user_id`, `emoji`，UNIQUE(`message_id`,`user_id`,`emoji`) | 同一人对同一消息同 emoji 只能加一次 |
| `workspace_files` | `id`, `user_id`, `channel_id`, `parent_id`, `name`, `is_directory`, `mime_type`, `size_bytes`, `source` (`upload` / `message`), `source_message_id` | per-channel 文件树 |
| `remote_nodes` | `id`, `user_id`, `machine_name`, `connection_token` UNIQUE, `last_seen_at` | `remote-agent` 注册 |
| `remote_bindings` | `id`, `node_id`, `channel_id`, `path`, `label`，UNIQUE(`node_id`,`channel_id`,`path`) | channel ↔ 远端目录绑定 |
| `organizations` | `id`, `name`, `created_at` | Phase 1 / CM-1.1 (`schema_migrations` v2)。1 person = 1 org, UI 永不暴露; 数据层 first-class。见 `migrations.md §7.1`。 |
| `agent_invitations` | `id` PK, `channel_id`, `agent_id`, `requested_by`, `state` (`pending` / `approved` / `rejected` / `expired`，CHECK 约束), `created_at`, `decided_at?`, `expires_at?` | Phase 2 / CM-4.0 (`schema_migrations` v3)。跨 org 邀请 agent 进 channel 的状态机表 (blueprint §4.2 流程 B)。CM-4.0 仅落表 + 状态机 helper, **没有 HTTP / BPP / UI**, 留给 CM-4.1。|
| `presence_sessions` | `id` PK AUTOINCREMENT, `session_id` UNIQUE NOT NULL, `user_id` NOT NULL, `agent_id` (nullable), `connected_at` (Unix ms), `last_heartbeat_at` (Unix ms) | **Phase 4 / AL-3.1 (`schema_migrations` v=12)**。`PresenceTracker` 接口 (#277) 的真实存储；多 session per user 合法 (web + mobile + plugin)。`agent_id NULL = 人 session, 非 NULL = agent session`。索引: `idx_presence_sessions_user_id` (full) + `idx_presence_sessions_agent_id WHERE agent_id IS NOT NULL` (partial)。**反向约束**: 不挂 `cursor` 列 (与 RT-1 事件序列分清, 瞬时态 vs 不可回退序列); 字段名 `last_heartbeat_at` 非 `last_seen_at` (#302 §1.1)。写端 (`TrackOnline / TrackOffline`) 走 AL-3.2 hub lifecycle hook。 |
| `artifact_anchors` | `id` PK, `artifact_id` NOT NULL FK, `artifact_version_id` NOT NULL FK (immutable, 设计 ② version-pin), `start_offset` INTEGER, `end_offset` INTEGER CHECK `end_offset >= start_offset`, `created_by` NOT NULL FK users, `created_at`, `resolved_at` NULL | **Phase 3 / CV-2.1 (`schema_migrations` v=14, PR #359)**。锚点对话 v2 数据契约。设计 ① owner-only schema 层不锁 (server CV-2.2 #360 enforce); 设计 ② anchor 钉死创时 `artifact_version_id` 不跨版本自动迁移; 设计 ③ range CHECK end>=start。**反向约束**: 不挂 `kind`/`anchor_kind`/`author_kind` 列 (kind 仅在 `anchor_comments`); 不挂 `cursor` 列 (RT-1 envelope cursor 分清)。索引: `idx_anchors_artifact_version`. |
| `anchor_comments` | `id` PK AUTOINCREMENT, `anchor_id` NOT NULL FK, `body` TEXT, `author_kind` CHECK ('agent','human'), `author_id` NOT NULL FK, `created_at` | **Phase 3 / CV-2.1 (`schema_migrations` v=14, PR #359)**。anchor thread 评论行。`author_kind` 不复用 CV-1 `committer_kind` (anchor 是评论作者非 commit 提交者, 设计 ① 字面拆开让反查 grep 不混淆)。PK AUTOINCREMENT 全局序保 audit log 跨 anchor 单调。索引: `idx_anchor_comments_anchor`. |
| `message_mentions` | `id` PK AUTOINCREMENT, `message_id` NOT NULL FK, `target_user_id` NOT NULL FK, `created_at`, UNIQUE(`message_id`, `target_user_id`) | **Phase 3 / DM-2.1 (`schema_migrations` v=15, PR #361)**。蓝图 `concept-model.md §4` mention 路由 — 落库后 server 解析 body 中 `@<user_id>` token → 写一行一目标; UNIQUE 同 message 同 target dedup (设计 ⑥ user/agent 同语义)。**反向约束** (acceptance §1.0.e): 不挂 `cursor` / `fanout_to_owner_id` / `cc_owner_id` / `owner_id` / `target_kind` / `read_at` / `acknowledged_at` 7 列 (mention 永不抄送 owner; user/agent 同表同语义不分叉; 阅读态留 Phase 5+)。索引: `idx_message_mentions_target_user_id` (mention fanout 路由热路径)。 |
| `agent_runtimes` | `id` PK, `agent_id` NOT NULL FK agents UNIQUE, `endpoint_url` TEXT NOT NULL, `process_kind` CHECK ('openclaw','hermes'), `status` CHECK ('registered','running','stopped','error'), `last_error_reason` (nullable, 复用 AL-1a #249 6 reason 枚举), `last_heartbeat_at` (nullable, Unix ms), `created_at`, `updated_at` | **Phase 4 / AL-4.1 (`schema_migrations` v=16, PR #398)**。蓝图 `agent-lifecycle.md §2.2` plugin process descriptor registry 表。设计 #7 "Borgee 不带 runtime" — 此表存的是 plugin process descriptor, 不存 LLM 调用本身。**反向约束**: 不挂 `is_online` 列 (跟 AL-3 #310 SessionsTracker 边界分清, runtime status ≠ session presence); 不挂 `llm_provider`/`api_key` 列 (Borgee 不调 LLM)。索引: `idx_agent_runtimes_agent_id` (lookup 热路径)。 |
| `artifact_iterations` | `id` PK, `artifact_id` NOT NULL FK, `requested_by` NOT NULL FK users, `intent_text` (隐私, admin god-mode 不返 raw), `target_agent_id` NOT NULL FK users role='agent', `state` CHECK ('pending','running','completed','failed'), `created_artifact_version_id` NULL FK artifact_versions, `error_reason` NULL (复用 AL-1a #249 6 reason 枚举 + AL-4 stub `runtime_not_registered`), `created_at`, `completed_at` NULL | **Phase 3 / CV-4.1 (`schema_migrations` v=18, PR #405)**。蓝图 `canvas-vision.md §1.4-§1.5` agent iterate 数据契约。设计 ① 域隔离 (messages 表无反指列); 设计 ② CV-1 commit 单一来源 (commit 走 `?iteration_id=` query atomic UPDATE, 不开 `POST /iterations/:id/commit` 旁路); 设计 ③ server 不算 diff (jsdiff client 端)。**反向约束**: 不挂 `cursor` 列 (RT-1 envelope cursor 分清, IterationStateChangedFrame 9 字段 cursor 是 frame 路径); 不挂 `retry_count` 列 (failed 重试 = 新 iteration_id)。索引: `idx_iterations_artifact_id_state` + `idx_iterations_target_agent`。 |
| `user_channel_layout` | (`user_id`, `channel_id`) PK 复合, `collapsed` INTEGER NOT NULL DEFAULT 0, `position` REAL NOT NULL, `created_at`, `updated_at` | **Phase 3 / CHN-3.1 (`schema_migrations` v=19, PR #410)**。蓝图 `channel-model.md §1.4 + §3.4` 作者定义大局 + 个人偏好微调; 个人偏好两维 (collapsed + position)。pin = MIN(已有 position) - 1.0 (设计 ③ 单调小数, 不拆 pinned BOOL 双源排序)。**反向约束**: 不挂 hidden / muted / pinned / is_pinned / group_id 列 (mute 走 Phase 5+, hide 留 v3+); 不挂 cursor 列 (RT-1 envelope cursor 分清); 表无 ON DELETE CASCADE (作者删 group → 个人 layout 行 lazy GC 90d cron, 设计 ⑦)。索引: `idx_user_channel_layout_user_id` (本人 GET /me/layout 热路径)。 |
| `agent_configs` | `agent_id` PK, `schema_version` INTEGER NOT NULL, `blob` TEXT NOT NULL (JSON), `created_at`, `updated_at` | **Phase 4 / AL-2a.1 (`schema_migrations` v=20, 本 PR)**。蓝图 `agent-lifecycle.md §2.1` (用户完全自主决定 agent name/prompt/能力/model) + `plugin-protocol.md §1.4` (Borgee=单一来源 字段划界) + §1.5 (热更新分级, AL-2a 不含 BPP frame). blob 仅含 Borgee 管字段 (name / avatar / prompt / model / 能力开关 / 启用状态 / memory_ref); schema_version 单调递增 (4.1.a 并发 update 末次胜出 + 严格递增 防丢失)。**反向约束**: 不挂 api_key / temperature / token_limit / retry_policy 列 (runtime-only 走 AL-2a.2 server REST API fail-closed + 4.1.c reflect scan); 不挂 cursor 列 (BPP frame agent_config_update 留 AL-2b + BPP-3 同合); 不挂 org_id (跨 org 隔离走 users.org_id 单一来源 CM-1 #176, 不重复持有避免双源); PK 单 agent_id (单一来源 blob 整体替换 PATCH 语义, 不拆 multi-row by config_key); 表无 ON DELETE CASCADE (lazy GC v3+)。索引: `idx_agent_configs_agent_id` (PATCH/GET /api/v1/agents/:id/config 热路径)。 |
| `admin_actions` | **post-ADM-3 (v=43, PR #586)**: VIEW alias of `audit_events` (read-transparent + INSTEAD OF INSERT/UPDATE triggers route to table). Original schema preserved (`id`, `actor_id`, `target_user_id`, `action`, `metadata`, `created_at` + AL-7.1 `archived_at`); CHECK enum extended to 12 字面 (post AL-7 audit_retention_override + BPP-8 5 plugin lifecycle events). | **Phase 4 / ADM-2.1 (`schema_migrations` v=22, PR #470) → ADM-3 (v=43, PR #586) RENAME → `audit_events` 表; alias view 留 backward compat 至 Phase 5+ deprecation.** 蓝图 `admin-model.md §1.4` "谁能看到什么" + §2 "Audit 100% 留痕" + §1.4 三红线 (受影响者必感知 + admin 之间互留痕)。设计沿用 ADM-2.1 设计 ①+⑤+⑥ 全 byte-identical (alias view 不破)。**反向约束** (ADM-3 设计扩展 §1.3 红线): forward-only 不可 DELETE/UPDATE on `audit_events` (sweeper archive 走 `archived_at` 列 AL-7.1); admin 仍走 `/admin-api/*` god-mode endpoint (grep 检查 `/admin-api.*audit_events` 0 hit); INSERT into view routed by `admin_actions_insert` INSTEAD OF trigger (gorm `(AdminAction).TableName() == "admin_actions"` 战马原代码 0 改). |
| `audit_events` | (post-ADM-3 RENAME 自 admin_actions, v=43, PR #586): `id` PK (UUID), `actor_id` NOT NULL, `target_user_id` NOT NULL, `action` TEXT NOT NULL CHECK 12 字面 byte-identical (ADM-2.1 5 admin 字面 + AP-2 'permission_expired' + BPP-8 5 plugin lifecycle 'plugin_connected/disconnected/reconnected/cold_started/heartbeat_timeout' + AL-7 'audit_retention_override'), `metadata` TEXT NOT NULL DEFAULT '' (JSON 上下文), `created_at` Unix ms, `archived_at` Unix ms NULL (AL-7.1 sparse idx WHERE archived_at IS NOT NULL — sweeper soft-archive 14d retention) | **Phase 4 / ADM-3 (`schema_migrations` v=43, PR #586)**。蓝图 `admin-model.md §1.3` 红线扩展 (ADM-3 设计 ③ "audit-forward-only 单表 = 全 actor type / forward-only 不可 DELETE/UPDATE / admin 仍走 /admin-api/* 单独路径")。**RENAME 元数据 0 数据迁移** — SQLite RENAME admin_actions → audit_events 索引/sparse idx_archived_at AL-7.1/CHECK enum 全自动跟随。**设计沿用** audit-forward-only 守护链跨 11 处: ADM-2.1+AP-2+BPP-4+BPP-7+BPP-8+HB-3 v2+AL-7+AL-8+HB-5+CHN-5+ADM-3。**actor_kind 隐性维度** (admin / system sweeper / plugin lifecycle / 未来类型) 全装单表, 名实相符 — BPP-8 #532 5 plugin lifecycle 事件入此表是合法的 (跟 ADM-2.1 admin actor 同精神). 索引: 跟随 admin_actions 原表 (RENAME 透明) — `idx_admin_actions_target_user_id_created_at` + `idx_admin_actions_actor_id_created_at` + `idx_admin_actions_archived_at WHERE archived_at IS NOT NULL` (sparse idx AL-7.1)。**reverse compat**: alias view `admin_actions` AS SELECT * FROM `audit_events` 留 deprecated 至 Phase 5+, 战马原 gorm `(AdminAction).TableName() == "admin_actions"` 0 改。 |
| `impersonation_grants` | `id` PK (UUID), `user_id` NOT NULL (FK users.id 逻辑, 业主自己 grant), `granted_at` Unix ms, `expires_at` Unix ms (granted_at + 24h server 固定), `revoked_at` Unix ms NULL (业主主动撤销 stamp; NULL 表示有效) | **Phase 4 / ADM-2.2 (`schema_migrations` v=23, PR #484)**。蓝图 `admin-model.md §1.4` 红线 2 第二档 ("Impersonate 必须显眼, 红色横幅 + 倒计时") + §3 数据模型片段 ("由 user 创建, admin 仅消费这条记录") + §4.1 R3 (ADM-1 文案 "24h 时窗顶部红色横幅常驻可随时撤销" 兑现出处)。设计 ⑦ impersonate 显眼 (业主授权 → 红横幅 + 24h 倒计时 + `[立即撤销]` 入口; admin 写动作需 grant 存在 + 未过期 + 未撤销, 否则 403 `impersonate.no_grant`)。**反向约束**: 蓝图 §3 字面 "由 user 创建" — INSERT 路径仅 user-rail 进入, admin SPA 不开授予自己 impersonate 路径 (grep 检查 `force_impersonate\|admin_impersonate_self` count==0); 期限固定 24h (server `expires_at = granted_at + 24h`, 不接受 client 传 expires_at); revoked_at 是唯一允许的 UPDATE 路径 (forward-only 设计 ⑤ 例外 — 撤销是业主主动权); 不挂 `actor_id` 列 (admin 不在此表); 不挂 `cursor` / `token` / `session_token` / `api_key` / `duration` 列。索引: `idx_impersonation_grants_user_id_expires` (ActiveGrant query 热路径)。 |
| `agent_status` | `agent_id` PK (1 row per agent), `state` NOT NULL CHECK ('busy','idle'), `last_task_id` (nullable, BPP frame 上行 task_id), `last_task_started_at` (nullable, Unix ms, busy 态填), `last_task_finished_at` (nullable, Unix ms, idle 态填), `created_at`, `updated_at` | **Phase 4 / AL-1b.1 (`schema_migrations` v=21, 本 PR)**。蓝图 `agent-lifecycle.md §2.3` 5-state busy/idle 真值 (2026-04-28 4 人 review #5 决议: source 必须 plugin 上行 `task_started` / `task_finished` frame, 没 BPP 不 stub)。设计 ① 拆三路径 — schema 仅 2 态 (busy/idle), API 层 (`GET /api/v1/agents/:id/status`) 5-state 合并 (error > busy > idle > online > offline), 跟 AL-3 presence_sessions + AL-4 agent_runtimes 真分清 (三表三路径互不污染)。**反向约束**: 不挂 `is_online` / `presence` (AL-3 presence_sessions 真分清, agent task in-flight 但 hub 心跳超时是合法态); 不挂 `last_error_reason` / `endpoint_url` / `process_kind` (AL-4 agent_runtimes process-level 真分清, busy/idle 是 task-level); 不挂 `source` / `set_by` (设计 ② BPP 单一来源反人工伪造, server `PATCH /status` 405 admin god-mode 也 reject); 不挂 `cursor` 列 (跟 al_3_1 / al_4_1 同模式); 表无 ON DELETE CASCADE (蓝图 §2.3 保留状态历史 — agent 删后 status row 留作存档 admin 审计)。索引: `idx_agent_status_state` (busy 列表 lookup 热路径)。配套 server (PR #457 已 merged 进 #453 baseline): `IdleThreshold = 5 * time.Minute` reaper (acceptance §2.4 — 5min 无 frame 自动 idle); store helpers `SetAgentTaskStarted` / `SetAgentTaskFinished` / `ReapStaleBusyToIdle` (BPP-2 frame dispatcher 接入入口 stub). |
| `agent_state_log` | `id` PK AUTOINCREMENT (单调 history 序号), `agent_id` NOT NULL (FK users.id role='agent' 逻辑), `from_state` NOT NULL ('' / 'online' / 'busy' / 'idle' / 'error' / 'offline' 5 字面 union, AL-1a 三态 + AL-1b 5-state 复用), `to_state` NOT NULL (同上), `reason` NOT NULL DEFAULT '' (error 转移复用 AL-1a 6 字面 byte-identical), `task_id` NOT NULL DEFAULT '' (BPP-2.2 task lifecycle 触发记 task_id; presence 触发留空), `ts` Unix ms | **Phase 4 / AL-1.4 (`schema_migrations` v=25, AL-1 wrapper milestone PR)**。蓝图 `agent-lifecycle.md §2.3` (4 态 + cross-state transition lock + 故障可解释) 兑现 — wrapper milestone 跟 AL-1a (#249) + AL-1b (#453/#457/#462) + BPP-2.2 (#485) 同源, 此表是历史 audit 轨迹 + state machine validator (`store.ValidateTransition` server-side gate)。设计 ① forward-only audit log (跟 admin_actions ADM-2.1 设计 ⑤ 同精神 — 不挂 `updated_at`, 错误录入只能再写新行); 设计 ② state machine 单一来源 (`AppendAgentStateTransition` 是 server-side state set 唯一入口); 设计 ③ task-driven (BPP-2.2 触发 busy/idle 转移时 task_id 必填, presence 触发留空); 设计 ④ reason 复用 AL-1a 6 字面 byte-identical (改 = 改 8 处单测守护链同源 — #249 + #305 + #321 + #380 + #454 + #458 + #481 + 此)。**反向约束**: 不挂 `cursor` (跟 al_3_1 / al_4_1 / cv_*_1 / dm_2_1 / cv_4_1 / chn_3_1 / al_2a_1 / adm_2_1 / adm_2_2 同模式 RT-1 envelope frame 路径不下沉); 不挂 `org_id` (派生 users.org_id); 不挂 `state` 单字段 (语义在 to_state, 双字段引诱 redundant write); 不挂 `owner_id` (派生 users.owner_id)。索引: `idx_agent_state_log_agent_id_ts` DESC (owner GET /api/v1/agents/:id/state-log 热路径)。endpoint: `GET /api/v1/agents/:id/state-log` owner-only (non-owner → 403, non-agent → 404, unauthenticated → 401)。 |
| `schema_migrations` | `version` PK, `applied_at`, `name` | Phase 0 / INFRA-1a。版本化迁移引擎状态表。 |

**`org_id` 列**: CM-1.1 给 `users / channels / messages / workspace_files / remote_nodes` 各加一列 `org_id TEXT NOT NULL DEFAULT ''` + 同名 `idx_*_org_id` 索引。v0 默认空串占位 (audit 表登记), CM-1.2 起注册流程开始写真值, CM-3 切到基于 `org_id` 直查。

**自动建 org (CM-1.2)**: `POST /api/v1/auth/register` 与管理员 `POST /api/v1/admin/users` 在创建 user 之后立即在同一事务中创建一行 `organizations(id=uuid, name="<DisplayName>'s org")` 并把 `users.org_id` 更新为新 org 的 id。失败则注册整体 5xx, 不留孤儿用户。Agent 创建 (`POST /api/v1/agents`) 继承所有人的 `org_id` (blueprint §1.1: agents 是 org 内资源, 不独立成 org)。schema 已允许空串, 但 app-layer 契约：注册路径产出的 user 永远 `org_id != ''`; v1 后续在 column constraint 上收紧。API 序列化 (`sanitizeUser` / `sanitizeUserAdmin`) 永不暴露 `org_id` (UI 不可见, blueprint §1.1)。

**Admin stats by org (CM-1.3)**: `GET /admin-api/v1/stats` 与 `GET /api/v1/admin/stats` 在原 `user_count / channel_count / online_count` 之外新增 `by_org: [{org_id, user_count, channel_count}, ...]` 字段。聚合见 `store.StatsByOrg()`: 对 `users` / `channels` 各跑一次 `GROUP BY org_id` (均过滤 `deleted_at IS NULL`) 后按 `org_id` 合并 + 字典序排序。空串 (`""`) 不丢弃, 显式作为一个 bucket 出现, 让 v0 历史孤儿数据可见 (audit 已登记)。验证不变量: `Σ by_org[*].user_count == user_count`, `Σ by_org[*].channel_count == channel_count`。Blueprint §2 "数据层一等公民"行为对照点。

**Agent invitations 状态机 (CM-4.0)**: 蓝图 §4.2 默认流程 B 的数据落地, **本步只到 schema + 状态机 helper, 不到 HTTP/UI**。表 `agent_invitations` 的 `state` 列用 TEXT enum + CHECK 约束 (`pending / approved / rejected / expired`)。状态机由 `store.AgentInvitation.Transition(to, nowMillis)` 实现, 仅允许 `pending → {approved, rejected, expired}` 三条边, 终态无出边; 任何非法转移返回 `errors.Is(err, ErrInvalidTransition) == true` 且不会改写 `inv.State` / `inv.DecidedAt`。`DecidedAt` 在每次成功转移时由调用方注入的 `nowMillis` 戳上 (Phase 1 testutil/clock 注入约定)。索引: `(agent_id, state)` 给 owner inbox 列待办、`(channel_id, state)` 给 channel 反查活跃邀请、`(requested_by)` 给 audit。CM-4.1 起在 handler 层使用; CM-4.0 仅有单测覆盖每条非法边 (acceptance 行为不变量 4.1)。v0 audit: enum 直接落 string, v1 切回时再考虑拆 lookup 表。

**Agent invitations API (CM-4.1)**: 蓝图 §4.2 流程 B 的 HTTP 落地, 复用 CM-4.0 的状态机 helper, **不动 BPP frame (CM-4.3) / client UI (CM-4.2) / offline 检测 (CM-4.3b)**。Endpoints: `POST /api/v1/agent_invitations` (channel 成员发起 — handler 显式 `inv.State = AgentInvitationPending`, 不依赖 GORM default), `GET /api/v1/agent_invitations[?role=owner|requester]` (owner=待办 inbox, requester=自己发出去的; admin 看全量), `GET /api/v1/agent_invitations/{id}` (requester / agent owner / admin), `PATCH /api/v1/agent_invitations/{id}` body `{state: "approved"|"rejected"}` (仅 agent owner / admin; `expired` 走后台 sweep 不开放给 owner)。Side-effect: `approved` 同步 `Store.AddChannelMember(agent → channel)` (idempotent FirstOrCreate); 失败只记 log 不回滚 — 持久化的决策才是单一来源, 重试或 sweep reconcile。Sanitizer hand-built (`map[string]any`, `decided_at` / `expires_at` 走 omitempty), 永不 `json.Marshal *store.AgentInvitation` (admin 模式)。`(state, expires_at)` 复合索引留给后台 sweep, 本步不补。`Now func() time.Time` 注入支持 testutil/clock 约定。覆盖率 ≥ 80% (handler ~85%)。

## 2. 迁移策略

`store.Migrate()` 是幂等函数：

1. `PRAGMA foreign_keys = OFF`
2. `createSchema()` — 全部 `CREATE TABLE IF NOT EXISTS`
3. `applyColumnMigrations()` — 一个 `ALTER TABLE ADD COLUMN` 列表，每条用 `columnExists()` 守卫
4. `createSchemaIndexes()` — 索引
5. `PRAGMA foreign_keys = ON`
6. **回填**：默认权限（AP-0: human → `(*, *)`; agent → `(message.send, *)`; 旧 v0 dev DB 上残留的 `channel.create / message.send / agent.manage` 三元组不主动清, 只增不减）、creator 的频道级权限、LexoRank 重平衡（默认 `"0|aaaaaa"` 的 channel）、DM 去重（按双方 id 排序检测重复）

**约束**：只允许加列，不允许改/删；要重命名或删列必须新建表 + 拷贝数据 + drop。没有版本表，因此每次启动跑全量幂等迁移。

## 3. 事件日志（`events` 表）

业务侧任何"会被订阅者看到"的状态变化都会写一行 `events`：

```
cursor       INTEGER PK AUTOINCREMENT
kind         TEXT      -- new_message / message_edited / channel_created / ...
channel_id   TEXT NOT NULL  -- 全局事件目前不写表，由 Hub 直接广播
payload      TEXT      -- JSON
created_at   TIMESTAMP
```

读路径（`GetEventsSinceWithChanges`）：

```sql
SELECT * FROM events
 WHERE cursor > ?
   AND (channel_id IN (?) OR kind IN (?))   -- 后者用于全局事件
 ORDER BY cursor ASC LIMIT 100
```

三个 push 通道（WS / SSE / 长轮询）共用这张表，配合 `Hub.SignalNewEvents` 唤醒等待者，实现：

- **断线续传**：客户端记 cursor，重连时 `Last-Event-ID` / poll body 带回。
- **多端一致**：同一用户在多端订阅，每端按自己 cursor 各自回放。

## 4. LexoRank

实现：`store/lexorank.go`，测试：`store/lexorank_test.go`。

- 字符串形如 `"0|<base26 6位>"`，例 `"0|hzzzzz"`。
- `GenerateRankBetween(before, after)` 计算 base-26 字典序中点；冲突时 `Rebalance(items)` 在 `[a, z]` 范围里均匀分配。
- 排序就是普通字符串比较 `ORDER BY position ASC`。
- 拖拽时只更新被移动那一项的 `position`，避免连锁写入。

## 5. 软删 vs 硬删

| 实体 | 默认 | 备注 |
|------|------|------|
| Message | 软删 (`deleted_at`) | 列表查询 `WHERE deleted_at IS NULL` |
| Channel | 软删 | admin 可硬删 `ForceDeleteChannel` |
| User | 软删 | `SoftDeleteUser`；其消息和频道**不会**级联 |
| Reaction / Mention / Member | 硬删 | 不需要历史保留 |

`ForceDeleteChannel`（admin only）按顺序删 messages → members → mentions → events → channel，是唯一的级联硬删路径。

## 6. DM 唯一性

DM channel 的 `name` 形如 `dm:<id_a>_<id_b>`。`normalizeDMName` 用 `sort.Strings` 对两个 user id 做**字典序**排序后拼接（不是数值大小），因此无论谁先发起，name 都一致；UNIQUE 约束保证重复创建会复用而不是新建。迁移时 `cleanupDuplicateDMs` 保底处理历史数据中可能存在的乱序重复，`cleanupDMExtraMembers` 删除 DM 频道里非两位参与者的成员。

## 7. 与 PRD 的映射

| PRD 概念 | 数据库实现 |
|----------|------------|
| user / agent 三角色 | `users.role ∈ {member, agent, system}` (ADM-0.3 后) + `users.owner_id`；admin 走独立 `admins` 表 |
| admin 全权 | `admins` (ADM-0.1) + `admin_sessions` (ADM-0.2)，路由 `/admin-api/*` 独立中间件，不在 `user_permissions` 写默认行 |
| 频道归属 | `channels.created_by`（agent 创建的会归到 agent；通过 `users.owner_id` 反查到人类 owner） |
| 公开频道 24h 预览 | `GET /api/v1/channels/{id}/preview` 不写表，只在 handler 里限定时间窗 |
| 邀请制注册 | `invite_codes` 表 + `auth/register` 校验 `used_at IS NULL && expires_at > now()` |
