# task-2-private-indicator-visual-treatment

Purpose:
- Make private-channel indicators accurate without dominating the sidebar.

Scope:
- Adjust the private-channel visual treatment so it is quieter and coexists with unread, fault, presence, selection, and hover states.
- Preserve existing channel authority and ACL behavior.

Out of scope:
- No broad visual redesign, pixel-art restyling, or privacy/security boundary change.

Depends on:
- `task-1-private-indicator-state-inventory`

Blueprint anchors:
- `CH-1`: `docs/blueprint/next/migration-analysis.md` §4.3

Acceptance slice:
- A reviewer can verify private channels remain identifiable while higher-priority unread/fault/presence states stay visible.

Parallelism:
- Runs after state inventory. Blocks regression proof.

Sensitive paths:
- private-channel visibility, ACL meaning
