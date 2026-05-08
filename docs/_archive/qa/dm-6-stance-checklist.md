# DM-6 立场反查清单 (战马D v0)

> 战马D · 2026-04-30 · 立场 review checklist (跟 DM-5/CV-7/CHN-9 stance 同模式)
> **目的**: DM-6 三段实施 (6.1 server 0 prod / 6.2 client / 6.3 closure) PR review 时, 飞马/野马/烈马按此清单逐立场 sign-off.
> **关联**: spec `dm-6-spec.md` + acceptance `acceptance-templates/dm-6.md` + content-lock `dm-6-content-lock.md`
> **content-lock 必锁** — DMThread 折叠 toggle DOM + 文案 + 同义词反向.

## §0 立场总表 (3 立场 + 3 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | **0 schema 改 / 0 server production code** — messages.reply_to_id 既有列 + POST /channels/{id}/messages 既有接受 reply_to_id byte-identical 不动 | DM-5 #549 / CV-7 #535 / CV-9..12 等 0-server 模式承袭 + messages 表 既有 reply_to_id 列 (CHN-1 既有) | 反向 grep `dm_6` 在 production *.go 0 hit (仅 _test.go); 反向 grep `migrations/dm_6_\d+\|ALTER TABLE messages.*reply\|reply.*new.*endpoint` 0 hit |
| ② | owner-only ACL 锁链第 18 处 (CHN-9 #17 承袭) — DM thread reply 走既有 message ACL (channel.member 必传; admin god-mode 不挂 PATCH/POST DM thread) | admin-model.md ADM-0 §1.3 红线 + DM-3 #508 既有 message ACL 承袭 | 反向 grep `admin.*dm.*thread\|admin.*reply` 在 admin*.go 0 hit |
| ③ | thinking 5-pattern 锁链第 9 处 (DM-5 第 8 处承袭) — DM thread reply 不豁免 thinking 反向断言 | RT-3 #488 + DM-3 + DM-4 + DM-5 thinking 反向断言承袭 | 反向 grep `<thinking>\|<thought>\|<reasoning>\|<reflection>\|<internal>` 在 dm_6 production 0 hit |

边界:
- **④** thread depth 1 层强制 — 反向断言走既有 server validation; 现网行为零变 (反向 grep server prod 代码 0 行变更; 走 既有 messages.go path).
- **⑤** 文案 byte-identical 跟 content-lock §1 — DMThread 折叠 toggle `▼ 隐藏 N 条回复` (展开) / `▶ 显示 N 条回复` (折叠) + reply input placeholder `回复...` 2 字 + data-testid `dm6-thread-toggle` + 同义词反向 reject (`reply/comment/discussion/讨论/评论/评论区`).
- **⑥** AST 锁链延伸第 15 处 forbidden 3 token (`pendingDMThread / dmThreadQueue / deadLetterDMThread`) 在 internal/api 0 hit.

## §1 立场 ① 0 server prod (DM-6.1 守)

- [ ] migrations/ 0 新文件 (反向 grep `migrations/dm_6_` 0 hit)
- [ ] internal/api/ 0 新 production *.go (仅 _test.go) — 反向 grep `dm_6` 在 production 0 hit
- [ ] internal/server/server.go 字面 byte-identical 跟 main 不动
- [ ] messages 表 schema byte-identical (PRAGMA reply_to_id 列 existing)
- [ ] CreateMessageFull 既有路径 byte-identical 不动 (反向 grep server prod git diff 仅命中 _test.go + client + docs)

## §2 立场 ② owner-only + admin god-mode 不挂 (DM-6.1 守)

- [ ] DM thread reply 走既有 channel.member ACL (反向断言: 跟 DM-3 既有 unit 同精神)
- [ ] admin god-mode 不挂 PATCH/POST DM thread (反向 grep 0 hit)
- [ ] cross-org DM thread 不 leak (cross-org user 不可调 POST messages reply_to_id; 走既有 ACL fail-closed)
- [ ] owner-only ACL 锁链第 18 处一致 (CHN-9 #17 承袭)

## §3 立场 ③ thinking 5-pattern 锁链第 9 处 (DM-6.1 守)

- [ ] dm_6_*.go 反向 grep `<thinking>\|<thought>\|<reasoning>\|<reflection>\|<internal>` 在 production 0 hit (仅 _test.go reverse assertion)
- [ ] 跟 DM-5 #549 第 8 处 + DM-4 #553 第 7 处 + DM-3 #508 第 6 处 + RT-3 #488 第 5 处承袭

## §4 蓝图边界 ④⑤⑥ — 不漂

- [ ] thread depth 1 层 (反向断言 — 既有 server validation 不漂)
- [ ] DMThread.tsx toggle DOM byte-identical (`▼ 隐藏 N 条回复` / `▶ 显示 N 条回复`)
- [ ] reply input placeholder `回复...` 2 字 byte-identical
- [ ] data-testid="dm6-thread-toggle" byte-identical
- [ ] 同义词反向 reject 6 case 0 hit (`reply/comment/discussion/讨论/评论/评论区`)
- [ ] AST 锁链延伸第 15 处 forbidden 3 token 0 hit

## §5 退出条件

- §1 (5) + §2 (4) + §3 (2) + §4 (6) 全 ✅
- 反向 grep 5 项全 0 hit (server prod / schema / admin / 同义词 / DM thread queue)
- thinking 5-pattern 锁链第 9 处不漂
- audit 5 字段链 DM-6 = 第 15 处
- AST 锁链延伸第 15 处
- owner-only ACL 锁链第 18 处一致
- 文案 byte-identical 跟 content-lock + 同义词反向
- 登记 REG-DM6-001..006
