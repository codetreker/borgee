# Stance: Settings PermissionsView Reachability

## Product Stances

- `CT-1`: If Borgee already has a user permission/capability surface, it must be reachable from production Settings rather than existing only as an unmounted component.
- `CT-1`: Reachability must be truthful. Empty, loading, error, and capability-row states remain visible states, not blank screens or fake success.
- `PS-1`: gh#654 does not remove existing security/capability controls, but it also does not authorize new user-facing privacy or compliance product surfaces.
- This task exposes capability visibility, not role management. The UI must not present RBAC role names as the user's authority model.
- Settings remains a user SPA surface. It does not mount the admin SPA and does not show admin-wide audit data.
- Existing Settings privacy/admin-awareness content stays intact; the task adds reachability for permissions without rewriting account/sidebar IA.

## Anti-Constraints

- Do not build a privacy dashboard, compliance center, GDPR/DPA workflow, legal policy page, audit viewer, or impersonation-consent expansion.
- Do not change ArtifactComments, ArtifactPanel, sidebar footer entries, avatar/account panel, or Helper/Remote Nodes placement.
- Do not add client-side authorization. Server ACL/capability checks remain authoritative.
- Do not make AgentManager permission-card behavior depend on this Settings mount.

## Blacklist Grep

- Implementation should not add new Settings text containing `GDPR`, `DPA`, `compliance center`, `audit viewer`, or `privacy dashboard`.
- Settings reachability tests should not assert admin SPA selectors or AgentManager permission-card selectors.
