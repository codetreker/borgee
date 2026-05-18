# task-4-production-surface-e2e-reverse-proof

Purpose:
- Prove the named production surfaces and forbidden states are real without expanding into a broad quality-platform task.

Scope:
- Add e2e reverse proof for ArtifactComments production mount, ArtifactComments/ArtifactPanel forbidden states, Settings PermissionsView reachability, and Settings PermissionsView forbidden/empty/error state.
- Keep proof tied to product behavior rather than string-only or fake route checks.

Out of scope:
- No broad e2e platform rewrite, mobile coverage expansion, or modal accessibility sweep.

Depends on:
- `task-2-acl-forbidden-state-ux`
- `task-3-security-permission-surface-reachability`

Blueprint anchors:
- `CT-1`: `docs/blueprint/next/migration-analysis.md` §5.3

Acceptance slice:
- A reviewer can see behavior-level e2e proof for ArtifactComments, Settings PermissionsView, and non-leaky forbidden states without pulling in the full quality backlog.

Parallelism:
- Runs after named production surface and forbidden-state tasks.

Sensitive paths:
- auth, ACL, privacy, test evidence
