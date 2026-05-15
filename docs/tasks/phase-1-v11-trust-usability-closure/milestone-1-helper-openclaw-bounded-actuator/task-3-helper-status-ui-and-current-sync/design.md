# Implementation Design: Helper Status UI And Current Sync

## Scope And Inputs

This design prepares task 3 for `bf-task-execute`. It intentionally stops before production code. Implementation must start with TDD after Teamlead/design review approval.

Owned task artifacts live under:

- `docs/tasks/phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator/task-3-helper-status-ui-and-current-sync/`

Likely implementation write areas after review:

- `packages/client/src/lib/api.ts`
- `packages/client/src/components/HelperStatusPanel.tsx` or a similarly named user-owned Helper/Host Bridge status component
- `packages/client/src/App.tsx`
- `packages/client/src/components/Sidebar.tsx`
- `packages/client/src/lib/mainView.ts`
- `packages/client/src/index.css`
- Focused client tests under `packages/client/src/__tests__/`
- `docs/current/client/ui-map.md`
- `docs/current/client/feature-surfaces.md`
- `docs/current/host-bridge/README.md` and/or `docs/current/host-bridge/helper-daemon.md`
- `docs/current/security/README.md`, `docs/current/server/api-auth-admin-rails.md`, `docs/current/server/data-model-and-migrations.md`, and `docs/current/remote-agent/README.md` only if task 3 changes or clarifies their accepted status behavior

Task 3 should not edit `AGENTS.md`, `docs/tasks/README.md`, milestone state files, or `docs/blueprint/next/README.md` while task 2 owns shared state remediation.

## Recommended Approach

Create a distinct user-owned Helper status sidepane in the authenticated shell, backed by task 1's user-rail Helper enrollment list/detail API. This keeps Helper status separate from Remote Agent nodes and Settings privacy/admin-awareness while matching the current UI map rule for cross-channel user-owned surfaces.

Alternative 1: add Helper status inside Remote Nodes. This is rejected because it makes Remote Agent and Helper look like one rail and risks users reading Remote Agent online status as Helper readiness.

Alternative 2: add Helper status inside Settings. This is rejected because Settings is currently the user privacy/admin-awareness surface; using it for Host Bridge status risks turning status into privacy/compliance product copy and weakens discoverability for Configure OpenClaw preparation.

Alternative 3: expose only a passive status badge in an existing Configure OpenClaw entrypoint. This may be useful later, but it is too narrow for the task acceptance slice because reviewers need to inspect connected/offline/last-seen/revoked/uninstalled and allowed categories.

## API Data Flow

Task 1 exposes user-management routes for Helper enrollments. Task 3 should consume only the user-authenticated list/detail route from browser code:

```text
HelperStatusPanel mounts
  -> fetchHelperEnrollments()
  -> GET /api/v1/helper/enrollments with user auth
  -> response.enrollments[] redacted server projection
  -> local component state renders list/detail status
```

Client types should represent the server projection explicitly:

```ts
export type HelperEnrollmentStatus = 'pending' | 'connected' | 'offline' | 'revoked' | 'uninstalled';

export interface HelperEnrollmentStatusView {
  enrollment_id: string;
  host_label: string;
  helper_device_id?: string;
  allowed_categories: string[];
  status: HelperEnrollmentStatus | string;
  fresh: boolean;
  last_seen_at?: number;
  created_at: number;
  claimed_at?: number;
  revoked_at?: number;
  uninstalled_at?: number;
}
```

Use `status` plus `fresh` as the display source of truth. Do not derive connected/offline from WebSocket state, Remote Agent status, plugin runtime state, browser online/offline state, or local timers except for formatting relative timestamps. A reload should show the server's redacted status snapshot.

Do not add browser calls to:

- `POST /api/v1/helper/enrollments/{id}/claim`
- `POST /api/v1/helper/enrollments/{id}/status`
- `POST /api/v1/helper/enrollments/{id}/uninstall`

Those are Helper credential rail endpoints, not user UI status endpoints.

## UI State Flow

The surface should render a compact list and selected detail. Keep state local to the sidepane:

```text
local state:
  loading | error | enrollments[] | selectedEnrollmentId | lastRefreshStartedAt

on mount:
  fetch list
  select first enrollment when no selection exists

on refresh:
  refetch list
  preserve selected id if still present

on empty:
  render empty Helper status state without claiming install/configure success
```

Status rendering rules:

- `connected`: render as Helper enrollment/device recently seen. Show `last_seen_at` when present. Do not mention OpenClaw connected or configured.
- `offline`: render as Helper enrollment/device not recently seen. Show last seen if present; otherwise show a safe missing-last-seen state.
- `revoked`: render as terminal server-side enrollment revoke. Show `revoked_at` when present. Do not imply local cleanup or revoke race settlement is complete.
- `uninstalled`: render as terminal server-known helper-originated uninstall. Show `uninstalled_at` when present. Do not imply every local artifact was removed unless a later task proves that.
- `pending`: allowed as dependency-state carry-over because task 1 can create pending enrollments. Render as waiting for local Helper claim, not as connected/offline/revoked/uninstalled.
- unknown status: render as an unknown Helper status with no action affordance and no success styling.

Allowed categories should be displayed as non-interactive category chips/list rows. Category labels may be friendlier than enum tokens after content review, but the UI must preserve that these are bounded categories, not runnable commands. Unknown categories should be shown as unknown categories, not buttons or commands.

