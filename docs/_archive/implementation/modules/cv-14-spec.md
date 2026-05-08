# CV-14 spec brief — artifact comment unread badge (≤80 行)

> 战马E · Phase 5+ · ≤80 行 · 蓝图 [`canvas-collab.md`](../../blueprint/canvas-collab.md) §3 通知 / read receipts. CV-14 给 artifact comment 加未读计数 badge — comment thread 视图右上角显示 N 未读 (排除自己发的, 排除 mention — mention 走 CV-9 既有 mention badge). **0 server production code** — 复用 `useArtifactCommentAdded` 既有 WS frame hook (CV-5 #530) + 文案 + DOM byte-identical 锁. 跟 CV-9..13 / DM-5..6 / AP-4 / AP-5 0-server client-only 同模式.

## 0. 关键约束 (5 项立场)

1. **0 server production code** (跟 CV-9..13 同模式): comment unread 走既有 `useArtifactCommentAdded` WS hook (CV-5 #530 已建); 客户端纯计数, 不另起 server endpoint / 不开 schema. **反约束**: server-go internal/ git diff 0 行 production code; 反向 grep `comment_unread_endpoint\|cv14.*server\|cv14_unread_table` count==0.

2. **CV-14 跟 CV-9 mention badge 共存** (avoid mid-frame state coupling): mention 已被 CV-9 `ArtifactCommentsMentionBadge` 接管 (`mention_target_id == currentUserId`); CV-14 仅 filter `sender_id !== currentUserId` (不计自己发的). **反约束**: 不 import CV-9 hook 同步状态 — 双 badge 视觉独立挂在 thread 视图. mention comment 同时计入 mention badge + comment unread badge — 这是预期行为 (mention 是更强的 signal, unread 是总览); 反向 vitest 自己发的 frame 不增 CV-14 计数.

3. **thinking 5-pattern 锁链第 10 处** (RT-3+DM-3+DM-4+CV-7+CV-8+CV-9+CV-11+CV-12+CV-13+CV-14): badge 不暴露 reasoning, 反向 grep `processing\|responding\|thinking\|analyzing\|planning` 在 CommentUnreadBadge.tsx production count==0.

4. **文案 byte-identical**: badge 文案 `${N} 条新评论` (中文 byte-identical) + count > 99 显示 `99+` (跟 Sidebar.tsx unread-badge 既有同精神); count == 0 时不渲染 (反向锁). 改 = 改 content-lock SSOT 一处.

5. **DOM data-attr 锁**: `data-cv14-unread-count="<N>"` (presence + 整数值) + `data-cv14-comment-unread-badge` (root) byte-identical. e2e + vitest 双锁; 反向不写 sessionStorage / localStorage (跟 CV-9 ArtifactCommentsMentionBadge 同精神, 纯 component state).

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| CV-14.1 server | (无) — 反向断言 0 行 server diff (反向 grep test 守门) | 0 production code; 复用 ws.ArtifactCommentAddedFrame CV-5 #530 既有 + frame.mentions 字段已带 (CV-9 #539) |
| CV-14.2 client | `packages/client/src/components/CommentUnreadBadge.tsx` (新, ≤70 行) | 订阅 useArtifactCommentAdded; double-filter sender_id != currentUserId && !mentions.includes(currentUserId); count > 99 → "99+"; click → reset 0 |
| CV-14.3 closure | REG-CV14-001..005 + acceptance + content-lock + PROGRESS [x] + thinking 5-pattern 反向 grep + e2e cv-14-comment-unread-badge.spec.ts (deferred 占位 复用 CV-9 e2e harness) | 4 件套全闭 + thinking 锁链第 10 处 |

## 2. 文案 / DOM 锁 (content-lock SSOT)

```
badge label:               "${N} 条新评论"  (中文 byte-identical)
overflow display:           "99+"          (count > 99)
zero state:                  null         (不渲染, 反向锁)
data-cv14-comment-unread-badge (root presence)
data-cv14-unread-count="<N>"   (整数值, "0" 不渲染所以反向锁)
```

**0 新错码** — 客户端纯计数, 无 server-side error path.

## 3. 反向 grep 锚 (CV-14 实施 PR 必跑)

```
git grep -nE 'comment_unread_endpoint|cv14.*server|cv14_unread_table' packages/server-go/internal/  # 0 hit
git diff origin/main -- packages/server-go/ | grep -c '^\+'  # 0 production lines
git grep -nE 'processing|responding|thinking|analyzing|planning' packages/client/src/components/CommentUnreadBadge.tsx  # 0 hit (锁链第 10 处)
git grep -nE 'sessionStorage|localStorage' packages/client/src/components/CommentUnreadBadge.tsx  # 0 hit
git grep -c 'data-cv14-comment-unread-badge' packages/client/src/components/CommentUnreadBadge.tsx  # ≥1
```

## 4. 不在本轮范围 (deferred)

- ❌ persistent unread (跨 reload) — 复用 channel.unread_count 服务端聚合, 留 v2
- ❌ admin god-mode unread visibility (ADM-0 §1.3 红线 永久不挂)
- ❌ desktop notification (DL-4 Web Push gateway #490 路径)
- ❌ schema 改 / 新 endpoint
- ❌ unread per-comment pin / 历史 highlight (留 v3)
