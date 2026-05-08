# AL-5 立场反查清单 (战马C v0)

> 战马C · 2026-04-29 · 立场 review checklist (跟 BPP-3.2 #498 + AL-1 #492 + REFACTOR-REASONS #496 同模式)
> **目的**: AL-5 三段实施 (5.1 server DM / 5.2 client UI + endpoint / 5.3 e2e + closure) PR review 时, 飞马 / 野马 / 烈马按此清单逐立场 sign-off, 反向断言代码层守住每条立场.
> **关联**: spec `docs/implementation/modules/al-5-spec.md` (战马C v0, 1dded5e) + acceptance `docs/qa/acceptance-templates/al-5.md` + content-lock `docs/qa/al-5-content-lock.md`. 复用 AL-1 #492 5-state graph + REFACTOR-REASONS #496 6-dict + BPP-3.2.1 #498 SendSystemDM + DM-2 message_mentions.

## §0 立场总表 (3 立场 + 5 边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | 错误状态 owner 可见 = system DM (复用 BPP-3.2.1 path) | auth-permissions.md §1.3 主入口 + agent-lifecycle.md §1.6 失联与故障 | 调既有 `Store.SendSystemDM`; 不开新 channel 类型 / 不写新 system_message_kind enum / 不另起 recovery_dm 路径 |
| ② | 错误恢复 = 单 helper SSOT | agent-lifecycle.md §2.3 5-state + AL-1 #492 ValidateTransition | `agent.RecoverFromError` 走既有 `error → online` valid edge + `AppendAgentStateTransition` single-gate; 不另起 recovery 状态机 / 不在 5-state 加新态 |
| ③ | recovery reason 不另起字典 | REFACTOR-REASONS #496 SSOT 6 字面 | `reasons.IsValid` 守; 反约束 `'recovering'/'reconnecting'/'recovery_in_progress'/'auto_reconnect'` 等中间态字面 0 新增 |
| ④ (边界) | quick_action JSON shape 4-enum 扩 `recover` | 跟 BPP-3.2 grant/reject/snooze 同模式 | `MeRecoverActionRecover = "recover"` const; client UI 单按钮 "重连" data-action='recover' (跟 BPP-3.2 三按钮同 attr 模式) |
| ⑤ (边界) | owner-only ACL (admin god-mode 不入) | ADM-0 §1.3 红线 | `agent.OwnerID == user.ID` gate; admin path 不挂 endpoint; 反向 grep `admin.*\/agents\/.*\/recover` count==0 |
| ⑥ (边界) | DM body byte-identical 跟 content-lock | 蓝图 §1.6 + 野马签字 | DM body template const + 反向 grep "重启/reset/restart" 同义词禁词 0 hit |
| ⑦ (边界) | recovery 不复用 BPP-3.2 retry cache | BPP-3.2.3 RequestRetryCache plugin-side, AL-5 server-side | 拆三路径: BPP-4 watchdog server 检测 / BPP-3.2.3 plugin in-memory / AL-5 owner manual recovery; 反向 grep `RequestRetryCache.*recover\|al5.*retry_cache` count==0 |
| ⑧ (边界) | 不另起 endpoint, agent-id 已锁路径 | RESTful 模式同既有 /agents/:id/runtimes/* | endpoint = `POST /api/v1/agents/:id/recover`; 反向 grep `/api/v1/recover\|/api/v1/agents/recover` (无 :id) count==0 |

## §1 立场 ① owner DM 走 BPP-3.2.1 既有 path (AL-5.1 守)

**蓝图字面源**: `auth-permissions.md` §1.3 主入口字面 + `agent-lifecycle.md` §1.6 失联与故障状态字面承袭

**反约束清单**:

- [ ] `notifyOwnerOnError(agentID, reason)` 调 `Store.SendSystemDM(ownerID, body, quickActionJSON)` 既有 helper (复用 BPP-3.2.1 既有 path); 不开新 channel 类型 / 不写新 system_message_kind enum
- [ ] DM 走 owner 既有 `type='system'` channel (CM-onboarding #203 既有, idempotent lookup); 反向 grep `system_message_kind\s*=\s*"recovery"` count==0
- [ ] 反约束 grep `"recovery_dm"|"agent_error_channel"|"al5_dm"` 在 internal/api/ count==0

## §2 立场 ② 错误恢复 = 单 helper SSOT (AL-5.2 守)

**蓝图字面源**: `agent-lifecycle.md` §2.3 5-state graph + AL-1 #492 ValidateTransition single-gate 同源

**反约束清单**:

- [ ] 新建 `agent.RecoverFromError(agentID, reason)` 唯一入口 — 内部调 `AppendAgentStateTransition(agentID, error, online, reason)` 复用 AL-1 既有 helper
- [ ] state graph 不改 — `error → online` 已是 AL-1 既有 valid edge (TestValidateTransition_RecoveryPath 已锁); 反约束 grep `ValidTransition.*error.*recover|recover.*online.*direct` count==0
- [ ] 反约束 grep `"recovering"|"reconnecting"|"recovery_in_progress"|"auto_recover"` 在 internal/agent/ count==0 (5-state 不裂)

## §3 立场 ③ recovery reason 不另起字典 (REFACTOR-REASONS #496 SSOT 同源)

**蓝图字面源**: `agent-lifecycle.md` §1.6 + AL-1a #249 6-dict + REFACTOR-REASONS #496 reasons SSOT 包

**反约束清单**:

- [ ] reason 必走 `reasons.IsValid(reason)` 校验 (6 字面 + 第 9 处单测锁链承袭, 跟 #249/#305/#321/#380/#454/#458/#481/#492/#499 同源)
- [ ] 反约束 grep `"reconnect_attempt"|"recovery_started"|"manual_retry"|"auto_recover"` 在 internal/ count==0
- [ ] AL-5 不在 `internal/agent/reasons/reasons.go` 加新 const (REFACTOR-REASONS #496 SSOT 不动)

## §4 联签清单 (实施 PR 时填)

- [ ] 飞马 (spec ↔ 立场对齐): _(签)_
- [ ] 野马 (DM body + 单按钮 label byte-identical 跟 content-lock §1+§2): _(签)_
- [ ] 烈马 (反向 grep + 单测覆盖率 ≥84% + 8 反约束全 count==0): _(签)_
- [ ] 战马C (实施代码 ↔ 立场反查 8 项全过): _(签)_
