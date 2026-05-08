# Acceptance Template — DM-4: agent message edit 多端同步

> 蓝图 `channels-dm-collab.md` §3 (DM 编辑) + RT-3 #488 fan-out + DM-3 #508 cursor 复用. Spec `dm-4-spec.md` (战马D v0 5cf7381) + Stance `dm-4-stance-checklist.md` (战马D v0). 不需 content-lock — server-only PATCH + client hook. 拆 PR: 整 milestone 一 PR (`spec/dm-4` 三段一次合). Owner: 战马D 实施 / 飞马 review / 烈马 验收.

## 验收清单

### §1 DM-4.1 — server PATCH /api/v1/channels/{dmID}/messages/{id}

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 PATCH endpoint owner-only ACL + DM-only path (channel.kind != "dm" → 403 `dm.edit_only_in_dm`) + body schema {content, edited_at} | unit (5 sub-case) | 战马D / 烈马 | `internal/api/dm_4_message_edit_test.go::TestDM41_HappyPath` + `_NonOwnerRejected` + `_NonDMReject` + `_Unauthorized401` + `_NotFound404` |
| 1.2 events 表 INSERT op="edit" — 复用 RT-3 fan-out 路径 (不新建 channel/frame/sequence) | unit + grep | 战马D / 烈马 | `TestDM41_EventsInsertOpEdit` (查 events 表 op="edit" 行真写入) + 反向 grep `dm_edit_event\|message_edit_channel\|edit_sync_frame` count==0 |
| 1.3 立场 ③ — PATCH body 反向断言 thinking 5-pattern count==0 (agent edit 是机械修订, 不暴露 reasoning) | unit + grep | 战马D / 烈马 | `TestDM41_NoThinkingPatternInBody` (反向 grep 5 字面 0 hit) |

### §2 DM-4.2 — client useDMEdit hook + 4 vitest

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 useDMEdit(dmChannelID) hook — REST PATCH wrapper + optimistic update; 不订阅 dm-only frame (复用 useDMSync DM-3 #508) | vitest (4 case) | 战马D / 烈马 | `packages/client/src/hooks/__tests__/useDMEdit.test.ts` 4 vitest PASS (HappyPath / 错误 toast / pre-edit cursor / multi-device 复用 useDMSync) |
| 2.2 立场 ② — useDMEdit 不写独立 sessionStorage cursor (cursor 进展全归 useDMSync) | vitest + grep | 战马D / 烈马 | `useDMEdit.test.ts::DoesNotWriteOwnCursor` + 反向 grep `borgee.dm4.cursor\|useDMEdit.*sessionStorage` count==0 |

### §3 DM-4.3 — e2e + REG-DM4 + AST 兜底

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 e2e 双 tab — owner edit message → tab2 ≤3s 收 edit reflect (RT-3 fan-out 真验证, RT-1.2 #292 latency 同源) | E2E (Playwright) | 战马D / 烈马 / 野马 | `packages/e2e/tests/dm-4-edit-multi-device.spec.ts` REST-driven dual-tab |
| 3.2 反向 grep 5 锚 0 hit (不另起 channel/frame + 不另起 sequence + 5-pattern 反约束 + 不挂 audit table + admin god-mode 红线) | CI grep | 飞马 / 烈马 | CI lint 每 DM-4 PR 必跑 |

## 边界

- RT-3 #488 (fan-out 路径复用 + thinking 5-pattern 反约束承袭, 锁链第 3 处) / DM-3 #508 (useDMSync cursor 复用, edit 是 cursor 子集) / RT-1.3 #296 (cursor monotonic 守门) / RT-1.2 #292 (≤3s latency 同源) / AL-2a #480 + BPP-3.2 #498 + AL-1 #492 + AL-5 #516 (owner-only 5 处同模式) / ADM-0 §1.3 红线

## 退出条件

- §1 (3) + §2 (2) + §3 (2) 全绿 — 一票否决
- thinking 5-pattern 反约束锁链 DM-4 = 第 3 处 (RT-3 第 1 + DM-3 第 2 链承袭不漂)
- 反向 grep `dm_message_edits/edit_history/edit_audit_log/dm_edit_event/message_edit_channel/edit_sync_frame` 全 0 hit
- 登记 REG-DM4-001..005
