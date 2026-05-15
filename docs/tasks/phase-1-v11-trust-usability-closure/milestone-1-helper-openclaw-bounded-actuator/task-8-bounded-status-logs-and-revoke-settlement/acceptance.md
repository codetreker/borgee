# Acceptance: Bounded Status Logs And Revoke Settlement

## Source Alignment

- Task: `task-8-bounded-status-logs-and-revoke-settlement`
- Milestone: `phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator`
- Blueprint anchors: `remote-actuator-design.md` sections 1.2, 10, and 11; `migration-analysis.md` section 6.1
- Dependencies: Task6 PR #943 (`c2c61e6e8500218ae0e841a9edde3f1187c78c7d`) and Task7 PR #942 (`642fb5761b141a633169f39e31f77931bf85f0c1`)

## Segment A: Deterministic Terminal State

Acceptance checks:

- Revoked, stale, denied, expired, failed, cancelled, and lease-lost work reaches a terminal status with a deterministic failure reason.
- Terminal replay is idempotent only when status, reason, message, and result references match exactly.
- Conflicting terminal replay cannot overwrite failed/cancelled/expired work as success.

## Segment B: Revoke/Uninstall/Stale Precedence

Acceptance checks:

- Revocation settles queued, leased, and running jobs as cancelled with reason `revoked`.
- Uninstall settles queued, leased, and running jobs as cancelled with reason `uninstalled`.
- Stale credential observation settles active leased/running jobs as cancelled with reason `stale_credential` while preserving Task6 current-credential polling semantics.

## Segment C: Bounded Result Metadata

Acceptance checks:

- Result upload accepts only closed terminal status, closed failure code, bounded redacted failure message, and bounded opaque `audit_refs`/`log_refs`.
- Unknown result fields, raw log fields, token/credential fields, private-content fields, command/script/path/URL/service-unit fields, oversized messages, and too many refs are rejected.
- Stored failure messages are redacted before persistence and response serialization.

## Segment D: API Visibility Without Sensitive Leakage

Acceptance checks:

- API responses for terminal jobs include terminal status, failure code, redacted failure message when present, and normalized `result_summary` references when present.
- API responses do not expose owner/org internals, helper credentials, credential digests, payload hashes, raw stored JSON fields, raw logs, private content, or Remote Agent data.

## Segment E: Current-Doc Sync And Evidence

Acceptance checks:

- `docs/current` describes the implemented terminal settlement and bounded redacted result-reference boundary.
- `progress.md` records RED/GREEN verification, current-doc sync, and any residual verification blocker.
- No content-lock file is required because Task8 adds no UI copy or DOM literals.

## Evidence

| Segment | Evidence | Result |
|---|---|---|
| A: Deterministic Terminal State | Store/API tests cover non-success reason requirement, idempotent matching terminal replay, conflicting terminal replay, TTL/lease-lost/revoke/stale/uninstall settlement carried from Task6 tests, and raw-log rejection before terminal settlement. | PASS |
| B: Revoke/Uninstall/Stale Precedence | Existing store tests continue to pass for revoked queued settlement, ack/result after revoke/uninstall, stale credential active-job settlement, TTL expiry, and lease-lost expiry under `go test -tags sqlite_fts5 ./internal/store`. | PASS |
| C: Bounded Result Metadata | New store/API tests prove reasonless non-success terminal results are rejected, sensitive failure messages are redacted, and `raw_logs` inside `result_summary` is rejected with `forbidden_field`. | PASS |
| D: API Visibility Without Sensitive Leakage | New API tests prove terminal responses include bounded `failure_message` and normalized `result_summary` references while rejecting raw `result_summary_json`; existing serializer assertions continue to reject owner/org/credential/payload-hash leaks. | PASS |
| E: Current-Doc Sync And Evidence | Updated `docs/current/host-bridge/README.md`, `docs/current/host-bridge/helper-daemon.md`, `docs/current/security/README.md`, `docs/current/known-gaps.md`, plus task progress evidence. | PASS |

Verification commands:

- `GOTMPDIR=$PWD/.gotmp go test -tags sqlite_fts5 -count=1 ./internal/store ./internal/api ./internal/datalayer` from `packages/server-go`.
- `GOTMPDIR=/workspace/borgee/.worktrees/task-8-bounded-status-logs-and-revoke-settlement/.tmp/go-build go test -tags sqlite_fts5 -count=1 ./...` from `packages/server-go`.
- `GOTMPDIR=$PWD/.gotmp go test -count=1 ./internal/outbound ./internal/jobpolicy` from `packages/borgee-helper`.
- `GOTMPDIR=$PWD/.gotmp go test -count=1 ./...` from `packages/borgee-helper`.
- `git diff --check` from repo root.
