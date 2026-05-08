# CHN-7 立场反查清单 (战马D v0)

> 战马D · 2026-04-30 · 立场 review checklist (跟 CHN-6/CHN-5/AL-8 stance 同模式)
> **目的**: CHN-7 三段实施 (7.1 server / 7.2 client / 7.3 closure) PR review 时, 飞马/野马/烈马按此清单逐立场 sign-off, 反向断言代码层守住每条立场.
> **关联**: spec `chn-7-spec.md` + acceptance `acceptance-templates/chn-7.md` + content-lock `chn-7-content-lock.md`
> **content-lock 必锁** — mute button DOM + `已静音` indicator + 同义词反向 + MuteBit 双向锁.

## §0 立场总表 (3 立场 + 3 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | **0 schema 改** — user_channel_layout 列 byte-identical 跟 CHN-3.1 #410 不动; mute 走 `collapsed & 2` bitmap 字面约定 + MuteBit=2 const 双向锁 (server + client byte-identical). collapsed bit 0 = 折叠 (CHN-3 既有), bit 1 = 静音 (CHN-7 新增) — 现有写 0/1 行为零变 | CHN-3.1 #410 user_channel_layout 单源 + AL-8/CHN-5/CHN-6 0 schema 立场承袭 | 反向 grep `migrations/chn_7_\d+\|ALTER TABLE user_channel_layout.*muted` 0 hit |
| ② | owner-only ACL — mute per-user (cm.user_id 走 IsChannelMember 跟 CHN-3.2/CHN-6 同精神); 别 user 看不到 mute 状态; admin god-mode **不挂 PATCH/POST** — owner-only ACL 锁链第 15 处 (CHN-6 #14 承袭) | admin-model.md ADM-0 §1.3 红线 + CHN-6 owner-only 立场承袭 | 反向 grep `admin.*mute_channel\|admin.*mute\b\|/admin-api/.*mute` 在 admin*.go 0 hit |
| ③ | mute 不 drop messages best-effort — 只影响 push notification (DL-4 web-push gateway 走 IsMutedForUser skip); messages 写入 + RT-3 fan-out + WS frame 全 byte-identical 不动 | DL-4 push gateway 立场承袭 + best-effort 跟 BPP-4/5 同精神 | 反向 grep `mute.*skip.*broadcast\|mute.*drop.*message\|mute.*hub.*skip` 在 internal/ws+internal/api/messages*.go 0 hit; messages CreateMessage 路径不动 |

边界:
- **④** REST 二态 — POST + DELETE /api/v1/channels/{channelId}/mute; user-rail authMw 必经; body 空; DM 400 byte-identical 跟 CHN-6 同源; non-member 403.
- **⑤** 文案 byte-identical 跟 content-lock §1 — button `静音` 2 字 ↔ `取消静音` 4 字 + indicator `已静音` 3 字 + 同义词反向 reject (`mute/silence/dnd/disturb/quiet/屏蔽/关闭通知/勿扰`).
- **⑥** AST 锁链延伸第 12 处 forbidden 3 token (`pendingChannelMute / channelMuteQueue / deadLetterChannelMute`) 在 internal/api+push 0 hit.

## §1 立场 ① 0 schema 改 (CHN-7.1 守)

- [ ] migrations/ 0 新文件 (反向 grep `migrations/chn_7_` 0 hit)
- [ ] registry.go 字面 byte-identical 跟 main 不动
- [ ] user_channel_layout 列复用 CHN-3.1 既有, 不另起 muted_until / muted 列
- [ ] CHN-3.2 既有 PUT /me/layout endpoint byte-identical 不动
- [ ] MuteBit=2 字面单源 + IsMuted(collapsed) 谓词单源

## §2 立场 ② owner-only + admin god-mode 不挂 (CHN-7.1 守)

- [ ] SetMuteBit(userID, channelID, muted) — user_id 必传
- [ ] IsMutedForUser(userID, channelID) — user_id 必传
- [ ] non-member 403 (跟 CHN-1 + CHN-3.2 + CHN-6 ACL 同源)
- [ ] DM 400 (跟 CHN-6 同源)
- [ ] admin god-mode 不挂 (反向 grep `admin.*mute_channel\|admin.*mute\b` 在 admin*.go 0 hit)
- [ ] owner-only ACL 锁链第 15 处一致 (AL-2a/BPP-3.2/AL-1/AL-5/DM-4/CV-4 v2/BPP-7/BPP-8/CHN-5/CHN-6/CHN-7)

## §3 立场 ③ mute 不 drop messages (CHN-7.1 守)

- [ ] CreateMessage 路径 byte-identical 不动 — 反向 grep `mute.*skip.*broadcast` 0 hit
- [ ] RT-3 fan-out 不查 mute — 反向 grep `IsMutedForUser` 在 internal/ws/ 0 hit
- [ ] DL-4 push notifier 加 IsMutedForUser check (mute 跳过 push 通知)
- [ ] WS frame 投递不动 — mute 用户 connection 仍收 message frame
- [ ] mute 仅是 push notification skip — 反向 grep `mute.*hub.*skip\|mute.*drop.*message` 0 hit

## §4 蓝图边界 ④⑤⑥ — 不漂

- [ ] REST POST + DELETE /api/v1/channels/{channelId}/mute
- [ ] user-rail authMw 必经
- [ ] DM 400 + non-member 403 + Unauthenticated 401
- [ ] button 文案 byte-identical (`静音` / `取消静音`)
- [ ] indicator 文案 `已静音` byte-identical
- [ ] 同义词反向 (`mute/silence/dnd/disturb/quiet/屏蔽/关闭通知/勿扰`) 0 hit
- [ ] AST 锁链延伸第 12 处 forbidden 3 token 0 hit

## §5 退出条件

- §1 (5) + §2 (6) + §3 (5) + §4 (7) 全 ✅
- 反向 grep 5 项全 0 hit (新 schema / admin / 同义词 / mute queue / drop message)
- audit 5 字段链 CHN-7 = 第 12 处 (ADM-2.1+AP-2+BPP-4+BPP-7+BPP-8+HB-3 v2+AL-7+AL-8+HB-5+CHN-5+CHN-6+CHN-7)
- AST 锁链延伸第 12 处
- owner-only ACL 锁链第 15 处一致
- MuteBit=2 双向锁 (server + client byte-identical)
- 文案 byte-identical 跟 content-lock + 同义词反向
- 登记 REG-CHN7-001..006
