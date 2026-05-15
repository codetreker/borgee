# Progress

## Resume

| Field | Value |
|---|---|
| Worktree | `.worktrees/m1-task10-helper-openclaw-bounded-actuator` |
| Branch | `m1-task10-helper-openclaw-bounded-actuator` |
| PR | pending |
| Owner | M1 Task10 owner worker |
| State | LOCAL_VERIFIED |
| Blocker | none |

## Dependency Decision

Task10 is unblocked. Task9 PR #956 is merged at `5575b53f657276c57ba319b144281286865db630`. M2 Task6 is not a prerequisite because Task10 does not expose channel management actions; it uses existing channel access checks to authorize typed plugin binding jobs.

## Checkpoints

- [x] Worktree/branch created from `origin/main` at `16e2db6ad0dd172047c6b886333ac44a40a59c7c`.
- [x] Task contract, milestone, Task9 docs, M2 Task6 docs, current host/security/server docs, and Helper job code reviewed.
- [x] Dependency decision recorded: no M2 Task6 blocker.
- [x] Focused baseline passed for server store/API/datalayer and helper jobpolicy/outbound after using executable `GOTMPDIR` under ignored worktree storage.
- [x] Four-piece task docs created: `spec.md`, `stance.md`, `acceptance.md`, `progress.md`, plus `design.md`.
- [x] RED tests written and observed failing for Task10 behavior.
- [x] Implementation complete for server-owned Borgee plugin connection/channel binding jobs and Helper policy alignment.
- [x] Docs/current synced.
- [x] Focused and broader local verification complete.
- [ ] PR opened.
- [ ] CI monitored.

## Evidence

| Item | Evidence | Result |
|---|---|---|
| Baseline server | `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task10-server go test -tags sqlite_fts5 -count=1 ./internal/store ./internal/api ./internal/datalayer` passed before Task10 implementation. | PASS |
| Baseline helper | `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task10-helper go test -count=1 ./internal/jobpolicy ./internal/outbound` passed before Task10 implementation. | PASS |
| RED: store plugin job | `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task10-server go test -tags sqlite_fts5 -count=1 ./internal/store -run 'TestHelperJobPluginConfigureConnectionIsServerBound|TestHelperJobEnqueueRejectsInactiveDelegationAndClosedTaxonomy'` failed before implementation because `borgee_plugin.configure_connection` returned `helper job: type not enabled`. | PASS |
| RED: API plugin job | `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task10-server go test -tags sqlite_fts5 -count=1 ./internal/api -run 'TestHelperJobsEnqueuePluginConfigureConnectionRequiresChannelAuthority|TestHelperJobsEnqueueRejectsUnauthorizedRailsAndInvalidEnvelopes'` failed before implementation with `job_type_not_enabled`. | PASS |
| RED: helper policy | `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task10-helper go test -count=1 ./internal/jobpolicy -run 'TestEvaluateAllowsPluginConfigureConnectionWithServerBoundChannelPayload'` failed before implementation with `schema_invalid`. | PASS |
| GREEN: focused store | Same store command passed after implementation. | PASS |
| GREEN: focused API | Same API command passed after implementation. | PASS |
| GREEN: focused helper policy | Same helper policy command passed after implementation. | PASS |
| Broader server verification | `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task10-server go test -tags sqlite_fts5 -count=1 ./internal/store ./internal/api ./internal/datalayer` -> `ok` for store, api, and datalayer. | PASS |
| Broader helper verification | `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task10-helper go test -count=1 ./internal/jobpolicy ./internal/outbound` -> `ok` for jobpolicy and outbound. | PASS |
| Full server verification | `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task10-server go test -tags sqlite_fts5 -count=1 ./...` from `packages/server-go` -> all packages `ok` / no test files. | PASS |
| Full helper verification | `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task10-helper go test -count=1 ./...` from `packages/borgee-helper` -> all packages `ok` / no test files. | PASS |
| Whitespace | `git diff --check` -> no output, exit 0. | PASS |

## Implementation Summary

- Enabled `borgee_plugin.configure_connection` as an `openclaw_config` manifest-required Helper job.
- Added server-derived effective payload with deterministic `borgee-plugin:` connection id, target agent id, and target channel id.
- Added channel binding authority checks requiring same org, ordinary channel type, owner access, and target-agent access.
- Bound plugin jobs to the signed runtime manifest plus approved `borgee_plugin_config` path id.
- Expanded forbidden payload preflight for client-supplied connection, base URL, API-key, account, credential, path, domain, manifest, service, command, and TTL authority.
- Updated Helper local policy to validate the server-bound plugin payload schema before manifest/path policy allows.
