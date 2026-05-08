# DM-10 spec brief — DM message pin (≤80 行)

> 战马E · Phase 5+ · ≤80 行 · 蓝图 [`dm-model.md`](../../blueprint/dm-model.md) §3 future per-user message layout v2 (本 PR v0 = per-DM pin, 双方共享 pinned_at 列). DM-10 给 DM message 加 pin/unpin 视觉 + 列表; **schema 1 列 ALTER** (跟 DM-7.1 edit_history 风格), DM-only scope 守门. 跟 CHN-6 channel pin / CHN-7 mute / CHN-15 readonly 互补二式同模式.

## 0. 关键约束 (5 项立场)

1. **schema 1 列 ALTER** — `messages.pinned_at INTEGER NULL` 单源 (跟 DM-7.1 edit_history / AL-7.1 archived_at / AP-2.1 revoked_at 跨九 milestone ALTER ADD COLUMN nullable 同模式). NULL = unpinned (默认), Unix ms = pinned. **反约束**: 不另起 `pinned_messages` 表 (反向 grep `pinned_messages\|message_pin_log\|dm10_pin_table` 0 hit); 不挂 `pinned_by` 列 (DM 双方都可 pin, per-DM scope, 反 per-user pin 留 v2 跟 CHN-3.2 user_channel_layout 风格不同源).

2. **DM-only path** — channel.Type != "dm" → 400 `pin.dm_only_path` (跟 dm_4_message_edit.go::handleEdit DM-only 同精神). 反约束: 非 DM channel pin reject (vitest+unit 双锁).

3. **channel-member ACL gate** — 复用 AP-4 #551 reactions + AP-5 #555 messages 同 helper `Store.IsChannelMember + Store.CanAccessChannel`; 非 member → 404 "Channel not found" fail-closed (跟 messages.go::handleCreateMessage 既有同字符).

4. **POST/DELETE 互补二式 + GET list** — POST 立 pinned_at = now() (idempotent, 二次 pin 覆写); DELETE 立 pinned_at = NULL (idempotent, unpin 未 pinned 200); GET list `pinned_at IS NOT NULL ORDER BY pinned_at DESC` 走 sparse partial idx 现网零开销 (跟 AL-7.1 idx_archived_at 同模式).

5. **admin god-mode 不挂** — 反向 grep `admin.*pin.*messages|/admin-api/.*messages.*pin` 在 admin*.go 0 hit (ADM-0 §1.3 红线, 跟 DM-4/CV-7/AP-4/AP-5 owner-only 锁链承袭).

## 1. 拆段实施 (单 PR 全闭)

| 段 | 文件 | 范围 |
|---|---|---|
| DM-10.1 schema | `internal/migrations/dm_10_1_messages_pinned_at.go` (新 v=45) + `_test.go` (4 unit) | ALTER messages ADD pinned_at INTEGER NULL + sparse partial idx (channel_id + pinned_at DESC WHERE pinned_at IS NOT NULL); idempotent + nullable + version=45 锁 |
| DM-10.2 server | `internal/api/dm_10_pin.go` (新, 3 endpoint) + `internal/store/dm_10_pin_queries.go` (新, 2 helper) + server.go register + models.go Message struct 加 PinnedAt 字段 + `_test.go` (8 acceptance) | DM-only ACL gate + channel-member gate + POST/DELETE/GET 三 endpoint + happy/non-DM/unauthz/non-member/deleted/cross-channel/idempotent 8 case |
| DM-10.3 closure | REG-DM10-001..006 + acceptance + content-lock + PROGRESS [x] | 6 立场 byte-identical 锁 + 反向 grep 4 锚 |

## 2. 错误码 byte-identical (跟 chn_6_pin / dm_4 既有同模式)

- `pin.dm_only_path` (400) — non-DM channel pin reject (跟 dm.edit_only_in_dm 同精神)

**0 新错码**: 401 "Unauthorized" / 404 "Channel not found" / 404 "Message not found" 全复用既有.

## 3. 反向 grep 锚 (DM-10 实施 PR 必跑)

```
git grep -nE 'pinned_messages|message_pin_log|dm10_pin_table' packages/server-go/internal/  # 0 hit (单源 messages.pinned_at 列)
git grep -nE 'admin.*pin.*messages|/admin-api/.*messages.*pin' packages/server-go/internal/api/admin*.go  # 0 hit (ADM-0 §1.3)
git grep -nE 'pinned_by\|pin_reason' packages/server-go/internal/store/models.go  # 0 hit (per-DM scope, 反 per-user/reason 留 v2)
git grep -c 'IsChannelMember.*messageID\|channel-member.*pin' packages/server-go/internal/api/dm_10_pin.go  # ≥1 (ACL gate 复用 AP-4/AP-5)
```

## 4. 不在本轮范围 (deferred)

- ❌ per-user pin (留 v2 — 走 user_message_layout 表, 跟 CHN-3.2 user_channel_layout 同模式)
- ❌ pin reason / pin_note (留 v2 — JSON metadata 列复用 messages.metadata)
- ❌ 非 DM channel pin (留 v2 — 现版 DM-only scope 硬锁)
- ❌ admin god-mode pin override (永久不挂, ADM-0 §1.3)
- ❌ pin 计数限制 (留 v3 spam 防御)
- ❌ pinned message WS push (留 v3 — pin/unpin 不实时同步多端)
