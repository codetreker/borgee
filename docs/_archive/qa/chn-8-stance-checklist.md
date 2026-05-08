# CHN-8 立场反查清单 (战马D v0)

> 战马D · 2026-04-30 · 立场 review checklist (跟 CHN-7/CHN-6 stance 同模式)
> **目的**: CHN-8 三段实施 (8.1 server / 8.2 client / 8.3 closure) PR review 时, 飞马/野马/烈马按此清单逐立场 sign-off, 反向断言代码层守住每条立场.
> **关联**: spec `chn-8-spec.md` + acceptance `acceptance-templates/chn-8.md` + content-lock `chn-8-content-lock.md`
> **content-lock 必锁** — pref dropdown DOM + 三选一文案 + 同义词反向 + NotifPref const 三向锁.

## §0 立场总表 (3 立场 + 3 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | **0 schema 改** — user_channel_layout.collapsed bitmap 扩展: bit 0 (CHN-3 折叠) / bit 1 (CHN-7 静音) / **bits 2-3 (CHN-8 通知偏好 3 态)**. NotifPrefShift=2 / NotifPrefMask=3 + NotifPrefAll=0 / NotifPrefMention=1 / NotifPrefNone=2 const 双向锁. (collapsed >> 2) & 3 = 3 reserved/invalid 反向 reject | CHN-3.1 #410 user_channel_layout 单源 + CHN-6 PinThreshold + CHN-7 MuteBit 双向锁模式承袭 | 反向 grep `migrations/chn_8_\d+\|ALTER TABLE user_channel_layout` 0 hit |
| ② | owner-only ACL — per-user 偏好 (cm.user_id 跟 CHN-3.2/CHN-6/CHN-7 同精神); admin god-mode **不挂 PUT/POST** — owner-only ACL 锁链第 16 处 (CHN-7 #15 承袭) | admin-model.md ADM-0 §1.3 红线 + CHN-7 owner-only 立场承袭 | 反向 grep `admin.*notification.*pref\|admin.*notif_pref\b\|/admin-api/.*notification` 在 admin*.go 0 hit |
| ③ | mention/all/none 不 drop messages — CreateMessage / RT-3 fan-out / WS frame 全 byte-identical 不动. 偏好仅影响 DL-4 push notifier: `all` 现网行为零变 / `mention` push 仅 @mention 触发 (DM-2 mention dispatcher 谓词 check) / `none` 不发 push 但 in-app frame 仍投递 | DL-4 push gateway 立场承袭 + CHN-7 立场 ③ best-effort 承袭 | 反向 grep `notif_pref.*skip.*broadcast\|notif_pref.*drop.*message` 在 internal/ws+messages*.go 0 hit |

边界:
- **④** REST PUT /api/v1/channels/{channelId}/notification-pref body `{pref: 'all'|'mention'|'none'}`; user-rail authMw 必经; spec 外值 → 400 `notification_pref.invalid_value`; DM 400 byte-identical 跟 CHN-6/7; non-member 403; Unauthorized 401.
- **⑤** 文案 byte-identical 跟 content-lock §1 — dropdown 三选一 `所有消息` 4 字 / `仅@提及` 4 字 / `不打扰` 3 字 + 同义词反向 reject (`subscribe/follow/unsubscribe/snooze/订阅/关注/取消订阅`).
- **⑥** AST 锁链延伸第 13 处 forbidden 3 token (`pendingNotifPref / notifPrefQueue / deadLetterNotifPref`) 在 internal/api+push 0 hit.

## §1 立场 ① 0 schema 改 (CHN-8.1 守)

- [ ] migrations/ 0 新文件 (反向 grep `migrations/chn_8_` 0 hit)
- [ ] registry.go byte-identical 跟 main 不动
- [ ] user_channel_layout 列复用 CHN-3.1 既有, 不另起 notif_pref / preferences 列
- [ ] CHN-3.2 既有 PUT /me/layout endpoint byte-identical 不动
- [ ] NotifPrefShift=2 + NotifPrefMask=3 + NotifPrefAll=0/Mention=1/None=2 字面单源
- [ ] (collapsed >> 2) & 3 = 3 reserved/invalid 反向 reject (反向 SetNotifPref(3) 0 hit)

## §2 立场 ② owner-only + admin god-mode 不挂 (CHN-8.1 守)

- [ ] SetNotifPref(userID, channelID, pref) — user_id 必传
- [ ] GetNotifPref(userID, channelID) — user_id 必传
- [ ] non-member 403 (跟 CHN-1 + CHN-3.2 + CHN-6/7 ACL 同源)
- [ ] DM 400 byte-identical (跟 CHN-6/7)
- [ ] admin god-mode 不挂 (反向 grep 0 hit)
- [ ] owner-only ACL 锁链第 16 处一致 (AL-2a/BPP-3.2/AL-1/AL-5/DM-4/CV-4 v2/BPP-7/BPP-8/CHN-5/CHN-6/CHN-7/CHN-8)

## §3 立场 ③ 不 drop messages (CHN-8.1 守)

- [ ] CreateMessage 路径 byte-identical 不动
- [ ] RT-3 fan-out 不查 notif_pref — 反向 grep `GetNotifPref` 在 internal/ws/ 0 hit
- [ ] DL-4 push notifier 加 GetNotifPref check (mention/all/none 三态 skip 逻辑)
- [ ] WS frame 投递不动 — 反向 grep `notif_pref.*hub.*skip` 0 hit
- [ ] in-app indicator 跟 DL-4 push 拆死 (none = 不 push 但仍 frame 投递)

## §4 蓝图边界 ④⑤⑥ — 不漂

- [ ] REST PUT /api/v1/channels/{channelId}/notification-pref
- [ ] body `pref` ∈ {'all','mention','none'} 反向 reject 外值 400 invalid_value
- [ ] user-rail authMw 必经 + DM 400 + non-member 403 + 401
- [ ] dropdown 文案 byte-identical (`所有消息` / `仅@提及` / `不打扰`)
- [ ] 同义词反向 0 hit (`subscribe/follow/unsubscribe/snooze/订阅/关注/取消订阅`)
- [ ] AST 锁链延伸第 13 处 forbidden 3 token 0 hit

## §5 退出条件

- §1 (6) + §2 (6) + §3 (5) + §4 (6) 全 ✅
- 反向 grep 5 项全 0 hit (新 schema / admin / 同义词 / pref queue / drop)
- audit 5 字段链 CHN-8 = 第 13 处
- AST 锁链延伸第 13 处
- owner-only ACL 锁链第 16 处一致
- NotifPref 三向锁 (server + client + bitmap)
- bitmap bit 2-3 跟 CHN-3 bit 0 + CHN-7 bit 1 互不干扰 (反向断言: 改 pref 不动 collapsed/mute bit)
- 文案 byte-identical 跟 content-lock + 同义词反向
- 登记 REG-CHN8-001..006
