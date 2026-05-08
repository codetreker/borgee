# CV-13 spec brief — artifact comment quote / reference (≤80 行)

> 战马E · Phase 5+ · ≤80 行 · 蓝图 [`canvas-collab.md`](../../blueprint/canvas-collab.md) §2 quote 视觉. CV-13 给 artifact comment 加 quote 视觉 — comment body 携 `reply_to_id` 时, UI 渲染引用块 (quoted body inline + 作者 + collapse). **0 server code** (复用 messages.reply_to_id 既有列 + 既有 fetchMessages list 内存 cache) — 跟 CV-9..12 / DM-5 / AP-4 / AP-5 同模式 client-only 加 UI.

## 0. 关键约束 (5 项立场)

1. **0 server production code** (跟 CV-9..12 / DM-5 同模式): reply_to_id 列 CV-8 thread #441 已建; messages.go::handleCreateMessage 既有路 已支持 ReplyToID; 既有 fetchMessages 返回 message.reply_to_id 字段. **反约束**: server-go internal/ git diff 0 行 production code; CV-13 不开新 endpoint, 不改 schema, 不加 server validator. 反向 grep `quote_event\|comment_quote_table\|cv13.*server` count==0.

2. **quote 块从内存 cache 渲染** (跟 CV-9 mention preview 同精神): client 已有 `messages: Message[]` list (MessageList.tsx); QuotedCommentBlock 组件 props 接收 `quotedMessage` (parent ref by id, 父组件 lookup) 不另起 fetch. **反约束**: QuotedCommentBlock.tsx 不 import fetchMessage/api; 反向 grep `fetch.*quoted\|api.*quoted\|getMessage\(.*quote` 在 components/ 0 hit.

3. **thinking 5-pattern 锁链第 9 处** (RT-3 #1 + DM-3 #2 + DM-4 #3 + CV-7 #4 + CV-8 #5 + CV-9 #6 + CV-11 #7 + CV-12 #8 + CV-13 #9): quote block 不暴露 reasoning, 反向 grep `processing\|responding\|thinking\|analyzing\|planning` 在 cv-13*.tsx production count==0.

4. **文案 byte-identical**: quote block prefix `> ` (markdown blockquote 风格 byte-identical) + author 前缀 `@{name}` + collapse 文案 `展开` / `收起`; missing parent fallback `(原消息已删除)` byte-identical. 改 = 改 content-lock SSOT 一处.

5. **DOM data-attr 锁**: `data-cv13-quoted-block` (root) + `data-cv13-quoted-author` (author span) + `data-cv13-quoted-id` (parent message id) + `data-cv13-collapsed` ("true"/"false") byte-identical. e2e + vitest 双锁.

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| CV-13.1 server | (无) — 反向断言 0 行 server diff (反向 grep test 守门) | 0 production code; reply_to_id 列既有, fetchMessages 已返此字段 |
| CV-13.2 client | `packages/client/src/components/QuotedCommentBlock.tsx` (新, ≤80 行) + `ArtifactCommentItem.tsx` 加 props `quotedMessage?` 字段 + render QuotedCommentBlock when present (≤10 行 wire) | quoted body 截 200 char + author + collapse toggle + missing fallback |
| CV-13.3 closure | REG-CV13-001..005 + acceptance + content-lock + PROGRESS [x] + thinking 5-pattern 反向 grep | e2e ap-13 e2e 占位 (deferred — 复用 CV-7 e2e harness, 留 follow-up) |

## 2. 文案 / DOM 锁 (content-lock SSOT)

```
quote prefix:        "> "
author prefix:        "@"
collapse expanded:    "收起"
collapse collapsed:   "展开"
missing fallback:     "(原消息已删除)"
truncate suffix:      "…"
truncate length:      200 chars
data-cv13-quoted-block (root)
data-cv13-quoted-author (author span)
data-cv13-quoted-id (parent message id)
data-cv13-collapsed ("true"|"false")
```

**0 新错码** — quote 视觉无 server-side error path.

## 3. 反向 grep 锚 (CV-13 实施 PR 必跑)

```
git grep -nE 'quote_event|comment_quote_table|cv13.*server' packages/server-go/internal/  # 0 hit
git diff origin/main -- packages/server-go/ | grep -c '^\+'  # 0 production lines
git grep -nE 'processing|responding|thinking|analyzing|planning' packages/client/src/components/QuotedCommentBlock.tsx  # 0 hit (锁链第 9 处)
git grep -nE 'fetch.*quoted|api.*quoted' packages/client/src/components/QuotedCommentBlock.tsx  # 0 hit
git grep -c 'data-cv13-quoted-block' packages/client/src/components/QuotedCommentBlock.tsx  # ≥1
```

## 4. 不在本轮范围 (deferred)

- ❌ quote chain (quote of a quote) — 仅渲染一层, 不递归
- ❌ markdown 富文本 quote (留 CV-11 既有 markdown 渲染)
- ❌ admin god-mode quote audit (ADM-0 §1.3 红线)
- ❌ schema 改 / 新 endpoint (反约束 0 server)
- ❌ quote 跨 channel (反向锁: 仅 reply_to_id 同 channel parent)
