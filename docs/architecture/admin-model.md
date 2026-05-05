# Implementation · Admin Model

> 蓝图: [`../../blueprint/current/admin-model.md`](../../blueprint/current/admin-model.md)
> 现状: `users.role='admin'` 与普通 user 同表; cookie 共享; `/admin-api/v1/*` 走同一 session; 无独立 SPA, 无明确隐私契约, 无分层透明
> 阶段: ⚡ v0
> 所属 Phase: ADM-0 在 **Phase 2** 落地 (闸 #2 admin 路线决议派生); ADM-1/2/3 仍在 Phase 4

## 1. 现状 → 目标 概览

**现状**: admin endpoint 嵌在主 API, admin 是 `users.role='admin'` 行, cookie 与普通 user 串, 没独立 UI, 隐私边界不清。
**目标**: blueprint R3 四条立场 — admin **完全独立身份** (蓝图 #188 §1.2 锁定: B env bootstrap, 无 promote, `admins` 独立表), 独立 SPA (§1.1), 硬隔离 (god-mode endpoint **仅元数据**, 绝不返回 `message.body` / `artifact` 内容, §1.3 + §2 不变量), 分层透明 (§1.4)。

## 2. Milestones

### ADM-0: admin 拆表 (admins 独立表 + cookie 拆 + god-mode 元数据-only)

> Status (2026-04-28): ADM-0.1 ✅ DONE (#197) / ADM-0.2 ✅ DONE (#201) / ADM-0.3 🔄 IN PROGRESS (task #63, v=10)

- **目标**: blueprint admin-model §1.2 + §1.3 + §3 — `users.role` 收成二态 (`'member' | 'agent'`); admin 迁到独立 `admins` 表; admin 走独立 cookie + `/admin-api/auth/login` env bootstrap; god-mode endpoint 强制只返回元数据。
- **Owner**: 飞马 (review 边界 / 数据契约) / 战马 (dev) / 烈马 (cookie 串扰反向断言, 一票否决)
- **范围** (拆 3 段 PR, **顺序串行** — 飞马 R3 R1 修正: 先建表, 再切 cookie 路径, 最后才砍 users.role 老分支):
  - **ADM-0.1 admins 独立表 + `/admin-api/auth/login` env bootstrap**: 新增 `admins(id, login, password_hash, created_at)`; `cmd/server` 启动时读 env (`BORGEE_ADMIN_LOGIN` / `BORGEE_ADMIN_PASSWORD_HASH`) bootstrap 第一个 admin; 独立 cookie name `borgee_admin_session`; `internal/admin/auth.go` 与 `internal/auth.go` 完全分裂 (不共享 store / middleware)。**此阶段 users.role='admin' 仍可登录, 双轨并存。**
  - **ADM-0.2 cookie 拆分 + `RequirePermission` 去 admin 短路 + god-mode 元数据-only**: `RequirePermission` 中间件移除"role='admin' 直通"分支; `/admin-api/v1/*` 改吃 admin cookie; god-mode endpoint (org list / user list / channel list / count / status) 加 response struct 白名单, **绝不携带** `message.body` / `artifact.content` 字段 (单测固化); 客户端 admin SPA 改吃新 cookie path。**此阶段 users.role='admin' 调 user-api 401, 但还在表里。**
  - **ADM-0.3 `users.role` enum 收成二态 + backfill**: schema_migrations v=N — `ALTER TABLE users ... CHECK (role IN ('member','agent'))`; backfill 扫 `users WHERE role='admin'` → 在 `admins` 表里建对应行 (复用同 login/hash) + 删 user 行 + revoke 该 user 所有 session; testutil/server.go fixture 改造 (`role=admin` 全删)。**此阶段 users.role='admin' 行数恒为 0。**
- **不在范围**:
  - ADM-1 用户隐私承诺页 (野马 P2 文案, 派生 milestone)
  - ADM-2 分层透明 audit log (派生)
  - impersonation_grants (蓝图 §3 v1+, 不在 v0)
  - admin SPA 独立打包 / 路由分裂 (ADM-1 范围)
- **依赖**: blueprint R3 (#188) 已 merged; 不挡 CM-1 / CM-4, 但**挡 CM-3** (org_id 直查的 admin 分支假设)
- **预估**: ⚡ v0 server 4-6 天 + client 1 天 (≤ 800 LOC, ~4 处代码点: `admin_auth.go:96` / `users.role` enum / testutil fixtures / 4 处 client UI 字符串校验)
- **Acceptance**:
  - **数据契约**: `admins` 表存在且字段固化 (id/login/password_hash/created_at); `users.role` enum 限定 `('member','agent')` (DB CHECK 约束 + Go 枚举); response struct 白名单表 (god-mode endpoint 列出每个返回字段, 无 message.body / artifact.content)
  - **行为不变量** (烈马一票否决, 反向断言必须全绿):
    - 4.1.a admin cookie 调任意 `/api/v1/*` (user-api) → **一律 401** (单测覆盖 messages / channels / agents 三端)
    - 4.1.b user cookie 调任意 `/admin-api/v1/*` (god-mode) → **一律 401** (单测覆盖 users / orgs / channels list 三端)
    - 4.1.c god-mode endpoint 返回 JSON 经反射扫描, 不存在 `body` / `content` / `text` 字段名 (单测 fail-closed)
    - 4.1.d `users WHERE role='admin'` 行数恒等于 0 (post-migration assertion)
  - **蓝图行为对照**: blueprint admin-model §1.2 (B env bootstrap, 无 promote) / §1.3 (god-mode 仅元数据) / §3 (admins 表 schema) 三条逐条挂引用

### ADM-1: admin SPA 独立 + 元数据/内容硬隔离 + 用户隐私承诺可见

- **目标**: blueprint §1.1 + §1.3 + 核心 §13 — admin 独立 SPA, 内容必须用户授权; **用户侧能读到 admin 能看 X / 看不到 Y 的承诺**。
- **Owner**: 飞马 (review 边界) / 战马 / 野马 (隐私立场) / 烈马
- **范围**:
  - 独立 admin SPA 路由
  - 中间件强制只允许"元数据"接口 (org / user 列表 / 计数 / 状态)
  - 内容接口 (messages / artifacts) 必须带用户授权 token
  - **用户设置页加"隐私承诺"区**: 显式列 admin 能看到 (org / user list / count / status) vs 看不到 (message body / artifact 内容) — 野马 P2
- **依赖**: CM-1 (org_id 聚合)
- **预估**: ⚡ v0 1-2 周
- **Acceptance**:
  - 行为不变量 4.1 (无授权访问内容 → 拒绝, 单测)
  - 数据契约 (元数据 vs 内容接口枚举表)
  - 用户感知截屏 4.2 (野马: 用户设置页"隐私承诺"区, 截 1 张, 验立场 §13)

### ADM-2: 分层透明 (用户可见性)

> 野马 R2: **取消 ⭐ 标志性** — 分层透明只对 admin 可感, 普通用户零感知, 留 ⭐ 浪费 PM 签字时间。降级为内部 milestone, 不进野马签字流。

- **目标**: blueprint §1.4 — 用户能看到 admin 对自己做了什么 (audit log 用户视角)。
- **Owner**: 野马 (立场主, 但不签字闸 4) / 战马 / 飞马 / 烈马
- **范围**: `admin_actions(actor_id, target_user_id, action, when)` 表; 用户设置页可看自己被 admin 影响的记录
- **依赖**: ADM-1
- **预估**: ⚡ v0 1 周
- **Acceptance**:
  - 行为不变量 4.1: 任何 admin action → 自动写一行 admin_actions (单测覆盖每种 action 类型)
  - E2E: admin 改用户 role → 用户设置页 audit 列表多一行

### ADM-3: ~~admin 来源 C 混合~~ → **已被 ADM-0 取代**

> 飞马 R3 备注: blueprint #188 把 §1.2 从 "C 混合 (用户升级 + admin key)" 改成 **"B env bootstrap, 无 promote"**, 不再走 `is_admin` flag。本 milestone 内容 (用户升级路线) 与 R3 立场冲突, **标 obsolete**, 由 ADM-0 §PR-(b) 的 `admins` 独立表 + env bootstrap 全量替代。
> 保留行用于 review 追溯, 不再排期。

## 3. 不在 admin-model 范围

- 多 admin 角色 (super-admin / org-admin) ❌ v1+
- 跨 org admin ❌ (蓝图明确禁止)

## 4. Blueprint 反查表

| Milestone | §X.Y | 立场一句话 |
|-----------|------|-----------|
| ADM-0 | admin-model §1.2 + §1.3 + §2 不变量 + §3 | admin 独立身份 (admins 表 + env bootstrap + cookie 拆) + god-mode 仅元数据 |
| ADM-1 | admin-model §1.1 + §1.3 + 核心 §13 | 独立 SPA + 元数据/内容硬隔离 + 用户侧隐私承诺可见 |
| ADM-2 | admin-model §1.4 | 分层透明, 用户看到自己被影响什么 |
| ADM-3 | ~~§1.2 C 混合~~ | **obsolete** — 被 ADM-0 取代 |
