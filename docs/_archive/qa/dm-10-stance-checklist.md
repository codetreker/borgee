# DM-10 stance checklist — DM message pin

> 5 立场 byte-identical 跟 spec §0 (≤80 行).

## 1. schema 1 列 ALTER (跟 DM-7.1 / AL-7.1 / AP-2.1 跨九 milestone 同模式)

- [x] `messages.pinned_at INTEGER NULL` 单源 (NULL = unpinned, Unix ms = pinned)
- [x] sparse partial idx `idx_messages_pinned_at(channel_id, pinned_at DESC) WHERE pinned_at IS NOT NULL` (跟 AL-7.1 archived_at sparse 同模式现网零开销)
- [x] hasColumn(channel_id) guard — minimal test seed `messages(id TEXT PRIMARY KEY)` 无 channel_id 时跳 idx (forward-only best-effort)
- [x] **反约束**: 不另起 `pinned_messages` 表; 不挂 `pinned_by` / `pin_reason` 列 (per-DM scope, 反 per-user pin 留 v2)
- [x] v=45 sequencing (team-lead 占号 reservation, post chn-15 v=44 / ADM-3 v=43)

## 2. DM-only path (跟 dm_4_message_edit.go::handleEdit 同精神)

- [x] channel.Type != "dm" → 400 `pin.dm_only_path` (跟 `dm.edit_only_in_dm` 同模式)
- [x] 单测 + 反向枚举 — non-DM channel pin reject 字面 byte-identical
- [x] v2 留账: 非 DM channel pin (留 future per-user pin 跟 user_channel_layout 风格)

## 3. channel-member ACL gate (复用 AP-4 #551 + AP-5 #555 同 helper)

- [x] `Store.IsChannelMember(channelID, user.ID) && Store.CanAccessChannel(channelID, user.ID)` 双 helper
- [x] 失败 → 404 "Channel not found" byte-identical 跟 messages.go::handleCreateMessage 既有 fail-closed 字符
- [x] 反 cross-channel pin (msg.ChannelID != path channelID → 404 fail-closed)
- [x] AP-4/AP-5 owner-only ACL 锁链承袭 (DM-10 是第 N 处)

## 4. POST/DELETE 互补二式 + GET list

- [x] POST `/api/v1/channels/{channelId}/messages/{messageId}/pin` → 立 pinned_at = now() (idempotent, last-write-wins)
- [x] DELETE 同路径 → 立 pinned_at = NULL (idempotent, unpin 未 pinned 200)
- [x] GET `/api/v1/channels/{channelId}/messages/pinned` → list pinned_at IS NOT NULL ORDER BY pinned_at DESC (走 sparse partial idx)
- [x] 单测覆盖 happy/non-DM/unauthz/non-member/deleted/cross-channel/idempotent-pin/idempotent-unpin 8 case 全 PASS

## 5. admin god-mode 不挂 (ADM-0 §1.3 红线)

- [x] 反向 grep `admin.*pin.*messages|/admin-api/.*messages.*pin` 在 admin*.go 0 hit
- [x] 跟 DM-4/CV-7/AP-4/AP-5 owner-only 锁链承袭
- [x] PR #571 admin-godmode 总表 §2 模式延伸 — admin 不入 message pin 业务路径

## 反约束

- ❌ pinned_messages 表 (单源 messages.pinned_at 列)
- ❌ pinned_by / pin_reason 列 (per-DM scope, 反 per-user/reason 留 v2)
- ❌ 非 DM channel pin (现版 DM-only 硬锁)
- ❌ admin god-mode pin override (永久不挂)
- ❌ pin 计数限制 (留 v3 spam 防御)
- ❌ pinned message WS push (留 v3 — pin/unpin 不实时同步多端)

## 跨 milestone byte-identical 锁链

- DM-7.1 #558 messages.edit_history (ALTER ADD COLUMN nullable 同模式)
- AL-7.1 #533 admin_actions.archived_at (sparse partial idx WHERE 同模式)
- AP-2.1 #525 / AP-1.1 #493 / AP-3.1 / HB-5.1 (跨九 milestone ALTER ADD nullable)
- CHN-6 #544 channel pin (PinThreshold 互补 — 但 DM-10 是 message-level, CHN-6 是 channel-level)
- CHN-7 mute / CHN-15 readonly — 互补二式同模式 (POST/DELETE)
- AP-4 #551 reactions ACL gate / AP-5 #555 messages ACL gate (channel-member ACL helper 复用)
- dm_4_message_edit.go #549 DM-only path (`dm.edit_only_in_dm` 同精神)
- ADM-0 §1.3 红线 (admin god-mode 不挂)
