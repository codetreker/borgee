# AL-5 spec brief — agent 错误恢复路径 wrapper milestone

> 战马C · 2026-04-29 · ≤80 行 spec lock (4 件套之一; Phase 5 候选, 跟 BPP-3.2 同期)
> **蓝图锚**: [`agent-lifecycle.md`](../../blueprint/current/agent-lifecycle.md) §2.3 (5-state error 状态字面 — `error → online` 恢复路径) + §1.6 (失联与故障状态: BPP-4 watchdog 检测异常翻 state, AL-5 处理后续恢复) + [`auth-permissions.md`](../../blueprint/current/auth-permissions.md) §1.3 主入口字面承袭 (owner DM 通知机制 BPP-3.2 已落)
> **关联**: AL-1 #492 5-state state machine + ValidateTransition (`error → online` valid edge 已锁) + BPP-4 watchdog spec (timeout 翻 error) + BPP-3.2 #498 owner DM 通知机制 (复用 DM-2 + system message 既有 path) + REFACTOR-REASONS #496 SSOT (6 reason 字面单源, AL-5 不新增字典)
> **命名**: 原"AL-3"已被 presence (Phase 3 #310/#317/#324/#327 三段全闭) 占用; 此新 wrapper milestone 命名 AL-5 避免 registry 命名冲突

> ⚠️ AL-5 是 **wrapper milestone** (跟 AL-1 #492 wrapper 同模式) — 复用既有 5-state graph + 6-reason 字典 + DM-2 既有 path + AL-1 helper, **不裂新组件**, 仅补 owner-facing recovery 路径.

## 0. 关键约束 (3 条立场, 蓝图字面承袭)

1. **错误状态 owner 可见 = system DM 通知** (蓝图 `auth-permissions.md` §1.3 主入口字面 + BPP-3.2 既有 owner DM 路径承袭): agent state 由 BPP-4 watchdog (或手动) 翻 error → server 调既有 `Store.SendSystemDM(ownerID, body, quickActionJSON)` (复用 BPP-3.2.1 + CM-onboarding #203 既有 path); **反约束**: 不开新 channel 类型 / 不写新 system_message_kind enum / DM 走 owner 既有 type='system' channel
2. **错误恢复 = 单 helper SSOT** (跟 AL-1 #492 `AppendAgentStateTransition` 同模式 — server-side state set 唯一入口走 ValidateTransition gate): 新建 `agent.RecoverFromError(agentID, reason)` helper 走既有 5-state graph `error → online` valid edge; **反约束**: 不另起 recovery 状态机 / 不在 5-state 加新态 / 不裂 ValidateTransition; reason 字典复用 REFACTOR-REASONS #496 SSOT (6 字面 + IsValid)
3. **recovery reason 不另起字典** (REFACTOR-REASONS #496 SSOT 同源): recovery 路径 reason ∈ AL-1a 6 字面 (api_key_invalid / quota_exceeded / network_unreachable / runtime_crashed / runtime_timeout / unknown); 反约束: 不新增 `recovery_in_progress` / `auto_reconnect` 等中间态字面 (反向 grep `reasons.IsValid` 守 future drift)

## 1. 拆段实施 (AL-5.1 / 5.2 / 5.3, ≤3 PR 同 branch 叠 commit)

| 段 | 范围 | 闭锁 | owner |
|---|---|---|---|
| **AL-5.1** server 错误状态 owner system DM 通知 | `internal/api/agent_recover.go` 新 `notifyOwnerOnError(agentID, reason)` (BPP-4 触发后或手动); 调既有 `Store.SendSystemDM` + DM body byte-identical 跟蓝图 §1.6 文案锁 (e.g. `"{agent_name} 状态变更: error ({reason_label}). 点击重连"`); quick_action JSON shape `{action: 'recover', agent_id, reason, request_id}` (跟 BPP-3.2 quick_action shape 同模式, action 新加 `recover` 4-enum); 6 unit (DM body 字面 + quick_action shape + reason ∈ 6 dict + DM 不另起 channel + admin path 不挂 + 反向 grep recovery 字典 0 新增) | 待 PR (战马C) | 战马C / 野马 文案 |
| **AL-5.2** client SPA "重连" 按钮 + recovery endpoint | `POST /api/v1/agents/:id/recover` body `{request_id}` (无 reason — server 端读 agent_state_log 最近 error reason); owner-only ACL (跟 BPP-3.2.2 me_grants 同模式); 调 `agent.RecoverFromError(agentID, lastReason)` helper → `AppendAgentStateTransition(agentID, error→online, lastReason)` (复用 AL-1 #492 single-gate); SystemMessageBubble 检测 quick_action.action='recover' → 渲染单按钮 "重连" (DOM `data-action="recover"`); 6 unit + 4 vitest (endpoint 200 happy + non-owner 403 + 错误 reason 字典外 reject + agent 非 error 态 reject + 单按钮 DOM 字面 + 反向 grep "重启"/"reset"/"restart" 同义词禁词 0 hit) | 待 PR (战马C) | 战马C / 野马 |
| **AL-5.3** e2e + closure | server-side full-flow integration test (agent state error → owner DM → owner POST /agents/:id/recover → state error→online → agent_state_log row + AL-1 ValidateTransition gate 守); registry §3 REG-AL5-001..N + acceptance + PROGRESS [x] AL-5 + docs/current sync (server/agent-state.md §recover + client/ui/message.md §4g) | 待 PR (战马C) | 战马C / 烈马 |

## 2. 留账边界 (不接 v2+)

- v2 retry queue (留 BPP-4 watchdog) — AL-5 仅处理 owner-driven manual recovery + BPP-3.2.3 plugin in-memory cache 已覆盖 plugin 端; server-side persistent retry queue 是 BPP-4 范围 (拆三路径: BPP-4 watchdog server 检测 / AL-5 owner manual recovery / BPP-3.2.3 plugin in-memory)
- cross-org admin recover (留 ADM-3+) — AL-5 仅 owner-only (跟 BPP-3.2 同精神, admin god-mode 走 /admin-api 单独 mw, ADM-0 §1.3 红线)
- plugin SDK 自重连 (留 BPP-4 watchdog) — AL-5 是 owner UI 路径, plugin 自动重连是 plugin runtime 责任, 不属此 milestone
- recovery 历史 audit UI — 走 ADM-2 #484 既有 admin_actions audit 路径 (admin god-mode 看, 业务面不暴露; ADM-0 §1.3 红线)
- 重连失败后的 fallback (e.g. 自动 escalate 到 owner DM 二次通知) — v2 留账, v1 仅一次 DM + 一次手动 retry

## 3. 反查 grep 锚 (5 反约束, count==0)

```bash
# 1) recovery 状态字典 0 新增 (复用 AL-1 5-state, 不裂 'recovering'/'reconnecting' 等中间态)
git grep -nE '"recovering"|"reconnecting"|"recovery_in_progress"|"auto_recover"' \
  packages/server-go/internal/agent/  # 0 hit
# 2) state graph 0 改 (AL-1 ValidateTransition 单源, AL-5 仅复用既有 error→online edge)
git grep -nE 'ValidTransition.*error.*recover|recover.*online.*direct' \
  packages/server-go/internal/agent/  # 0 hit
# 3) reason 字典 0 新增 (复用 REFACTOR-REASONS #496 SSOT 6 字面)
git grep -nE '"reconnect_attempt"|"recovery_started"|"manual_retry"' \
  packages/server-go/internal/  # 0 hit
# 4) DM 走 DM-2 单源 (复用 BPP-3.2.1 既有 SendSystemDM path)
git grep -nE '"recovery_dm"|"agent_error_channel"|system_message_kind\s*=\s*"recovery"' \
  packages/server-go/internal/  # 0 hit
# 5) cross-org admin recover (反约束 ⑥ admin 不入业务路径)
git grep -nE 'admin.*\/agents\/.*\/recover|admin-api.*\/recover' \
  packages/server-go/internal/api/  # 0 hit
```

## 4. 不在范围

- BPP-4 timeout/watchdog (server-side 检测, 拆三路径)
- AP-3 cross-org owner-only 强制 (后续 milestone)
- ABAC v2 condition / multi-owner / grant 历史 UI / deny list (留 v2+, 跟 BPP-3.2 同源)
- plugin SDK 跨语言 recovery 实现 (本 spec 锁 server-side + 一个 reference plugin)
- recovery 历史 audit UI (走 ADM-2 既有 admin_actions)

## 5. 跨 milestone byte-identical 锁

- 跟 AL-1 #492 ValidateTransition 5-state graph + AppendAgentStateTransition single-gate 同源 (改 = 改两处: AL-1 + AL-5)
- 跟 REFACTOR-REASONS #496 reasons SSOT 6 字面 (改 = 改一处, AL-5 仅消费)
- 跟 BPP-3.2.1 #498 SendSystemDM + quick_action JSON shape 同模式 (action enum 4-enum 扩 grant/reject/snooze/recover, 跨 PR drift 守)
- 跟 ADM-0 §1.3 红线 admin 不入业务路径同源 (admin recover 走 ADM-3+)
