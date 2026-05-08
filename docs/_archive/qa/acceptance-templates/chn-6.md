# Acceptance Template — CHN-6: channel pin/unpin 顶置

> 蓝图 channel-model.md §3 layout per-user. Spec `chn-6-spec.md` (战马D v0). Stance `chn-6-stance-checklist.md`. Content-lock `chn-6-content-lock.md`. **0 schema 改** — user_channel_layout 列由 CHN-3.1 #410 已落. Owner: 战马D 实施 / 飞马 review / 烈马 验收.

## 验收清单

### §1 CHN-6.1 — server REST endpoints + 双源 const

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 0 schema 改 反向断言 — migrations/ 0 新文件 + registry.go byte-identical 不动 + user_channel_layout 6 列复用 CHN-3.1 #410 既有 | grep | 战马D / 飞马 / 烈马 | `internal/api/chn_6_pin_test.go::TestCHN61_NoSchemaChange` (filepath.Walk migrations/ 反向 chn_6 0 hit) |
| 1.2 POST /api/v1/channels/{channelId}/pin owner-only 二态 — 200 + position = -(nowMs) (ASC 排序最顶); non-member 403; DM 400 `layout.dm_not_grouped` byte-identical 跟 CHN-3.2 同源; Unauthorized 401 | unit (4 sub-case) | 战马D / 烈马 | `TestCHN61_PinChannel_HappyPath` (position < 0) + `_PinChannel_NonMemberRejected` + `_PinChannel_DMRejected` + `_PinChannel_Unauthorized` |
| 1.3 DELETE /api/v1/channels/{channelId}/pin un-pin — 200 + position > 0 (跟 max-positive+1.0 单调小数模式承袭); idempotent (二次 DELETE 200) | unit (3 sub-case) | 战马D / 烈马 | `_UnpinChannel_HappyPath` (position > 0) + `_UnpinChannel_Idempotent` + `_UnpinChannel_RoundTrip` (pin → unpin → pin → 三次 position 不漂) |
| 1.4 PinThreshold=0 字面单源 + IsPinned(position) 谓词单源 — server const byte-identical 跟 client `POSITION_PIN_THRESHOLD=0` 双向锁 | unit + grep | 战马D / 飞马 / 烈马 | `_PinThreshold_ByteIdentical` (PinThreshold==0 + IsPinned(-1)==true + IsPinned(0)==false + IsPinned(1.0)==false) + 双向锁: vitest 跟 go test 双向 reflect const |
| 1.5 admin god-mode 不挂 反向断言 — `admin.*pin_channel\|admin.*pin\b\|/admin-api/.*pin` 在 admin*.go + chn_6_*.go 0 hit | grep | 战马D / 飞马 / 烈马 | `TestCHN61_NoAdminPinPath` (双反向 pattern 0 hit + adminMw 不挂任何 pin endpoint) |

### §2 CHN-6.2 — client PinButton + PinnedChannelsSection + 文案锁

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 PinButton.tsx toggle button DOM byte-identical 跟 content-lock §1 (button data-action="pin" / "unpin" + 文案 `置顶` 1 字 ↔ `取消置顶` 3 字 byte-identical) + click → pinChannel(id, true/false) lib/api 单源 | vitest (3 PASS) | 战马D / 野马 / 烈马 | `packages/client/src/__tests__/PinButton.test.tsx` (initial 文案 / pinned 状态 toggle 文案 / click → API call) |
| 2.2 PinnedChannelsSection.tsx 顶部 section DOM byte-identical 跟 content-lock §2 (`<section>` + `<header>已置顶频道</header>` + `data-testid="pinned-channels-section"` + filter `channel.position < POSITION_PIN_THRESHOLD` byte-identical) + empty state 不渲染 (无 pin → null) | vitest (2 PASS) | 战马D / 野马 / 烈马 | `PinnedChannelsSection.test.tsx` (列表渲染只含 position<0 的 channels + empty state null 不渲染) |
| 2.3 文案 byte-identical 跟 content-lock §3 + 同义词反向 reject (`收藏/标星/star/favorite/top` 0 hit in PinButton+PinnedSection) | vitest (1 PASS) | 战马D / 野马 / 烈马 | `_NoSynonyms` (反向 grep 5 同义词 0 hit) |
| 2.4 lib/api.ts::pinChannel(id, pinned) 单源 + POSITION_PIN_THRESHOLD=0 字面 byte-identical 跟 server PinThreshold | grep + vitest | 战马D / 烈马 | `_APIClientSingleSource` + `_PinThresholdByteIdentical` |

### §3 CHN-6.3 — closure + AST 锁链延伸第 11 处

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 立场 ⑥ AST 锁链延伸第 11 处 — forbidden 3 token (`pendingChannelPin / channelPinQueue / deadLetterChannelPin`) 在 internal/api production *.go 0 hit | AST scan | 飞马 / 烈马 | `TestCHN63_NoChannelPinQueue` (filepath.Walk 反向 3 forbidden 0 hit) |

## 边界

- CHN-3.1 #410 user_channel_layout 列复用 / CHN-3.2 #412 既有 layout endpoint byte-identical 不动 / CHN-3.3 #415 client 拖拽 MIN-1.0 单调小数模式承袭 / ADM-0 §1.3 红线 admin god-mode 不挂 PATCH/POST / owner-only ACL 锁链 14 处一致 (AL-2a/BPP-3.2/AL-1/AL-5/DM-4/CV-4 v2/BPP-7/BPP-8/CHN-5/CHN-6) / audit 5 字段链第 11 处 (ADM-2.1+AP-2+BPP-4+BPP-7+BPP-8+HB-3 v2+AL-7+AL-8+HB-5+CHN-5+CHN-6) / AST 锁链延伸第 11 处 / **0 schema 改** / PinThreshold 双向锁 (server + client byte-identical)

## 退出条件

- §1 (5) + §2 (4) + §3 (1) 全绿 — 一票否决
- 0 schema 改 / 0 新 migration
- CHN-3.2 既有 unit (TestCHN32_*) 不破
- audit 5 字段链 CHN-6 = 第 11 处
- AST 锁链延伸第 11 处
- owner-only ACL 锁链 14 处一致
- PinThreshold=0 双向锁 (server + client byte-identical)
- 文案 byte-identical 跟 content-lock + 同义词反向
- 登记 REG-CHN6-001..006
