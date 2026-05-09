# Client DM Reaction Picker — composite (current)

> 锚: 蓝图 [`channel-model.md §1.2`](../../blueprint/current/channel-model.md) (DM 概念独立, 底层复用 message/reaction) · 实现 PR #603
> 落点: `packages/client/src/components/DMMessageReactionPicker.tsx`
> 关联: DM-9 emoji 入口 + DM-5 reaction 汇总 + AP-4 ACL gate (server-side, byte-identical 跟 channel reactions)

## 文件清单

| 文件 | 角色 |
|---|---|
| `packages/client/src/components/DMMessageReactionPicker.tsx` | composite root — wires DM-9 EmojiPickerPopover (add new) + DM-5 ReactionSummary (display + toggle) |
| `packages/client/src/components/EmojiPickerPopover.tsx` | DM-9 #585 — 5-emoji preset picker (复用) |
| `packages/client/src/components/ReactionSummary.tsx` | DM-5 #549 — aggregated chips 渲染 + toggle (复用) |

**0 server / 0 schema / 0 endpoint** (composite 复用 CV-7 #535 `PUT /api/v1/messages/{id}/reactions` + AP-4 #551 channel-member ACL).

## 设计原则 (spec §0 byte-identical)

| § | 设计原则 |
|---|---|
| ① | 0 server production code — 复用 CV-7 PUT/DELETE/GET reactions endpoint + AP-4 ACL gate; 跟 DM-9 + DM-5 同 endpoint 单源 |
| ② | DM-only mounting path — 父组件 (MessageItem.tsx for DM channels) 仅在 `channel.type === 'dm'` 时挂此 composite (反 cross-channel mount) |
| ③ | 复用 EmojiPickerPopover (add 新 emoji) + ReactionSummary (display 既有 + toggle); 不另起组件复制功能 |
| ④ | thinking 5-pattern 锁链第 12 处 (DM-9 第 11 后续) — composite 不暴露 reasoning, 反向 grep 5 字面 0 hit |
| ⑤ | DOM data-attr 锁: `data-dm12-reaction-picker` (root) + delegate to DM-9 `data-dm9-*` + DM-5 `data-dm5-*` (反向不重复 attr) |

## Props 契约

| prop | 类型 | 角色 |
|---|---|---|
| `messageId`        | `string`                 | 目标 message id (ULID) |
| `currentUserId`    | `string`                 | 当前 user id (ReactionSummary 用作 own/other 区分) |
| `initialReactions` | `AggregatedReaction[]?`  | 父组件传初值; absent 时 mount auto-fetch |

## 行为契约

| 阶段 | 动作 |
|---|---|
| mount | 若 `initialReactions === undefined` → `getMessageReactions(messageId)` auto-fetch |
| add new emoji | EmojiPickerPopover click → server PUT → composite `onChanged` → refetch |
| toggle existing | ReactionSummary click → server PUT/DELETE → composite `onChanged` → refetch |
| fetch fail | best-effort (chip + picker 仍可点; 父组件可重新 trigger) |

## DOM 锚 (改 = 改两处: 此组件 + acceptance template)

| 锚 | 来源 |
|---|---|
| `div[data-dm12-reaction-picker]`               | composite root |
| `div[data-dm12-loading="true\|false"]`         | mount 期间 fetch 状态 |
| `div[data-dm9-*]`                              | EmojiPickerPopover 内, 不在此层重复 (设计 ⑤) |
| `div[data-dm5-*]`                              | ReactionSummary 内, 不在此层重复 (设计 ⑤) |

## 5-emoji preset (DM-9 SSOT byte-identical)

```ts
const DM9_EMOJI_PRESET = ['👍', '❤️', '😄', '🎉', '🚀'] as const;
```

字面顺序 SSOT 在 `EmojiPickerPopover.tsx`. 改 = 改一处.

## 反向 grep 守门

| 锚 | 期望 |
|---|---|
| 另起 emoji preset 数组 | 0 hit (反 [👍, ❤️, ...] 字面散落) |
| 另起 reaction chip 渲染 | 0 hit (复用 DM-5 ReactionSummary 单源) |
| 另起 reaction fetch | 0 hit (仅用 `getMessageReactions` from `lib/api`) |
| `sessionStorage` / `localStorage` 写入 | 0 hit (纯 component state) |
| 跨 channel mount (`channel.type !== 'dm'`) | 0 hit (父组件 type-guarded) |
| admin god-mode 旁路 | 0 hit (跟 ADM-0 §1.3 红线一致) |

## 不在范围 (留尾)

- 自定义 emoji (留 v2; 现 5-emoji 单源)
- emoji aria-label i18n (现走 emoji 字符直渲, 反 `<img alt="thumbs up">` 复杂化)
- reaction analytics / 排序 (现按 server 返回顺序; 留 v2 popularity 排序)
- channel 外其它场景 reaction picker (此 composite **DM-only**, 跟蓝图 §1.2 守一致: DM 视觉与交互跟 channel **明确不同**)
