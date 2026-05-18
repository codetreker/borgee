# Acceptance

## Checklist

- [x] Dependency preflight confirms Task 5 depends only on Task 4, which is merged as PR #948.
- [x] Task 2 PR #951 is not required for Task 5 and remains out of scope.
- [x] Settings channel management shows explicit read-only availability for leave/delete/archive/owner-transfer.
- [x] Self-created or owned channels do not expose leave as an available action.
- [x] Joined-only channels expose leave as available and keep delete/archive/owner-transfer unavailable.
- [x] General/default channel rules keep leave/delete/archive unavailable.
- [x] Settings does not render action mutation buttons or call leave/delete/archive APIs.
- [x] Active channel header uses the same leave rule, so owner-created channels do not show the existing leave button.
- [x] Current docs and task docs are updated.

## Verification Evidence

| Acceptance segment | Evidence | Result |
|---|---|---|
| Dependency decision | `task-5-channel-allowed-action-rules/task.md` lists only Task 4 as dependency; `origin/main` includes Task 4 merge `077cb8c6` | PASS |
| TDD RED | Focused client test run failed before implementation because `buildChannelAllowedActionRules` did not exist and row actions were absent | PASS |
| Allowed rules | `channel-management-api.test.ts` covers owner-created, joined-only, and general-channel rule outputs | PASS |
| Settings rendering | `ChannelManagementSurface.test.tsx` covers per-row `data-action` and `data-allowed` values and asserts no `button[data-action]` is rendered | PASS |
| Header leave rule | `ChannelView.tsx` imports `canLeaveChannel(...)` and gates the existing leave button through it | PASS |

## Remaining Work

- Task 6 must add server-authoritative enforcement and mutation wiring for any exposed action controls.
- Owner transfer remains unavailable unless a later task explicitly reopens it.
