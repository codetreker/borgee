# Admin Model — 管理面与隐私契约

> Admin 是 Borgee 在产品模型外的运维身份。本文规范 admin 的目标态——尤其是它**能看什么不能看什么**这条隐私契约。
> 状态：建军 + 飞马 + 野马 对齐（2026-04-27）。前置阅读：[`concept-model.md`](concept-model.md)、[`auth-permissions.md`](auth-permissions.md)。

## 0. 一句话定义

> **Borgee admin 强权但不窥视——元数据可管、内容不可读、用户始终知道与自己相关的事。**

---

## 1. 目标态（Should-be）— 四条立场

### 1.1 Admin 形态：独立 SPA（B）

- Admin **不是** org 成员（[concept-model §2](concept-model.md) 已钉）——不是 user，不属于任何 org。
- 物理隔离的运维 SPA + 独立鉴权域（独立 cookie / 独立路由）。
- 这条把现状从"奇怪历史包袱"升级成"有意设计"——**身份模型干净**：
  - 产品代码不为 admin 写分支（admin 没有 agent、没有 channel、不在 user 表的 role 里走 product 流程）
  - 否则会引爆"admin 有没有自己的 agent / org / DM"等连锁矛盾

### 1.2 Admin 来源：B (env bootstrap, 无 promote)

> **2026-04-28 4 人 review 立场冲突 #2 决议 (B29 路线)**: 撤销原 "promote 已有 user" 路径; admin **完全独立于 users 表**, 只通过 env 创建。

| 阶段 | 入口 |
|------|------|
| **首个 admin** | 环境变量 `ADMIN_USER` / `ADMIN_PASSWORD` bootstrap | 防空表自举漏洞, 唯一启动入口 |
| **后续 admin** | 现有 admin 在 SPA 里**新建 admins 行** (输入用户名 + 密码, 不是 promote 任何 user) | 走 audit log |

#### 数据模型

- **独立 `admins(id, username, password_hash, created_at, created_by, last_login_at)` 表**
- ❌ ~~`users.role = 'admin'`~~ — 撤销, users.role 现在只有 `('member','agent')`
- ❌ ~~`admin_grants(promoted_user_id, ...)` 表~~ — 撤销, admin 不再 promote 自 user
- `admin_audit(admin_id, action, target_type, target_id, metadata, created_at)` 保留

> **关键收益 (飞马 R 锁定)**: admin 走独立 cookie + 独立 auth path, 不再有"admin 拿 user cookie 走 user-api"的边角漏洞; CM-3 (org_id 直查) 之后再也不用考虑"admin 没 org 怎么办"。

### 1.3 边界：硬隔离 + 内容必须用户授权 + admin 走独立 god-mode endpoint

> **2026-04-28 飞马盲点 A1 加补**: admin 看 channel **元数据** (列表/成员/计数) 走 `/admin-api/channels/:id` (god-mode 路径, 跳过 capability 检查), **不复用 user-api**。
> ⚠️ **god-mode 仅暴露元数据**, **绝不返回 message.body / artifact 内容**; admin 读消息正文仍必须走 §1.3 的 impersonation 路径 (用户主动 24h grant)。这是平台运维语义, 跟普通用户的 capability gate 是两套体系, 但都不破坏 §0 "内容不可读" 契约。

#### Admin **可看**（元数据层）

- ✅ 用户列表（含禁用/删除状态）
- ✅ Channel **元数据**：id、名、成员列表、创建时间、消息计数
- ✅ 统计：在线数、活跃数、容量等
- ✅ API key 状态（是否存在、最后使用时间，**不是 key 本身**）
- ✅ Agent 数 / runtime 状态

#### Admin **不可看**（内容层）

- ❌ 任何 channel 的消息内容
- ❌ 任何 DM 的内容（**包括 owner-agent 内置 DM**）
- ❌ artifact 内容
- ❌ 用户上传的文件内容

#### Admin **可做**（合法运维动作）

- ✅ Force delete channel
- ✅ 重置 user API key
- ✅ 改 user 密码（user 下次登录被强制）
- ✅ Disable / enable user
- ✅ Soft delete user（[server §existing](server/data-model.md)）

#### Impersonate（保留，但严格）

- **不**默认开启
- 用户在设置面**主动**勾选"允许 support 调试 24h"
- 仅限 24h 时窗，到期自动失效
- 范围：**只读实时入站，不读历史 DM，不能代发消息**
- 用户可随时撤销

#### 核心契约

> **"管控元数据 = OK，读内容 = 必须用户主动授权"**

这条边界跟 [host-bridge §1.3](host-bridge.md) 的"装时轻、用时问、问时有理由"同范式——Borgee 默认信任用户的隐私。

> **ADM-3 #570 立场扩展 (audit-forward-only 全 actor type)**: audit 单表 (`audit_events`, ADM-3.1 v=43 RENAME 自 `admin_actions`) 装所有 actor type — admin / system / plugin lifecycle / 未来类型, forward-only **不可 DELETE/UPDATE** (sweeper archive 走 `archived_at`); admin 仍走 `/admin-api/*` 单独路径 (god-mode endpoint). 别名 view `admin_actions` 留 backward compat 至 Phase 5+ deprecation.

### 1.4 可见性：分层透明（A + C 组合）

> 实质方案：**100% 留痕** + **按受众分层**——既杜绝 admin 滥用，又不在跨 org 场景泄漏隐私。

#### 谁能看到什么

