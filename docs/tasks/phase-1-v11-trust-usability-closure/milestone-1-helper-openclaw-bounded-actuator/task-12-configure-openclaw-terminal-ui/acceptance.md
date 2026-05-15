# Acceptance: Configure OpenClaw Terminal UI

## Source Alignment

- Task: `task-12-configure-openclaw-terminal-ui`
- Milestone: `phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator`
- Dependencies: Task9 PR #956 (`5575b53`), Task10 PR #958 (`ad50575`), Task11 PR #963 (`d8d179e`)

## Segment A: Server Projection

Acceptance checks:

- Helper enrollment list/detail responses can include `configure_openclaw` for authenticated owner/org rows.
- Projection is derived from Helper job metadata for the required Configure OpenClaw closure job types.
- Success appears only after all required latest closure job types succeeded.
- Revoked or uninstalled Helper enrollment state overrides job-derived state.

## Segment B: Terminal State Truthfulness

Acceptance checks:

- Queued, running, succeeded, failed, denied, revoked, and manual-debug states are distinguishable.
- Denial and failure states expose bounded reason fields and bounded audit/log refs only.
- Expired/cancelled/incomplete terminal chains require manual debug instead of false success.

## Segment C: Redaction And Rail Separation

Acceptance checks:

- Public responses and client state do not expose raw payloads, payload hashes, manifest digests, result-summary JSON, raw logs, credentials, owner/org internals, commands, paths, domains, service units, or Remote Agent rail data.
- Helper Status UI does not call Helper credential endpoints or Remote Node fallback paths while loading Configure OpenClaw state.

## Segment D: Docs And Phase State

Acceptance checks:

- Task12 docs record scope, design, progress, and acceptance evidence.
- M1 marks Task10 and Task11 accepted, Task12 as the closure task, and M2/M3 as accepted without opening status-only PRs.

## Evidence

| Segment | Evidence | Result |
|---|---|---|
| A: Server Projection | Focused Go API tests cover list/detail projection derivation, no false success before all closure jobs succeed, terminal success, and safe serialization. | PASS |
| B: Terminal State Truthfulness | Focused Go and client tests cover queued/running/succeeded/failed/denied/revoked/manual-debug states. | PASS |
| C: Redaction And Rail Separation | Server/client tests cover no raw payload/log exposure, bounded refs, and no Helper credential or Remote Node fallback calls. | PASS |
| D: Docs And Phase State | Task docs plus phase/milestone state sync included in this PR. | PASS |

Verification commands:

- `GOTMPDIR=/workspace/borgee/.worktrees/m1-task12-configure-openclaw-terminal-ui/.gotmp/server-go go test -tags sqlite_fts5 -count=1 ./internal/api -run 'TestHelperEnrollmentsConfigureOpenClawProjection|TestHelperJobsSerializeLeaseAndJobOptionalFields'` from `packages/server-go`.
- `GOTMPDIR=/workspace/borgee/.worktrees/m1-task12-configure-openclaw-terminal-ui/.gotmp/server-go go test -tags sqlite_fts5 -count=1 ./...` from `packages/server-go`.
- `GOTMPDIR=/workspace/borgee/.worktrees/m1-task12-configure-openclaw-terminal-ui/.gotmp/borgee-helper go test -count=1 ./...` from `packages/borgee-helper`.
- `pnpm --filter @borgee/client exec vitest run --reporter=dot src/__tests__/HelperStatusPanel.test.tsx src/__tests__/helper-enrollments-api.test.ts`.
- `pnpm --filter @borgee/client test`.
- `pnpm --filter @borgee/client typecheck`.
- `git diff --check`.
