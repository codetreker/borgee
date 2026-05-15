# task-3-security-permission-surface-reachability

Purpose:
- Make the user permission surface reachable where the product already claims capability visibility.

Scope:
- Mount `PermissionsView` from `packages/client/src/components/PermissionsView.tsx` under the user Settings surface (`packages/client/src/components/Settings/SettingsPage.tsx`) or an equivalent user settings route chosen during task design.
- Keep existing AgentManager permission-card behavior intact; this task is about the standalone user permission surface.
- Preserve backend authority, admin/privacy/security controls, and the no-new-compliance-product guardrail.

Out of scope:
- No new privacy dashboard, compliance center, GDPR/DPA workflow, user-facing audit surface, or impersonation consent product expansion.

Depends on:
- Canonical Milestone 3 start

Blueprint anchors:
- `CT-1`: `docs/blueprint/next/migration-analysis.md` §5.3
- `PS-1`: `docs/blueprint/next/migration-analysis.md` §6.1

Acceptance slice:
- A reviewer can reach the user PermissionsView from production Settings and see capability entries or truthful empty/error state without adding new user-facing compliance scope or weakening backend controls.

Parallelism:
- Can run independently of task 1 if Settings files are separable. Blocks task 4.

Sensitive paths:
- admin, auth, privacy, security surface visibility
