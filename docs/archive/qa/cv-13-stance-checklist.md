# CV-13 stance checklist — quote / reference 视觉

> 5 立场 byte-identical 跟 spec §0 (≤80 行).

## 1. 0 server production code (跟 CV-9..12 / DM-5 / AP-4 / AP-5 同模式)

- [x] 复用 messages.reply_to_id 列 (CV-8 thread #441 已建)
- [x] 复用既有 messages.go::handleCreateMessage ReplyToID 入参支持
- [x] 复用既有 fetchMessages list endpoint, 返字段含 reply_to_id
- [x] 反向断言: `git diff origin/main -- packages/server-go/` 0 production 行 (CV-13 PR 守门 grep)
- [x] 不开新 endpoint / 不改 schema / 不加 server validator

## 2. quote 块从内存 cache 渲染 (跟 CV-9 mention preview 同精神)

- [x] QuotedCommentBlock.tsx props: { quotedMessage: Message | null }
- [x] 父组件 ArtifactCommentItem (CV-7 #535 既有) 从 messages list lookup parent
- [x] 不 import api / fetchMessage / fetchMessages — 纯 props-driven 渲染
- [x] missing parent → fallback "(原消息已删除)" byte-identical
- [x] 反向 grep `fetch.*quoted|api.*quoted` 0 hit

## 3. thinking 5-pattern 反约束锁链第 9 处

- [x] RT-3 #488 第 1 + DM-3 #508 第 2 + DM-4 #549 第 3 + CV-7 #535 第 4 + CV-8 第 5 + CV-9 第 6 + CV-11 第 7 + CV-12 第 8 + CV-13 第 9
- [x] quote block 不暴露 agent reasoning, 反向 grep `processing|responding|thinking|analyzing|planning` 在 cv-13*.tsx production 0 hit
- [x] 反约束: 不渲染 quoted 消息的 thinking metadata (即使 server 返字段, UI 也只 render content)

## 4. 文案 byte-identical (改 = 改 content-lock SSOT 一处)

- [x] quote prefix `"> "` (markdown blockquote 风格)
- [x] author 前缀 `"@"`
- [x] collapse expanded `"收起"` / collapsed `"展开"`
- [x] missing fallback `"(原消息已删除)"`
- [x] truncate suffix `"…"` + length 200 chars
- [x] **content-lock SSOT**: `docs/qa/cv-13-content-lock.md` (本 PR 同步加)

## 5. DOM data-attr 锁

- [x] `data-cv13-quoted-block` (root container)
- [x] `data-cv13-quoted-author` (author span)
- [x] `data-cv13-quoted-id="<parent message id>"`
- [x] `data-cv13-collapsed="true"|"false"` (toggle 状态)
- [x] e2e + vitest 双锁 (e2e by data-attr selector + vitest by render output)

## 反约束

- ❌ quote chain (quote of a quote) — 仅一层
- ❌ markdown 富文本 quote (CV-11 既有 markdown 渲染只对 body, quote 视觉是纯文本截断)
- ❌ admin god-mode quote audit (ADM-0 §1.3 红线)
- ❌ schema 改 / 新 endpoint
- ❌ quote 跨 channel (parent 同 channel; 反向断 — quotedMessage.channel_id == message.channel_id)
- ❌ quote 编辑历史 (跟 CV-7 forward-only 同精神)

## 跨 milestone byte-identical 锁链

- CV-7 #535 ArtifactCommentItem (props 加 quotedMessage 续)
- CV-8 #441 reply_to_id 列 (quote SSOT 字段)
- CV-9 #539 mention preview 内存 cache 渲染同精神
- CV-11 #543 markdown 渲染 (CV-13 quote body 不走 markdown — 纯截断)
- CV-12 search (无 cross 影响)
- DM-5 #549 / AP-4 #551 / AP-5 #555 同 0-server / client-only 模式
- thinking 5-pattern 锁链第 9 处
- ADM-0 §1.3 红线 (admin god-mode 不挂)
