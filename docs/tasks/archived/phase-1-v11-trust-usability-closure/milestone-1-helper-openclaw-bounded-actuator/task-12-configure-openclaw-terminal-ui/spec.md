# Spec: Configure OpenClaw Terminal UI

## Source Alignment

Task12 closes the Milestone 1 Helper/OpenClaw loop by showing a truthful Configure OpenClaw status derived from accepted Helper job history. It consumes the already accepted typed job chain:

- Task9 OpenClaw install/config jobs, PR #956 (`5575b53`).
- Task10 Borgee plugin channel binding job, PR #958 (`ad50575`).
- Task11 service lifecycle reliability and `service.lifecycle` typed job, PR #963 (`d8d179e`).

## Required Behavior

- The Helper enrollment list and detail user rail may include a safe `configure_openclaw` projection for each enrollment.
- The projection is derived server-side from Helper job metadata for `openclaw.install_from_manifest`, `openclaw.configure_agent`, `borgee_plugin.configure_connection`, and `service.lifecycle`.
- The visible states are `queued`, `running`, `succeeded`, `failed`, `denied`, `revoked`, and `manual_debug`.
- `succeeded` is only valid when all required Configure OpenClaw closure job types have latest successful rows.
- Revoked or uninstalled Helper enrollments override job state with a revoked Configure OpenClaw state.
- Denied, failed, and manual-debug states may expose bounded failure code/message plus bounded audit/log refs. They must not expose payloads, payload hashes, manifest digests, raw result summaries, raw logs, credentials, owner/org internals, command strings, shell, argv, path, domain, service unit, or Remote Agent rail data.
- The client sanitizer keeps only the safe projection fields and renders terminal states without calling Helper credential endpoints or Remote Node fallback paths.

## Out Of Scope

- No job enqueue button, action runner, local execution, service-manager call, raw log download surface, broad onboarding copy rewrite, broad visual redesign, or privacy/compliance product expansion.
- No change to Helper job authority, local policy, service assets, Remote Agent grants, or channel management behavior.

## Acceptance Checks

- Server tests prove safe list/detail projection, no false success before all closure jobs succeed, denial/manual-debug/revoked behavior, and no raw payload/log exposure.
- Client tests prove sanitizer redaction and UI rendering for queued/running/succeeded/failed/denied/revoked/manual-debug states.
- Focused and broader server/client verification pass with executable `GOTMPDIR` and `sqlite_fts5` where required.
