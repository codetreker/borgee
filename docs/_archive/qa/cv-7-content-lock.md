# CV-7 Content-Lock — DOM 锚 + 文案 byte-identical

> 战马E · 2026-04-29 · spec `cv-7-spec.md` 立场 ④ — UI 必锁 DOM + 文案 byte-identical 跨链.

## 1. DOM 锚 (反向 grep ≥1 hit, CI lint 自动闸)

| # | 锚 | 字面 | 用途 | 反向 grep 命令 |
|---|---|---|---|---|
| ① | edit 按钮 data-attr | `data-cv7-edit-btn` | e2e 选择器 + 反向断 sender-only 渲染 | `git grep -n 'data-cv7-edit-btn' packages/client/src/` count≥1 |
| ② | reaction button data-attr | `data-cv7-reaction-target` | e2e 选择器 + 反向断 click → PUT /reactions | `git grep -n 'data-cv7-reaction-target' packages/client/src/` count≥1 |
| ③ | edit modal root data-attr | `data-cv7-edit-modal` | modal 挂载锚 + e2e 关闭检测 | `git grep -n 'data-cv7-edit-modal' packages/client/src/` count≥1 |

## 2. 文案 byte-identical (反向 grep ≥1 hit)

| # | 文案 | 触发 | 反向 grep |
|---|---|---|---|
| ① | "确认删除这条评论?" | delete 按钮 click confirm | `git grep -n '确认删除这条评论\?' packages/client/src/` count≥1 |
| ② | "保存" | edit modal 保存按钮 | (复用既有 button label, 不另锁) |
| ③ | "取消" | edit modal 取消按钮 | (复用既有 button label, 不另锁) |

## 3. 反约束 (CI grep 0 hit)

| # | 反约束 | 反向 grep |
|---|---|---|
| ① | 不另起 emoji picker — 复用现有 message reaction unicode | `git grep -nE 'CV7EmojiPicker\|cv7-emoji-picker' packages/client/src/` count==0 |
| ② | edit history 不渲染 — forward-only 即覆写, 不显历史版本 | `git grep -nE 'CommentEditHistory\|edit_history.*comment' packages/client/src/` count==0 |
| ③ | admin god-mode UI 不挂 — admin /admin-console/* 不渲染 comment edit/delete | `git grep -nE 'admin.*ArtifactComments\|admin.*comment.*edit' packages/client/src/' count==0` |

## 4. 5-pattern thinking subject 错误码 byte-identical (跟 CV-5 同字符串)

- `comment.thinking_subject_required` — 5-pattern 第 5 处链 (RT-3 + BPP-2.2 + AL-1b + CV-5 + CV-7), 改 = 改 5 处.
