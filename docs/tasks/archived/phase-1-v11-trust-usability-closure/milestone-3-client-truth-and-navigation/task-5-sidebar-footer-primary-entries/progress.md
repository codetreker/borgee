# Progress

## Resume

| Field | Value |
|---|---|
| Worktree | `.worktrees/task-5-sidebar-footer-primary-entries` |
| Branch | `feat/task-5-sidebar-footer-primary-entries` |
| PR | #947 |
| Owner | Dev worker |
| State | ACCEPTING |
| Blocker | none |

## Checkpoints

- [x] Worktree/branch created from `origin/main`
- [x] Required task, milestone, and blueprint anchors read
- [x] Existing Sidebar/footer implementation inspected
- [x] Baseline focused Sidebar test run recorded
- [x] Four-piece baseline created
- [x] Implementation design created
- [x] TDD RED tests written and verified failing
- [x] Production implementation complete
- [x] Current-doc sync completed
- [x] Acceptance evidence recorded
- [x] PR opened
- [x] CI failure triaged and fix pushed

## Evidence Log

| Item | Evidence | Result |
|---|---|---|
| Worktree | `git worktree add .worktrees/task-5-sidebar-footer-primary-entries -b feat/task-5-sidebar-footer-primary-entries origin/main` | PASS |
| Branch | `git branch --show-current` -> `feat/task-5-sidebar-footer-primary-entries` | PASS |
| Baseline setup note | `pnpm --filter @borgee/client test -- Sidebar-dm-agent-presence.test.tsx` installed dependencies but pnpm 11 stopped on ignored-builds approval before Vitest | INFO |
| Baseline focused test | From `packages/client`: `../../node_modules/.bin/vitest run src/__tests__/Sidebar-dm-agent-presence.test.tsx --config vitest.config.ts` -> 1 file passed, 2 tests passed | PASS |
| TDD RED footer contract | From `packages/client`: `../../node_modules/.bin/vitest run src/__tests__/Sidebar-footer-primary.test.tsx --config vitest.config.ts` failed as expected: missing `sidebar-footer-primary-actions` and secondary toggle/menu selectors before production code existed | PASS |
| GREEN focused footer tests | From `packages/client`: `../../node_modules/.bin/vitest run src/__tests__/Sidebar-footer-primary.test.tsx --config vitest.config.ts` -> 1 file passed, 4 tests passed | PASS |
| GREEN adjacent app-shell tests | From `packages/client`: `../../node_modules/.bin/vitest run src/__tests__/Sidebar-footer-primary.test.tsx src/__tests__/Sidebar-dm-agent-presence.test.tsx src/__tests__/main-view.test.ts --config vitest.config.ts` -> 3 files passed, 22 tests passed | PASS |
| Client typecheck | From `packages/client`: `../../node_modules/.bin/tsc --noEmit` -> exit 0 | PASS |
| Client build | From `packages/client`: `./node_modules/.bin/vite build` -> build completed; existing chunk-size warning only | PASS |
| Full client test suite | From `packages/client`: `./node_modules/.bin/vitest run --config vitest.config.ts` -> 130 files passed, 829 tests passed, 1 skipped | PASS |
| Diff hygiene | `git diff --check` -> exit 0 with no output | PASS |
| Scope overlap check | Changed production files are limited to `packages/client/src/components/Sidebar.tsx` and `packages/client/src/index.css`; no ArtifactComments, Settings PermissionsView, NodeManager, or HelperStatusPanel production files edited | PASS |
| Current-doc sync | Updated `docs/current/client/ui-map.md`, `docs/current/client/app-shell-state.md`, `docs/current/client/ui/main-desktop.md`, and `docs/current/client/ui/sidepane.md` for primary footer and secondary overflow behavior | PASS |
| PR opened | PR #947: `feat(client): simplify sidebar footer entries` | PASS |
| CI e2e triage | PR #947 e2e failed because old invitation tests expected a primary `button.invitations-btn`; task intentionally moved Invitations behind More | FIXED |
| Focused e2e after CI fix | From `packages/e2e`: `CI=1 GOTMPDIR="$PWD/.playwright-data/gotmp" ./node_modules/.bin/playwright test tests/chat-name-display-regression.spec.ts tests/chat-realtime-message-fanout.spec.ts --workers=1` -> 2 passed | PASS |
| CI-fix focused app-shell tests | From `packages/client`: `../../node_modules/.bin/vitest run src/__tests__/Sidebar-footer-primary.test.tsx src/__tests__/Sidebar-dm-agent-presence.test.tsx src/__tests__/main-view.test.ts --config vitest.config.ts` -> 3 files passed, 22 tests passed | PASS |
| CI-fix client typecheck | From `packages/client`: `../../node_modules/.bin/tsc --noEmit` -> exit 0 | PASS |
| CI-fix client build | From `packages/client`: `./node_modules/.bin/vite build` -> build completed; existing chunk-size warning only | PASS |
| CI-fix full client test suite | From `packages/client`: `./node_modules/.bin/vitest run --config vitest.config.ts` -> 130 files passed, 829 tests passed, 1 skipped | PASS |
| CI-fix diff hygiene | `git diff --check` -> exit 0 with no output | PASS |

## Current-Doc Sync Targets

| File | Handling |
|---|---|
| `docs/current/client/ui-map.md` | Updated for primary footer and secondary overflow actions |
| `docs/current/client/app-shell-state.md` | Updated view-model notes for Helper status and footer primary/overflow navigation |
| `docs/current/client/ui/main-desktop.md` | Updated desktop shell sketch/footer description |
| `docs/current/client/ui/sidepane.md` | Updated sidepane navigation sketch to distinguish primary row from secondary overflow |
| `docs/current/client/feature-surfaces.md` | Checked after implementation; no edit needed because feature-surface data boundaries did not change |

## Acceptance Evidence

| Segment | Evidence | Result |
|---|---|---|
| Segment A - Primary footer set | `Sidebar-footer-primary.test.tsx` verifies avatar, Agents, Workspaces, Settings, and More are the only primary footer actions for member sessions; pending invitation count decorates More without restoring a primary Invitations action | PASS |
| Segment B - Secondary reachability | `Sidebar-footer-primary.test.tsx` verifies Invitations, Remote Nodes, Helper Status, and Logout remain reachable through More; logout still calls `logout()` and `onLogout` | PASS |
| Segment C - Agent-session boundary | `Sidebar-footer-primary.test.tsx` verifies agent sessions keep Workspaces and Logout but do not gain owner-only Agents, Invitations, Remote Nodes, Helper Status, or Settings | PASS |
| Segment D - Tests and verification | TDD RED and GREEN runs plus adjacent Sidebar/app-shell tests and typecheck recorded above | PASS |
| Segment E - Current-doc sync | Current docs updated for implemented shell behavior without claiming Task 6 account panel or Task 7 final Helper/Remote Nodes placement | PASS |

Verifier: Dev worker
Date: 2026-05-15
Scope: UI/app shell/current-doc sync
Fixtures: Vitest mocked Sidebar app context; no real tenant data
Out-of-scope findings: N/A
Decision: LGTM
