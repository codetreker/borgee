# Acceptance

## Checklist

- [x] Dependency on M3 Task5 is satisfied on `origin/main` before implementation starts.
- [x] Settings exposes a Runtime tab with separate Remote Nodes and Helper Status launch entries.
- [x] Remote Nodes and Helper Status are no longer reachable from the sidebar footer overflow.
- [x] Invitations remain reachable from the footer overflow and keep the More badge behavior.
- [x] Runtime entry tests verify separate Remote Agent and Helper actuator rail metadata.
- [x] No `NodeManager`, `HelperStatusPanel`, Remote Agent API, Helper API, credential, grant, or enforcement code is changed.
- [x] Current client docs describe the new placement and rail separation.

## Evidence

| Segment | Evidence | Result |
|---|---|---|
| Dependency | `gh pr view 947` reports MERGED with merge commit `47dc6805abaf98fffcd727ec5917b641367f2eeb`; `git merge-base --is-ancestor 47dc6805abaf98fffcd727ec5917b641367f2eeb HEAD` passed | PASS |
| TDD RED | Focused jsdom run failed before production changes: missing Settings `[data-tab="runtime"]` and sidebar still rendered `sidebar-secondary-remote-nodes` | PASS |
| Settings placement | `SettingsPage.test.tsx` verifies Runtime tab entries for Remote Nodes and Helper Status, separate `data-authority-rail` values, and separate callbacks | PASS |
| Footer move | `Sidebar-footer-primary.test.tsx` verifies sidebar More exposes Invitations only, with no Remote Nodes, Helper Status, or Logout entries | PASS |
| Rail separation scope | Diff review changes only Settings/App/Sidebar/CSS/tests/docs; no `NodeManager`, `HelperStatusPanel`, Remote Agent API, Helper API, credentials, grants, or enforcement files changed | PASS |
| Focused client verification | Post-rebase `pnpm --filter @borgee/client test:jsdom -- src/__tests__/SettingsPage.test.tsx src/__tests__/Sidebar-footer-primary.test.tsx` -> 105 files passed, 666 tests passed, 1 skipped; `pnpm --filter @borgee/client typecheck` -> exit 0 | PASS |
| Full client verification | Pre-rebase `pnpm --filter @borgee/client test:jsdom` -> 104 files passed, 662 tests passed, 1 skipped; `pnpm --filter @borgee/client test:node` -> 30 files passed, 192 tests passed. Post-rebase `pnpm --filter @borgee/client test` -> 135 files passed, 858 tests passed, 1 skipped; `pnpm --filter @borgee/client build` -> exit 0 with existing Vite large-chunk warning | PASS |
| Diff hygiene | `git diff --check` -> exit 0 | PASS |
| Repo lint command | `pnpm lint` cannot run because ESLint 9 reports no `eslint.config.(js\|mjs\|cjs)` in the repository | BLOCKED - repo tooling |

Verifier: Task7 rescue owner worker
Date: 2026-05-15
