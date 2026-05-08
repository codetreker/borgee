# Acceptance Template — AP-4: reactions ACL 收紧 (CV-7 gap 修) ✅ closed

> CV-7 #535 既存 reactions ACL gap 闭合, DM-5 #549 e2e §3.3 发现 (REG-DM5-005 文档化). **0 schema 改 + 0 新错码 + 0 新 endpoint**. Spec + Stance + Content-lock **不需** (server only).

## 验收清单

### §1 AP-4.1 — server reactions.go 3 handler 加 ACL gate

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 立场 ① PUT /reactions 加 channel-member gate (复用 IsChannelMember + CanAccessChannel) | unit | `internal/api/reactions_acl_test.go::TestAP4_PutReaction_NonMember404` PASS |
| 1.2 立场 ① DELETE /reactions 同 gate | unit | `TestAP4_DeleteReaction_NonMember404` PASS |
| 1.3 立场 ① GET /reactions 加 user==nil reject 401 + channel-member gate | unit | `TestAP4_GetReactions_NonMember404 + TestAP4_GetReactions_Unauth401` PASS |
| 1.4 反向 sanity — channel member 三动作全 200 byte-identical (既有行为不破) | unit | `TestAP4_Member_AllOK` PASS |
| 1.5 立场 ④ 0 schema 改 — git diff migrations/ 0 行 | git diff | 0 production 行 |

### §2 AP-4.2 — e2e cross-channel reject

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 e2e: 私 channel msg → non-member PUT → 404 | E2E | `ap-4-reactions-acl.spec.ts::PUT non-member 404` |
| 2.2 e2e: 同 → DELETE → 404 | E2E | `ap-4-reactions-acl.spec.ts::DELETE non-member 404` |
| 2.3 e2e: 同 → GET → 404 (or 401 unauth) | E2E | `ap-4-reactions-acl.spec.ts::GET non-member 404` |
| 2.4 5/5 deterministic 跑 5 次全绿 | playwright multi-run | 5 次 |

### §3 AP-4.3 — closure

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 反向 grep 4 锚: 3 处 0 hit + ACL helper ≥3 hit | CI grep | spec §3 |
| 3.2 REG-AP4-001..005 5 行 🟢 + REG-DM5-005 cross-link gap 闭合备注 | regression-registry.md | 5 行 + 1 cross-link 备注 |
| 3.3 PROGRESS [x] 第 13 项 | PROGRESS.md | 第 13 项 |

## 边界

- CV-7 #535 (既存 reactions endpoint) / DM-5 #549 REG-DM5-005 (gap 文档化, 此 PR 闭合) / messages.go (既有 ACL helper IsChannelMember + CanAccessChannel 复用源) / REG-INV-002 fail-closed / ADM-0 §1.3 admin rail 红线

## 退出条件

- §1+§2+§3 全绿
- 0 schema 改 + 0 新错码 + 0 新 endpoint
- 反向 grep 4 锚通过
- REG-AP4-001..005 5 行
- REG-DM5-005 cross-link 备注 (gap 闭合)
