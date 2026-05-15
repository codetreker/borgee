# Phase 1: v1.1 Trust And Usability Closure

## Status

CLOSING. This is the single active v1.1 Phase; M2 and M3 are accepted, and M1 is in its Task12 closure branch.

## Exit Condition

Close selected v1.1 trust/usability gaps: Helper/OpenClaw bounded actuator onboarding, channel attention/authority clarity, production client truthfulness, and account/sidebar IA, without expanding privacy/compliance scope or merging Helper/Remote Agent rails.

## Source Anchors

- `HB-RA-1A`: Helper bounded actuator product guardrails.
- `HB-RA-1B`: Helper actuator execution contract for implementation planning.
- `MR-1`: Mention routing granularity and `@Everyone` broadcast.
- `CH-1`: Channel authority and user-side channel management.
- `CT-1`: Client truthfulness and forbidden-state visibility.
- `PS-1`: Privacy scope guard.
- `IA-1`: Sidebar footer and account entry IA.

## Replan Rationale

The previous v1.1 shape split selected work into three Phases and eight milestones. That fragmentation slowed execution without adding a real prerequisite, integration, or downstream coordination boundary.

- The prior Helper/OpenClaw milestones were one prerequisite chain toward one user-facing Helper/OpenClaw loop, so they are now one coarse milestone.
- The prior channel-control and client-truth slices were execution slots, not dependency or integration boundaries, so they are now coarse milestones inside the same active Phase.
- The former Helper/OpenClaw phase directory and the unexecuted former channel-control and client-truth phase folders were removed; their accepted task artifacts and unexecuted task skeletons now live under the canonical Milestone 1, Milestone 2, and Milestone 3 directories.
- A new Phase must not be opened casually. It needs a real prerequisite boundary, integration boundary, or downstream integration/coordination reason.

## Boundary

In scope:

- Helper/OpenClaw bounded actuator onboarding and Configure OpenClaw closure.
- Channel attention controls, channel authority, and private/sidebar state clarity.
- Production client truthfulness, forbidden-state UX, Settings permissions reachability, and reverse proof for selected production surfaces.
- Account/sidebar IA cleanup, including avatar/logout and Helper/Remote Nodes placement.

Out of scope:

- Arbitrary host command channel, shell, argv, executable path, script, or client-supplied service unit dispatch.
- Reusing or merging Helper actuator and Remote Agent credentials, grants, or enforcement rails.
- New user-facing privacy/compliance product expansion.
- Broad visual redesign, broad e2e platform expansion, or unrelated quality backlog unless explicitly pulled into a task.

## Milestones

| Milestone | Goal | Status | Canonical doc |
|---|---|---|---|
| Milestone 1: Helper / OpenClaw Bounded Actuator | Complete one user-facing Helper/OpenClaw loop from accepted enrollment through bounded typed jobs, service reliability, and Configure OpenClaw terminal UI | CLOSING | `milestone-1-helper-openclaw-bounded-actuator/milestone.md` |
| Milestone 2: Channel Attention And Authority | Let users understand and control channel attention, membership, allowed actions, authority, and private/sidebar state meaning | ACCEPTED | `milestone-2-channel-attention-and-authority/milestone.md` |
| Milestone 3: Client Truth And Navigation | Make selected production surfaces truthful and reachable while simplifying account/sidebar navigation | ACCEPTED | `milestone-3-client-truth-and-navigation/milestone.md` |

This Phase has three user-facing milestones, within the project default. More milestones or a second Phase require an explicit boundary reason.

## Accepted History Remap

Accepted work remains accepted and is remapped into Milestone 1:

| PR | Commit | Accepted scope | Canonical milestone |
|---|---|---|---|
| #934 | `547f869` | Helper enrollment/status foundation | Milestone 1 |
| #936 | `1ca5f95` | Helper credential rotation/revoke lifecycle | Milestone 1 |
| #937 | `2872905` | Helper status UI and current-doc sync | Milestone 1 |
| #938 | `64d56f1` | Helper job envelope and enqueue authority | Milestone 1 |
| #939 | `96dc0dc` | Helper outbound service prerequisites | Milestone 1 |
| #942 | `642fb57` | Helper local policy / manifest / sandbox profile | Milestone 1 |
| #943 | `c2c61e6` | Helper pull / lease / result | Milestone 1 |
| #954 | `419c5bf` | Bounded status logs and revoke settlement | Milestone 1 |
| #956 | `5575b53` | OpenClaw install and agent config jobs | Milestone 1 |
| #958 | `ad50575` | Borgee plugin channel binding job | Milestone 1 |
| #963 | `d8d179e` | Service lifecycle boot/crash reliability | Milestone 1 |

Accepted task docs and progress now live under the canonical milestone directories. Milestone 1 accepted history is summarized in `milestone-1-helper-openclaw-bounded-actuator/accepted-history.md`, with `task-12-configure-openclaw-terminal-ui` carrying the final Helper/OpenClaw closure work. Milestone 2 and Milestone 3 task indexes are status-synced as accepted in their canonical milestone docs.

## Exit Gates

Strict checks:

- Helper/OpenClaw work keeps Helper actuator credentials, grants, and enforcement separate from Remote Agent rails.
- Server enqueue authorization and Helper local policy both validate owner, org, enrollment, delegation, job type, manifest/artifact, paths/domains, service IDs, and revocation state before action.
- Channel attention and channel management remain server-authoritative for ACL, membership, ownership, and fanout.
- Forbidden states do not leak private channel, artifact, message, file, or body content before authorization succeeds.
- `PS-1` blocks new user-facing privacy/compliance product expansion while preserving existing admin, privacy, security, audit, data-minimization, capability, and rail-separation controls.

User-perceivable checks:

- Users can configure OpenClaw through bounded Helper jobs and see truthful terminal status, bounded logs, and revoke/uninstall behavior.
- Users can understand channel mention/attention behavior, channel ownership/actions, and private/sidebar state signals.
- Users can reach selected production surfaces, understand forbidden/empty/error states, and find account/logout plus primary sidebar entries without footer clutter.
