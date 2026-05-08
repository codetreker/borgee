# DM-9 content-lock — emoji picker 文案 + DOM SSOT

> 改 = 改本文件一处. e2e + vitest + 实施代码三轨字面 byte-identical.

## 1. Emoji preset 字面 byte-identical (5 个固定)

| 顺序 | Emoji | 备注 |
|---|---|---|
| 0 | `👍` | thumbs up |
| 1 | `❤️` | red heart (with VS16 selector) |
| 2 | `😄` | smile |
| 3 | `🎉` | tada |
| 4 | `🚀` | rocket |

```ts
const DM9_EMOJI_PRESET = ['👍', '❤️', '😄', '🎉', '🚀'] as const;
```

**改 5 emoji 顺序 = 改本文件 + EmojiPickerPopover.tsx + vitest + e2e 同步.**

## 2. 文案字面 byte-identical

| 用途 | 字面 | 备注 |
|---|---|---|
| toggle button label | `+` | 单字符 |
| toggle title | `添加表情` | 中文 byte-identical (a11y title attr) |

## 3. DOM data-attr SSOT

| 属性 | 值 | 用途 |
|---|---|---|
| `data-dm9-emoji-picker-toggle` | (无值, presence) | `+` toggle button, e2e selector |
| `data-dm9-emoji-picker-popover` | (无值, presence) | popover root, 仅 open 时渲染 |
| `data-dm9-emoji-option` | `<emoji>` (e.g. `"👍"`) | 5 个 option button, vitest 字面验证 |
| `data-dm9-popover-open` | `"true"` 或 `"false"` | toggle 状态, attached on toggle button |

## 4. 不变量

- popover 默认 closed; toggle click → open; outside click / Escape → close
- emoji option click → `addReaction(messageId, emoji)` + close popover + onChanged callback
- 不写 sessionStorage / localStorage (纯 component state)
- 不另起 fetch (复用 lib/api.ts::addReaction)

## 5. 反约束 grep (DM-9 PR 必跑)

```
git grep -nE 'data-dm9-emoji-picker-popover' packages/client/src/components/EmojiPickerPopover.tsx  # ≥1
git grep -nE '👍|❤️|😄|🎉|🚀' packages/client/src/components/EmojiPickerPopover.tsx  # ≥5 (preset 字面)
git grep -nE '添加表情' packages/client/src/components/EmojiPickerPopover.tsx  # ≥1
git grep -nE 'sessionStorage|localStorage' packages/client/src/components/EmojiPickerPopover.tsx  # 0 hit
git grep -nE 'fetch\(' packages/client/src/components/EmojiPickerPopover.tsx  # 0 hit (only addReaction api call)
```
