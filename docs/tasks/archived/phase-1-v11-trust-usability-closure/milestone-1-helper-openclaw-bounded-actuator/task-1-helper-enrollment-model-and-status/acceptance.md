# Acceptance: Helper Enrollment Model And Status

## Source Alignment

- Task: `task-1-helper-enrollment-model-and-status`
- Milestone: `phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator`
- Blueprint anchors: `remote-actuator-design.md` §1.1-§1.2 and §5; `migration-analysis.md` §2.1, §2.6, and §6.1
- Spec alignment note: Acceptance Segments A-F align to `spec.md` segmentation and reverse checks at documentation-stage granularity.

## Segment A: Distinct Helper Enrollment Identity

Acceptance checks:

- A reviewer can identify a Helper enrollment record that is explicitly separate from any Remote Agent file-proxy token or agent-runtime credential path.
- The Helper enrollment identity includes a host/device identity suitable for reporting last-seen or stale-device state without treating the Remote Agent as the enrolled Helper.
- Enrollment uses a short-lived local enrollment step before any persistent Helper credential is accepted for outbound poll or status reporting.

Negative checks:

- Remote Agent file-proxy tokens are not reused, widened, or renamed into Helper host-management credentials.
- The task does not introduce a general command credential, shell credential, or runtime-owner credential under the Helper enrollment name.

## Segment B: Owner / Org / Host Binding

Acceptance checks:

- The accepted enrollment foundation binds Helper enrollment to owner, org, and host/device identity as the authority base for later enqueue and local policy checks.
- Wrong-owner, wrong-org, revoked, unenrolled, or stale-device states are representable as rejected or inactive Helper enrollment states.
- The binding is server-authoritative enough for later task work to reject jobs before Helper execution is considered.

Negative checks:

- Enrollment cannot be treated as org-global, user-global, or host-label-only authority.
- The task does not add cross-org privilege expansion or any new user-facing privacy/compliance product surface.

## Segment C: Allowed Job Categories Shape

Acceptance checks:

- The enrollment status foundation exposes a closed allowed-category shape for Helper/OpenClaw lifecycle and configuration work, without requiring this task to implement queue, lease, result, or job execution behavior.
- The allowed-category shape is narrow enough for later work to distinguish OpenClaw/helper lifecycle and config categories from arbitrary host commands.
- Review evidence can show that allowed job categories are part of enrollment/status visibility, not blanket preauthorization for every host action.

Negative checks:

- Unknown job categories, arbitrary shell commands, client-supplied argv, executable paths, scripts, or arbitrary service names are out of scope and must not be accepted by this task.
- The task does not execute Configure OpenClaw, start service lifecycle actions, create a job queue, or define lease/result behavior.

## Segment D: Visible Helper Status Shape

Acceptance checks:

- The accepted foundation can represent Helper status as connected, offline, revoked, and uninstalled without implying Configure OpenClaw success.
- Status visibility includes host/device identity, owner/org context, last-seen or freshness signal, and allowed job categories at the level needed by later UI/API work.
- Revoked and uninstalled states are visibly distinct from offline or never-enrolled states.

Negative checks:

- Helper connected status cannot be reported as OpenClaw connected or Configure OpenClaw succeeded.
- Failed, revoked, uninstalled, unenrolled, or stale-device conditions cannot collapse into a successful or indefinitely pending visible state.

## Segment E: Remote Agent Rail Separation

Acceptance checks:

- Review evidence proves Helper actuator credentials, grants, enforcement rail, and enrollment state remain separate from Remote Agent file-proxy credentials, grants, and enforcement rail.
- Any shared owner/org concepts are treated as authority inputs only; they do not merge the Helper actuator rail with the Remote Agent file/runtime rail.
- Existing Remote Agent file-proxy behavior remains outside the Helper enrollment authority created by this task.

Negative checks:

- No shared token, shared grant, merged permission check, or Remote Agent credential extension is introduced for Helper enrollment.
- No Helper enrollment state grants Remote Agent filesystem authority, and no Remote Agent grant authorizes Helper host-management jobs.

## Segment F: Current-Doc Sync

Acceptance checks:

- If accepted behavior changes, `docs/current` is updated for the Helper enrollment identity/status foundation, especially host-bridge, Remote Agent separation, and security/data-isolation boundaries as applicable.
- Current-doc updates preserve the `migration-analysis.md` §6.1 privacy guard: no new user-facing privacy/compliance product surface is introduced while backend security boundaries remain documented.
- If no current-doc file needs a change, the task records a reviewer-checkable no-op rationale in task progress or review notes.

Negative checks:

- Current docs must not describe Helper enrollment as Remote Agent enrollment, runtime ownership, arbitrary command execution, or post-install sudo caching.
- Current docs must not weaken existing admin/user/agent rail separation, owner-only agent capability, or host permission minimization.
