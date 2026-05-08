# CHN-8 spec brief — channel notification preferences (战马D v0)

> Phase 6 channel notification 三态 — `所有消息` / `仅@提及` / `不打扰`.
> **0 schema 改** (复用 CHN-3.1 user_channel_layout.collapsed INTEGER 列
> bitmap bits 2-3, 跟 CHN-7 #550 bit 1 mute + CHN-3 bit 0 collapsed 拆死
> 同模式). 跟 CHN-6 PinThreshold + CHN-7 MuteBit 双向锁同精神 — 不另起列.

## §0 立场 (3 + 3 边界)

- **①** **0 schema 改** — user_channel_layout.collapsed bitmap 扩展:
  bit 0 (CHN-3 折叠) / bit 1 (CHN-7 静音) / **bits 2-3 (CHN-8 通知偏好
  3 态)** — `(collapsed >> 2) & 3` 字面约定 + NotifPrefShift=2 +
  NotifPrefMask=3 (覆盖 2 位) + NotifPrefAll=0 / NotifPrefMention=1 /
  NotifPrefNone=2 const 双向锁 (server + client byte-identical). 反向
  reject (collapsed >> 2) & 3 = 3 (3 留 reserved/invalid). 反向 grep
  `migrations/chn_8_\d+|ALTER TABLE user_channel_layout` 0 hit.
- **②** owner-only ACL — per-user 偏好 (cm.user_id 跟 CHN-3.2/CHN-6/CHN-7
  同精神); 别 user 看不到; admin god-mode **不挂 PUT/POST** (反向 grep
  `admin.*notification.*pref\|admin.*notif_pref\b` 在 admin*.go 0 hit) —
  owner-only ACL 锁链第 16 处 (CHN-7 #15 承袭).
- **③** mention/all/none 不 drop messages — CreateMessage / RT-3 fan-out
  / WS frame 全 byte-identical 不动. 偏好仅影响 DL-4 push notifier:
  - `all` (0): 现网行为零变 (push 收 message + mention 全).
  - `mention`: push 仅在 @mention 时触发 (DM-2 mention dispatcher 谓词
    check, 复用 isMutedForUser 同 seam).
  - `none`: 不发任何 push (合并 mute 行为, 但保留 in-app indicator 区分).

边界:
- **④** REST PUT /api/v1/channels/{channelId}/notification-pref + GET 复用
  既有 GET /api/v1/me/layout (collapsed 字段已含 bits 2-3); user-rail
  authMw; body `{pref: 'all'|'mention'|'none'}`; DM 400 (跟 CHN-6/7 同源
  byte-identical); non-member 403; spec 外值 → 400 `notification_pref.
  invalid_value`.
- **⑤** 文案 byte-identical 跟 content-lock §1 — dropdown 三选一选项
  `所有消息` 4 字 / `仅@提及` 4 字 (含 `@`) / `不打扰` 3 字; 同义词反向
  reject (`subscribe / follow / unsubscribe / snooze / 订阅 / 关注 / 取消订阅`).
- **⑥** AST 锁链延伸第 13 处 forbidden 3 token (`pendingNotifPref /
  notifPrefQueue / deadLetterNotifPref`) 在 internal/api+push 0 hit.

## §1 拆段

**CHN-8.1 — server**:
- `internal/store/queries.go::SetNotifPref(userID, channelID, pref)` —
  UPSERT collapsed = (collapsed & ~(NotifPrefMask<<NotifPrefShift)) |
  (pref << NotifPrefShift). 不动其他位.
- `GetNotifPref(userID, channelID)` — 返 NotifPrefAll/Mention/None.
- `internal/api/chn_8_notif_pref.go` PUT /api/v1/channels/{channelId}/
  notification-pref + 三态字面 const + IsNotifAll/Mention/None 谓词.

**CHN-8.2 — client**:
- `lib/api.ts::setNotificationPref(channelId, pref)` 单源.
- `lib/notif_pref.ts::NOTIF_PREF_SHIFT=2 / NOTIF_PREF_MASK=3 / NOTIF_*`
  const 字面 byte-identical 跟 server + getNotifPref(collapsed) 谓词单源.
- `components/NotificationPrefDropdown.tsx` `<select>` 三选一; 文案
  byte-identical 跟 content-lock; data-pref 三态.

**CHN-8.3 — closure**: REG-CHN8-001..006 6 🟢 + AST scan 反向 + audit
5 字段链第 13 处.

## §2 反约束 grep 锚

- 0 schema: `migrations/chn_8_\d+|ALTER TABLE user_channel_layout` 0 hit.
- admin 不挂: `admin.*notification.*pref\|admin.*notif_pref` 在 admin*.go 0 hit.
- 不 drop messages: `notif_pref.*skip.*broadcast` 在 internal/ws+api 0 hit.
- AST 锁链延伸第 13 处 forbidden 3 token 0 hit.
- NotifPref const 三向锁: server + client + bitmap 字面 byte-identical.

## §3 不在范围

- 自动通知 quiet hours / 时段静默 (留 v3 全局 user-settings).
- per-keyword 通知 (留 v3 keyword highlight 跟 mention 不同维度).
- admin god-mode notif pref override (永久不挂 ADM-0 §1.3).
- DM notif pref (永久不挂 — DM 走 CHN-4 dm-only).
- 跨设备 pref 推送 (RT-3 不挂 pref frame, 留 v3).
