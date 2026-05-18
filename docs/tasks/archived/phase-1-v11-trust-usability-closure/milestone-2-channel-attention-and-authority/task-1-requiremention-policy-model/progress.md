# Progress

## Resume

| Field | Value |
|---|---|
| Worktree | `.worktrees/task-1-requiremention-policy-model` |
| Branch | `feat/task-1-requiremention-policy-model` |
| PR | not opened |
| Owner | Blueprintflow owner worker |
| State | ACCEPTING |
| Blocker | none |

## Checkpoints

- [x] Worktree/branch created from `origin/main` at `642fb57`.
- [x] `AGENTS.md` reviewed.
- [x] Task, milestone, phase plan, and blueprint anchors reviewed.
- [x] Dependency check completed: no hard dependency on M1 task 6 Helper job transport.
- [x] Four-piece baseline docs created: `spec.md`, `stance.md`, `acceptance.md`, `progress.md`.
- [x] Code-facing `design.md` drafted before implementation.
- [x] RED tests written and observed failing for missing behavior.
- [x] Implementation completed.
- [x] Current docs synced.
- [x] Verification evidence recorded.
- [ ] PR opened.
- [ ] CI passed and merge completed.

## Dependency Evidence

| Item | Evidence | Result |
|---|---|---|
| M2 task 1 start | Task contract depends on canonical Milestone 2 start or explicit Teamlead parallel start decision; user authorized this owner-worker task start | PASS |
| M1 task 6 independence | Task 6 owns Helper outbound poll/lease/result and current Helper docs; task 1 owns channel/message/mention policy, store/migration/API/tests | PASS |
| Existing Task7 merge | `origin/main` includes PR #942 local job policy evaluator at `642fb57`; no task7 files touched by this task | PASS |

## Implementation Evidence

Implemented in this task branch:

- Added migration v52 `channel_member_require_mention_policy` plus legacy baseline compatibility for `channel_members.require_mention_policy` with `inherit` / `on` / `off` values.
- Added store policy helpers for normalization, effective resolution, owner-ceiling rejection, and non-mention agent recipient selection.
- Added `PUT /api/v1/channels/{channelId}/members/{userId}/require-mention` with channel manager, same-org, same-channel, agent-target, DM rejection, invalid-policy, and owner-ceiling checks.
- Integrated message creation with implicit non-mention agent dispatch when effective policy resolves to off, without writing implicit `message_mentions` rows.
- Updated current docs for data model/migrations, realtime/message dispatch, security boundaries, and the remaining client-control gap.

## Verification Evidence

Commands run from `packages/server-go` with `GOTMPDIR=/workspace/borgee-gotmp-m2-task1` because the host `/tmp` mount rejects Go test binaries:

- `go test -tags fts5 ./internal/migrations -run 'TestChannelMemberRequireMentionPolicy|TestMigrationRegistryIncludesChannelMemberRequireMention'` -> PASS.
- `go test -tags fts5 ./internal/store -run 'TestRequireMentionPolicy|TestListChannelAgentsAllowedWithoutMention'` -> PASS.
- `go test -tags fts5 ./internal/api -run 'TestChannelRequireMentionPolicy'` -> PASS.
- `go test -tags fts5 ./internal/api ./internal/store ./internal/migrations` -> PASS.
- `go test -tags fts5 ./...` -> PASS.
- `git diff --check` -> PASS.

## Acceptance State

Local task implementation and verification are complete. Acceptance is ready for PR review and CI gate handling.
