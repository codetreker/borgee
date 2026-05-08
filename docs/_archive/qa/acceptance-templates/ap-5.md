# Acceptance Template — AP-5: messages endpoints ACL audit ✅

> AP-4 #551 reactions ACL 同模式扩展; 3 message-scoped handler post-removal gap 闭合. **0 schema 改 + 0 新错码 + 0 新 endpoint**. Spec + Stance + Content-lock **不需** (server only).

## 验收清单

### §1 AP-5.1 — server 3 handler 加 ACL gate

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 PUT /messages/:id 加 IsChannelMember gate | unit | `messages_acl_audit_test.go::TestAP5_PutMessage_PostRemovalReject` PASS |
| 1.2 DELETE /messages/:id 加 IsChannelMember gate | unit | `TestAP5_DeleteMessage_PostRemovalReject` PASS |
| 1.3 PATCH /channels/:id/messages/:id (DM-4) 加 IsChannelMember gate | unit | `TestAP5_PatchDM_PostRemovalReject` PASS |
| 1.4 反向 sanity — channel member 三动作 200 byte-identical (既有不破) | unit | `TestAP5_Member_AllOK` PASS |
| 1.5 反向 sanity — non-sender member 仍 403 既有 (sender_id check 不破) | unit | `TestAP5_NonSenderMember_403` PASS |
| 1.6 0 schema 改 — git diff migrations/ 0 行 | git diff | 0 production 行 |

### §2 AP-5.2 — e2e cross-channel matrix

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 e2e: 私 channel sender → 被 remove 出 channel → PUT /messages/:id → 404 | E2E | `ap-5-messages-acl-matrix.spec.ts::PUT post-removal 404` |
| 2.2 e2e: 同 → DELETE /messages/:id → 404 | E2E | `::DELETE post-removal 404` |
| 2.3 e2e: 同 (DM 场景) → PATCH /channels/:id/messages/:id → 404 | E2E | `::PATCH DM post-removal 404` |
| 2.4 e2e: cross-org user 不能 PUT/DELETE 别人消息 (既有 cross-org reject 不破 sanity) | E2E | `::cross-org sanity` |
| 2.5 5/5 deterministic 跑 5 次全绿 | playwright multi-run | 5 次 |

### §3 AP-5.3 — closure

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 反向 grep 4 锚: 3 处 0 hit + IsChannelMember ≥3 hit | CI grep | spec §3 |
| 3.2 REG-AP5-001..005 5 行 🟢 + REG-AP4 cross-link 备注 (双轨 ACL 收紧成对) | regression-registry.md | 5 行 + 1 cross-link |
| 3.3 PROGRESS [x] 第 13 项 | PROGRESS.md | 第 13 项 |

## 边界

- AP-4 #551 (reactions ACL gap 闭合, 同模式) / messages.go (既有 PUT/DELETE handler) / dm_4_message_edit.go (PATCH handler) / Store.IsChannelMember + Store.CanAccessChannel (既有 helper 复用) / REG-INV-002 fail-closed / ADM-0 §1.3 admin rail 红线

## 退出条件

- §1+§2+§3 全绿
- 0 schema 改 + 0 新错码 + 0 新 endpoint
- 反向 grep 4 锚通过
- REG-AP5-001..005 5 行 + REG-AP4 cross-link

## 关闭

✅ 2026-04-30 战马E — server-go ./... 25 packages 全绿; TestAP5_* 5 unit PASS (含 TestAP5_PatchDM_PostRemovalReject DM 路径真路 admin role 解析); error_branches_test.go 副作用更新 (update/delete-message-forbidden 翻 403 → 404 因 post-leave-public gap fix 真生效); REG-AP5-001..005 + 跨 milestone cross-link AP-4 #551 + DM-5 #549 双轨 ACL 收紧成对.
