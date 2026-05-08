# CV-8 Content-Lock — DOM 锚 + 文案 byte-identical

> 战马E · 2026-04-29 · spec `cv-8-spec.md` 立场 ④ — UI 必锁 DOM + 文案 byte-identical 跨链.

## 1. DOM 锚 (反向 grep ≥1 hit, CI lint 自动闸)

| # | 锚 | 字面 | 用途 | 反向 grep |
|---|---|---|---|---|
| ① | thread toggle data-attr | `data-cv8-thread-toggle="<parent_id>"` | collapse/expand toggle 选择器 | `git grep -n 'data-cv8-thread-toggle' packages/client/src/` count≥1 |
| ② | reply target data-attr | `data-cv8-reply-target="<parent_id>"` | reply 入口选择器 + 反向断 1-level (nested reply 内 0 hit) | `git grep -n 'data-cv8-reply-target' packages/client/src/` count≥1 |
| ③ | reply input root | `data-cv8-reply-input` | reply textarea 挂载锚 | `git grep -n 'data-cv8-reply-input' packages/client/src/` count≥1 |

## 2. 文案 byte-identical (反向 grep ≥1 hit)

| # | 文案 | 触发 |
|---|---|---|
| ① | "▼ 隐藏 N 条回复" | thread expanded 状态 toggle button label (N 实际数) |
| ② | "▶ 显示 N 条回复" | thread collapsed 状态 toggle button label |
| ③ | "回复" | reply button label (复用既有) |

## 3. 反约束 (CI grep 0 hit)

| # | 反约束 |
|---|---|
| ① | thread depth 1 层 — 反向 grep `cv8.*depth.*[2-9]\|cv8.*recursive\|cv8.*nested.*reply` 在 internal/ + client/src/ count==0 |
| ② | nested reply 内不渲染 reply button — vitest 反向断 (data-cv8-reply-target count==0 in nested DOM) |
| ③ | admin god-mode 不挂 thread UI — `git grep -nE 'admin.*ArtifactCommentThread\|admin.*comment.*thread' packages/client/src/` count==0 |

## 4. 5-pattern thinking subject 错误码 byte-identical (跟 CV-5/CV-7 同字符)

- `comment.thinking_subject_required` — 5-pattern 第 6 处链 (RT-3 + BPP-2.2 + AL-1b + CV-5 + CV-7 + CV-8)
- `comment.reply_target_invalid` — 立场 ④ 新错码
- `comment.thread_depth_exceeded` — 立场 ④ 新错码
