# Acceptance Template — DM-5: DM message reaction summary ✅ closed

> 跟 CV-9/10/11/12 client-only 同模式 + CV-7 reaction endpoint 复用. **0 server code + 0 schema 改 + 0 新 endpoint + 0 新 lib**.

## 验收清单

### §1 DM-5.1 — server 0 code + 1 unit

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 0 server code | git diff | `git diff main..HEAD -- 'packages/server-go/**/*.go' ':!**/*_test.go'` 0 行 |
| 1.2 既有 GET reactions 在 DM channel 工作 byte-identical | unit | `internal/api/dm_5_reaction_summary_test.go::TestDM5_ReactionSummaryInDMChannel` PASS |

### §2 DM-5.2 — client ReactionChip + ReactionSummary

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 立场 ④ chip DOM `data-dm5-reaction-chip="<emoji>"` 锚 | vitest | `ReactionSummary.test.tsx::chip_dom_anchor` |
| 2.2 立场 ④ count DOM `data-dm5-reaction-count="<N>"` 锚 + 文案 `{emoji} {count}` byte-identical | vitest | `ReactionSummary.test.tsx::count_anchor + literal` |
| 2.3 立场 ④ current user reacted → `data-dm5-reaction-mine` highlight | vitest | `ReactionSummary.test.tsx::mine_highlight` |
| 2.4 chip click toggle — mine: DELETE; not-mine: PUT | vitest | `ReactionSummary.test.tsx::click_toggle` |
| 2.5 empty reactions — 0 chip 渲染 | vitest | `ReactionSummary.test.tsx::empty_state` |

### §3 DM-5.3 — e2e + closure

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 e2e: 2 users react same emoji → count==2 | E2E | `dm-5-reaction-summary.spec.ts::two users count` |
| 3.2 e2e: same user PUT idempotent — 重复 PUT 不增 count | E2E | `dm-5-reaction-summary.spec.ts::idempotent` |
| 3.3 e2e: cross-channel non-member 不能 react (fail-closed) | E2E | `dm-5-reaction-summary.spec.ts::cross-channel reject` |
| 3.4 反向 grep 5 锚 4 处 0 hit + DOM 3 锚 ≥1 hit | CI grep | content-lock §3 |
| 3.5 REG-DM5-001..005 5 行 🟢 | regression-registry.md | 5 行 |

## 边界

- CV-7 #535 (reaction endpoint + addReaction client wrapper 既有) / store/queries_phase2b.go (AggregatedReaction shape 既有) / messages.go (channel-member ACL 既有) / ADM-0 §1.3 admin rail 红线

## 退出条件

- §1+§2+§3 全绿
- 0 server code + 0 schema 改
- REG-DM5-001..005 5 行
