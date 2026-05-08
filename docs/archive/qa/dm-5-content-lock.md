# DM-5 Content-Lock — DOM 锚 + 文案 byte-identical

> spec `dm-5-spec.md` 立场 ④.

## 1. DOM 锚 (反向 grep ≥1 hit)

| # | 锚 | 字面 | 用途 | 反向 grep |
|---|---|---|---|---|
| ① | reaction chip | `data-dm5-reaction-chip="<emoji>"` | chip 选择器 + e2e 验证 | `git grep -n 'data-dm5-reaction-chip' packages/client/src/` count≥1 |
| ② | reaction count | `data-dm5-reaction-count="<N>"` | count 锚 (N 是数字) | `git grep -n 'data-dm5-reaction-count' packages/client/src/` count≥1 |
| ③ | mine highlight | `data-dm5-reaction-mine` | current user reacted 锚 (boolean attr, 仅 user 在 user_ids 内时渲染) | `git grep -n 'data-dm5-reaction-mine' packages/client/src/` count≥1 |

## 2. 文案 byte-identical

| # | 文案 | 触发 |
|---|---|---|
| ① | `{emoji} {count}` (e.g. "👍 3") | chip button label, 空格分隔 |

## 3. 反约束 (CI grep 0 hit)

| # | 反约束 | 反向 grep |
|---|---|---|
| ① | 不另起 emoji picker (复用现有 unicode 直接发) | `git grep -nE 'dm5.*emoji.*picker\|DM5EmojiPicker' packages/client/src/` count==0 |
| ② | 不另起 server-side aggregator (复用 CV-7 GET) | `git grep -nE 'dm5.*aggregator\|reaction_summary.*PRIMARY' packages/server-go/internal/` count==0 |
| ③ | admin god-mode UI 不挂 | `git grep -nE 'admin.*ReactionSummary\|admin.*dm.*reaction' packages/client/src/` count==0 |
| ④ | 5-pattern 不漂 | `git grep -nE 'dm5.*thinking\|dm5.*subject' packages/client/src/` count==0 |
