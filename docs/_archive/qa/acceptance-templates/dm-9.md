# Acceptance Template — DM-9: DM message emoji picker ✅

> 0 server code (跟 CV-9..14 / DM-5..6 / AP-4..5 同模式) — 5-emoji preset picker, 复用 CV-7 #535 PUT /api/v1/messages/{id}/reactions endpoint + AP-4 #551 ACL gate. 4 件套全闭. **0 schema 改 (反 v=39 migration 硬锁)**.

## 验收清单

### §1 DM-9.1 — server 反向断言 0 行 + 0 schema

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 0 server production 行 | git diff | `git diff origin/main -- packages/server-go/` 0 production lines |
| 1.2 反向 grep `dm9.*server\|emoji_picker_endpoint\|dm9_picker_table` 0 hit | grep | server-go internal/ 0 hit |
| 1.3 反向 grep `Version: ?39` 在 internal/migrations/ 0 hit (反 v=39 schema 硬锁) | grep | 0 hit |
| 1.4 复用 CV-7 #535 PUT /api/v1/messages/{id}/reactions + AP-4 #551 ACL | inspect | 既有 |

### §2 DM-9.2 — client EmojiPickerPopover

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 toggle button `+` 渲染 + title `添加表情` byte-identical | vitest | `EmojiPickerPopover.test.tsx::TestDM9_ToggleByteIdentical` PASS |
| 2.2 popover 默认 closed (data-dm9-emoji-picker-popover 不渲染) | vitest | `::TestDM9_DefaultClosed` PASS |
| 2.3 toggle click → popover open + 5 emoji 顺序 byte-identical | vitest | `::TestDM9_OpenAndPresetOrder` PASS |
| 2.4 emoji click → addReaction(messageId, emoji) + popover close + onChanged callback | vitest | `::TestDM9_EmojiClickTriggersAddReaction` PASS |
| 2.5 Escape key → popover close | vitest | `::TestDM9_EscapeCloses` PASS |
| 2.6 DOM 4 data-attr 锚 byte-identical | vitest | `::TestDM9_DOMAttrs` PASS |
| 2.7 不写 sessionStorage / localStorage | grep | 0 hit |
| 2.8 不 import fetch (only addReaction api) | grep | 0 hit |

### §3 DM-9.3 — closure

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 thinking 5-pattern 反向 grep 0 hit (锁链第 11 处) | grep | EmojiPickerPopover.tsx 0 hit |
| 3.2 5 emoji preset byte-identical 字面 (`👍 ❤️ 😄 🎉 🚀`) | grep | content-lock §1 同步 |
| 3.3 REG-DM9-001..005 5 行 🟢 | regression-registry.md | 5 行 |
| 3.4 PROGRESS [x] 加行 | PROGRESS.md | changelog 加行 |
| 3.5 acceptance template ✅ closed | 本文件 | 关闭区块加日期 |

## 边界

- CV-7 #535 PUT /api/v1/messages/{id}/reactions (DM-9 SSOT 入口) / AP-4 #551 channel-member ACL gate / DM-5 #549 ReactionSummary 视觉互补 / CV-7 ArtifactCommentItem 默认 `👍` 不另起 emoji picker (DM-9 反约束今天补) / ADM-0 §1.3 admin god-mode 红线 / thinking 5-pattern 锁链第 11 处

## 退出条件

- §1+§2+§3 全绿
- 0 server production 代码 + 0 schema 改 (反 v=39 硬锁)
- vitest EmojiPickerPopover 6 case PASS
- 文案 + DOM byte-identical (content-lock §1+§2+§3)
- REG-DM9-001..005 5 行

## 关闭

✅ 2026-04-30 战马E — 0 server diff (`git diff origin/main -- packages/server-go/` 0 行) + vitest 70 files / 510 tests 全 PASS (含 EmojiPickerPopover 8 case) + typecheck 全绿 + 反向 grep `Version: ?39` 在 internal/migrations/ 0 hit; thinking 5-pattern 锁链第 11 处 (RT-3 + DM-3 + DM-4 + CV-7 + CV-8 + CV-9 + CV-11 + CV-12 + CV-13 + CV-14 + DM-9); 5 emoji preset byte-identical (👍 ❤️ 😄 🎉 🚀); DOM 4 锚 (toggle + popover + 5 option + open 状态).
