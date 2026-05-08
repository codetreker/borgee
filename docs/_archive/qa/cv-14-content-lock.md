# CV-14 content-lock — comment unread badge 文案 + DOM SSOT

> 改 = 改本文件一处. e2e + vitest + 实施代码三轨字面 byte-identical.

## 1. 文案字面 byte-identical

| 用途 | 字面 | 备注 |
|---|---|---|
| badge label | `${N} 条新评论` | 中文 byte-identical (跟 Sidebar.tsx unread-badge 同精神, count 内嵌 template) |
| overflow display | `99+` | count > 99 时仅显示 `99+` (label 改为 `99+ 条新评论`) |
| zero state | (不渲染) | `unreadCount === 0 → return null` 反向锁 |

## 2. DOM data-attr SSOT

| 属性 | 值 | 用途 |
|---|---|---|
| `data-cv14-comment-unread-badge` | (无值, presence) | badge root, e2e selector 锚 |
| `data-cv14-unread-count` | `<N>` (整数 string, e.g. `"3"` 或 `"99+"`) | 计数字面值, e2e 真验证 |

## 3. 不变量

- `unreadCount === 0` → `return null` (badge 不渲染, 反向锁)
- click → reset to 0 → badge 消失
- 不写 sessionStorage / localStorage (纯 component state)
- 不另起 fetch / API 调用 (props + WS hook driven)

## 4. 反约束 grep (CV-14 PR 必跑)

```
git grep -nE 'data-cv14-comment-unread-badge' packages/client/src/components/CommentUnreadBadge.tsx  # ≥1
git grep -nE '条新评论' packages/client/src/components/CommentUnreadBadge.tsx  # ≥1 (字面 byte-identical)
git grep -nE '99\+' packages/client/src/components/CommentUnreadBadge.tsx  # ≥1
git grep -nE 'sessionStorage|localStorage' packages/client/src/components/CommentUnreadBadge.tsx  # 0 hit
git grep -nE 'fetch\(|api\.' packages/client/src/components/CommentUnreadBadge.tsx  # 0 hit
```
