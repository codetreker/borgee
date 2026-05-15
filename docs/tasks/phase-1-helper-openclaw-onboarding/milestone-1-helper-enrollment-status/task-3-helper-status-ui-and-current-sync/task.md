# task-3-helper-status-ui-and-current-sync

Purpose:
- Make Helper enrollment status truthful to users and record accepted status contracts in current docs after implementation.

Scope:
- Surface connected/offline/last-seen/revoked/uninstalled status and allowed job categories without implying Configure OpenClaw success.
- Sync `docs/current` for UI/status behavior accepted by this task; task 1 owns current-doc sync for the enrollment identity/status foundation.

Out of scope:
- No job progress UI, bounded logs, terminal job states, or OpenClaw connected state.
- No privacy/compliance product expansion.

Depends on:
- `task-1-helper-enrollment-model-and-status`

Blueprint anchors:
- `HB-RA-1A`: `docs/blueprint/next/remote-actuator-design.md` §1.2
- `HB-RA-1B`: `docs/blueprint/next/remote-actuator-design.md` §11
- `PS-1`: `docs/blueprint/next/migration-analysis.md` §6.1

Acceptance slice:
- A reviewer can distinguish Helper connected, offline, revoked, and uninstalled states in the product surface without leaking private content or implying configuration success.

Parallelism:
- Can run after task 1. Can run alongside task 2 if UI/current-doc files are separate.

Sensitive paths:
- auth, privacy, status visibility, current-doc sync
