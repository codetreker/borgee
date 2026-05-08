# CV-9 Content-Lock — DOM 锚 + 文案 byte-identical

> 战马E · 2026-04-29 · spec `cv-9-spec.md` 立场 ④ — UI 必锁 DOM + 文案 byte-identical 跨链.

## 1. DOM 锚 (反向 grep ≥1 hit, CI lint 自动闸)

| # | 锚 | 字面 | 用途 | 反向 grep |
|---|---|---|---|---|
| ① | unread count badge data-attr | `data-cv9-unread-count` (值是数字) | header badge 选择器 + e2e 验 unread count | `git grep -n 'data-cv9-unread-count' packages/client/src/` count≥1 |
| ② | mention toast data-attr | `data-cv9-mention-toast` | toast 挂载锚 | `git grep -n 'data-cv9-mention-toast' packages/client/src/` count≥1 |

## 2. 文案 byte-identical (反向 grep ≥1 hit)

| # | 文案 | 触发 | 反向 grep |
|---|---|---|---|
| ① | "你被 @ 在 N 条评论中" | unread count > 0 时 badge title / toast (N 是数字占位符) | `git grep -nE '你被 @ 在 [0-9N]+ 条评论中\|你被 @ 在 \\$\\{.*\\} 条评论中' packages/client/src/` count≥1 |

count==0 时不渲染 — 反向断 (vitest TestCount0NotRendered).

## 3. 反约束 (CI grep 0 hit)

| # | 反约束 | 反向 grep |
|---|---|---|
| ① | 不另起 mention state — 复用 useMentionPushed 既有 hook | `git grep -nE 'useCV9MentionState\|cv9-mention-state' packages/client/src/` count==0 |
| ② | 不另起 fan-out 路径 — 复用 DM-2.2 既有 | `git grep -nE 'cv9.*fanout\|cv9.*dispatch' packages/server-go/internal/` count==0 |
| ③ | admin god-mode UI 不挂 — admin /admin-console/* 不渲染 mention badge | `git grep -nE 'admin.*ArtifactCommentsMentionBadge' packages/client/src/` count==0 |

## 4. 5-pattern thinking subject 错误码 byte-identical (跟 CV-5/CV-7/CV-8 同字符)

- `comment.thinking_subject_required` — 5-pattern **第 7 处链** byte-identical (RT-3 + BPP-2.2 + AL-1b + CV-5 + CV-7 + CV-8 + CV-9)
- `mention.target_not_in_channel` — DM-2.2 既有错码复用 (cross-channel mention reject)
