# Acceptance Template — CHN-5: channel archived UI + 列表

> 蓝图 channel-model.md §2 不变量 #3 archive 留 history. Spec `chn-5-spec.md` (战马D v0). Stance `chn-5-stance-checklist.md`. Content-lock `chn-5-content-lock.md`. **0 schema 改** — channels.archived_at 列由 CHN-1.1 #267 已落. Owner: 战马D 实施 / 飞马 review / 烈马 验收.

## 验收清单

### §1 CHN-5.1 — schema 0 行 + CHN-5.2 — server endpoints + system DM

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 0 schema 改 反向断言 — migrations/ 0 新文件 + registry.go 字面 byte-identical 不动 + ListChannelsWithUnread 既有 archived_at IS NULL filter byte-identical 不动 | grep + Idempotent | 战马D / 飞马 / 烈马 | `internal/api/chn_5_archived_test.go::TestCHN51_NoSchemaChange` (filepath.Walk migrations/ 反向 chn_5 0 hit) |
| 1.2 GET /api/v1/me/archived-channels owner-only — 只见 user 自己 member 的 archived 频道; cross-org 用户 0 行 (跟 ListChannelsWithUnread 同精神) + admin god-mode 调 user-rail 401 (admin cookie 走 user middleware 401) | unit (3 sub-case) | 战马D / 烈马 | `TestCHN52_ListMyArchived_HappyPath` (3 archived owner + 1 cross-org foreign 0 漏) + `_RejectsAdminRailUserPath` (admin cookie 401) + `_EmptyListWhenNoArchived` |
| 1.3 GET /admin-api/v1/channels/archived admin-rail readonly — admin cookie 必经; 反向 grep PATCH/PUT/DELETE 在 admin handler 0 hit; user cookie → 401 | unit (3 sub-case) + grep | 战马D / 烈马 | `TestCHN52_AdminListArchived_HappyPath` + `_RejectsUserRail` (user-rail 401) + `TestCHN52_NoAdminPatchPath` (反向 grep 0 hit) |
| 1.4 unarchive system DM fanout — fanoutUnarchiveSystemMessage 跟 fanoutArchiveSystemMessage 同模式 + 文案 byte-identical 跟 content-lock §1 (`channel #{name} 已被 {actor} 恢复`) + WS frame `channel_unarchived` 互补 `channel_archived` | unit + 文案锁 | 战马D / 烈马 | `TestCHN52_UnarchiveFanoutsSystemMessage` (PATCH archived:false → 每 member 1 system DM body byte-identical) + `_UnarchiveBroadcastsWSFrame` (channel_unarchived frame fired) |

### §2 CHN-5.3 — client ArchivedChannelsPanel + 文案锁

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 ArchivedChannelsPanel 折叠区 + 恢复 button DOM byte-identical 跟 content-lock §2 (`<details>` + `<summary>已归档频道</summary>` + `data-testid="archived-channels-panel"` + `data-archived="true"` + button data-action="restore" + 文案 `恢复` 1 字) | vitest (4 PASS) | 战马D / 野马 / 烈马 | `packages/client/src/__tests__/ArchivedChannelsPanel.test.tsx` 4 sub-case (空态 / 列表渲染 / 恢复 button onClick / 折叠 toggle 状态) |
| 2.2 文案 byte-identical 跟 content-lock §3 (`频道已恢复` toast / `恢复失败` toast 互补 archive 既有) + 同义词反向 reject (`存档/封存/还原/解档/restore/archive` 0 hit) | vitest (2 PASS) | 战马D / 野马 / 烈马 | `_ToastsByteIdentical` + `_NoSynonyms` (反向 grep 同义词 0 hit 跟 CHN-1.3 #288 同模式) |
| 2.3 lib/api.ts::listArchivedChannels() 单源 — 调用方走此函数, 反向 grep `fetch.*archived-channels` 在 components/ 直接调 0 hit (走 api.ts 中转) | grep | 战马D / 烈马 | `_APIClientSingleSource` (反向 grep components/ 0 hit) |

### §3 CHN-5.4 — closure + AST 锁链延伸第 10 处

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 立场 ⑥ AST 锁链延伸第 10 处 — forbidden 3 token (`pendingChannelArchive / channelArchiveQueue / deadLetterChannelArchive`) 在 internal/api production *.go 0 hit (跟 BPP-4/5/6/7/8/HB-3 v2/AL-7/AL-8/HB-5 同模式) | AST scan | 飞马 / 烈马 | `TestCHN53_NoChannelArchiveQueue` (AST ident scan 3 forbidden 0 hit) |
| 3.2 admin god-mode PATCH 不挂 反向断言 — `admin.*archive_channel\|admin.*unarchive` 在 admin*.go 0 hit + `mux\.Handle\("(PATCH\|PUT\|DELETE).*admin-api/v1/channels/archived` 0 hit | grep | 战马D / 飞马 / 烈马 | `TestCHN52_NoAdminPatchPath` (反向 grep 双 pattern 0 hit) |

## 边界

- CHN-1.1 #267 channels.archived_at 列复用 / CHN-1.2 #265 archive system DM 立场承袭 / CHN-1.3 #288 SortableChannelItem `已归档` badge byte-identical / ADM-0 §1.3 红线 admin god-mode 不挂 PATCH / CM-3 #208 cross-org 过滤承袭 / owner-only ACL 锁链 13 处一致 (AL-2a/BPP-3.2/AL-1/AL-5/DM-4/CV-4 v2/BPP-7/BPP-8/CHN-5) / audit 5 字段链第 10 处 (ADM-2.1+AP-2+BPP-4+BPP-7+BPP-8+HB-3 v2+AL-7+AL-8+HB-5+CHN-5) / AST 锁链延伸第 10 处 / **0 schema 改** / **0 新 enum** / system DM 文案互补二式 byte-identical

## 退出条件

- §1 (4) + §2 (3) + §3 (2) 全绿 — 一票否决
- 0 schema 改 / 0 新 migration / 0 新 admin_actions enum
- ListChannelsWithUnread 既有 unit 不破 (TestADM22 系列 + 各 channel 现有 unit 全 PASS)
- audit 5 字段链 CHN-5 = 第 10 处
- AST 锁链延伸第 10 处
- owner-only ACL 锁链 13 处一致
- 文案 byte-identical 跟 content-lock + CHN-1.3 同源
- 登记 REG-CHN5-001..006
