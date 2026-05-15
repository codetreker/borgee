# task-7-private-indicator-state-inventory

Purpose:
- Define which sidebar states private indicators must coexist with before changing visuals.

Scope:
- Produce a concrete state matrix for private, unread, fault, presence, selection, and hover states in the relevant channel/sidebar components.
- Record the existing DOM/CSS anchors and collision boundaries that the visual treatment task must preserve.

Out of scope:
- No visual redesign, authority change, ACL change, or channel management action work.

Depends on:
- Canonical Milestone 2 start or explicit parallel UI slot

Blueprint anchors:
- `CH-1`: `docs/blueprint/next/migration-analysis.md` §4.3
- `PS-1`: `docs/blueprint/next/migration-analysis.md` §6.1

Acceptance slice:
- A reviewer can inspect a state matrix with DOM/CSS anchors that makes private/unread/fault/presence collision cases concrete enough for the visual treatment task.

Parallelism:
- First task after UI slot clears. Blocks visual treatment.

Sensitive paths:
- private-channel visibility, ACL meaning
