# DM-12 content-lock — DM message reaction picker composite SSOT

> 改 = 改本文件一处. 文案/preset 全 delegate 到 DM-9 / DM-5 SSOT.

## 1. DOM data-attr SSOT

| 属性 | 值 | 用途 |
|---|---|---|
| `data-dm12-reaction-picker` | (无值, presence) | composite root, e2e selector 锚 |
| `data-dm12-loading` | `"true"` 或 `"false"` | auto-fetch in-flight 状态 (initialReactions undefined → mount 时 true → fetch 完 false) |

## 2. 文案 / preset delegate (改 = 改 DM-9/DM-5 SSOT)

| 用途 | 来源 |
|---|---|
| emoji preset (5 个 byte-identical 顺序 `👍 ❤️ 😄 🎉 🚀`) | DM-9 content-lock §1 (DM9_EMOJI_PRESET) |
| toggle button label `+` + title `添加表情` | DM-9 content-lock §2 |
| chip 文案 `{emoji} {count}` byte-identical | DM-5 content-lock §1 |
| chip data attr (`data-dm5-reaction-chip`/`-count`/`-mine`) | DM-5 content-lock §2 |

**0 新文案 / 0 新 preset** — composite 仅 compose, 不另起字面.

## 3. 不变量

- `initialReactions === undefined` → 自动 fetch on mount (loading=true → false)
- `initialReactions !== undefined` → 跳过 mount fetch (loading=false), 仅在 onChanged 时 refetch
- `reactions.length === 0` → ReactionSummary 不渲染 (DM-5 既有), 仅 picker toggle 显示
- `reactions.length > 0` → ReactionSummary chip + picker toggle 同时显示
- emoji picker click → addReaction → onChanged → composite refetch → ReactionSummary 自动更新
- 不写 sessionStorage / localStorage (纯 component state)
- 不另起 fetch (复用 lib/api.ts::getMessageReactions)

## 4. 反约束 grep (DM-12 PR 必跑)

```
git grep -nE 'data-dm12-reaction-picker' packages/client/src/components/DMMessageReactionPicker.tsx  # ≥1
git grep -nE 'EMOJI_PRESET|emojiPreset' packages/client/src/components/DMMessageReactionPicker.tsx  # 0 hit (DM-9 单源)
git grep -nE 'sessionStorage|localStorage' packages/client/src/components/DMMessageReactionPicker.tsx  # 0 hit
git grep -nE '👍|❤️|😄|🎉|🚀' packages/client/src/components/DMMessageReactionPicker.tsx  # 0 hit (preset 全 delegate DM-9)
```
