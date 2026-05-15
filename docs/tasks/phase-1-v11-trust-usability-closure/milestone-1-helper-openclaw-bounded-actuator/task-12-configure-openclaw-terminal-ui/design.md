# Dev Design: Configure OpenClaw Terminal UI

## Approach

Add a narrow server-side projection on the existing Helper enrollment user rail, then let the existing Helper Status panel render it. The projection is computed from Helper job metadata that already passed the accepted enqueue, lease/result, policy, log-settlement, install/config, plugin-binding, and service-lifecycle boundaries.

This keeps Task12 out of local execution and authority. The server decides what is safe to expose; the client sanitizes the response again and displays only bounded state, reason, and reference fields.

## Server Projection

`HelperEnrollmentHandler` receives the Helper job repository and asks it for Configure OpenClaw status for the list/detail enrollment rows in the authenticated owner/org scope.

The job repository loads only recognized Configure OpenClaw closure job types:

- `openclaw.install_from_manifest`
- `openclaw.configure_agent`
- `borgee_plugin.configure_connection`
- `service.lifecycle`

The projection keeps the latest row per job type and derives:

- `queued` when a required latest step is queued.
- `running` when a required latest step is leased/running.
- `succeeded` only when all required latest steps succeeded.
- `denied` when a failed step has a denial-style failure code.
- `failed` for other failed steps.
- `manual_debug` for expired/cancelled or incomplete terminal chains.
- `revoked` when the Helper enrollment is revoked or uninstalled.

Only state, label, failure code/message, bounded audit refs, bounded log refs, and per-step safe status metadata are serialized. Raw payloads, hashes, manifest bindings, result summaries, credentials, and execution details remain private.

## Client Rendering

`packages/client/src/lib/api.ts` adds `HelperConfigureOpenClawStatusView` sanitizer types and keeps only safe projection fields. Unknown states are preserved as strings but default missing projection labels to manual debug.

`HelperStatusPanel` renders the projection in the existing Helper Status surface:

- list row status text for quick scanning;
- detail badge for the selected Helper;
- bounded reason and evidence refs;
- safe per-step job type labels and statuses.

The surface does not add run/retry/log-download controls and does not call Helper credential or Remote Node APIs.

## Tests

- Go API tests seed Helper job rows and verify no false success before all required closure jobs succeed, terminal success after all required jobs, denial/manual-debug/revoked states, safe bounded refs, and no raw private fields.
- Client API tests verify the sanitizer drops raw payload/log fields while preserving safe Configure OpenClaw projection fields.
- Client panel tests verify queued/running/succeeded/failed/denied/revoked/manual-debug rendering and blacklist false success copy.
