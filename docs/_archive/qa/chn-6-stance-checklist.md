# CHN-6 立场反查清单 (战马D v0)

> 战马D · 2026-04-30 · 立场 review checklist (跟 CHN-5/AL-8/AL-7 stance 同模式)
> **目的**: CHN-6 三段实施 (6.1 server / 6.2 client / 6.3 closure) PR review 时, 飞马/野马/烈马按此清单逐立场 sign-off, 反向断言代码层守住每条立场.
> **关联**: spec `chn-6-spec.md` + acceptance `acceptance-templates/chn-6.md` + content-lock `chn-6-content-lock.md`
> **content-lock 必锁** — pin/unpin button DOM + `已置顶频道` section + 同义词反向.

## §0 立场总表 (3 立场 + 3 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | **0 schema 改** — user_channel_layout 列 byte-identical 跟 CHN-3.1 #410 不动 (user_id, channel_id, collapsed, position, created_at, updated_at); pin 状态走 `position < 0` 字面约定单源 + PinThreshold=0 const 双向锁 | CHN-3.1 #410 user_channel_layout 单源 + AL-8/CHN-5 0 schema 立场承袭 | 反向 grep `migrations/chn_6_\d+\|ALTER TABLE user_channel_layout\|pinned_at` 在 internal/migrations/ 0 hit |
| ② | owner-only ACL — pin/unpin per-user (走 cm.user_id 跟 CHN-3.2 layout endpoint 同精神); 别 user 看不到 pin 状态 (cross-user 不漏); admin god-mode **不挂 PATCH/POST** (反向 grep `admin.*pin_channel\|admin.*pin\b\|/admin-api/.*pin` 在 admin*.go 0 hit) — owner-only ACL 锁链第 14 处 (CHN-5 #13 承袭) | admin-model.md ADM-0 §1.3 红线 + CHN-3.2 layout 立场承袭 + AL-2a/BPP-3.2/AL-1/AL-5/DM-4/CV-4 v2/BPP-7/BPP-8/CHN-5 13 处承袭 | 反向 grep 双 pattern 0 hit; PinChannel/UnpinChannel store helper 必带 user_id (反向 grep `PinChannel\(channelID` 不带 user_id 0 hit) |
| ③ | pin 状态 client/server 双源不漂 — server `PinThreshold=0` const 字面单源 + client `POSITION_PIN_THRESHOLD=0` 字面 byte-identical (改一处 = 改两处, 双向锁); server PinChannel 写 position = -(nowMs) (ASC 排序自然 "最近 pin 在最顶"); UnpinChannel 写 position = max(positive)+1.0 (跟 CHN-3.3 #415 拖拽 MIN-1.0 单调小数模式 byte-identical) | CHN-3.3 #415 client 拖拽 MIN-1.0 模式承袭 | const 双向锁 vitest + go test (改 = 改两处编译期检查) |

边界:
- **④** REST 二态 — POST /api/v1/channels/{channelId}/pin (置顶) + DELETE /api/v1/channels/{channelId}/pin (取消); user-rail authMw 必经; body 空; non-member 403 (跟 CHN-1 ACL 同源); DM 400 错码字面 byte-identical 跟 CHN-3.2 `layout.dm_not_grouped`.
- **⑤** 文案 byte-identical 跟 content-lock §1 — button `置顶` 1 字 / `取消置顶` 3 字 + section `已置顶频道` 4 字 + 同义词反向 reject (`收藏/标星/star/favorite/top` 0 hit).
- **⑥** AST 锁链延伸第 11 处 forbidden 3 token (`pendingChannelPin / channelPinQueue / deadLetterChannelPin`) 在 internal/api 0 hit (跟 BPP-4/5/6/7/8 + HB-3 v2 + AL-7/8 + HB-5 + CHN-5 同模式).

## §1 立场 ① 0 schema 改 (CHN-6.1 守)

- [ ] migrations/ 0 新文件 (反向 grep `migrations/chn_6_` 0 hit)
- [ ] registry.go 字面 byte-identical 跟 main 不动
- [ ] user_channel_layout 列复用 CHN-3.1 #410 既有 6 列, 不另起 pinned_at
- [ ] CHN-3.2 既有 PUT /me/layout endpoint + GET 字面 byte-identical 不动
- [ ] PinThreshold=0 字面单源 + IsPinned(position) 谓词单源

## §2 立场 ② owner-only + admin god-mode 不挂 (CHN-6.1 守)

- [ ] PinChannel(userID, channelID) — user_id 必传, 反向 grep 不带 user_id 0 hit
- [ ] UnpinChannel(userID, channelID) — 同上
- [ ] non-member 403 (跟 CHN-1 + CHN-3.2 ACL 同源)
- [ ] DM 400 `layout.dm_not_grouped` byte-identical 跟 CHN-3.2
- [ ] admin god-mode 不挂 (反向 grep `admin.*pin_channel\|admin.*pin\b\|/admin-api/.*pin` 在 admin*.go 0 hit)
- [ ] owner-only ACL 锁链第 14 处 (AL-2a/BPP-3.2/AL-1/AL-5/DM-4/CV-4 v2/BPP-7/BPP-8/CHN-5/CHN-6)

## §3 立场 ③ 双源不漂 (CHN-6.1+6.2 守)

- [ ] server PinThreshold=0 const 字面
- [ ] client POSITION_PIN_THRESHOLD=0 字面 byte-identical
- [ ] server PinChannel 写 position = -(nowMs) (ASC asc 排序最顶)
- [ ] server UnpinChannel 写 position = max(positive)+1.0 跟 CHN-3.3 MIN-1.0 模式互补
- [ ] client filter `channel.position < POSITION_PIN_THRESHOLD` byte-identical
- [ ] vitest + go test 双向 const 锁 (改 = 改两处)

## §4 蓝图边界 ④⑤⑥ — 不漂

- [ ] REST POST/DELETE /api/v1/channels/{channelId}/pin
- [ ] user-rail authMw 必经
- [ ] DM 400 + non-member 403 + Unauthenticated 401
- [ ] button 文案 byte-identical (`置顶` / `取消置顶`)
- [ ] section 文案 `已置顶频道` byte-identical
- [ ] 同义词反向 (`收藏/标星/star/favorite/top`) 在 PinButton/PinnedSection 0 hit
- [ ] AST 锁链延伸第 11 处 forbidden 3 token 0 hit

## §5 退出条件

- §1 (5) + §2 (6) + §3 (6) + §4 (7) 全 ✅
- 反向 grep 5 项全 0 hit (新 schema / admin god-mode / 同义词 / pin queue / pinned_at 列)
- audit 5 字段链 CHN-6 = 第 11 处 (ADM-2.1+AP-2+BPP-4+BPP-7+BPP-8+HB-3 v2+AL-7+AL-8+HB-5+CHN-5+CHN-6)
- AST 锁链延伸第 11 处
- owner-only ACL 锁链第 14 处一致
- PinThreshold 双向锁 (server + client byte-identical)
- 文案 byte-identical 跟 content-lock + 同义词反向
- 登记 REG-CHN6-001..006
