# task-6-avatar-account-panel-logout

Purpose:
- Make avatar/account the account entry and move logout into that panel.

Scope:
- Add the avatar account panel with account summary and logout behavior.
- Keep account settings expansion out unless a later task explicitly scopes it.

Out of scope:
- No account settings product expansion, privacy/compliance promises, or broad profile redesign.

Depends on:
- `task-5-sidebar-footer-primary-entries`

Blueprint anchors:
- `IA-1`: `docs/blueprint/next/migration-analysis.md` §7.3

Acceptance slice:
- A reviewer can open account behavior from the avatar and log out from the account panel without a separate footer logout control.

Parallelism:
- Can run after task 1. Can run alongside task 3 if account-panel and runtime-entry files are separable.

Sensitive paths:
- auth, account session, logout
