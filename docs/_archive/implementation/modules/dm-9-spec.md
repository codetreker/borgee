# DM-9 spec brief — DM message emoji picker (≤80 行)

> 战马E · Phase 5+ · ≤80 行 · 蓝图 [`channels-dm-collab.md`](../../blueprint/channels-dm-collab.md) §3 reactions UX. DM-9 给 DM message bubble 加 5-emoji preset picker — 点 `+` button 弹 5 emoji (👍 ❤️ 😄 🎉 🚀), 点 emoji 触发既有 `addReaction()` API. **0 server production code** — 复用 messages.go::handleAddReaction (CV-7 #535 + AP-4 #551 既有 endpoint) + DM-5 #549 ReactionSummary chips 视觉. 跟 CV-9..14 / DM-5..6 / AP-4 / AP-5 0-server client-only 同模式.

## 0. 关键约束 (5 项立场)

1. **0 server production code** (跟 CV-9..14 / DM-5..6 / AP-4..5 同模式): 复用 PUT `/api/v1/messages/{id}/reactions` (CV-7 #535) + AP-4 #551 channel-member ACL gate. **反约束**: server-go internal/ git diff 0 行 production code; 反向 grep `dm9.*server\|emoji_picker_endpoint\|dm9_picker_table` count==0; **不开 v=39 schema migration** (team-lead 建议 v=39 reactions JSON ALTER 列被否 — 既有 reactions 表 + endpoint 已盖, 加 JSON 列重复存储).

2. **5-emoji preset byte-identical**: 字面 `["👍","❤️","😄","🎉","🚀"]` 顺序 byte-identical (改 = 改 content-lock SSOT 一处). 反约束: 不另起 emoji unicode 集 (跟 CV-7 ArtifactCommentItem `👍` 默认 + DM-5 ReactionSummary 既有 unicode chip 同精神).

3. **thinking 5-pattern 锁链第 11 处** (RT-3+DM-3+DM-4+CV-7+CV-8+CV-9+CV-11+CV-12+CV-13+CV-14+DM-9): picker 不暴露 reasoning, 反向 grep `processing\|responding\|thinking\|analyzing\|planning` 在 EmojiPickerPopover.tsx production count==0.

4. **DOM data-attr 锁**: `data-dm9-emoji-picker-toggle` (`+` button) + `data-dm9-emoji-picker-popover` (popover root) + `data-dm9-emoji-option="<emoji>"` (5 个 byte-identical) + `data-dm9-popover-open="true"|"false"` (toggle 状态). e2e + vitest 双锁.

5. **互斥共存** (跟 DM-5 ReactionSummary): picker = 加新 emoji 入口; ReactionSummary = 显示既有 + toggle. 两组件视觉同 message bubble 挂; 不 import 互相 hook (各自独立). 反约束: picker click 后 — DM-5 既有 ReactionSummary 通过 `onChanged` 回调 refetch (跟 CV-7 ArtifactCommentItem onChanged 同模式).

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| DM-9.1 server | (无) — 反向断言 0 行 server diff (反向 grep test 守门) | 0 production code; 复用 CV-7 #535 PUT/DELETE/GET reactions endpoint + AP-4 #551 ACL gate |
| DM-9.2 client | `packages/client/src/components/EmojiPickerPopover.tsx` (新, ≤80 行) | 5 emoji preset + toggle state + click → addReaction(messageId, emoji) + onChanged callback + click outside / Escape close |
| DM-9.3 closure | REG-DM9-001..005 + acceptance + content-lock + PROGRESS [x] + thinking 5-pattern 反向 grep + 反向 v=39 0 schema 改断言 | 4 件套全闭 + thinking 锁链第 11 处 |

## 2. 文案 / DOM 锁 (content-lock SSOT)

```
emoji preset (5):    ["👍","❤️","😄","🎉","🚀"]  (顺序 byte-identical)
toggle button label:  "+"   (单字符)
toggle title:         "添加表情"   (中文 byte-identical)
data-dm9-emoji-picker-toggle    (root toggle button)
data-dm9-emoji-picker-popover   (popover root, presence on open)
data-dm9-emoji-option="<emoji>" (each option, 5 个)
data-dm9-popover-open="true"|"false"  (toggle 状态)
```

**0 新错码** — 复用 CV-7 既有 reactions endpoint (PUT 200 happy / 404 channel hidden / 401).

## 3. 反向 grep 锚 (DM-9 实施 PR 必跑)

```
git grep -nE 'dm9.*server|emoji_picker_endpoint|dm9_picker_table' packages/server-go/internal/  # 0 hit
git grep -nE 'Version: ?39' packages/server-go/internal/migrations/  # 0 hit (反 v=39 schema)
git diff origin/main -- packages/server-go/ | grep -c '^\+'  # 0 production lines
git grep -nE 'processing|responding|thinking|analyzing|planning' packages/client/src/components/EmojiPickerPopover.tsx  # 0 hit (锁链第 11 处)
git grep -c 'data-dm9-emoji-picker-popover' packages/client/src/components/EmojiPickerPopover.tsx  # ≥1
```

## 4. 不在本轮范围 (deferred)

- ❌ custom emoji upload (留 v3 + 蓝图 §3 未提)
- ❌ emoji search filter (preset 5 个固定, 不要分类树)
- ❌ admin god-mode emoji picker (ADM-0 §1.3 红线 永久不挂)
- ❌ schema 改 / 新 endpoint / v=39 migration (反约束硬锁 0 server)
- ❌ skin tone modifier picker (留 v2)
