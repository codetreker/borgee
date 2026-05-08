# CHN-13 acceptance — channel list search/filter

战马D · 2026-04-30 · spec `chn-13-spec.md` + stance.

## §1 schema / server

- §1.1 ✅ 0 schema 改 (复用 channels 既有表 + name 既有 index).
- §1.2 ✅ server 最小补丁 ≤15 行 production — handleListChannels 加 q 解析
  + ListChannelsWithUnread(userID, q) sig 加参数 + LIKE COLLATE NOCASE.
- §1.3 ✅ 空 q 走既有 SQL path byte-identical (q="" 跳过 LIKE 子句, 既有
  ORDER BY position ASC 不变).
- §1.4 ✅ q != "" 走 LIKE 子串 + COLLATE NOCASE; 既有 ordering 不变.
- §1.5 ✅ 既有 admin-rail /admin-api/v1/channels 不动.
- §1.6 ✅ 既有 callsites (welcome / fanout) 全填 q="" byte-identical.

## §2 client ChannelSearchInput + ChannelList 空态/count

- §2.1 ✅ ChannelSearchInput placeholder `搜索频道` 4 字 byte-identical.
- §2.2 ✅ 空态文案 `未找到匹配` 5 字 byte-identical.
- §2.3 ✅ count 文案 `共 N 个频道` (N 占位) byte-identical.
- §2.4 ✅ debounce 200ms — 拖键 onChange 不立即 fetch, 200ms 静默期后才
  触发 listChannels(q).
- §2.5 ✅ 同义词反向 reject (`find/lookup/locate/查找/检索/查询`).
- §2.6 ✅ 空 q (清空 input) 走既有 path byte-identical (反向 grep 锚).

## §3 反约束

- §3.1 ✅ 0 schema.
- §3.2 ✅ 0 新 endpoint (复用 GET /api/v1/channels).
- §3.3 ✅ 既有 ListChannelsWithUnread 空 q 行为 byte-identical.
- §3.4 ✅ admin god-mode 不挂 search (反向 grep admin-api/v1/.../q= 0 hit).
- §3.5 ✅ AL-1a reason 锁链不漂 (停在 HB-6 #19).
- §3.6 ✅ AST 锁链延伸第 21 处.

## §4 测试矩阵

- TestCHN131_NoSchemaChange ✅
- TestCHN132_ListChannelsWithQuery ✅ (q="" + q="match" 两 case)
- TestCHN132_QueryCaseInsensitive ✅
- TestCHN132_QuerySubstringMatch ✅
- TestCHN133_NoSearchQueue ✅
- TestCHN133_NoAdminSearchPath ✅
- ChannelSearchInput.test.tsx 5 vitest ✅
