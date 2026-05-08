# AP-4 立场反查清单 (战马E v0)

> 战马E · 2026-04-29 · CV-7 #535 既存 reactions ACL gap 闭合 (DM-5 #549 e2e §3.3 发现).
> **关联**: spec `ap-4-spec.md` (b5cb98e) + acceptance + content-lock **不需** (server only).

## §0 立场总表 (4 立场 + 3 边界)

| # | 立场 | 反约束 |
|---|---|---|
| ① | 3 handler 加 channel-member ACL gate, 复用 `Store.IsChannelMember` + `Store.CanAccessChannel` 既有 helper | 反向 grep `IsChannelMember.*reactions\|CanAccessChannel.*reactions` 在 reactions.go ≥3 hit; reactions.*PUT.*\!channel_member 0 hit |
| ② | owner-only ACL #16 处, admin god-mode 不挂 | 反向 grep `admin.*reactions\|admin.*reaction.*PUT` 在 admin*.go count==0 |
| ③ | REG-INV-002 fail-closed 真锁 — cross-channel non-member 3 动作全 reject (404/403 byte-identical messages.go) | unit + e2e 反向断 PUT/DELETE/GET 全 reject |
| ④ | 0 schema 改 + audit 不另起 (events 表既有 reaction_update 不动) | 反向 grep `reaction_audit\|reactions.*log` 在 internal/ count==0 |

## §1 立场 ① ACL gate 3 处

- [ ] handleAddReaction: GetMessageByID → IsChannelMember + CanAccessChannel → reject 404 (跟 messages POST 同模式)
- [ ] handleRemoveReaction: 同
- [ ] handleGetReactions: 同 + user==nil 加 401
- [ ] 反向 grep ACL helper 在 reactions.go ≥3 hit

## §2 立场 ② admin 不挂

- [ ] reactions endpoint 不在 /admin-api/* 注册
- [ ] 反向 grep `admin.*reactions` 0 hit

## §3 立场 ③ fail-closed 真锁

- [ ] e2e seed 私 channel + msg → other-user PUT/DELETE/GET 全 reject (404)
- [ ] 错误字符 byte-identical messages.go ("Channel not found" / "Message not found" / "Unauthorized")
- [ ] member 既有 OK path 0 改 (反向 sanity)

## §4 立场 ④ 0 schema + audit 不另起

- [ ] 0 schema 改 (git diff migrations/ 0 行)
- [ ] hub.BroadcastEvent + events 表既有 reaction_update 不动
- [ ] 反向 grep `reaction_audit\|reactions.*log` 0 hit

## §5 边界 ⑤⑥⑦ — fail-closed / forward-only / 不裂表

- [ ] cross-channel reject 跟 messages POST 同源
- [ ] forward-only — gap 闭合即修, 不留 backward-compatible 路径
- [ ] 不裂表 — 0 schema 改

## §6 退出条件

- §1+§2+§3+§4 全 ✅
- 反向 grep 4 锚: 3 处 0 hit + ACL helper ≥3 hit
- 4 unit + 3 e2e 全 PASS, 5/5 deterministic
- 0 schema 改 + 0 新错码
- REG-AP4-001..005 5 行 + REG-DM5-005 cross-link gap 闭合