## Avoiding Configure OpenClaw Success Claims

Task 3 must keep three concepts separate:

| Concept | Can task 3 show it? | Rule |
| --- | --- | --- |
| Helper enrollment/device status | Yes | Connected/offline/revoked/uninstalled/pending from user-rail Helper enrollment API. |
| Allowed Helper category delegation | Yes | Category visibility only, non-interactive, no job semantics. |
| Configure OpenClaw success | No | No installed/configured/connected/succeeded copy or success badge unless a later job/config task proves it. |

Tests should include a reverse-copy assertion that the Helper status component does not contain strings equivalent to Configure OpenClaw success or OpenClaw connected. Exact localized copy should be locked in `content-lock.md` only after review chooses it.

## Edge Cases

- Empty list: show no Helper enrollments without implying Remote Agent setup or OpenClaw setup is missing/successful.
- Pending enrollment: show that a Helper enrollment is waiting for local claim; do not count it as connected.
- Missing `last_seen_at`: show a safe absent last-seen state; avoid `Invalid Date`, epoch zero, or fabricated freshness.
- Stale/offline with old `last_seen_at`: display offline and last seen; do not keep a spinner running indefinitely.
- Revoked with old `last_seen_at`: status is revoked, not offline.
- Uninstalled with old `last_seen_at`: status is uninstalled, not offline.
- Revoked and uninstalled timestamps both present due to future data repair: prefer terminal wording that exposes server-known terminal state without claiming local cleanup; implementation should follow server precedence if task 1 serializer already resolves it.
- Unknown status/category: render safe unknown labels and keep the row non-actionable.
- API 401: let existing auth/session behavior own it; do not add Helper-specific login copy.
- API 403/404 for detail refresh: remove or deselect stale local detail and show generic unavailable state.
- API 5xx/network failure: show a reloadable error state without falling back to Remote Agent or browser offline status.

## Test Plan For Later Execution

Implementation should use TDD. Suggested RED tests before production changes:

1. Client API helper test proves `fetchHelperEnrollments()` calls `GET /api/v1/helper/enrollments`, returns typed `enrollments`, and never exposes secret/credential fields.
2. `HelperStatusPanel` renders connected, offline, revoked, uninstalled, and pending fixtures distinctly.
3. Last-seen tests cover present, missing, and old timestamps without `Invalid Date` or epoch-zero output.
4. Allowed category tests render known category values as non-actionable category display and unknown category safely.
5. Reverse-copy test asserts the status component does not render Configure OpenClaw success/OpenClaw connected/job success copy.
6. Rail-separation source test checks the Helper status component/API does not import or call Remote Node status helpers as status fallback.

Keep the test scope focused on client behavior and docs sync for this task. Do not run long server suites during task-prep; implementation dispatch may run targeted client unit tests and `git diff --check` first, then broader checks if the change reaches shell navigation.

## Docs/Current Sync Targets

Required review targets after implementation:

- `docs/current/client/ui-map.md`: add Helper status to the surface hierarchy/map if a new sidepane or shell surface is added.
- `docs/current/client/feature-surfaces.md`: describe Helper status as user-owned Host Bridge status, distinct from Remote nodes and Settings.
- `docs/current/host-bridge/README.md`: ensure visible status semantics include connected/offline/last-seen/revoked/uninstalled and no Configure OpenClaw success claim.
- `docs/current/host-bridge/helper-daemon.md`: update only if UI/status behavior changes helper-daemon-facing current behavior; otherwise record no-op.
- `docs/current/security/README.md`: verify rail separation language remains accurate; update only if UI placement adds a new status surface description.
- `docs/current/server/api-auth-admin-rails.md`: verify browser user-rail reads and Helper credential rail separation remain accurate; update only if client API docs need status-surface mention.
- `docs/current/server/data-model-and-migrations.md`: likely no schema change for task 3; record no-op if the UI consumes existing projection only.
- `docs/current/remote-agent/README.md`: verify Remote Agent docs still state Helper enrollment/status is separate; update only if UI placement could be confused with Remote Nodes.

Do not edit task indexes, milestone state, `docs/tasks/README.md`, or blueprint-next README in this task while task 2 owns shared state remediation.

## Privacy And Security Review Points

- The UI must not display `enrollment_secret`, `helper_credential`, credential digests, Remote Agent `connection_token`, API keys, local private paths, message content, file content, environment dumps, or raw audit/log content.
- The client must not treat Remote Agent online status, WebSocket presence, or plugin runtime presence as Helper enrollment status.
- The UI must not create a new privacy dashboard, compliance center, legal promise, user-facing audit stream, or admin impact record.
- Status and categories are read-only visibility. No action buttons should run jobs, commands, services, scripts, or Configure OpenClaw steps in this task.
- Terminal states should fail closed in presentation: revoked/uninstalled cannot be shown as connected/successful even when a stale last-seen timestamp exists.

## Design Review Checklist

- [ ] Product/PM agrees on surface placement before implementation.
- [ ] Exact UI copy and DOM/test selectors are either locked in `content-lock.md` or explicitly left unlocked with rationale.
- [ ] TDD RED tests are written before production code.
- [ ] Task 2 shared files remain untouched unless Teamlead transfers ownership.
- [ ] `docs/current` sync/no-op rationale is recorded before PR finalization.
