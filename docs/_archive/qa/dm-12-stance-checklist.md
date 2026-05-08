# DM-12 stance checklist — DM message reaction picker composite

> 5 立场 byte-identical 跟 spec §0 (≤80 行).

## 1. 0 server production code (跟 DM-9 / DM-5 / CV-9..14 同模式)

- [x] 复用 CV-7 #535 PUT /api/v1/messages/{id}/reactions endpoint
- [x] 复用 AP-4 #551 channel-member ACL gate
- [x] 复用既有 client API: `addReaction`, `removeReaction`, `getMessageReactions` (lib/api.ts 既有)
- [x] 反向断: `git diff origin/main -- packages/server-go/` 0 production 行
- [x] 反向 grep `dm12.*server\|reactions_picker_table` 0 hit

## 2. DM-only mounting path (父组件决定)

- [x] composite 本身不知 channel.type — 父组件 (MessageItem.tsx, 留 follow-up) 决定挂载条件
- [x] follow-up wire-up PR 必加 `channel.type === 'dm'` filter byte-identical (跟 dm_4_message_edit.go #549 / dm_10_pin.go #597 DM-only 立场承袭)
- [x] composite 不发非 DM channel reject — 因复用 reactions endpoint 已经 channel-member ACL 守门

## 3. 复用 DM-9 + DM-5 不另起组件

- [x] DMMessageReactionPicker 只 import + compose, 不另写 emoji preset / 不另写 chip 渲染
- [x] **反向 grep**: `EMOJI_PRESET|emojiPreset` 在 DMMessageReactionPicker.tsx 0 hit (复用 DM-9 单源)
- [x] 不另起 reaction add helper — 走 DM-9 内置 onPick (调既有 addReaction)
- [x] 不另起 chip 渲染 — 走 DM-5 ReactionSummary 既有 onToggle (调既有 add/removeReaction)

## 4. thinking 5-pattern 锁链第 12 处

- [x] RT-3 #488 第 1 + DM-3 #508 第 2 + DM-4 #549 第 3 + CV-7 #535 第 4 + CV-8 #537 第 5 + CV-9 #539 第 6 + CV-11 #543 第 7 + CV-12 #545 第 8 + CV-13 #557 第 9 + CV-14 #581 第 10 + DM-9 #585 第 11 + DM-12 第 12
- [x] composite 不暴露 reasoning, 反向 grep `processing|responding|thinking|analyzing|planning` 在 DMMessageReactionPicker.tsx 0 hit

## 5. DOM data-attr 锁 + 反向 storage

- [x] `data-dm12-reaction-picker` (root presence)
- [x] `data-dm12-loading="true"|"false"` (auto-fetch in-flight 状态)
- [x] 子组件 delegate to DM-9 `data-dm9-*` + DM-5 `data-dm5-*` 锚 (反向不重复 attr)
- [x] e2e + vitest 双锁
- [x] 反向 grep `sessionStorage|localStorage` 在 DMMessageReactionPicker.tsx 0 hit (纯 component state)

## 反约束

- ❌ MessageItem.tsx 真挂集成 (留 follow-up wire-up PR)
- ❌ admin god-mode reaction visibility (永久不挂, ADM-0 §1.3)
- ❌ custom emoji preset (留 v3 跟 DM-9 同期)
- ❌ optimistic UI (留 v2)
- ❌ 另起 emoji preset / chip 渲染 / reaction helper (反 DM-9/DM-5 dup)
- ❌ schema 改 / 新 endpoint
- ❌ 自动选 channel.type === 'dm' (composite 不感知 channel context, 父决定)

## 跨 milestone byte-identical 锁链

- DM-9 #585 EmojiPickerPopover (5-emoji preset 复用)
- DM-5 #549 ReactionSummary (chip 渲染复用)
- CV-7 #535 PUT/DELETE/GET reactions endpoint (SSOT)
- AP-4 #551 channel-member ACL gate
- thinking 5-pattern 锁链第 12 处
- ADM-0 §1.3 红线
- DM-10 #597 / DM-11 #600 client-only 同模式
