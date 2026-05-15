# Progress

## Resume

| Field | Value |
|---|---|
| Worktree | `.worktrees/task-9-openclaw-install-agent-config-jobs` |
| Branch | `task-9-openclaw-install-agent-config-jobs` |
| PR | #956 |
| Owner | M1 Task9 owner worker |
| State | CI_GREEN |
| Blocker | none |

## Checkpoints

- [x] Worktree/branch created from `origin/main` at Task8 merge SHA `419c5bf57637941df5670f08615304e4a9ef8277`.
- [x] Task contract, milestone, dependencies, blueprint anchors, Task6/Task7/Task8 docs, and current host/security/server docs reviewed.
- [x] Focused baseline passed for server store/API/datalayer and helper jobpolicy/outbound after using executable `GOTMPDIR` under ignored worktree storage.
- [x] Four-piece task docs created: `spec.md`, `stance.md`, `acceptance.md`, `progress.md`, plus `design.md`.
- [x] `content-lock.md` N/A because Task9 adds no UI copy or DOM literals.
- [x] RED tests written and observed failing for Task9 behavior.
- [x] Implementation complete for server-owned OpenClaw install/config job binding and Helper policy alignment.
- [x] Docs/current synced.
- [x] Final verification complete.
- [x] PR opened: #956.
- [x] CI monitored: all PR #956 checks passed on commit `bb1178e` before the final CI evidence doc update.

## Evidence

| Item | Evidence | Result |
|---|---|---|
| RED: install enqueue | `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task9-server go test -tags sqlite_fts5 -count=1 ./internal/store -run 'TestHelperJobOpenClawInstallFromManifestIsServerBound|TestHelperJobEnqueueAuthorityAndActiveIdempotency'` failed before implementation: install enqueue returned `helper job: manifest required`, and configure-agent job lacked `manifest_binding_json`. | PASS |
| RED: install API lease | `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task9-server go test -tags sqlite_fts5 -count=1 ./internal/api -run 'TestHelperJobsEnqueueOpenClawInstallLeaseCarriesServerManifestBinding'` failed before implementation with `400 manifest_required`. | PASS |
| RED: helper policy config binding | `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task9-helper go test -count=1 ./internal/jobpolicy -run 'TestEvaluateAllowsMinimalConfigureAgentWhenEnvelopeAndEnrollmentMatch|TestEvaluateConfigureAgentRequiresSignedManifestAndApprovedConfigPath'` failed before implementation because the new effective config payload was `schema_invalid` and manifest/path denials were not reached. | PASS |
| GREEN: focused store | `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task9-server go test -tags sqlite_fts5 -count=1 ./internal/store -run 'TestHelperJobOpenClawInstallFromManifestIsServerBound|TestHelperJobEnqueueAuthorityAndActiveIdempotency'` -> `ok borgee-server/internal/store 0.049s`. | PASS |
| GREEN: focused API | `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task9-server go test -tags sqlite_fts5 -count=1 ./internal/api -run 'TestHelperJobsEnqueueOpenClawInstallLeaseCarriesServerManifestBinding|TestHelperJobsEnqueueRejectsUnauthorizedRailsAndInvalidEnvelopes'` -> `ok borgee-server/internal/api 0.585s`. | PASS |
| GREEN: focused helper policy | `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task9-helper go test -count=1 ./internal/jobpolicy -run 'TestEvaluateAllowsMinimalConfigureAgentWhenEnvelopeAndEnrollmentMatch|TestEvaluateConfigureAgentRequiresSignedManifestAndApprovedConfigPath|TestEvaluateInstallManifestRequiresSignedManifestArtifactAndBinding'` -> `ok borgee-helper/internal/jobpolicy 0.007s`. | PASS |
| Focused server verification | `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task9-server go test -tags sqlite_fts5 -count=1 ./internal/store ./internal/api ./internal/datalayer` -> `ok` for store, api, and datalayer. | PASS |
| Focused helper verification | `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task9-helper go test -count=1 ./internal/jobpolicy ./internal/outbound` -> `ok` for jobpolicy and outbound. | PASS |
| Broad server verification | `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task9-server go test -tags sqlite_fts5 -count=1 ./...` from `packages/server-go` -> all packages `ok` / no test files. | PASS |
| Broad helper verification | `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task9-helper go test -count=1 ./...` from `packages/borgee-helper` -> all packages `ok` / no test files. | PASS |
| Whitespace | `git diff --check` -> no output, exit 0. | PASS |
| PR CI | PR #956 checks passed on commit `bb1178e`: PR lint, bpp-envelope-lint, check, client-vitest, e2e, go-test-cov, go-test-race, go-test-race-heavy, and hb20-ipc-prereq on macOS/Ubuntu/Windows. | PASS |

## Implementation Summary

- Enabled `openclaw.install_from_manifest` for `openclaw_lifecycle` delegation with server-derived install payload and server-owned manifest/artifact/path/domain binding.
- Updated `openclaw.configure_agent` to store server-owned manifest/path binding while preserving target agent config and optional channel access checks.
- Exposed manifest binding only on Helper lease projection, not user enqueue responses.
- Expanded forbidden payload preflight for client-supplied manifest, artifact, path, domain, service, install plan, config hash, command, credential, TTL, and expiry authority.
- Updated Helper local policy so configure-agent requires signed manifest plus approved config path binding before allow.
