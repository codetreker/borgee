# AL-5 agent error recovery endpoint — implementation note

> AL-5 (#待定) · Phase 5 · 蓝图 [`agent-lifecycle.md`](../../blueprint/current/agent-lifecycle.md) §2.3 (5-state error → online recovery edge) + AL-1 #492 single-gate helper + REFACTOR-REASONS #496 单一来源.

## 1. 设计

owner-driven manual recovery — agent state 由 BPP-4 watchdog (或手动) 翻 error → owner 收到 system DM 通知 (AL-5.1, 后续 PR) → owner 点击 "重连" 按钮 → POST /api/v1/agents/:id/recover → server 走 AL-1 #492 single-gate helper `AppendAgentStateTransition(agent, error→online, lastReason)` → state-log 行落 (forward-only audit).

设计原则 (跟 al-5-spec.md §0):
- ② recovery = 单 helper 单一来源 (走 AppendAgentStateTransition, 不拆状态机)
- ③ recovery reason 不另起字典 (复用 last error transition reason, AL-1a 6 字面)
- 反向约束: admin god-mode 不挂此路径 (ADM-0 §1.3 红线)

## 2. Endpoint

| Path | Method | Auth | Body | Returns |
|---|---|---|---|---|
| `/api/v1/agents/{id}/recover` | POST | user-rail (borgee_token) | `{request_id?: string}` | `{state: "online", reason: string}` |

## 3. ACL + 错误码

- **401** Unauthenticated — no borgee_token
- **400** agent id 缺
- **404** agent not found / not role='agent'
- **403** non-owner (agent.OwnerID !== current_user.ID)
- **409** agent not currently in `error` state (no history OR last state-log row's `to_state !== 'error'`)
- **500** internal (state-log read fail / append fail)
- **200** success — recovery transition appended to state-log, reason carried forward

## 4. Flow

1. Auth check (user-rail)
2. Path id present + agent lookup (Role='agent', OwnerID match)
3. `Store.ListAgentStateLog(agentID, 1)` — discover most recent transition
4. Verify `last.to_state == 'error'` (otherwise 409)
5. `Store.AppendAgentStateTransition(agentID, error, online, last.reason, "")` — AL-1 #492 single-gate
6. Return 200 with `{state, reason}`

## 5. 反向约束

- 不另起 recovery 状态字典 (grep 检查 `recovering|reconnecting|recovery_in_progress|auto_recover` 0 hit)
- 不在 5-state graph 加新态 (走 AL-1 ValidateTransition 既有 error→online edge)
- reason 不新增字面 (复用 last transition reason, REFACTOR-REASONS 单一来源)
- admin-api 不挂此路径 (TestAL5_Recover_AdminAPINotMounted 守)

## 6. 测试覆盖

`internal/api/al_5_recover_test.go` 7 unit:
- `_Owner_HappyPath` — recovery 200 + state-log 第 3 行 (error→online + reason 沿用)
- `_NonOwnerRejected` — 403
- `_Unauthenticated401` — 401
- `_AgentNotFound` — 404 (含 non-agent user)
- `_NotInErrorStateConflict` — 409 (online 状态 reject)
- `_NoStateLogConflict` — 409 (无历史 reject)
- `_AdminAPINotMounted` — admin-api 不挂 (ADM-0 §1.3 红线)

## 7. 跨 milestone byte-identical 锁定

- AL-1 #492 single-gate helper (改 = 改 AL-1 ValidateTransition + agent_state_log schema)
- REFACTOR-REASONS #496 单一来源 (reason 字典单一来源, 改 = 改 reasons.ALL 一处)
- ADM-0 §1.3 红线 (admin god-mode 仅元数据, 不入业务态变更)
- 客户端按钮 `data-al5-button="recover" data-action="recover"` 跟 BPP-3.2 quick_action shape 同模式 (改 = 改 SystemMessageBubble.tsx + content-lock 同步)
