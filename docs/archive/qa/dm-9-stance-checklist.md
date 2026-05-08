# DM-9 stance checklist — DM message emoji picker

> 5 立场 byte-identical 跟 spec §0 (≤80 行).

## 1. 0 server production code (跟 CV-9..14 / DM-5..6 / AP-4..5 同模式)

- [x] 复用 PUT /api/v1/messages/{id}/reactions (CV-7 #535 既有)
- [x] 复用 AP-4 #551 channel-member ACL gate
- [x] 复用 client `addReaction(messageId, emoji)` API (lib/api.ts:456 既有)
- [x] **不开 v=39 schema migration** (反约束硬锁: team-lead 建议 v=39 reactions JSON 列被否, 既有 reactions 表 + endpoint 已盖, 加 JSON 列重复存储 + 跟既有 reactions 表 SSOT 漂)
- [x] 反向断言: `git diff origin/main -- packages/server-go/` 0 production 行
- [x] 反向 grep `Version: ?39` 在 internal/migrations/ 0 hit

## 2. 5-emoji preset byte-identical

- [x] 字面 `["👍","❤️","😄","🎉","🚀"]` 顺序 byte-identical (跟 content-lock §1 SSOT)
- [x] 反约束: 不另起 emoji unicode 集 (跟 CV-7 ArtifactCommentItem 默认 `👍` + DM-5 ReactionSummary 既有 unicode chip 同精神)
- [x] vitest 真测 5 emoji 顺序 + 字面 byte-identical
- [x] 改 = 改 content-lock §1 一处

## 3. thinking 5-pattern 反约束锁链第 11 处

- [x] RT-3 #488 第 1 + DM-3 #508 第 2 + DM-4 #549 第 3 + CV-7 #535 第 4 + CV-8 #537 第 5 + CV-9 #539 第 6 + CV-11 #543 第 7 + CV-12 #545 第 8 + CV-13 #557 第 9 + CV-14 #581 第 10 + DM-9 第 11
- [x] picker 不暴露 reasoning, 反向 grep 5 字面 在 EmojiPickerPopover.tsx 0 hit

## 4. DOM data-attr 锁

- [x] `data-dm9-emoji-picker-toggle` (`+` button)
- [x] `data-dm9-emoji-picker-popover` (popover root, presence on open)
- [x] `data-dm9-emoji-option="<emoji>"` (5 个 byte-identical)
- [x] `data-dm9-popover-open="true"|"false"` (toggle 状态)
- [x] e2e + vitest 双锁

## 5. 互斥共存 (跟 DM-5 ReactionSummary)

- [x] picker = 加新 emoji 入口; ReactionSummary = 显示 + toggle 既有
- [x] 两组件同 message bubble 视觉挂; 不 import 互相 hook (各自独立 props)
- [x] picker click 后 → onChanged callback → 父组件 refetch reactions (DM-5 既有 ReactionSummary 跟着刷新)
- [x] 反向: 不直接 mutate DM-5 state (各自独立 component state)

## 反约束

- ❌ custom emoji upload (留 v3)
- ❌ emoji search filter (preset 5 固定)
- ❌ admin god-mode picker (ADM-0 §1.3 红线 永久)
- ❌ schema 改 / 新 endpoint / v=39 migration (硬锁)
- ❌ skin tone modifier (留 v2)

## 跨 milestone byte-identical 锁链

- CV-7 #535 PUT /api/v1/messages/{id}/reactions endpoint (DM-9 SSOT 入口)
- AP-4 #551 channel-member ACL gate (复用)
- DM-5 #549 ReactionSummary chips (视觉互补)
- CV-7 ArtifactCommentItem 默认 `👍` + 不另起 emoji picker (CV-7 反约束今 DM-9 真 picker 是补)
- thinking 5-pattern 锁链第 11 处
- ADM-0 §1.3 红线 (admin god-mode 不挂)
