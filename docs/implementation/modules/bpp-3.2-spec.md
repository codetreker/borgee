# BPP-3.2 spec brief — `permission_denied` plugin UX 流 (owner DM + 一键 grant + 自动重试)

> 战马C · 2026-04-29 · ≤80 行 · Phase 5 plugin-protocol 第三段 (BPP-3.1 #494 真后续, 4 件套 spec 第一件)
> 关联: BPP-3.1 #494 PermissionDeniedFrame (server→plugin) ✅ + AP-1 #493 abac.go::HasCapability + capabilities.go const + DM-2 #361/#372/#388 message_mentions + system DM 路径 ✅ + 蓝图 [`auth-permissions.md`](../../blueprint/current/auth-permissions.md) §1.3 主入口 (动态请求 → server 给 owner 写 system message → 一键 grant) + §2 不变量 "Permission denied 走 BPP" 闭环
> Owner: 战马C 后续整 milestone 实施 (3 PR 拆 / 同 branch 叠 commit)

---

## 0. 关键约束 (3 条立场, 蓝图 §1.3 主入口字面)

### 立场 ① — plugin → owner DM 走现有 DM-2 路径 (不另起 channel 类型)
- plugin 收 `permission_denied` frame → plugin client SDK 解析 `{required_capability, current_scope, request_id, attempted_action}` → 通过 BPP-2.1 dispatch 层 `request_agent_join` / 或新 `request_capability_grant` semantic op (复用 BPP-2.1 7 op + 1 扩到 8 op) → server 触发 owner DM 写 system message
- system DM 走 DM-2 #372 既有 `message_mentions` 路径 + onboarding `quick_action` 字段 (CM-onboarding #203 既有 schema), **反约束**: 不开新 channel 类型 / 不开 plugin-only 通道 / 不写新 system_message_kind enum
- 文案锁 byte-identical 跟蓝图 §1.3 字面: `"{agent_name} 想 {attempted_action} 但缺权限 {required_capability}"` + 三 quick_action button: `[✓ 一键 grant 限于此 channel] [✗ 拒绝] [⚙ 高级设置]`

### 立场 ② — owner 一键 grant 真改 user_permissions (走 AP-1 const, 不 hardcode)
- owner 点 quick_action `grant` → server 调既有 `Store.GrantPermission(&UserPermission{UserID: agentID, Permission: <const>, Scope: <scope>})` (AP-1 #493 既有, 不另起 grant API)
- `Permission` 字面必走 `auth.Capabilities` const 白名单 (AP-1 #493 14 项: read_channel/write_channel/.../change_role); 反约束: 不 hardcode 字面 (反向 grep `GrantPermission.*"[a-z_]+"` count==0, 跟 AP-1 反约束 #1 同源)
- `Scope` byte-identical 跟 BPP-3.1 frame `current_scope` 字段 (`channel:<id>` / `artifact:<id>` / `*`); 反约束: scope 不漂出 v1 三层 (跟 AP-1 反约束 #4 同源)
- grant 后 server 推 BPP-2.3 `agent_config_update` frame 触发 plugin 重读权限 (AL-2b #481 既有路径), **反约束**: 不另起 `capability_granted` BPP frame (蓝图 §4.2 提到但 v1 不做, 复用既有 config_update 路径)

### 立场 ③ — 自动重试 ≤3 次 30s 退避 (跟 BPP-4 timeout/retry 立场对齐)
- plugin 收 `permission_denied` 后 dispatcher 缓存 `request_id → 原 semantic_action frame` (内存 map, 不持久化)
- owner grant 后 plugin 收 `agent_config_update` frame → 检 cache → 自动 retry 原 action; **退避**: 30s 固定 (反约束: 不指数退避, 跟 BPP-4 watchdog 60s heartbeat 同精神 — server-side timing 单源)
- 上限 ≤3 次重试; 超限 abort + plugin log warn `bpp.retry_exhausted` (跟 `bpp.task_subject_empty` / `bpp.config_field_disallowed` 错码命名同模式)
- 反约束: retry 路径不复用 BPP-4 watchdog 队列 (BPP-4 是 server→plugin heartbeat 反向, BPP-3.2 是 plugin 端 in-memory 缓存)

---

## 1. 拆段实施 (3 段, ≤3 PR 同 branch 叠 commit)

| 段 | 范围 | 闭锁 | owner |
|---|---|---|---|
| **BPP-3.2.1** plugin 收 frame + DM dispatch | server-side: 新 `request_capability_grant` semantic op 加入 `ValidSemanticOps` (BPP-2.1 7→8); handler 调 DM-2 写 owner system DM (复用 `message_mentions` + `quick_action` JSON payload `{action: 'grant', agent_id, capability, scope, request_id}`); 文案 byte-identical 跟蓝图 §1.3 + 野马签字; 反约束 grep `system_message_kind` 不新增 enum + DM 路径不另起 | 待 PR (战马C 实施) | 战马C / 野马 文案 |
| **BPP-3.2.2** owner DM UI 一键 grant | client SPA: SystemMessageBubble 渲染 quick_action 三按钮 (跟 CM-onboarding QuickActionButton 同模式); `grant` 路径 POST `/api/v1/me/grants` (新 endpoint, 复用 `Store.GrantPermission`); body 走 AP-1 `auth.Capabilities` 白名单校验 (枚举外值 reject + log warn `bpp.grant_capability_disallowed`); 反约束 e2e: hardcode 字面 / scope 漂出 v1 三层 0 hit | 待 PR (战马C + 野马 UI 文案) | 战马C / 野马 |
| **BPP-3.2.3** plugin 自动重试 + e2e + closure | plugin SDK: `RequestRetryCache` (`map[requestID]*SemanticActionFrame` + TTL 5min); on `agent_config_update` 触发 cache 扫 → retry; 30s 固定退避 + ≤3 次 + `bpp.retry_exhausted` 错码; e2e: agent commit_artifact 无权 → owner DM 一键 grant → plugin 自动 retry 成功 真路径; REG-BPP32-001..N + acceptance + PROGRESS [x] + docs/current sync | 待 PR (战马C) | 战马C / 烈马 验收 |

---

## 2. 留账边界 (不接 v2+)

- ABAC v2 condition (e.g. time-based / IP-based) — v1 仅 `(user_id, permission, scope)` 三元组, BPP-3.2 不引入条件位
- multi-owner grant — v1 单 owner 模型 (跟 AP-1 #493 立场承袭), 多人审批留 v3+
- grant 历史 audit UI (admin SPA) — 走 ADM-2 #484 既有 `admin_actions` audit 路径 (admin god-mode 看, 业务面不暴露; ADM-0 §1.3 红线)
- 拒绝 (`reject` quick_action) 路径仅 dismiss DM (不持久化反向 grant); v2+ 加 deny list 时另议
- BPP-2.1 `request_capability_grant` 加入 8 op 必同步更新 BPP-2 spec §0 立场 ① + content-lock §1 ① 字面 (改 = 改三处+: spec + content-lock + dispatcher.go ValidSemanticOps + 此 spec)

---

## 3. 反查 grep 锚 (5 反约束, count==0)

```bash
# 1) hardcode capability 字面 (走 AP-1 auth.Capabilities const)
git grep -nE 'GrantPermission.*Permission:.*"[a-z_]+"' packages/server-go/internal/api/  # 0 hit (走 auth.<Const>)

# 2) DM 路径不走 DM-2 既有 (反约束: 不开新 channel 类型)
git grep -nE '"capability_request"|"permission_denied_dm"|system_message_kind.*permission' packages/server-go/internal/  # 0 hit

# 3) retry 路径复用 BPP-4 watchdog 队列 (反约束: in-memory cache, 不持久化)
git grep -nE 'BPP-?4.*retry|watchdog.*permission_denied' packages/plugin-sdk/  # 0 hit

# 4) scope 漂出 v1 三层 (跟 AP-1 反约束 #4 同源)
git grep -nE 'Scope.*"workspace:|Scope.*"org:' packages/server-go/internal/api/  # 0 hit

# 5) capability_granted 新 BPP frame (反约束: 复用 AL-2b agent_config_update)
git grep -nE 'FrameTypeBPPCapabilityGranted|"capability_granted"' packages/server-go/internal/  # 0 hit
```

---

## 4. 不在范围

- BPP-4 timeout/retry watchdog (server-side heartbeat, 跟 BPP-3.2 plugin in-memory cache 拆三路径)
- AP-3 cross-org owner-only 强制 (后续 milestone)
- ABAC v2 condition / multi-owner / deny list / grant 历史 UI (留 v2+)
- plugin SDK 跨语言 retry cache 实现 (本 spec 锁 server-side 协议 + 一个 reference plugin 实现, 其它语言 SDK 跟 spec 字面对齐 follow-up)

---

## 5. 跨 milestone byte-identical 锁

- 跟 BPP-3.1 #494 frame 字段 byte-identical (`required_capability` + `current_scope` + `request_id` 三处串通: BPP-3.1 frame body → DM-2 quick_action payload → AP-1 GrantPermission 入参)
- 跟 AP-1 #493 abac.go::Capabilities const 白名单同源 (改 = 改三处: AP-1 capabilities.go + BPP-3.2 GrantPermission validator + acceptance ap-1.md §3.1)
- 跟 CM-onboarding #203 quick_action JSON payload schema 同模式 (`{action, ...args}` 字面)
- 跟 ADM-0.1 §1.3 红线: admin god-mode 不入此路径 (admin grant 走 /admin-api 单独 mw, 不走 owner DM)

---
