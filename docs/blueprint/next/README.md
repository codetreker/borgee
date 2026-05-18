# Blueprint Next State

Target version: v1.1
Last updated: 2026-05-18
Resume from: v1.1 promotion complete (all 7 anchors COMPLETED). Tag `blueprint-v1.1` after this PR merges; open the next selection round when the user names new backlog scope.

This directory tracks planned or in-discussion blueprint work that is not yet accepted into `docs/blueprint/current/`. `current/` remains the implemented-and-accepted product truth. `docs/tasks/` is used only after a next anchor is locked for execution.

## §0 Status Ledger

| Anchor | Topic | Decision | Work | Source issues | Reference | Milestone path | Next action |
|---|---|---|---|---|---|---|---|
| `HB-RA-1A` | Helper bounded actuator product guardrails | LOCKED | COMPLETED | gh#681, gh#659 | promoted into `docs/blueprint/current/host-bridge.md` §1.2 / §2 / §3 | `docs/tasks/archived/phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator` | tag `blueprint-v1.1` |
| `HB-RA-1B` | Helper actuator execution contract | LOCKED | COMPLETED | gh#681, gh#659 | promoted into `docs/blueprint/current/host-bridge.md` §1.6 / §3.1-§3.3 | `docs/tasks/archived/phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator` | tag `blueprint-v1.1` |
| `MR-1` | Mention routing granularity and `@Everyone` broadcast | LOCKED | COMPLETED | gh#674, gh#693 | promoted into `docs/blueprint/current/channel-model.md` §5 | `docs/tasks/archived/phase-1-v11-trust-usability-closure/milestone-2-channel-attention-and-authority` | tag `blueprint-v1.1` |
| `CH-1` | Channel authority and user-side channel management | LOCKED | COMPLETED | gh#685, gh#688, gh#690 | promoted into `docs/blueprint/current/channel-model.md` §6 | `docs/tasks/archived/phase-1-v11-trust-usability-closure/milestone-2-channel-attention-and-authority` | tag `blueprint-v1.1` |
| `CT-1` | Client truthfulness and forbidden-state visibility | LOCKED | COMPLETED | gh#724 | promoted into `docs/blueprint/current/client-shape.md` §5 + `canvas-vision.md` §6 | `docs/tasks/archived/phase-1-v11-trust-usability-closure/milestone-3-client-truth-and-navigation` | tag `blueprint-v1.1` |
| `PS-1` | Privacy scope guard | LOCKED | COMPLETED | gh#654 | preserved across `host-bridge.md` §1.2/§2 (rail separation), `client-shape.md` §5.3, `admin-model.md` (no expansion); no new user-facing privacy/compliance product surface in v1.1 | all archived v1.1 milestones | tag `blueprint-v1.1` |
| `IA-1` | Sidebar footer and account entry IA | LOCKED | COMPLETED | gh#669, gh#670 | promoted into `docs/blueprint/current/client-shape.md` §6 | `docs/tasks/archived/phase-1-v11-trust-usability-closure/milestone-3-client-truth-and-navigation` | tag `blueprint-v1.1` |

Decision values are `OPEN`, `LOCKED`, or `REOPENED`. Work values are `PENDING`, `IMPLEMENTING`, or `COMPLETED`. Only `LOCKED` anchors may move into `docs/tasks/` Phase/Milestone planning.

All v1.1 anchors are promoted; the v1.1 canonical Phase folder has been archived under `docs/tasks/archived/phase-1-v11-trust-usability-closure/`. The phase-exit task folder remains under `docs/tasks/phase-1-v11-trust-usability-closure/milestone-phase-exit/` until the §7 gate signoffs land; after that it may be archived by the human.

## §1 Iteration positioning (v1.1, closed)

v1.1 closed the v1 usability / trust gaps surfaced after first real use: Helper / remote actuator onboarding, mention routing, channel authority, client truthfulness, privacy scope discipline, and sidebar / account IA. Final bump judgment: **minor** (sandbox pillar preserved as bounded-actuator stance, not removed; no rail merge; no user-facing privacy/compliance product expansion).

## §2 Source issues

See `docs/blueprint/_meta/v1.1/source-issues.md` for picked-issue traceability.

## §3 Next selection round

Not opened. Open by user request after `blueprint-v1.1` tag lands, scanning GitHub `backlog` issues per `bf-blueprint-iteration` lifecycle.

