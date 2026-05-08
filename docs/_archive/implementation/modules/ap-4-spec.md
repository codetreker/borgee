# AP-4 spec brief — reactions ACL 收紧 (CV-7 #535 既存 gap 修)

> 战马E · Phase 5+ · ≤80 行 · 蓝图 [`auth-permissions.md`](../../blueprint/current/auth-permissions.md) §1.2 + REG-INV-002 fail-closed + ADM-0 §1.3 admin rail 红线 + CV-7 #535 既存 gap 修. AP-4 给 reactions PUT/DELETE/GET 加 channel-member ACL gate (跟 messages POST/GET 同源既有 ACL helper). DM-5 #549 e2e §3.3 反向断时发现此 gap (REG-DM5-005 文档化), AP-4 闭合.

## 0. 关键约束 (4 项立场, 蓝图字面)

1. **reactions 三 handler (PUT/DELETE/GET) 加 channel-member ACL gate, 复用既有 helper** (跟 messages.go::handleListMessages + handleCreateMessage 既有 ACL 同源, 反约束: 不另起 ACL helper / 不另写 channel-resolution path): handler 在 `Store.GetMessageByID` 后必查 `Store.IsChannelMember(msg.ChannelID, user.ID) && Store.CanAccessChannel(msg.ChannelID, user.ID)`; 失败 → 404 (private channel hidden) / 非 member 私 channel → 403 fail-closed (跟 既有 messages POST 同 status 同模式承袭). **反约束**: GET 也必检 (不再 unauth pass-through), 反向 grep `reactions.*PUT.*\!channel_member\|reactions.*GET.*public\b` count==0.

2. **owner-only ACL #16 处一致, admin god-mode 不挂** (跟 ADM-0 §1.3 + CV-5..CV-12 + DM-* 同源): admin /admin-api/* rail 不入 reaction path; user rail user 必登录. **反向 grep**: `admin.*reactions\|admin.*reaction.*PUT` 在 admin*.go count==0.

3. **REG-INV-002 fail-closed 真锁** (跟 messages POST 既有 fail-closed 同模式): cross-channel non-member → 404 channel hidden (跟 messages.go::handleCreateMessage 行 230-232 既有 path 同字符 `Channel not found`); 同 channel 非 member 私 channel → 403 (但既有 messages-rail 是 404 一致, AP-4 沿用 404 byte-identical). **反向断**: e2e seed 私 channel 消息 → other-user PUT/DELETE/GET 全 reject (3 sub-case fail-closed 全锁).

4. **0 schema 改 + audit 不另起** (forward-only audit byte-identical 跟 messages.go 既有 audit 锁链同源): reaction PUT/DELETE 既有 hub.BroadcastEvent + Store.CreateEvent (kind="reaction_update") 不动; AP-4 仅加 ACL gate, 不改 audit. **反约束**: 反向 grep `reaction_audit\|reactions.*log` 在 internal/ count==0 (复用既有 events 表).

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| AP-4.1 server | `internal/api/reactions.go::handleAddReaction` + `handleRemoveReaction` + `handleGetReactions` 改 (≤30 行 — 3 处加同模式 ACL gate, 复用 IsChannelMember + CanAccessChannel; GET 也加 user-required check) | 0 schema 改; 错误 byte-identical "Channel not found" / "Message not found"; 既有行为 (member 操作) 0 改 |
| AP-4.2 unit + e2e | `internal/api/reactions_acl_test.go` (新, 4 unit case) + `packages/e2e/tests/ap-4-reactions-acl.spec.ts` (3 case) | unit: PUT non-member reject / DELETE non-member reject / GET non-member reject / member 三动作 OK; e2e: 真 cross-channel 路径 5/5 deterministic |
| AP-4.3 closure | REG-AP4-001..005 + acceptance + PROGRESS [x] + content-lock 不需 (server only) | 反向 grep 守门 + REG-DM5-005 cross-link (gap 闭合, 翻 active 备注) |

## 2. 错误码 byte-identical (跟 messages.go 既有同字符)

- 复用 messages.go::handleCreateMessage 既有 `"Channel not found"` (404) 跟 messages POST 同 fail-closed.
- 复用既有 `"Message not found"` (404) 当 message 不存在.
- 复用既有 `"Unauthorized"` (401) 当 user==nil.

**0 新错码**.

## 3. 反向 grep 锚 (AP-4 实施 PR 必跑)

```
git grep -nE 'reactions.*PUT.*\!channel_member|reactions.*GET.*public' packages/server-go/internal/  # 0 hit (gap 修)
git grep -nE 'admin.*reactions|admin.*reaction.*PUT' packages/server-go/internal/api/admin   # 0 hit (ADM-0 §1.3)
git grep -nE 'reaction_audit|reactions.*log' packages/server-go/internal/  # 0 hit (audit 不另起)
git grep -nE 'IsChannelMember.*reactions|CanAccessChannel.*reactions' packages/server-go/internal/api/reactions.go  # ≥ 3 hit (3 handler 各加 gate)
```

## 4. 不在本轮范围 (deferred)

- ❌ 抗 enumeration message_id (留 v2; 当前 UUID 难枚举已是事实上保护)
- ❌ admin god-mode reactions visibility (ADM-0 §1.3 红线)
- ❌ reactions 限频 (rate-limit 留 spam Phase)
- ❌ 跨 org 反向断 (复用 store.CrossOrg 既有, 不属 AP-4)
- ❌ schema migration (0 schema 改)
