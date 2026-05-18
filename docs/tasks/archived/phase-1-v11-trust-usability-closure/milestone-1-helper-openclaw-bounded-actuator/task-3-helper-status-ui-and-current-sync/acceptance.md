# Acceptance: Helper Status UI And Current Sync

## Source Alignment

- Task: `task-3-helper-status-ui-and-current-sync`
- Milestone: `phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator`
- Blueprint anchors: `remote-actuator-design.md` section 1.2 and section 11; `migration-analysis.md` section 6.1
- Dependency: consumes `task-1-helper-enrollment-model-and-status`; avoids task 2 shared state/doc remediation files until Teamlead clears ownership.

## Segment A: User-Rail API Projection

Acceptance checks:

- Client API types and fetch helpers represent Helper enrollment status fields needed by UI: enrollment id, host label, optional helper device id, allowed categories, status, fresh flag, last seen, and terminal timestamps.
- Browser status UI reads user-authenticated list/detail endpoints only and treats REST as authoritative on load/refresh.
- Raw enrollment secrets, persistent Helper credentials, credential digests, org internals, Remote Agent tokens, API keys, and local filesystem paths are not rendered or stored in shared client state by the status UI.

Negative checks:

- The browser UI does not call Helper credential claim, heartbeat/status, or uninstall endpoints.
- The client does not infer Helper status from Remote Agent node status, WebSocket presence, or OpenClaw plugin status.

## Segment B: Visible Status Distinction

Acceptance checks:

- A reviewer can distinguish connected, offline, revoked, and uninstalled Helper enrollment states in the rendered product surface.
- Connected status uses the server-provided status/freshness projection and last-seen signal; offline status shows stale or missing freshness without claiming terminal failure.
- Revoked and uninstalled states are visible as terminal enrollment states and do not look like successful configuration.

Negative checks:

- Connected Helper status is not labeled as OpenClaw connected or Configure OpenClaw succeeded.
- Revoked, uninstalled, pending, and offline states do not collapse into a single generic offline or loading state.

## Segment C: Last-Seen And Allowed Categories

Acceptance checks:

- Last seen is displayed in a bounded, user-safe way when present; missing last-seen is handled without leaking internals or showing an impossible timestamp.
- Allowed categories are displayed from the closed category list and remain category-level visibility.
- Unknown category values, if ever received from a future server, render as safe unknown data rather than command affordances.

Negative checks:

- Allowed categories are not rendered as runnable actions, job payloads, shell commands, service unit names, or success/failure outcomes.
- The UI does not expose private message content, private file content, environment dumps, tokens, secrets, or full local paths.

## Segment D: Rail Separation

Acceptance checks:

- Helper status is placed and named so users and reviewers do not confuse it with Remote Agent filesystem proxy status.
- Remote Agent node management remains separate from Helper enrollment status, even if both are user-owned host-adjacent surfaces.
- Host grants remain consent records and do not become Helper enrollment status or allowed categories.

Negative checks:

- No shared token, shared grant, merged status endpoint, or Remote Agent node status fallback is introduced.
- No Host Bridge/Helper status UI grants Remote Agent filesystem authority, and no Remote Agent UI grants Helper host-management authority.

## Segment E: Current-Doc Sync

Acceptance checks:

- `docs/current/client/ui-map.md` and `docs/current/client/feature-surfaces.md` are updated if the implementation adds or changes a user SPA surface.
- `docs/current/host-bridge/README.md` and/or `docs/current/host-bridge/helper-daemon.md` are updated if the accepted status behavior changes Host Bridge/Helper current behavior.
- `docs/current/server/api-auth-admin-rails.md`, `docs/current/server/data-model-and-migrations.md`, `docs/current/security/README.md`, and `docs/current/remote-agent/README.md` are checked for rail/status accuracy; changed files or no-op rationale are recorded in `progress.md`.

Negative checks:

- Current docs must not describe Helper status as Remote Agent status, OpenClaw success, job execution status, or privacy/compliance product promise UI.
- Current docs must not weaken admin/user/agent rail separation, data minimization, or Helper/Remote Agent credential separation.

## Segment F: Test And Review Evidence For Later Execution

Acceptance checks:

- Implementation begins with failing tests for API type/client fetch behavior and UI rendering states before production code changes.
- Focused client tests cover connected, offline, revoked, uninstalled, missing last-seen, allowed categories, no credential display, and no OpenClaw success copy.
- Doc hygiene checks run at minimum with `git diff --check`; broader test suites are deferred to implementation dispatch.

Negative checks:

- No production code is implemented before design review and TDD dispatch.
- No task 2 shared state/doc remediation files are edited by this task while task 2 owns them.
