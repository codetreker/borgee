# AP-5 spec brief — messages endpoints ACL audit (CV-7 #535 同模式 gap 防御性扫)

> 战马E · Phase 5+ · ≤80 行 · 蓝图 [`auth-permissions.md`](../../blueprint/current/auth-permissions.md) §1.2 + REG-INV-002 fail-closed + AP-4 #551 reactions 同模式扩展. AP-4 闭合 reactions 一处 gap; AP-5 系统扫 messages.go + dm_4_message_edit.go 全 endpoint 是否同 gap, 一次锁满.

## 0. 关键约束 (4 项立场, 跨链承袭)

1. **messages 全 endpoint 反查 IsChannelMember** (audit 后 message-scoped handler 加 ACL gate; channel-scoped 已有 gate 不动): handleListMessages / handleSearchMessages / handleCreateMessage 已含 IsChannelMember + CanAccessChannel (channel-scoped, audit 反向锁); handleUpdateMessage (PUT) / handleDeleteMessage (DELETE) / DM-4 handleEdit (PATCH) 三 message-scoped path **当前仅查 sender_id + cross-org, 不查 channel-member** — 复用 AP-4 抽出的 canAccessMessage 模式 (跟 reactions 同源) 加 channel-member gate 防 post-removal 漏洞 (sender 被 remove 后仍能 edit/delete 自己消息). 反向 grep `messages.*PUT.*\!channel_member|messages.*DELETE.*\!channel_member|PATCH.*messages.*\!channel_member` 在 internal/api/ count==0.

2. **owner-only ACL #17 处一致** (AP-4 #16 续): admin god-mode 不挂 (跟 ADM-0 §1.3 + CV-5..CV-12 + AP-4 同源). **反向 grep**: `admin.*messages.*PUT|admin.*messages.*DELETE|admin.*PATCH.*messages` 在 admin*.go count==0.

3. **错误字符 byte-identical messages.go 既有** (跟 AP-4 同模式不另起): 加的 ACL gate fail-closed 错误返 404 "Channel not found" (跟 messages POST 既有 fail-closed 同字符). 既有 sender_id != user.ID → 403 "Can only edit your own messages" / "Permission denied" 字符不动. **0 新错码**.

4. **0 schema 改 + 0 新 endpoint + audit 不另起** (forward-only, 复用既有 message_edited / message_deleted 事件): events 表既有不动. **反向 grep** `messages_acl_audit|messages.*new.*log` count==0.

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| AP-5.1 server | `internal/api/messages.go::handleUpdateMessage` + `handleDeleteMessage` 加 IsChannelMember gate (≤6 行 each, sender_id check 之前); `internal/api/dm_4_message_edit.go::handleEdit` 加同 gate (≤6 行); 不引新 helper, 复用 Store.IsChannelMember + Store.CanAccessChannel | 0 schema 改; 错误字符 byte-identical "Channel not found" 404 fail-closed |
| AP-5.2 unit + e2e | `internal/api/messages_acl_audit_test.go` (新, 5 unit case): own message edit/delete OK / sender removed from channel → edit/delete reject / cross-org reject 既有不破 / e2e `ap-5-messages-acl-matrix.spec.ts` 4 case (PUT/DELETE non-member 全 reject + DM-4 PATCH non-member reject) | 5/5 deterministic |
| AP-5.3 closure | REG-AP5-001..005 + acceptance + PROGRESS [x] + content-lock 不需 (server only) + REG-AP4 cross-link 备注 (AP-4 reactions + AP-5 messages 双轨 ACL 收紧成对) | 反向 grep 5 锚守门 |

## 2. 错误码 (0 新 — 全复用既有)

- "Channel not found" (404, byte-identical messages.go)
- "Can only edit your own messages" (403, 既有不动)
- "Permission denied" / "Forbidden" (403, 既有不动)
- "dm.edit_non_owner_reject" / "dm.edit_only_in_dm" (DM-4 既有不动)

## 3. 反向 grep 锚 (AP-5 实施 PR 必跑)

```
git grep -nE 'messages.*PUT.*\!channel_member|messages.*DELETE.*\!channel_member|PATCH.*messages.*\!channel_member' packages/server-go/internal/api/  # 0 hit (gap 修)
git grep -nE 'admin.*messages.*PUT|admin.*messages.*DELETE|admin.*PATCH.*messages' packages/server-go/internal/api/admin   # 0 hit (ADM-0 §1.3)
git grep -nE 'messages_acl_audit|messages.*new.*log' packages/server-go/internal/  # 0 hit (audit 不另起)
git grep -nE 'IsChannelMember.*existing.ChannelID' packages/server-go/internal/api/messages.go packages/server-go/internal/api/dm_4_message_edit.go  # ≥ 3 hit (PUT + DELETE + PATCH 三 handler 加 gate)
```

## 4. 不在本轮范围 (deferred)

- ❌ 抗 enumeration message_id (留 v2 — UUID 难枚举已是事实保护)
- ❌ admin god-mode 看消息内容 (ADM-0 §1.3 红线)
- ❌ schema migration (0 schema 改)
- ❌ rate-limit (留 spam Phase)
- ❌ post-removal grace window (即 sender 被 remove 后是否给 5min 编辑窗口 — 留 UX Phase)
