# task-3-sidebar-state-collision-regression

Purpose:
- Prove private indicator changes do not regress other sidebar state signals.

Scope:
- Add selected regression coverage or review evidence for private/unread/fault/presence state combinations.
- Keep IA/visual movement from changing membership or ACL rules.

Out of scope:
- No broad e2e platform expansion or unrelated sidebar redesign.

Depends on:
- `task-2-private-indicator-visual-treatment`

Blueprint anchors:
- `CH-1`: `docs/blueprint/next/migration-analysis.md` §4.3
- `PS-1`: `docs/blueprint/next/migration-analysis.md` §6.1

Acceptance slice:
- A reviewer can verify regression coverage or evidence catches collisions between private indicators and unread/fault/presence states.

Parallelism:
- Runs after visual treatment.

Sensitive paths:
- private-channel visibility, regression proof
