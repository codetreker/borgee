# CHN-7 spec brief — channel mute / 静音 (战马D v0)

> Phase 6 channel mute/notification 静默 — per-user mute, **0 schema 改**
> (复用 CHN-3.1 user_channel_layout.collapsed INTEGER 列 bitmap 编码:
> bit 0 = collapsed (CHN-3 既有), bit 1 = muted (CHN-7 新增)). 跟 CHN-6
> #544 + CHN-5 #542 + AL-8 #538 同精神 0 schema. mute 走 best-effort
> notification skip — 不影响 fan-out 投递, 仅 push 通知端走 muted check.

## §0 立场 (3 + 3 边界)

- **①** **0 schema 改** — user_channel_layout 列 byte-identical 跟 CHN-3.1
  #410 不动. mute 状态走 `collapsed & 2` bitmap 字面约定 + MuteBit=2 const
  双向锁 (server + client byte-identical). collapsed 兼容性: bit 0 (=1)
  = 折叠态 (CHN-3 既有), bit 1 (=2) = 静音态 (CHN-7 新增); 现有 client
  写 collapsed=0/1 行为零变 (bit 1=0 默认 = 未静音). 反向 grep
  `migrations/chn_7_\d+|ALTER TABLE user_channel_layout ADD.*muted` 0 hit.
- **②** owner-only ACL — mute per-user (cm.user_id 走 IsChannelMember
  跟 CHN-3.2 / CHN-6 同精神); 别 user 看不到 mute 状态; admin god-mode
  **不挂 PATCH/POST** (反向 grep `admin.*mute_channel\|admin.*mute\b`
  在 admin*.go 0 hit) — owner-only ACL 锁链第 15 处 (CHN-6 #14 承袭).
- **③** mute 不影响 message 投递 best-effort — 只影响 push notification
  (DL-4 web-push gateway 走 muted skip); messages 表写入 + RT-3 fan-out
  + WS frame 全 byte-identical 不动 (反 message DROP / fan-out skip ——
  反向 grep `mute.*skip.*broadcast\|mute.*drop.*message` 在 internal/ws/
  + internal/api/messages*.go 0 hit). DL-4 push notifier 加 IsMuted 谓词
  check (复用 push.MentionNotifier seam).

边界:
- **④** REST 二态 — POST /api/v1/channels/{channelId}/mute (静音) +
  DELETE /api/v1/channels/{channelId}/mute (取消静音); user-rail authMw;
  body 空; DM 400 (跟 CHN-6/3.2 同源 byte-identical); non-member 403.
- **⑤** 文案 byte-identical 跟 content-lock §1: button `静音` 2 字 ↔
  `取消静音` 4 字; indicator `已静音` 3 字 (channel 行旁 emoji+text);
  同义词反向 reject (`mute / silence / dnd / disturb / quiet / 屏蔽 /
  关闭通知 / 勿扰`).
- **⑥** AST 锁链延伸第 12 处 forbidden 3 token (`pendingChannelMute /
  channelMuteQueue / deadLetterChannelMute`) 在 internal/api+push 0 hit
  (跟 BPP-4/5/6/7/8 + HB-3 v2 + AL-7+8 + HB-5 + CHN-5+6 同模式).

## §1 拆段

**CHN-7.1 — server**:
- `internal/store/queries.go::SetMuteBit(userID, channelID, muted, nowMs)`
  — UPSERT user_channel_layout SET collapsed = (collapsed & ~MuteBit) |
  (muted ? MuteBit : 0). MuteBit=2 const 单源.
- `IsMutedForUser(userID, channelID) (bool, error)` — SELECT collapsed
  WHERE (user_id, channel_id) → return collapsed & MuteBit != 0.
- `internal/api/chn_7_mute.go` POST + DELETE /api/v1/channels/{channelId}/
  mute; user-rail authMw; non-member 403; DM 400.
- `internal/push/mention_notifier.go` — Notify 调 IsMutedForUser 走
  best-effort skip (mute 用户该 channel 不收 push, 但 messages + WS
  frame 投递不变, 立场 ③).

**CHN-7.2 — client**:
- `lib/api.ts::muteChannel(channelId, muted: boolean)` 单源.
- `lib/mute.ts::MUTE_BIT=2` 字面 byte-identical 跟 server const + isMuted
  谓词单源.
- `components/MuteButton.tsx` toggle button — 文案 `静音` ↔ `取消静音`;
  data-action="mute" / "unmute".
- `components/MutedChannelIndicator.tsx` — `已静音` 3 字 emoji+text 行内
  indicator + 同义词反向.

**CHN-7.3 — closure**: REG-CHN7-001..006 6 🟢 + AST scan 反向 + audit
5 字段链第 12 处.

## §2 反约束 grep 锚

- 0 schema: `migrations/chn_7_\d+|ALTER TABLE user_channel_layout.*muted` 0 hit.
- admin god-mode 不挂: `admin.*mute_channel\|admin.*mute\b` 在 admin*.go 0 hit.
- 不 drop messages: `mute.*skip.*broadcast\|mute.*drop.*message` 0 hit.
- AST 锁链延伸第 12 处 forbidden 3 token 0 hit.
- MuteBit 双向锁: server 2 字面 = client 2 字面.

## §3 不在范围

- 自动 mute 时长 / muted_until 过期解锁 (留 v3 — 永久 mute v1).
- 跨设备 mute 状态推送 (RT-3 fan-out 不挂 mute frame, 留 v3).
- admin god-mode mute override (永久不挂 ADM-0 §1.3).
- DM mute (永久不挂 — DM 走 CHN-4 dm-only).
- 全局静默 (DnD 模式留 user-settings v3, 跟 mute 不同维度).
