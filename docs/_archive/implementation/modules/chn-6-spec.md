# CHN-6 spec brief — channel pin/unpin 顶置 (战马D v0)

> Phase 6 channel pin/unpin 闭环 — 用户顶置 favorite channels. CHN-3.1
> #410 user_channel_layout 既有 (user_id, channel_id, collapsed, position,
> created_at, updated_at) 已落; **0 schema 改** — pin 状态通过既有
> `position < 0` 字面约定承载 (server 分配 `position = -(unix_ms)` →
> ASC 排序自然得 "最近 pin 在最顶"; un-pin 走正 position 回归普通排序).
> 跟 CHN-5 #542 + AL-8 #538 同精神 0 schema.

## §0 立场 (3 + 3 边界)

- **①** **0 schema 改** — user_channel_layout 列 byte-identical 跟 CHN-3.1
  #410 不动. pin 状态走 `position < 0` 字面约定单源 (PinThreshold=0
  const + IsPinned 谓词). 反向 grep `migrations/chn_6_\d+|ALTER TABLE
  user_channel_layout ADD COLUMN.*pinned\|pinned_at` 在 internal/migrations/
  0 hit. CHN-3.2 既有 PUT /me/layout 字面 byte-identical 不动.
- **②** owner-only ACL — pin/unpin per-user (走 cm.user_id 跟 CHN-3.2
  layout endpoint 同精神); 别 user 看不到 pin 状态 (cross-user 不漏);
  admin god-mode 不挂 PATCH/POST (反向 grep `admin.*pin_channel\|
  admin.*pin\b` 在 admin*.go 0 hit) — admin 看不到也改不了 pin (per-user
  preference, 立场 ⑤ ADM-0 §1.3 红线 + CHN-3.2 既有承袭). owner-only
  ACL 锁链第 14 处.
- **③** pin 状态 client/server 双源不漂 — server const PinThreshold=0
  字面单源 + client constant `POSITION_PIN_THRESHOLD=0` 字面 byte-
  identical (双向锁: 改一处 = 改两处). server PinChannel 写 position =
  -(unix_ms); UnpinChannel 写 position = max(positive)+1.0 (client 既有
  MIN-1.0 单调小数模式 byte-identical 跟 CHN-3.3 #415 拖拽承袭).

边界:
- **④** POST /api/v1/channels/{channelId}/pin (置顶) + DELETE
  /api/v1/channels/{channelId}/pin (取消) — REST 二态; user-rail only;
  body 空 (action 由 method 决定).
- **⑤** 文案 byte-identical 跟 content-lock §1: button `置顶` 1 字 /
  `取消置顶` 3 字; section `已置顶频道` 4 字; 同义词反向 reject
  (`收藏 / 标星 / star / favorite / top`).
- **⑥** AST 锁链延伸第 11 处 forbidden 3 token (`pendingChannelPin /
  channelPinQueue / deadLetterChannelPin`) 在 internal/api 0 hit (跟
  BPP-4/5/6/7/8/HB-3 v2/AL-7/AL-8/HB-5/CHN-5 同模式).

## §1 拆段

**CHN-6.1 — server**:
- `internal/store/queries.go::PinChannel(userID, channelID, nowMs)` —
  UPSERT user_channel_layout SET position = -(nowMs) (跟 CHN-3.2 既有
  upsert 同模式 ON CONFLICT DO UPDATE). DM 反 (channel.Type='dm' →
  err `layout.dm_not_grouped` byte-identical 跟 CHN-3.2 同源).
- `UnpinChannel(userID, channelID, nowMs)` — UPSERT position = "max正
  position+1.0" (跟 client MIN-1.0 单调小数模式互补).
- `internal/api/chn_6_pin.go` POST + DELETE /api/v1/channels/{channelId}/
  pin; user-rail authMw; non-member 403; DM 400; PinThreshold=0 const.

**CHN-6.2 — client**:
- `lib/api.ts::pinChannel(channelId, pinned: boolean)` 单源.
- `components/PinButton.tsx` toggle button — 文案 `置顶` ↔ `取消置顶`;
  data-action="pin" / "unpin"; click → pinChannel + reload.
- `components/PinnedChannelsSection.tsx` 顶部 section — `已置顶频道`
  4 字 + filter `channel.position < POSITION_PIN_THRESHOLD` byte-identical.
- vitest 5 case (button 文案 / 同义词反向 / pin click / unpin click /
  section 顶部分组).

**CHN-6.3 — closure**: REG-CHN6-001..006 6 🟢 + AST scan 反向 + audit 5
字段链第 11 处 (ADM-2.1+AP-2+BPP-4+BPP-7+BPP-8+HB-3 v2+AL-7+AL-8+HB-5+
CHN-5+CHN-6).

## §2 反约束 grep 锚

- 0 schema: `migrations/chn_6_\d+|ALTER TABLE user_channel_layout` 0 hit.
- admin god-mode 不挂: `admin.*pin_channel\|admin.*pin\b\|/admin-api/.*pin`
  在 admin*.go 0 hit.
- AST 锁链延伸第 11 处 forbidden 3 token 0 hit.
- 同义词反向 (`收藏\|标星\|star\|favorite\|top`) 在 PinButton/PinnedSection 0 hit.
- PinThreshold 双向锁: server const 0 字面 = client const 0 字面.

## §3 不在范围

- pin 数量上限 / pin 排序拖拽 (留 v3 — 现网 pin asc 自然按时间排).
- pin 跨设备同步推送 (RT-3 fan-out 不挂 pin frame, 留 v3).
- admin god-mode pin override (永久不挂 ADM-0 §1.3).
- DM pin (永久不挂 — DM 走 CHN-4 dm-only, 立场 ④).
- pin folder / nested grouping (留 v3 跟 CHN-3.3 follow-up).
