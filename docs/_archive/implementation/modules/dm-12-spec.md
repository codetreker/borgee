# DM-12 spec brief — DM message reaction picker composite (≤80 行)

> 战马E · Phase 5+ · ≤80 行 · 蓝图 [`channels-dm-collab.md`](../../blueprint/channels-dm-collab.md) §3 reactions UX. DM-12 是 **DM-9 EmojiPickerPopover (add 新) + DM-5 ReactionSummary (display 既有 + toggle) + auto-fetch reactions** 的 composite — 给 DM message bubble 一站式 reaction UX. **0 server production code** (复用 CV-7 #535 + AP-4 #551 同 endpoint). 跟 CV-9..14 / DM-5 / DM-6 / DM-9 / DM-10 / DM-11 / AP-4 / AP-5 0-server client-only 同模式.

## 0. 关键约束 (5 项立场)

1. **0 server production code** — 复用 CV-7 #535 PUT /api/v1/messages/{id}/reactions + AP-4 #551 ACL gate. 不开新 endpoint / 不改 schema. 反向断: `git diff origin/main -- packages/server-go/` 0 production 行; 反向 grep `dm12.*server\|reactions_picker_table` 在 internal/ 0 hit.

2. **DM-only mounting path** — 父组件 (MessageItem.tsx for DM channels, 留 follow-up wire-up PR) 决定只在 channel.type === 'dm' 时挂此 composite. 反向枚举: `channel.type === 'dm'` filter byte-identical 跟 dm_4_message_edit.go #549 / dm_10_pin.go #597 DM-only 立场承袭.

3. **复用 DM-9 + DM-5 不另起组件** — DMMessageReactionPicker 只 import + compose, 不另写 emoji preset / 不另写 chip 渲染. 反向 grep `EMOJI_PRESET\|emojiPreset` 在 DMMessageReactionPicker.tsx 0 hit (复用 DM-9 单源); 反向 grep `data-dm5-reaction-chip\b.*=' or 自定义 chip render` 0 hit.

4. **thinking 5-pattern 锁链第 12 处** (RT-3+DM-3+DM-4+CV-7+CV-8+CV-9+CV-11+CV-12+CV-13+CV-14+DM-9+DM-12): composite 不暴露 reasoning, 反向 grep `processing\|responding\|thinking\|analyzing\|planning` 在 DMMessageReactionPicker.tsx 0 hit.

5. **DOM data-attr 锁**: `data-dm12-reaction-picker` (root presence) + `data-dm12-loading` (`"true"|"false"` 状态) + delegate to DM-9 `data-dm9-*` + DM-5 `data-dm5-*` 锚 (反向不重复 attr). e2e + vitest 双锁; 反向不写 sessionStorage / localStorage (纯 component state).

## 1. 拆段实施 (单 PR 全闭)

| 段 | 文件 | 范围 |
|---|---|---|
| DM-12.1 server | (无) — 反向断言 0 production diff | 0 行 server (跟 DM-9 同模式) |
| DM-12.2 client | `packages/client/src/components/DMMessageReactionPicker.tsx` (新, ≤70 行) | 复用 DM-9 EmojiPickerPopover + DM-5 ReactionSummary; auto-fetch on mount when initialReactions undefined; refetch on add/toggle; 7 vitest case |
| DM-12.3 closure | REG-DM12-001..006 + acceptance + content-lock + PROGRESS [x] | 5 立场 byte-identical 锁 + 反向 grep 4 锚 |

## 2. 文案 / DOM 锁 (content-lock SSOT)

```
data-dm12-reaction-picker        (root container)
data-dm12-loading="true"|"false" (auto-fetch in-flight 状态)
```

文案 byte-identical 全 delegate 给 DM-9 (`+`/`添加表情`) + DM-5 (`{emoji} {count}`) — 改 = 改 DM-9/DM-5 content-lock.

**0 新错码** — 复用 CV-7 既有 reactions endpoint 错码.

## 3. 反向 grep 锚 (DM-12 实施 PR 必跑)

```
git diff origin/main -- packages/server-go/ | grep -c '^\+'  # 0 行 (0 server code)
git grep -nE 'EMOJI_PRESET|emojiPreset' packages/client/src/components/DMMessageReactionPicker.tsx  # 0 hit (复用 DM-9 单源)
git grep -nE 'processing|responding|thinking|analyzing|planning' packages/client/src/components/DMMessageReactionPicker.tsx  # 0 hit (锁链第 12 处)
git grep -nE 'sessionStorage|localStorage' packages/client/src/components/DMMessageReactionPicker.tsx  # 0 hit
git grep -c 'data-dm12-reaction-picker' packages/client/src/components/DMMessageReactionPicker.tsx  # ≥1
```

## 4. 不在本轮范围 (deferred)

- ❌ MessageItem.tsx 真挂 DMMessageReactionPicker (留 follow-up wire-up PR — channel.type === 'dm' filter + 替换既有 ReactionBar 重 emoji-mart 包)
- ❌ admin god-mode reaction visibility (永久不挂, ADM-0 §1.3)
- ❌ custom emoji preset (留 v3 跟 DM-9 同期)
- ❌ optimistic UI add/remove (留 v2 — 现版 await refetch chain 简单可靠)
- ❌ schema 改 / 新 endpoint
