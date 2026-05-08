# Acceptance Template — AL-5: agent 错误恢复路径 wrapper milestone

> Spec: `docs/implementation/modules/al-5-spec.md` (战马C v0, 1dded5e)
> 蓝图: `agent-lifecycle.md` §2.3 5-state + §1.6 失联与故障 + `auth-permissions.md` §1.3 主入口
> 前置: AL-1 #492 ValidateTransition + AppendAgentStateTransition ✅ + REFACTOR-REASONS #496 reasons SSOT ✅ + BPP-3.2 #498 SendSystemDM + quick_action JSON shape ✅ + DM-2 #361/#372/#388 既有 path ✅
> Owner: 战马C (主战) + 野马 (文案) + 烈马 (验收)

## 验收清单

### AL-5.1 server 错误状态 owner system DM 通知

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 `agent/recover.go::NotifyOwnerOnError(agentID, reason)` 调既有 `Store.SendSystemDM` (复用 BPP-3.2.1 path); 不开新 channel 类型 / 不写新 system_message_kind enum | unit + 反向 grep | 战马C / 烈马 | `internal/agent/recover_test.go::TestAL5_NotifyOwnerOnError_WritesSystemDM` (DM body literal + quick_action shape round-trip) |
| 1.2 DM body byte-identical 跟 content-lock §1: `"{agent_name} 状态变更: error ({reason_label}). 点击重连"` | unit | 战马C / 野马 | `TestAL5_NotifyOwnerOnError_DMBodyLiteral` (3 fragment toContain) |
| 1.3 quick_action JSON shape 4-enum 扩 `recover` byte-identical 跟 content-lock §2: `{action: 'recover', agent_id, reason, request_id}` | unit | 战马C / 烈马 | `TestAL5_NotifyOwnerOnError_QuickActionShape` |
| 1.4 reason 必走 `reasons.IsValid` (REFACTOR-REASONS SSOT 6-dict, 第 10 处单测锁链承袭) | unit | 战马C / 烈马 | `TestAL5_NotifyOwnerOnError_ReasonValidation` (6 valid + 4 invalid 含 'recovering'/'reconnecting' 反约束 reject) |
| 1.5 admin god-mode 不入此路径 (反约束 ⑤ + ADM-0 §1.3) | reverse grep | 烈马 | `TestAL5_ReverseGrep_NoAdminInRecoverPath` |
| 1.6 反约束 grep DM 不另起 channel 类型 / `"recovery_dm"|"agent_error_channel"` 0 hit | reverse grep | 烈马 | `TestAL5_ReverseGrep_NoNewDMChannelType` |

### AL-5.2 client SPA "重连" 按钮 + recovery endpoint

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 `POST /api/v1/agents/:id/recover` body `{request_id}`; server 端读 agent_state_log 最近 error reason; 调 `agent.RecoverFromError(agentID, lastReason)` 走 AL-1 ValidateTransition error→online valid edge | unit | 战马C / 烈马 | `internal/api/agent_recover_test.go::TestAL5_PostRecover_HappyPath` (200 + agent_state_log row + state online) |
| 2.2 owner-only ACL: `agent.OwnerID == user.ID`; non-owner → 403 + `bpp.recover_not_owner` 错码 (跟 BPP-3.2.2 me_grants 同模式) | unit | 战马C / 烈马 | `TestAL5_PostRecover_NonOwner403` |
| 2.3 agent 非 error 态 → reject (e.g. agent state==idle/online → 409 + `bpp.recover_state_invalid` 错码) | unit | 战马C / 烈马 | `TestAL5_PostRecover_AgentNotInError409` (4 case: online/idle/busy/offline 全 reject) |
| 2.4 admin god-mode endpoint 不挂 (反约束 ⑤ + ADM-0 §1.3) | reverse grep | 烈马 | `TestAL5_ReverseGrep_NoAdminPathRecover` (filepath.Walk 扫 internal/api/*.go count==0) |
| 2.5 SystemMessageBubble 检测 quick_action.action='recover' → 渲染单按钮 "重连" (DOM `data-action="recover"` + `data-bpp32-button="primary"`); 跟 BPP-3.2.2 三按钮 mode 同源 (复用既有 isBPP32GrantPayload 扩展或新 type guard) | vitest | 战马C / 野马 | `packages/client/src/__tests__/SystemMessageBubble.al5.test.tsx` (DOM 字面 + 8 同义词反向 grep + button click → postMeRecover API call) |
| 2.6 反约束 grep client 端 8 同义词禁词 (重启/reset/restart / 重置/reset_state/reboot / 重启动/restart_now) 0 hit on button label | vitest | 烈马 / 野马 | `SystemMessageBubble.al5.test.tsx::TestAL5_RecoverButton_NoSynonymsAllowed` |

### AL-5.3 server-side full-flow integration test + closure

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 server-side full-flow integration: agent state 翻 error → server emits system DM → owner POST /agents/:id/recover → state error→online → agent_state_log row + AL-1 ValidateTransition gate 守 | http e2e | 战马C / 烈马 | `internal/api/al_5_integration_test.go::TestAL5_FullFlow_ErrorThenOwnerRecoverThenOnline` (Step 1 SetError + Step 2 DM written + Step 3 POST recover 200 + Step 4 state==online verify + Step 5 agent_state_log 2 rows verify) |
| 3.2 closure: registry §3 REG-AL5-001..N + acceptance + PROGRESS [x] AL-5 + docs/current sync (server/agent-state.md §recover + client/ui/message.md §4g) | docs | 战马C / 烈马 | registry + PROGRESS + 4 件套全闭 |

## 不在本轮范围 (spec §4)

- v2 retry queue (BPP-4 watchdog 拆三路径)
- cross-org admin recover (ADM-3+)
- plugin SDK 自重连 (BPP-4)
- recovery 历史 audit UI (走 ADM-2 既有 admin_actions)
- 重连失败 fallback (v2)

## 退出条件

- AL-5.1 1.1-1.6 (server DM dispatch + 6 反约束) ✅
- AL-5.2 2.1-2.6 (server endpoint + client UI + 8 同义词反向 grep) ✅
- AL-5.3 3.1-3.2 (server-side full-flow integration + closure) ✅
- 现网回归不破: 全套 server + client + e2e 测试套全 PASS
- REG-AL5-001..N 落 registry + 8 反约束 grep 全 count==0
- 4 件套全闭 (spec ✅ + stance + acceptance + content-lock)

## 更新日志

- 2026-04-29 — 战马C v0 acceptance template (4 件套第二件): 3 段实施 (1.1-1.6 / 2.1-2.6 / 3.1-3.2) + 5 不在范围 + 退出条件 6 项. 联签 AL-5.1/.2/.3 三 PR 同 branch 叠 commit, BPP-3.2 同模式.
