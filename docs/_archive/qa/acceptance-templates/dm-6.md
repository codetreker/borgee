# Acceptance Template — DM-6: DM thread reply UI

> 蓝图 dm-model.md §3 thread reply + CHN-1 既有 messages.reply_to_id 列承袭. Spec `dm-6-spec.md` (战马D v0). Stance + content-lock. **0 schema 改 / 0 server production code** — messages.reply_to_id 既有 + POST /channels/{id}/messages 既有接受 reply_to_id byte-identical 不动. Owner: 战马D 实施 / 飞马 review / 烈马 验收.

## 验收清单

### §1 DM-6.1 — server 0 production code 反向断言

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 0 schema 改反向断言 — migrations/ 0 新 dm_6_* 文件 + ALTER TABLE messages.*reply 0 hit + registry.go byte-identical 不动 | grep | 战马D / 飞马 / 烈马 | `internal/api/dm_6_thread_test.go::TestDM61_NoSchemaChange` 1 unit PASS |
| 1.2 0 server production code 反向断言 — internal/api/ 反向 grep `dm_6` 在 production *.go 0 hit (仅 _test.go) + git diff server-go 仅命中 _test.go + docs (反向断言 production 0 行变更) | grep | 战马D / 飞马 / 烈马 | `TestDM61_NoServerProductionCode` (filepath.Walk + 反向 grep dm_6 在非 _test.go 0 hit) |
| 1.3 messages.reply_to_id 列 existing 反向断言 — PRAGMA messages 列必含 reply_to_id (CHN-1 既有, 不依赖新 migration) | unit | 战马D / 烈马 | `TestDM61_ReplyToIDColumnExists` (PRAGMA 反向断言 reply_to_id 列存在) |
| 1.4 DM thread reply HappyPath — POST /api/v1/channels/{dmID}/messages with reply_to_id → 200 + persisted (走既有 path byte-identical, 反向断言新 server prod 代码 0 行) | unit | 战马D / 烈马 | `TestDM61_DMThreadReply_HappyPath` (POST DM message + reply_to_id → 200 + reply_to_id 持久化) |

### §2 DM-6.2 — client DMThread.tsx + 文案锁

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 DMThread.tsx 折叠 toggle DOM byte-identical 跟 content-lock §1 (button data-testid="dm6-thread-toggle" + 文案展开态 `▼ 隐藏 N 条回复` / 折叠态 `▶ 显示 N 条回复` byte-identical, N 是动态计数) | vitest (2 PASS) | 战马D / 野马 / 烈马 | `packages/client/src/__tests__/DMThread.test.tsx` (展开 vs 折叠态文案 byte-identical + N 计数动态) |
| 2.2 reply input placeholder `回复...` 2 字 byte-identical + textarea data-testid="dm6-reply-input" + submit button data-testid="dm6-reply-submit" | vitest (1 PASS) | 战马D / 野马 / 烈马 | `_ReplyInputDOM_ByteIdentical` |
| 2.3 同义词反向 reject (`reply/comment/discussion/讨论/评论/评论区` 0 hit user-visible text) | vitest (1 PASS) | 战马D / 野马 / 烈马 | `_NoSynonyms` |
| 2.4 空 thread (replies.length === 0) 不渲染 toggle (return null) | vitest (1 PASS) | 战马D / 野马 / 烈马 | `_EmptyThread_NoToggle` |

### §3 DM-6.3 — closure + AST 锁链延伸第 15 处 + thinking 5-pattern #9

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 立场 ⑥ AST 锁链延伸第 15 处 forbidden 3 token (`pendingDMThread / dmThreadQueue / deadLetterDMThread`) 在 internal/api 0 hit + 立场 ③ thinking 5-pattern 锁链第 9 处 (反向 grep `<thinking>\|<thought>\|<reasoning>\|<reflection>\|<internal>` 在 dm_6 production 0 hit, 跟 RT-3+DM-3+DM-4+DM-5 承袭) | AST scan + grep | 飞马 / 烈马 | `TestDM63_NoDMThreadQueue` (AST scan 0 hit) + `TestDM61_NoThinkingPatternInProduction` (反向 grep 5 字面 0 hit) 2 unit PASS |

## 边界

- CHN-1 既有 messages.reply_to_id 列复用 / DM-3 #508 既有 channel.member ACL byte-identical 不动 / DM-5 #549 + CV-7 #535 + CV-9..12 0-server 模式承袭 / RT-3 #488 + DM-3/4/5 thinking 5-pattern 反向断言承袭 / ADM-0 §1.3 红线 admin god-mode 不挂 / owner-only ACL 锁链 18 处一致 / audit 5 字段链第 15 处 / AST 锁链延伸第 15 处 / thinking 5-pattern 锁链第 9 处 / **0 schema 改 / 0 server production code** / DOM byte-identical (折叠 toggle + reply input + 同义词反向)

## 退出条件

- §1 (4) + §2 (4) + §3 (1) 全绿 — 一票否决
- 0 schema 改 / 0 server production code (git diff 仅命中 _test.go + client + docs)
- 既有 DM-3/CV-7/DM-5 unit 不破
- audit 5 字段链 DM-6 = 第 15 处
- AST 锁链延伸第 15 处
- thinking 5-pattern 锁链第 9 处
- owner-only ACL 锁链第 18 处一致
- 文案 byte-identical 跟 content-lock + 同义词反向
- 登记 REG-DM6-001..006
