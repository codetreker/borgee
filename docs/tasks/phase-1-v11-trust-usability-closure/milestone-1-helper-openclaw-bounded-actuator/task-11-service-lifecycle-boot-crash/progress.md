# Progress

## Resume

| Field | Value |
|---|---|
| Worktree | `.worktrees/m1-task11-service-lifecycle-boot-crash` |
| Branch | `task/m1-task11-service-lifecycle-boot-crash` |
| PR | pending |
| Owner | M1 Task11 owner worker |
| State | LOCAL_VERIFIED |
| Blocker | none |

## Dependency Decision

Task11 is unblocked. Its declared dependency is `task-8-bounded-status-logs-and-revoke-settlement`, and the required M1 chain is merged into `origin/main`: Task6 PR #943 (`c2c61e6e8500218ae0e841a9edde3f1187c78c7d`), Task7 PR #942 (`642fb5761b141a633169f39e31f77931bf85f0c1`), Task8 PR #954 (`419c5bf57637941df5670f08615304e4a9ef8277`), Task9 PR #956 (`5575b53f657276c57ba319b144281286865db630`), and Task10 PR #958 (`ad50575e080fa4a56cdedc37d0e65823d768b3bc`). M2 Task6 PR #959 is merged into `origin/main` and is not a Task11 dependency because Task11 touches Helper/OpenClaw lifecycle service authority, not channel authority work.

## Checkpoints

- [x] Worktree/branch created from `origin/main` at `66c9a35493ef0dfa7a5aab39c65f5988d5c3210e`.
- [x] Task contract, milestone, blueprint anchors, Task8-10 docs, M2 Task6 PR state, current host/security/server docs, Helper policy, Helper install assets, and Helper job enqueue code reviewed.
- [x] Dependency decision recorded: no M2 Task6 blocker.
- [x] Baseline helper package passed after using executable `GOTMPDIR`; default `/tmp` is mounted `noexec` in this environment.
- [x] Four-piece task docs created: `spec.md`, `stance.md`, `acceptance.md`, `design.md`, plus this `progress.md`.
- [x] RED tests written and observed failing for Task11 behavior.
- [x] Implementation complete for bounded service assets, service lifecycle enqueue, and Helper policy service ID validation.
- [x] Focused GREEN tests passed.
- [x] Docs/current synced.
- [x] Broader local verification complete.
- [ ] PR opened and CI monitored.

## Evidence

| Item | Evidence | Result |
|---|---|---|
| Baseline helper default tmp | `go test ./...` from `packages/borgee-helper` failed before implementation with `fork/exec /tmp/go-build... permission denied`; `/tmp` is `noexec`. | ENVIRONMENT |
| Baseline helper executable tmp | `GOTMPDIR=$PWD/.tmp/go-build go test ./...` from `packages/borgee-helper` passed before implementation. | PASS |
| RED: helper assets/policy | `GOTMPDIR=$PWD/.gotmp go test ./install ./internal/jobpolicy` failed before implementation because Linux lacked `RestartSec=10s`, macOS lacked `ThrottleInterval=10`, and policy allowed invalid service declarations. | PASS |
| RED: server store | `GOTMPDIR=$PWD/.gotmp go test -tags sqlite_fts5 ./internal/store -run 'TestHelperJobServiceLifecycleIsServerBoundToDeclaredServiceID' -count=1` failed before implementation with `helper job: type not enabled`. | PASS |
| RED: server API | `GOTMPDIR=$PWD/.gotmp go test -tags sqlite_fts5 ./internal/api -run 'TestHelperJobsEnqueueServiceLifecycleLeaseCarriesDeclaredServiceID' -count=1` failed before implementation with `job_type_not_enabled`. | PASS |
| GREEN: helper focus | `GOTMPDIR=$PWD/.gotmp go test ./install ./internal/jobpolicy` from `packages/borgee-helper` passed after implementation. | PASS |
| GREEN: server store focus | `GOTMPDIR=$PWD/.gotmp go test -tags sqlite_fts5 ./internal/store -run 'TestHelperJob(ServiceLifecycleIsServerBoundToDeclaredServiceID|EnqueueRejectsInactiveDelegationAndClosedTaxonomy)' -count=1` passed after implementation. | PASS |
| GREEN: server API focus | `GOTMPDIR=$PWD/.gotmp go test -tags sqlite_fts5 ./internal/api -run 'TestHelperJobs(EnqueueServiceLifecycleLeaseCarriesDeclaredServiceID|EnqueueRejectsForbiddenClientAuthorityAndLaterScopeRoutes)' -count=1` passed after implementation. | PASS |
| BROAD: helper | `GOTMPDIR=/workspace/borgee/.worktrees/m1-task11-service-lifecycle-boot-crash/.gotmp/borgee-helper go test ./...` from `packages/borgee-helper` passed. | PASS |
| BROAD: server | `GOTMPDIR=/workspace/borgee/.worktrees/m1-task11-service-lifecycle-boot-crash/.gotmp/server-go go test -tags sqlite_fts5 ./...` from `packages/server-go` passed. The executable temp dir is outside `packages/server-go` so tree-walking tests do not inspect transient Go build directories. | PASS |
| DIFF CHECK | `git diff --check` passed. | PASS |

## Implementation Summary

- Added bounded Linux systemd restart/backoff settings for the installed non-root Helper service.
- Added bounded macOS launchd `ThrottleInterval` while preserving `RunAtLoad`, failure-only `KeepAlive`, sandbox wrapper, and `_borgee-helper` user/group.
- Enabled `service.lifecycle` as a manifest-required `openclaw_lifecycle` Helper job with server-owned `openclaw-user` service binding.
- Added strict server payload handling so clients can request only `target=openclaw` plus `operation=restart`; the effective leased payload contains only `operation=restart`.
- Hardened Helper local-policy service validation for logical IDs, duplicate IDs, sandbox/profile affordance, platform manager compatibility, and safe unit/label shape.
