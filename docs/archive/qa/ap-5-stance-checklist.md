# AP-5 立场反查清单 (战马E v0)

> 战马E · 2026-04-29 · AP-4 #551 reactions ACL 同模式扩展; messages.go + dm_4_message_edit.go 全 endpoint audit.
> **关联**: spec `ap-5-spec.md` (d585587) + acceptance + content-lock **不需** (server only).

## §0 立场总表 (4 立场 + 3 边界)

| # | 立场 | 反约束 |
|---|---|---|
| ① | 3 message-scoped handler 加 IsChannelMember + CanAccessChannel gate (PUT /messages/:id, DELETE /messages/:id, PATCH /channels/:id/messages/:id DM-4); 复用既有 helper 0 新 helper | 反向 grep `messages.*PUT.*\!channel_member\|messages.*DELETE.*\!channel_member\|PATCH.*messages.*\!channel_member` 在 internal/api/ count==0 |
| ② | owner-only ACL #17 处, admin 不挂 | 反向 grep `admin.*messages.*PUT\|admin.*messages.*DELETE\|admin.*PATCH.*messages` 在 admin*.go count==0 |
| ③ | 错误字符 byte-identical "Channel not found" 404 fail-closed (跟 messages POST + AP-4 同字符) | 既有 sender_id != user.ID → 403 字符不动; cross-org 既有 不动 |
| ④ | 0 schema 改 + 0 新 endpoint + audit 不另起 | 反向 grep `messages_acl_audit\|messages.*new.*log` count==0 |

## §1 立场 ① 3 handler ACL gate

- [ ] handleUpdateMessage (PUT /messages/:id) 加 IsChannelMember + CanAccessChannel gate (sender_id 检查之前)
- [ ] handleDeleteMessage (DELETE /messages/:id) 同
- [ ] handleEdit (PATCH /channels/:id/messages/:id, DM-4) 同
- [ ] 复用 既有 Store helper 不引新 helper
- [ ] 反向 grep `IsChannelMember.*existing.ChannelID` ≥ 3 hit

## §2 立场 ② admin 不挂

- [ ] /admin-api/* 不注册 messages PUT/DELETE/PATCH
- [ ] 反向 grep `admin.*messages.*PUT\|admin.*messages.*DELETE` 0 hit

## §3 立场 ③ fail-closed + 错误字符 byte-identical

- [ ] 加的 ACL gate 失败时返 404 "Channel not found" (跟 messages POST 既有 + AP-4 同字符)
- [ ] 既有 "Can only edit your own messages" / "Permission denied" / "Forbidden" / "dm.edit_non_owner_reject" / "dm.edit_only_in_dm" 字符不动
- [ ] e2e + unit 反向断 byte-identical

## §4 立场 ④ 0 schema + audit 不另起

- [ ] 0 schema 改 (git diff migrations/ 0 行)
- [ ] events 表既有 message_edited / message_deleted 事件不动
- [ ] 反向 grep `messages_acl_audit\|messages.*new.*log` 0 hit

## §5 边界 ⑤⑥⑦ — fail-closed / forward-only / 不裂表

- [ ] cross-channel reject 跟 AP-4 + messages POST 同源
- [ ] forward-only — gap 闭合即修, 不留兼容路径
- [ ] 不裂表 — 0 schema 改

## §6 退出条件

- §1+§2+§3+§4 全 ✅
- 反向 grep 4 锚: 3 处 0 hit + IsChannelMember helper ≥3 hit
- 5 unit + 4 e2e 全 PASS, 5/5 deterministic
- 0 schema 改 + 0 新错码
- REG-AP5-001..005 5 行 + REG-AP4 cross-link 双轨 ACL 收紧成对
