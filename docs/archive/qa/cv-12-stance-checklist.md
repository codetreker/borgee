# CV-12 立场反查清单 (战马E v0)

> 战马E · 2026-04-29 · 跟 CV-9/10/11 client-only + CV-5 namespace 单源同模式承袭.
> **关联**: spec `cv-12-spec.md` (924aaf5) + acceptance + content-lock.

## §0 立场总表 (4 立场 + 3 边界)

| # | 立场 | 反约束 |
|---|---|---|
| ① | search 走既有 `GET /api/v1/channels/{channelId}/messages/search?q=` 单源, **0 server code** | 反向 grep `/comments/search\|comment_fts\|artifact_search.*PRIMARY\|cv12.*fts\|cv12.*new.*search` 在 internal/ count==0 |
| ② | owner-only ACL byte-identical 14+ 处, admin god-mode 不挂 | 反向 grep `admin.*comment.*search\|admin.*search.*comment` 在 admin*.go count==0 |
| ③ | thinking 5-pattern 锁链不漂 (search 是 read-side, 不解 thinking; 5-pattern 仍 server CV-7/CV-8 既有 hook 第 8 处不变) | 反向 grep `cv12.*thinking\|cv12.*subject` 在 client/src 0 hit |
| ④ | DOM `data-cv12-search-input` + `data-cv12-search-result-id` 锚 + 文案 "未找到匹配评论" byte-identical; 空查询不打 server | 反向 grep `data-cv12-*` ≥2 hit; "未找到匹配评论" ≥1 hit |

## §1 立场 ① 0 server code

- [ ] CV-12 git diff packages/server-go/ excl _test.go = 0 行
- [ ] 1 unit 反向断 既有 message-search endpoint 在 artifact: namespace channel 工作 (跟 text channel 等价)
- [ ] 反向 grep 5 锚 0 hit

## §2 立场 ② owner-only / admin 不挂

- [ ] readPerm + private channel access check 既有不动
- [ ] 反向 grep `admin.*comment.*search` 0 hit

## §3 立场 ③ thinking 5-pattern 不漂

- [ ] search 路径不调 thinking validator (反向断)
- [ ] CV-7/CV-8 既有 write-side hook 不动; 5-pattern 锁链 8 处不变

## §4 立场 ④ DOM + 文案 + 空查询保护

- [ ] `data-cv12-search-input="<artifactId>"` 锚 (反向 grep ≥1)
- [ ] `data-cv12-search-result-id="<msgId>"` 锚 (反向 grep ≥1)
- [ ] 文案 "未找到匹配评论" byte-identical (反向 grep ≥1, 仅 0 result 时显)
- [ ] 空 query 不调 API (vitest 反向断)

## §5 边界 ⑤⑥⑦ — fail-closed / forward-only / 不裂表

- [ ] cross-channel reject 跟 CV-5..CV-11 同源 (search ACL 既有)
- [ ] forward-only — search 纯 read
- [ ] 不裂表 — 0 schema 改

## §6 退出条件

- §1+§2+§3+§4 全 ✅
- 反向 grep 5 锚: 4 处 0 hit + DOM/文案 ≥1 hit
- vitest 4 + e2e 3 + 1 server unit 全 PASS
- 0 schema 改 + 0 server production code