| 谁 | 能看到 |
|----|--------|
| **受影响者** | 必收 system message：例如"你的 channel #foo 被 admin 张三于 X 时间删除"。强制下发，不可关闭。 |
| **被 impersonate 用户** | 顶部**红色横幅**："support 张三正在协助你，剩 23h"。横幅常驻直到结束。 |
| **admin 之间** | 全部 audit log 互相可见——防内部滥用 |
| **普通 user** | 只能看到**与自己相关的** audit 条目（我的 channel 被删 / 我被封 / 我被 impersonate） |
| **没人能看** | 全站 audit log **不对全体 user 公开**——避免跨 org 隐私泄漏 |

#### 为什么不是"全员透明"

- 字面执行 = 野马 org 的 channel 被删，建军 org 的所有 user 都看到 → 跨 org 隐私破壞
- "用户始终知道与自己相关的事" 是契约，不是"所有人知道所有人发生了什么"

#### 三条用户感知的红线（不可让步）

1. ✅ 受影响者**必收**通知（不能静默）
2. ✅ Impersonate **必须显眼**（红色横幅 + 倒计时）
3. ✅ Admin 之间**互相留痕**（防 admin 互相包庇）

---

## 2. 关键不变量

| 不变量 | 含义 |
|--------|------|
| Admin ∉ users 表 | admin 走独立 `admins` 表, 永不出现在 users 表 (4 人 review #2 决议, 2026-04-28) |
| Admin ∉ Org | admin 永远不是任何 org 的成员，也不出现在 channel members |
| Admin 走独立 cookie | admin SPA 用独立 cookie name, 不与 user cookie 冲突; admin 永远不能调 user-api |
| Admin 默认看不到内容 | 必须用户主动 grant impersonate 才能"看进去" |
| Admin 看 channel 元数据走 god-mode endpoint | `/admin-api/channels/:id` 跳过 capability 检查 (平台运维语义), 不复用 user-api; **god-mode endpoint 绝不返回 message.body / artifact 内容**, 只返回元数据 (列表/成员/计数) — 飞马 P0 inter-doc 一致性补 |
| 受影响者**必感知** admin 操作 | 系统强制下发 system message，不能关闭 |
| Audit 100% 留痕 | admin 一切操作进 audit_log |
| Audit 分层可见 | admin 之间全可见，user 只见与己相关，全站不公开 |

---

## 3. 数据模型片段

```
admins:
  id, username, password_hash, created_at, created_by, last_login_at
  -- 完全独立于 users 表 (4 人 review #2 决议, 2026-04-28)
  -- 首个 admin 由 env 创建; 后续 admin 由现有 admin 在 SPA 新建 (不是 promote user)

admin_audit:
  id, admin_id, action, target_type, target_id,
  metadata (JSON), created_at

impersonation_grants:
  id, user_id, granted_at, expires_at, revoked_at
  -- 由 user 创建（设置面勾选）；admin 仅消费这条记录
```

> ❌ 撤销: ~~`admin_grants` 表~~ — admin 不再 promote 自 user, 不需此表。

## 4. 与现状的差距

| 目标态 | 现状 | 差距 |
|--------|------|------|
| 独立 `admins` 表 + 独立 cookie | users.role='admin' 共用 users 表 + 共用 cookie | **拆表 + 拆 auth path** (4 人 review #2 决议, 2026-04-28; 实施见 implementation/admin-model.md ADM-0) |
| Admin 走独立 god-mode endpoint | admin SPA 复用部分 user-api + 走 admin_grants 短路 | **新建 `/admin-api/channels/...` 等独立 endpoint, 跳过 capability 检查** |
| 硬隔离内容访问 | admin force delete 不读内容，但 API 层无显式限制 | server 加 admin 白名单 endpoint，明确**禁止** admin 调任何"读内容"的 user-facing API |
| Impersonate 主动授权 | 不存在 | 设置面开关 + `impersonation_grants` 表 + 24h 过期机制 + 顶部红色横幅 UX |
| 受影响者 system message | force delete 已有通知 | 扩展到所有 admin 写动作（封禁、重置、改密码等） |
| Audit log + 分层可见 | 无 audit | 新建 `admin_audit` 表 + admin SPA 列表 + per-user 自助查询 |
| 用户侧"隐私承诺"页 | 不存在 | ADM-1 加用户设置页"admin 完全不在你的协作圈"截屏 (野马盲点 B1, 2026-04-28) — 文案见 §4.1 锁定 |

### 4.1 用户侧隐私承诺页文案 (ADM-1 acceptance 硬标尺)

> **2026-04-28 野马 R3 锁定**: ADM-1 用户设置页必须含以下 3 条承诺文案 (一字不漏 / 顺序不变); ADM-1 截屏 acceptance 必须包含此页 + 至少 1 张 admin 写操作 system message 通知截屏, 否则 §13 不签。

1. **Admin 是平台运维, 不是协作者** — 永不出现在 channel / DM / 团队列表里。
2. **Admin 看不到消息 / 文件 / artifact 内容** — 除非你主动授权 impersonate (24h 时窗, 顶部红色横幅常驻, 可随时撤销)。
3. **Admin 能看的是元数据** (用户名 / channel 名 / 条数 / 登录时间), **看不到正文**。

> 文案目的: 让用户在第一次进设置页就能读懂"admin 跟我有什么关系", 不需要看 docs / 不需要问客服。这条是 §0 "强权但不窥视" 立场对用户的兑现。

---

## 5. 不在本轮范围

- Admin SPA 的具体页面 UI → 第 11 轮"Client (web SPA)"
- 多 admin 协作流程（互相 promote / demote） → v2
- Audit log 的导出 / 合规对接（GDPR delete request 等） → v2+
- Impersonate 期间的 BPP 行为（agent 端是否知道被 impersonate） → 第 5 轮已确定 BPP 不直接暴露此概念，agent 行为不变
