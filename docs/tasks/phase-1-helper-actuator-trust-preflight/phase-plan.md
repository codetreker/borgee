# Phase 1: Helper Actuator Trust Preflight

## Source Anchor

- Locked next anchor: `HB-RA-1A` in `docs/blueprint/next/README.md` lines 13 and 31-45.
- Reverse-check guardrail: `PS-1` in `docs/blueprint/next/README.md` lines 18 and 99-108.
- Explicit exclusion: `HB-RA-1B` remains `OPEN / PENDING` in `docs/blueprint/next/README.md` lines 14 and 47-56. Its manifest and artifact signing, helper credential model, sandbox and Linux poll, revoke race, service permissions, and queue contract blockers are not execution-approved by this Phase.

## Value Loop

Team can review and approve Borgee's bounded Configure OpenClaw delegation promise before any helper actuator implementation begins.

This Phase is a trust preflight, not a product implementation Phase. It makes the user-perceivable promise reviewable: after explicit local enrollment, Web-side Configure OpenClaw may ask the enrolled Helper to perform only bounded, schema-bound, pre-authorized OpenClaw lifecycle and configuration jobs. The server never dials the host, the Helper remains the local policy enforcement point, and failed or revoked jobs must be visible instead of looking successful.

## Boundary

In scope:

- Record the `HB-RA-1A` guardrail lock as a Phase/Milestone execution path.
- Preserve the closed schema-bound job promise, outbound-only Helper relationship, server-plus-helper validation, non-sudo long-lived services, revoke/uninstall visibility, bounded status/logs, and separate Helper versus Remote Agent rails from the locked anchor.
- Carry `PS-1` as a reverse-check guardrail: do not add user-facing privacy/compliance product scope, and do not weaken existing admin, privacy, security, impersonation, audit/enforcement, data-minimization, capability, or rail-separation controls.
- Create only the first planning task seed needed for milestone breakdown readiness.

Out of scope:

- No command channel. Borgee does not become a shell, runtime owner, or arbitrary host command dispatcher.
- No credential reuse. Remote Agent file-proxy credentials, grants, and enforcement rails stay separate from Helper actuator credentials, grants, and enforcement rails.
- No sudo cache. Long-lived Helper and OpenClaw services stay non-sudo; any privileged enrollment helper remains short-lived and visible.
- No `HB-RA-1B` execution-contract decisions. Manifest/artifact signing, helper credential shape, sandbox/Linux poll details, revoke race mechanics, service permissions, and exact queue/lease/result semantics stay parked in `HB-RA-1B`.
- No milestone breakdown, task skeletons, code, API, queue, credential, sandbox, or service changes in this Phase-plan PR.
- No new user-facing privacy/compliance product surface.

## Milestones

| Milestone | Goal | Status | Task-split trigger |
|---|---|---|---|
| `milestone-1-boundary-guardrail-lock` | Lock the reviewable boundary around the bounded Configure OpenClaw delegation promise and reject execution-contract drift | PLANNED | Run `bf-milestone-breakdown` only after this Phase plan is accepted and `HB-RA-1B` blockers remain explicitly out of scope |

## Exit Gates

Strict checks:

- The Phase plan and milestone cite `docs/blueprint/next/README.md` `HB-RA-1A` as the locked source anchor.
- The Phase plan and milestone cite `docs/blueprint/next/README.md` `PS-1` as the privacy-scope reverse-check guardrail.
- The Phase plan and milestone explicitly exclude `HB-RA-1B` blockers.
- Required forbidden-drift phrases are absent unless they appear in a reject/non-goal context.
- Placeholder markers for unfinished work do not appear in the touched planning files.
- `git show --check` passes before handoff.

User-perceivable checks:

- A reviewer can state the promise in one sentence: explicit local enrollment allows bounded Web-side Configure OpenClaw delegation without a second SSH step, but only inside closed, schema-bound Helper jobs.
- A reviewer can also state what users must still see: Helper connected state, allowed job categories, revoke/uninstall controls, truthful queued/running/succeeded/failed status, failure reasons, and bounded redacted logs.
- A reviewer can point to the visible trust boundary: no command channel, no credential reuse, no sudo cache, and Helper rails remain separate from Remote Agent rails.
- A reviewer can point to the privacy boundary: no new user-facing privacy/compliance product scope, while existing security/privacy/admin controls remain intact.

Carry-over checks:

- Deferred execution details must point back to `HB-RA-1B`; they are not silently carried as accepted assumptions.
- Any task created from this milestone must carry `PS-1` in its reverse-check table when it touches privacy, security, admin, audit, capability, or Helper/Remote Agent rail boundaries.
- Any future implementation task must start from milestone breakdown and its own task contract; this Phase plan does not authorize code changes.
- If `HB-RA-1B` changes or rejects a blocker, this Phase remains only the guardrail record and must not be treated as implementation evidence.

Fake-green rejection checks:

- Do not count a plan as accepted if it implies Configure OpenClaw can succeed without terminal status, failure reason, and bounded log visibility.
- Do not count a plan as accepted if revoked, cancelled, stale, denied, queued, leased, or failed work can appear successful or spin indefinitely.
- Do not count a plan as accepted if it merges Helper and Remote Agent credentials, grants, or enforcement rails through naming, IA, or shared tokens.
- Do not count a plan as accepted if it treats `HB-RA-1B` blockers as solved by this planning milestone.
- Do not count a plan as accepted if it uses `PS-1` to weaken existing admin/privacy/security controls.

## Next Step

After this Phase plan lands, select `milestone-1-boundary-guardrail-lock` for `bf-milestone-breakdown`. That later step creates reviewed task skeletons only for planning-preflight work, while execution-contract work waits for `HB-RA-1B` to lock.
