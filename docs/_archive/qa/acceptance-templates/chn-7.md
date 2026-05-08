# Acceptance Template — CHN-7: channel mute / 静音

> 蓝图 channel-model.md §3 layout per-user. Spec `chn-7-spec.md` (战马D v0). Stance `chn-7-stance-checklist.md`. Content-lock `chn-7-content-lock.md`. **0 schema 改** — user_channel_layout.collapsed bitmap (bit 0 = collapsed CHN-3 既有, bit 1 = muted CHN-7). Owner: 战马D 实施 / 飞马 review / 烈马 验收.

## 验收清单

### §1 CHN-7.1 — server REST endpoints + bitmap + push skip

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 0 schema 改反向断言 — migrations/ 0 新文件 + ALTER TABLE user_channel_layout.*muted 0 hit + registry.go byte-identical 不动 | grep | 战马D / 飞马 / 烈马 | `internal/api/chn_7_mute_test.go::TestCHN71_NoSchemaChange` 1 unit PASS |
| 1.2 POST /api/v1/channels/{channelId}/mute owner-only — 200 + collapsed bit 1 set; non-member 403; DM 400; Unauthorized 401 | unit (4 sub-case) | 战马D / 烈马 | `TestCHN71_MuteChannel_HappyPath` (collapsed & MuteBit != 0) + `_MuteChannel_NonMemberRejected` (403) + `_MuteChannel_DMRejected` (400) + `_MuteChannel_Unauthorized` (401) |
| 1.3 DELETE /api/v1/channels/{channelId}/mute un-mute — 200 + collapsed bit 1 cleared; idempotent (二次 DELETE 200) + collapse bit (bit 0) preserved | unit (3 sub-case) | 战马D / 烈马 | `_UnmuteChannel_HappyPath` + `_UnmuteChannel_Idempotent` + `_UnmuteChannel_PreservesCollapsedBit` (mute + collapse → unmute → collapse 仍设) |
| 1.4 MuteBit=2 字面单源 + IsMuted(collapsed) 谓词单源 — server const byte-identical 跟 client `MUTE_BIT=2` 双向锁 | unit + grep | 战马D / 飞马 / 烈马 | `_MuteBit_ByteIdentical` (MuteBit==2 + IsMuted(0)==false + IsMuted(1)==false + IsMuted(2)==true + IsMuted(3)==true) + 双向锁: vitest 跟 go test 双向 reflect const |
| 1.5 admin god-mode 不挂 反向断言 — `admin.*mute_channel\|admin.*mute\b\|/admin-api/.*mute` 在 admin*.go 0 hit | grep | 战马D / 飞马 / 烈马 | `TestCHN71_NoAdminMutePath` (双反向 pattern 0 hit) |
| 1.6 mute 不 drop messages — CreateMessage 路径 byte-identical 不动 + RT-3 fan-out 不查 mute (反向 grep `IsMutedForUser` 在 internal/ws/ 0 hit) — mute 仅 DL-4 push notifier skip | grep + unit | 战马D / 烈马 | `TestCHN71_MuteDoesNotDropMessages` (mute 用户 channel 后 message INSERT 仍 OK + WS frame 仍 fanout) + 反向 grep `mute.*skip.*broadcast\|mute.*drop.*message` internal/ws+messages*.go 0 hit |

### §2 CHN-7.2 — client MuteButton + MutedChannelIndicator + 文案锁

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 MuteButton.tsx toggle DOM byte-identical 跟 content-lock §1 (button data-action="mute" / "unmute" + 文案 `静音` 2 字 ↔ `取消静音` 4 字 byte-identical) + click → muteChannel(id, true/false) lib/api 单源 | vitest (4 PASS) | 战马D / 野马 / 烈马 | `packages/client/src/__tests__/MuteButton.test.tsx` (未 mute 文案 / 已 mute 文案 toggle / click pin/unpin → API call) |
| 2.2 MutedChannelIndicator.tsx DOM byte-identical 跟 content-lock §2 (`<span>` + `data-testid="muted-channel-indicator"` + 文案 `已静音` 3 字 byte-identical) + 不静音状态不渲染 (return null) | vitest (2 PASS) | 战马D / 野马 / 烈马 | `MutedChannelIndicator.test.tsx` (rendered with muted=true 文案 `已静音` + muted=false return null) |
| 2.3 同义词反向 reject (`mute/silence/dnd/disturb/quiet/屏蔽/关闭通知/勿扰` 0 hit in MuteButton+MutedChannelIndicator+lib/mute.ts user-facing string) | vitest (1 PASS) | 战马D / 野马 / 烈马 | `_NoSynonyms` (反向 grep 8 同义词 0 hit) |
| 2.4 lib/mute.ts::MUTE_BIT=2 字面 byte-identical 跟 server MuteBit + isMuted 谓词单源; lib/api.ts::muteChannel 单源 | grep + vitest | 战马D / 烈马 | `_MuteBitByteIdentical` + `_APIClientSingleSource` |

### §3 CHN-7.3 — closure + AST 锁链延伸第 12 处

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 立场 ⑥ AST 锁链延伸第 12 处 — forbidden 3 token (`pendingChannelMute / channelMuteQueue / deadLetterChannelMute`) 在 internal/api+push 0 hit | AST scan | 飞马 / 烈马 | `TestCHN73_NoChannelMuteQueue` (filepath.Walk 反向 3 forbidden 0 hit) |

## 边界

- CHN-3.1 #410 user_channel_layout 列复用 / CHN-3.2 #412 既有 PUT /me/layout endpoint byte-identical 不动 / CHN-6 #544 PinThreshold 双向锁模式承袭 / DL-4 push gateway 立场承袭 / ADM-0 §1.3 红线 admin god-mode 不挂 / owner-only ACL 锁链 15 处一致 (AL-2a/BPP-3.2/AL-1/AL-5/DM-4/CV-4 v2/BPP-7/BPP-8/CHN-5/CHN-6/CHN-7) / audit 5 字段链第 12 处 / AST 锁链延伸第 12 处 / **0 schema 改** / MuteBit=2 双向锁 (server + client byte-identical) / mute 不 drop messages (best-effort skip push 通知)

## 退出条件

- §1 (6) + §2 (4) + §3 (1) 全绿 — 一票否决
- 0 schema 改 / 0 新 migration
- CHN-3.2/CHN-6 既有 unit 不破
- audit 5 字段链 CHN-7 = 第 12 处
- AST 锁链延伸第 12 处
- owner-only ACL 锁链 15 处一致
- MuteBit=2 双向锁 (server + client byte-identical)
- mute 不 drop messages (反向 grep 0 hit)
- 文案 byte-identical 跟 content-lock + 同义词反向
- 登记 REG-CHN7-001..006
