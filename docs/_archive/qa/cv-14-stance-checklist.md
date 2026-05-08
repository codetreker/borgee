# CV-14 stance checklist — comment unread badge

> 5 立场 byte-identical 跟 spec §0 (≤80 行).

## 1. 0 server production code (跟 CV-9..13 / DM-5..6 / AP-4 / AP-5 同模式)

- [x] 复用 `ws.ArtifactCommentAddedFrame` (CV-5 #530 既有)
- [x] 复用 frame.mentions 字段 (CV-9 #539 既有, 用于 CV-14 mention double-count 反约束)
- [x] 反向断言: `git diff origin/main -- packages/server-go/` 0 production 行
- [x] 不开新 endpoint / 不改 schema / 不加 server validator

## 2. CV-14 跟 CV-9 mention badge 共存 (避免 mid-frame state coupling)

- [x] CommentUnreadBadge filter: `frame.sender_id !== currentUserId` (不计自己发的)
- [x] mention 走 CV-9 `ArtifactCommentsMentionBadge` 既有 (`mention_target_id == currentUserId`)
- [x] mention comment 同时计入 mention badge + comment unread badge — 这是预期行为 (mention=更强 signal; unread=总览)
- [x] 不 import CV-9 hook 做 state 同步 — 双 badge 视觉独立, 视觉非 race
- [x] 反向 vitest: 自己发的 frame 不增 CV-14 计数
- [x] 两 badge 同时挂在 thread 视图无冲突 (DOM data-attr 不重叠, CV-9 = `data-cv9-*`, CV-14 = `data-cv14-*`)

## 3. thinking 5-pattern 反约束锁链第 10 处

- [x] RT-3 #488 第 1 + DM-3 #508 第 2 + DM-4 #549 第 3 + CV-7 #535 第 4 + CV-8 第 5 + CV-9 #539 第 6 + CV-11 #543 第 7 + CV-12 第 8 + CV-13 #557 第 9 + CV-14 第 10
- [x] badge 不暴露 reasoning, 反向 grep `processing|responding|thinking|analyzing|planning` 在 CommentUnreadBadge.tsx 0 hit

## 4. 文案 byte-identical (改 = 改 content-lock SSOT 一处)

- [x] badge label `"${N} 条新评论"` (中文 byte-identical)
- [x] overflow display `"99+"` (count > 99)
- [x] zero state: 不渲染 (`null` 返回, 反向锁)
- [x] **content-lock SSOT**: `docs/qa/cv-14-content-lock.md`

## 5. DOM data-attr 锁 + 反向 storage

- [x] `data-cv14-comment-unread-badge` (root container, presence)
- [x] `data-cv14-unread-count="<N>"` (整数值)
- [x] e2e + vitest 双锁
- [x] 反向 grep `sessionStorage|localStorage` 在 CommentUnreadBadge.tsx 0 hit (纯 component state, 跟 CV-9 ArtifactCommentsMentionBadge 同精神)

## 反约束

- ❌ persistent unread (跨 reload) — 留 v2 复用 channel.unread_count
- ❌ admin god-mode unread visibility (ADM-0 §1.3 红线 永久)
- ❌ desktop notification (DL-4 Web Push gateway 路径)
- ❌ schema 改 / 新 endpoint
- ❌ unread per-comment highlight / 历史 (留 v3)
- ❌ admin console 挂 badge (admin god-mode 红线)

## 跨 milestone byte-identical 锁链

- CV-5 #530 ArtifactCommentAddedFrame WS hook (CV-14 SSOT 入口)
- CV-9 #539 ArtifactCommentsMentionBadge 双 badge 互斥同精神
- CV-13 #557 props-driven 0-fetch component 同模式
- thinking 5-pattern 锁链第 10 处
- ADM-0 §1.3 红线 (admin god-mode 不挂)
- Sidebar.tsx unread-badge `99+` overflow 同精神 (既有视觉规范)
