# Data Layer — 总账与分布式 ready

> 数据层是前 9 轮形状决策的总账落地。本文规范数据要素、events 模型、迁移策略、存储路径，以及**v1 分布式 ready 的边界**。
> 状态：建军 + 飞马 + 野马 对齐（2026-04-27）。前置阅读：所有 design 文档（本文是它们的数据汇总）。

## 0. 一句话定义

> **v1 是 SQLite 单机 + in-process hub，但协议与接口已为分布式 ready 锁死——以后切多节点不挖坟。凭指标切，不凭感觉切。**

---

## 1. 关键边界（必须显眼）

> ⚠️ **v1 协议层 portable + 接口层抽象，但运行时单机，不支持多节点同时跑。**

读者如果只看一句，就这一句——避免把"分布式 ready"误读成"已分布式"。

| 维度 | v1 状态 |
|------|---------|
| 协议层（ID / cursor / artifact 版本 / event bus 接口） | ✅ 上线即多节点 ready，数据迁移零破坏 |
| 接口层（Storage / Presence / EventBus / Repository） | ✅ 实现可换，业务不动 |
| 运行时 | ❌ SQLite + in-process hub + 本地 fs，**不支持多节点同时跑** |

切换触发：[`§5 阈值哨`](#5-阈值哨) 任一条命中。

---

## 2. 数据要素总账

> 散落在 9 篇文档里的数据决策汇总。每个表/列都有归属文档，没有孤儿字段。

### 2.1 Organization 与身份

| 表/列 | 来源 | 备注 |
|-------|------|------|
| `organizations` | [concept-model §1.1](concept-model.md) | UI 不暴露，数据层一等公民 |
| `users.org_id` | concept-model | UI 不返回 |
| `users.owner_id` | concept-model | agent → 人类的归属指针 |
| `users.role` | admin-model | 取值 `member` / `admin` / `agent` |

### 2.2 权限

| 表/列 | 来源 | 备注 |
|-------|------|------|
| `user_permissions(user_id, permission, scope)` | [auth-permissions §1.1](auth-permissions.md) | ABAC 存储 |
| `user_permissions.expires_at` | auth-permissions §1.2 | 列预留，UI 不暴露 |
| `permission_grants` 历史 | auth-permissions（野马补） | 用户半年后回查"何时授权"——长期保留 |

### 2.3 Channel & Workspace

| 表/列 | 来源 | 备注 |
|-------|------|------|
| `channels`（type, visibility, group_id, deleted_at） | [channel-model](channel-model.md) | DM 复用 type='dm' |
| `channel_groups` | channel-model §1.4 | 作者定义，全 org 共享 |
| `user_channel_layout(user_id, channel_id, collapsed, position)` | channel-model §1.4 | 个人折叠/排序 |
| `channel_members.last_read_at` | 野马补 | 未读小红点唯一数据源 |

### 2.4 Artifact / 协作产出

| 表/列 | 来源 | 备注 |
|-------|------|------|
| `workspace_artifacts` | [canvas-vision §1.4](canvas-vision.md) | 替换文件树概念 |
| `artifact_versions` | canvas-vision | 每次 iterate 一行 |
| `artifacts.anchor_map` | [realtime §2.1](realtime.md) | progress 跳段 |
| `artifact.version` | §3 | **opaque string** 协议（[§3.A.3](#a-必修-5-条-接口-id-协议-lock-in)） |

### 2.5 Agent 接入

| 表/列 | 来源 | 备注 |
|-------|------|------|
| `agent_config`（含 schema-driven blob） | [plugin-protocol §1.4](plugin-protocol.md) | Borgee 是 SSOT |
| `plugin_connections` | plugin-protocol §1.1 | 一 plugin = 一 runtime |
| `runtime_schemas` | plugin-protocol §1.4 | runtime 上报，Borgee 渲染 |
| `agent_invitations(channel_id, agent_id, requested_by, state)` | [concept-model §4.2](concept-model.md) | 跨 org 审批 |
| `offline_mention_notifications` | [concept-model §4.1](concept-model.md) | 5 分钟节流 |
| `messages.subject` | realtime §1.1 | thinking 语义化必填 |

### 2.6 Host Bridge

| 表/列 | 来源 | 备注 |
|-------|------|------|
| `installed_runtimes` | [host-bridge §1.2](host-bridge.md) | 签名 manifest 校验记录 |
| `permission_grants`（4 类授权） | host-bridge §1.3 | install/exec/fs/network |
| `uninstall_audit` | host-bridge §1.2 | 可逆性追溯 |

### 2.7 Realtime

| 表/列 | 来源 | 备注 |
|-------|------|------|
| `channel_events` | §3.Q10.2 | per-channel 事件流 |
| `global_events` | §3.Q10.2 | 全局事件，必落清单见 §3.4 |
| `session_resume_hint` | realtime §1.3 | agent replay 模式持久化 |

### 2.8 Admin

| 表/列 | 来源 | 备注 |
|-------|------|------|
| `admin_grants(promoted_user_id, promoted_by, promoted_at)` | [admin-model §1.2](admin-model.md) | 长期保留 |
| `admin_audit` | admin-model §1.4 | 长期保留 |
| `impersonation_grants(user_id, granted_at, expires_at)` | admin-model §1.3 | 24h 过期 |

### 2.9 注册 / 邀请

| 表/列 | 来源 | 备注 |
|-------|------|------|
| `invitations(code, role, invited_by, used_by, expires_at)` | 野马补，对应 PRD P4 | 跟 admin_grants 语义不同 |

### 2.10 不入库的概念

- ❌ `agent_capability_bundle`：UI 模板，不存数据库（[auth-permissions §1.1](auth-permissions.md)）
- ❌ Role enum：`role` 只用 `'admin' / 'agent' / 'member'`，不引入 PM/Dev/QA 角色

---

## 3. 关键设计决策

### 3.1 Q10.2 — Events：双流进 SQL + hub 实时 + 90 天 retention

#### 核心架构纠偏

> **events 表 ≠ 实时通道**。
> server-go **in-process hub** 做实时 fan-out；SQLite events 表只负责**持久化 + 断线 replay**。两者解耦——SQLite 故障不影响实时性，只影响 replay。

#### 双流分表

| 表 | 用途 |
|----|------|
| `channel_events` | per-channel 事件流，full replay 模式 |
| `global_events` | 全局事件流，必落清单点名（§3.4） |

cursor 协议同形（`kind + ulid`），客户端按订阅集合 merge。

#### 为什么不引 MQ（v1）

1. **架构纠偏**：实时通道根本不走 SQLite，换 NATS 是同 pattern 换组件，无收益
2. **量级**：v1 末估 1M events/天 ≈ 12 EPS 平均，SQLite WAL 上限 ~1k EPS，十倍冗余
3. **哲学一致**：v1 "单 binary + 单 DB 文件，部署 5 分钟"，加 NATS = +进程 +认证 +持久化目录 +HA 考虑
4. **C 双写是真坑**：outbox pattern 解一致性又加复杂度，v1 不值

#### Retention：90 天滚动

- 后台 cron 每天 `DELETE WHERE created_at < now() - 90d`
- WAL checkpoint 后跑，业务无感
- **长期保留表（不受 retention 影响）**：`admin_audit` / `impersonation_grants` / `agent_invitations` / `admin_grants` / `permission_grants` 历史

### 3.2 Q10.3 — 迁移：标准版本化（B）

- `schema_migrations(version, applied_at, checksum)` 表
- 编号 SQL 文件（`001_init.sql` / `002_add_artifacts.sql` …）
- 启动时自动 apply，**失败 fail-fast**
- **forward-only**——不写 down

> ⚠️ **回滚策略与阶段挂钩**：v0（无外部用户）阶段直接删库重建；上线第一个外部用户后切到"备份 + 不可逆 forward-only"模式。详见 [`../implementation/README.md`](../implementation/README.md) 的阶段策略。

### 3.3 Q10.4 — 存储：SQLite + 观察哨

- v1 守 SQLite 单机 + WAL
- artifact 并发由 server-go **单写者协议**保护（不是 SQLite 问题）

### 3.4 Global events 必落清单（产品硬要求）

不能泛指"全局事件"，必须点名——这是 [admin-model §1.4](admin-model.md) 隐私契约的承载：

| 必落 kind | 理由 |
|-----------|------|
| 权限 grant / revoke | Q9.4 用户必须知道权限变化 |
| impersonate 开始 / 结束 | Q9.3 红色横幅 + 24h 过期承诺 |
| agent 上 / 下线（状态切换） | Q4.3 团队感知视图 |
| admin 操作受影响项（force delete / disable） | Q9.4 "用户始终知道与自己相关的事" |

---

## 4. 分布式 Ready 三层（Q10.5 核心）

> v1 分布式 ready 的边界 = **"接口与 ID 协议"**——这两层 lock-in 必须现在做对；实现细节可以单机，Q10.4 阈值触发后再切。

### A. 必修 5 条（接口/ID 协议 lock-in）

| # | 决策 | 一句话理由 |
|---|------|-----------|
| 1 | **ID 方案 = ULID**（所有业务表主键，禁 INTEGER PK） | 分布式无冲突 + cursor 单调 |
| 2 | **Cursor 协议 = opaque string**，服务端编码不外露 | 切换 cursor 实现不破协议 |
| 3 | ⭐ **artifact.version = opaque string** | v1 实现 INTEGER stringify（单写者串行），未来切 HLC 不破协议——**协议 lock-in 的代表案例**（野马点名） |
| 4 | **events lex_id = ULID** | 同 #1 |
| 5 | **EventBus interface**（Publish/Subscribe），v1 实现 in-process map | 未来换 NATS/Redis 不动业务 |

### B. 可换 4 条（接口抽象，迁移低成本）

| # | 决策 | 实现切换路径 |
|---|------|-------------|
| 6 | **Repository pattern** 封装 `last_read_at` | v1 SQLite，多节点切共享 KV |
| 7 | **agent_config 热推送**靠 #5 EventBus | EventBus 切多节点自动覆盖 |
| 8 | **Storage interface**（getUrl/putBlob/delete） | v1 本地 fs，未来对象存储 |
| 9 | **PresenceStore interface** | v1 in-memory map，多节点 Redis SET |

### C. 必重写 3 条（v1 不投入）

| # | 决策 | 备注 |
|---|------|------|
| 10 | SQLite → PG/CockroachDB | v1 写标准 SQL，**不写 ORM 抽象**，但避免 SQLite-only 函数（grep 范围可控） |
| 11 | EventBus 实现切 NATS/Redis Streams | 留 #5 接口，不投入实现 |
| 12 | Rate limiting 全局视图 | v1 单机内存计数，切换时直接重写 |

### 否决项

- ❌ **Snowflake**（需中央协调，ULID 够）
- ❌ **Vector clock**（P2P CRDT 场景，Borgee 用不上）

### v1 分布式承诺（精确表述）

- ✅ **协议层 portable**：ID / cursor / artifact 版本 / event bus 上线即多节点 ready，数据迁移零破坏
- ✅ **接口层抽象**：Storage / Presence / EventBus / Repository 四个 interface，实现可换不动业务
- ❌ **运行时单机**：SQLite + in-process hub + local fs，**不支持多节点同时跑**

---

## 5. 阈值哨

> **凭指标切，不凭感觉切。** 同一套阈值适用 SQLite→PG、events→MQ、in-memory→Redis。

| 指标 | 触发动作 |
|------|---------|
| WAL checkpoint 滞后 > 10s | 一级警报 |
| Write lock wait > 100ms | 一级警报 |
| **DB 大小 > 3GB** | ⚡ 切换准备期：启动 PG / MQ 调研 |
| **DB 大小 > 5GB** | 🔥 必须切换：PG / MQ 上线 deadline |
| events 写 QPS / replay 延迟 / 单表行数 | 触发评估 MQ 引入 |

后台 admin dashboard 显示这些指标，admin 可视。

---

## 6. 与现状的差距

| 目标态 | 现状 | 差距 |
|--------|------|------|
| 完整数据要素（30+ 表/列） | 8 张主表 | 大量新建（按 §2 拆分到各模块） |
| ULID 全表 | INTEGER PK 大量使用 | **大改**：全 schema 切 ULID（v1 必修） |
| Cursor opaque string | INTEGER cursor 暴露 | 协议层加 base64/字符串包装 |
| EventBus / Storage / Presence interface | 直接调实现 | 加 interface 抽象层 |
| events 双流 + 90 天 retention | 单 events 表 + channel_id NOT NULL + 无 retention | 拆表 + 加 retention cron |
| schema_migrations 版本化 | 幂等 IF NOT EXISTS | **重写迁移机制** |
| 5 个量化预警 | 无 metrics | 加 metrics 收集 + admin dashboard |
| Global events 必落清单 | 多种事件不入表 | 实施时按清单逐一改写写路径 |

---

## 7. 不在本轮范围

- 各 admin dashboard 指标的具体阈值数值微调 → 上线观察后调
- ULID 库选型（标准 ULID / KSUID / UUIDv7） → 实施时定
- 对象存储 backend 选型（S3 / R2 / minio） → §B.8 触发时
- PG / CockroachDB 切换的具体迁移脚本 → §C.10 触发时
