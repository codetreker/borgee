# Acceptance Template — CHN-9: channel privacy 三态

> 蓝图 channel-model.md §2 不变量 + §1.4 红线. Spec `chn-9-spec.md` (战马D v0). Stance + content-lock. **0 schema 改** — channels.visibility TEXT 列复用 CHN-1.1 #267 既有, 加第 3 enum `creator_only` 跟 `private`/`public` 共三态. Owner: 战马D 实施 / 飞马 review / 烈马 验收.

## 验收清单

### §1 CHN-9.1 — server visibility 三态 + ACL + leak 反断

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 0 schema 改反向断言 — migrations/ 0 新文件 + ALTER TABLE channels 0 hit + registry.go byte-identical 不动 + 现有 'public'/'private' 行 byte-identical 保留 | grep | 战马D / 飞马 / 烈马 | `internal/api/chn_9_visibility_test.go::TestCHN91_NoSchemaChange` 1 unit PASS |
| 1.2 visibility 三态 const + IsValidVisibility 谓词 — server VisibilityCreatorOnly='creator_only' / VisibilityMembers='private' (alias) / VisibilityOrgPublic='public' (alias) byte-identical 跟 client VISIBILITY_* + DB 字面 (三向锁) | unit + grep | 战马D / 飞马 / 烈马 | `_VisibilityConsts_ByteIdentical` (consts == 'creator_only'/'private'/'public' + IsValidVisibility 三 case true + 反向 reject 4 case false) |
| 1.3 PATCH /api/v1/channels/{channelId} body.visibility=`creator_only` owner-only — 200 + visibility 持久化; non-owner 403 (channel.manage_visibility 既有 ACL 同源); existing public/private PATCH byte-identical 不破 (反向断言) | unit (3 sub-case) | 战马D / 烈马 | `_PatchVisibility_CreatorOnly_HappyPath` + `_PatchVisibility_NonOwnerRejected` + `_PatchVisibility_BackcompatPublicPrivate` |
| 1.4 PATCH 反向 reject 外值 → 400 byte-identical 报错文案 `Visibility must be 'creator_only', 'private', or 'public'`; spec 外值 4 case (`secret/team/Public/empty`) 全 reject | unit (1 sub-case 4 sub) | 战马D / 烈马 | `_PatchVisibility_RejectsInvalidValue` (4 case 全 400 + 文案 byte-identical) |
| 1.5 creator_only channel **不 leak** — 反向断言非 creator user 调 GET /api/v1/channels 列表不见 creator_only channel; ListChannelsWithUnread 既有 `visibility = 'public'` filter byte-identical 不动 (creator_only 不入 org-public preview path) | unit (2 sub-case) | 战马D / 烈马 | `_CreatorOnlyChannel_NotLeakedToOrgPeers` (创建 creator_only channel + 别 user GET /channels 列表 0 行匹配) + `_ListChannelsFilter_ByteIdentical` (反向 grep `visibility = 'public'` 在 queries.go byte-identical 不动) |
| 1.6 admin god-mode 不挂 PATCH visibility 反向断言 — `admin.*visibility\|admin.*channel.*visibility` 在 admin*.go 0 hit | grep | 战马D / 飞马 / 烈马 | `TestCHN91_NoAdminVisibilityPath` (双反向 grep 0 hit) |

### §2 CHN-9.2 — client VisibilityBadge + 文案锁

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 VisibilityBadge.tsx 三态 DOM byte-identical 跟 content-lock §1 (visibility=creator_only → `🔒 仅创建者` / =private → `👥 成员可见` / =public → `🌐 组织内可见` + data-visibility 三态) | vitest (3 PASS) | 战马D / 野马 / 烈马 | `packages/client/src/__tests__/VisibilityBadge.test.tsx` (creator_only / private / public 三态文案 byte-identical) |
| 2.2 VISIBILITY_* consts byte-identical 跟 server + isValidVisibility 谓词单源 | vitest (1 PASS) | 战马D / 飞马 / 烈马 | `_VisibilityConsts_ByteIdentical` (跟 server 三向锁) |
| 2.3 同义词反向 reject (`secret/exclusive/team-only/外部/外公/绝密` 0 hit user-visible text) | vitest (1 PASS) | 战马D / 野马 / 烈马 | `_NoSynonyms` |
| 2.4 lib/api.ts::setChannelVisibility 单源 + 调用 既有 updateChannel path byte-identical (反向 inline fetch 0 hit) | grep + vitest | 战马D / 烈马 | `_APIClientSingleSource` |

### §3 CHN-9.3 — closure + AST 锁链延伸第 14 处

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 立场 ⑥ AST 锁链延伸第 14 处 forbidden 3 token (`pendingVisibility / visibilityChangeQueue / deadLetterVisibility`) 在 internal/api 0 hit | AST scan | 飞马 / 烈马 | `TestCHN93_NoVisibilityQueue` (AST scan 0 hit) |

## 边界

- CHN-1.1 #267 channels.visibility 列复用 / CHN-1.2 既有 PATCH endpoint + channel.manage_visibility ACL byte-identical 不动 / ListChannelsWithUnread 既有 SQL filter byte-identical 不动 / ADM-0 §1.3 红线 / owner-only ACL 锁链 17 处一致 (AL-2a/BPP-3.2/AL-1/AL-5/DM-4/CV-4 v2/BPP-7/BPP-8/CHN-5/CHN-6/CHN-7/CHN-8/CHN-9) / audit 5 字段链第 14 处 / AST 锁链延伸第 14 处 / **0 schema 改** / Visibility 三向锁 (server + client + DB byte-identical) / creator_only 不 leak (反向 unit 守门)

## 退出条件

- §1 (6) + §2 (4) + §3 (1) 全绿 — 一票否决
- 0 schema 改 / 0 新 migration
- CHN-1.2 既有 unit 不破 (existing public/private PATCH byte-identical)
- audit 5 字段链 CHN-9 = 第 14 处
- AST 锁链延伸第 14 处
- owner-only ACL 锁链 17 处一致
- Visibility 三向锁 (server + client + DB byte-identical)
- creator_only 不 leak (反向 unit + filter byte-identical)
- 文案 byte-identical 跟 content-lock + 同义词反向
- 登记 REG-CHN9-001..006
