# CV-13 content-lock — quote block 文案 + DOM SSOT

> 改 = 改本文件一处. e2e + vitest + 实施代码三轨字面 byte-identical.

## 1. 文案字面 byte-identical

| 用途 | 字面 | 备注 |
|---|---|---|
| quote prefix | `> ` | markdown blockquote 风格 (含尾随空格) |
| author 前缀 | `@` | 不带尾随空格, 紧贴 author name |
| collapse expanded label | `收起` | quote 展开时按钮文字 |
| collapse collapsed label | `展开` | quote 折叠时按钮文字 |
| missing fallback | `(原消息已删除)` | parent message 不存在 / 已 soft-delete 时 |
| truncate suffix | `…` | unicode horizontal ellipsis (U+2026) 单字符 |
| truncate length | `200` | quoted body 字符数上限 (含截断后总长 ≤ 201 含 `…`) |

## 2. DOM data-attr SSOT

| 属性 | 值 | 用途 |
|---|---|---|
| `data-cv13-quoted-block` | (无值, presence) | quote 块根节点, e2e selector 锚 |
| `data-cv13-quoted-author` | (无值, presence) | author span 锚 |
| `data-cv13-quoted-id` | `<parent message uuid>` | parent message 跨 component 引用 |
| `data-cv13-collapsed` | `"true"` 或 `"false"` | toggle 状态 (initial false, click → true) |

## 3. 不变量

- truncate 仅在 collapsed 状态下触发; expanded 状态显示完整 body
- missing fallback 渲染时 `data-cv13-quoted-id` = ""(空字符串) 或不渲染 author span
- collapse toggle 不写 sessionStorage / localStorage (纯 component state, 反约束 — 跟 DM-3 / DM-4 / CV-9..12 同精神)

## 4. 反约束 grep (CV-13 PR 必跑)

```
git grep -nE 'data-cv13-quoted-block' packages/client/src/components/QuotedCommentBlock.tsx  # ≥1
git grep -nE '收起|展开' packages/client/src/components/QuotedCommentBlock.tsx  # ≥2 (字面 byte-identical)
git grep -nE '原消息已删除' packages/client/src/components/QuotedCommentBlock.tsx  # ≥1
git grep -nE 'sessionStorage|localStorage' packages/client/src/components/QuotedCommentBlock.tsx  # 0 hit
```
