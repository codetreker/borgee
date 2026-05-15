# Milestone 1: Production Surface Truthfulness

> Remapped history. This milestone remains the detailed task home for production truthfulness tasks, but the authoritative coarse grouping is now `docs/tasks/phase-1-v11-trust-usability-closure/milestone-3-client-truth-and-navigation/`.

## Capability Goal

Make named already-built client surfaces reachable in production and make forbidden states explicit without leaking protected data.

## Acceptance Boundary

Accepted by this milestone:

- ArtifactComments series is mounted inside `ArtifactPanel`, and `PermissionsView` is reachable under Settings where the product claims those surfaces.
- ACL forbidden states are visible and non-leaky.
- E2E reverse proof covers ArtifactComments, ArtifactPanel/ArtifactComments forbidden states, and Settings `PermissionsView` reachability without turning this milestone into a broad quality-platform expansion.

Rejected by this milestone:

- Broad e2e platform rewrite, mobile coverage expansion, modal a11y sweep, or visual redesign unless separately pulled in.
- User-facing privacy/compliance product promises.

## Task-Split Trigger

Break down after phase-plan acceptance. Expected tasks should cover production mounts, forbidden-state UX, and production-surface e2e reverse proof.

## Task Index

| Task | Status | Purpose | Depends on | Parallel? | First ready? |
|---|---|---|---|---|---|
| `task-1-artifactcomments-production-mount` | PLANNED | Mount ArtifactComments series inside the production ArtifactPanel surface | Canonical Milestone 3 start | no | after dependency clears |
| `task-2-acl-forbidden-state-ux` | PLANNED | Show non-leaky forbidden states for ArtifactPanel/ArtifactComments and Settings PermissionsView | `task-1-artifactcomments-production-mount` | yes, after task 1 | no |
| `task-3-security-permission-surface-reachability` | PLANNED | Mount user PermissionsView under Settings without expanding privacy product scope | Canonical Milestone 3 start | yes | no |
| `task-4-production-surface-e2e-reverse-proof` | PLANNED | Add reverse proof for ArtifactComments, forbidden states, and Settings PermissionsView | `task-2-acl-forbidden-state-ux`, `task-3-security-permission-surface-reachability` | no | no |

Dependency order: ArtifactComments reachability is first for comment-surface proof. Settings PermissionsView reachability can run independently of ArtifactComments if shell/settings files are separable. Reverse proof waits for both mounted surfaces and forbidden-state UX.

## Breakdown Review

| Role | Decision | Notes |
|---|---|---|
| Architect | LGTM | Production mounts and forbidden states name concrete surfaces while preserving server authority boundaries. |
| PM | LGTM | Truthfulness value stays on reachable surfaces and non-leaky states without broad quality-scope expansion. |
| QA | LGTM | Reverse proof is tied to ArtifactComments/ArtifactPanel and Settings PermissionsView behavior. |
| Dev | LGTM | Named surface scope is concrete enough for one-PR tasks and avoids vague selected-surface work. |
| Security | LGTM | Forbidden states, protected content visibility, and privacy guardrails are marked as sensitive paths. |
