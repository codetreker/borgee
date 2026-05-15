# Milestone 3: Channel Visual Truth

> Remapped history. This milestone remains the detailed task home for private/sidebar visual-truth tasks, but the authoritative coarse grouping is now `docs/tasks/phase-1-v11-trust-usability-closure/milestone-2-channel-attention-and-authority/`.

## Capability Goal

Make private-channel visual state accurate without overpowering unread, fault, or presence signals.

## Acceptance Boundary

Accepted by this milestone:

- Private channel indicators are visually quieter and do not conflict with other sidebar states.
- IA or visual movement does not change channel authority, ACL, or membership rules.

Rejected by this milestone:

- Broad visual redesign or pixel-art restyling.
- Any privacy/security boundary change hidden as an icon/UI change.

## Task-Split Trigger

Break down with channel-management tasks if shared components make that cheaper; otherwise keep this as a separate UI milestone.

## Task Index

| Task | Status | Purpose | Depends on | Parallel? | First ready? |
|---|---|---|---|---|---|
| `task-1-private-indicator-state-inventory` | PLANNED | Inventory private/unread/fault/presence sidebar states and define collision boundaries | Canonical Milestone 2 start or explicit parallel UI slot | no | after dependency clears |
| `task-2-private-indicator-visual-treatment` | PLANNED | Make private indicators quieter without changing authority | `task-1-private-indicator-state-inventory` | no | no |
| `task-3-sidebar-state-collision-regression` | PLANNED | Add regression proof that private indicators do not hide unread/fault/presence states | `task-2-private-indicator-visual-treatment` | no | no |

Dependency order: this milestone stays UI-focused and separate from authority changes. If shared sidebar components make it cheaper, execute with channel-management UI work; otherwise keep it separate.

## Breakdown Review

| Role | Decision | Notes |
|---|---|---|
| Architect | LGTM | Visual-truth tasks stay UI-focused and do not change channel authority. |
| PM | LGTM | Private indicator value is scoped to truthful, quieter state coexistence rather than redesign. |
| QA | LGTM | State matrix, visual treatment, and collision regression give checkable private/unread/fault/presence proof. |
| Dev | LGTM | The inventory artifact makes the later visual-treatment and regression tasks implementable as separate PRs. |
| Security | LGTM | Private-channel visibility and ACL meaning are flagged without moving authority into the client. |
