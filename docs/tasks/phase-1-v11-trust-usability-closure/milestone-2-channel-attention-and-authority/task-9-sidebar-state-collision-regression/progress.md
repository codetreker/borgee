# Progress

## Resume

| Field | Value |
|---|---|
| Worktree | `.worktrees/m2-task9` |
| Branch | `task/m2-task9` |
| PR | #961 |
| Owner | M2 Task9 owner worker |
| State | ACCEPTING |
| Blocker | none |

## Checkpoints

- [x] Fetched `origin/main` before branching.
- [x] Created isolated worktree/branch from current `origin/main`.
- [x] Dependency checked: Task9 depends on Task8 only; Task8 PR #952 is present on `origin/main`.
- [x] Installed workspace dependencies because `node_modules` was absent in the worktree.
- [x] Baseline jsdom suite passed before Task9 changes.
- [x] Wrote RED regression/evidence test before docs/current sync.
- [x] Added Task9 spec, design, stance, regression, acceptance, and progress docs.
- [x] Focused GREEN regression passed.
- [x] Final verification passed.
- [x] PR opened (#961).
- [ ] CI passed.
- [ ] PR merged and worktree cleaned up.

## Evidence

| Command | Result |
|---|---|
| `pnpm install` | PASS; lockfile unchanged, ignored-build-scripts warning only |
| `pnpm --filter @borgee/client test:jsdom -- packages/client/src/__tests__/SortableChannelItem-private-indicator.test.tsx` | Baseline PASS; 104 files, 661 tests passed, 1 skipped |
| `pnpm --filter @borgee/client test:jsdom -- src/__tests__/Sidebar-state-collision-regression.test.tsx` | RED PASS; failed on missing Task9 `acceptance.md` |
| `pnpm --filter @borgee/client exec vitest run --project=jsdom --reporter=dot src/__tests__/Sidebar-state-collision-regression.test.tsx` | GREEN PASS; 1 file, 4 tests |
| `pnpm --filter @borgee/client test` | PASS; 135 files, 857 tests passed, 1 skipped |
| `pnpm --filter @borgee/client typecheck` | PASS |
| `pnpm --filter @borgee/client build` | PASS with existing Vite large-chunk warning |
| `git diff --check` | PASS |
| `pnpm lint` | BLOCKED by repo-level missing ESLint v9 flat config (`eslint.config.*`) |

## Scope Locks

- In scope: sidebar collision regression coverage, source-level channel/DM boundary proof, Task9 docs, and current-doc evidence.
- Out of scope: production UI movement, CSS redesign, server/API changes, ACL/membership/fanout/management changes, channel-level presence/fault model, sidebar footer/account/Helper/Remote Nodes IA, and broad e2e expansion.
