# Spec: Settings PermissionsView Reachability

## 0. Task Contract

- Source task: `task-3-security-permission-surface-reachability/task.md`.
- Canonical milestone: `docs/tasks/phase-1-v11-trust-usability-closure/milestone-3-client-truth-and-navigation`.
- Blueprint anchors: `CT-1` in `docs/blueprint/next/migration-analysis.md` section 5.3 and `PS-1` in section 6.1.

## 1. Constraints

- Mount the existing `PermissionsView` under the user Settings surface in `packages/client/src/components/Settings/SettingsPage.tsx`.
- The surface remains on the user rail and uses the existing `/api/v1/me/permissions` behavior owned by `PermissionsView`.
- Do not change `packages/client/src/components/PermissionsView.tsx` capability rendering semantics unless a test proves reachability requires it.
- Keep AgentManager permission cards and admin SPA settings out of scope.
- Do not add a privacy dashboard, compliance center, audit viewer, GDPR/DPA workflow, impersonation-consent expansion, or legal/policy copy.

## 2. Segments

### 2.1 Settings Mount

User Settings renders the standalone `PermissionsView` in the existing Settings content area so a signed-in user can reach their capability entries from production Settings.

### 2.2 Truthful Permission States

The mounted view preserves `PermissionsView` loading, empty, error, and capability-row states. Settings must not hide the empty/error states behind privacy/admin-awareness content.

### 2.3 Authority And Scope Guard

The Settings mount does not create a new authorization decision. Backend permission authority remains unchanged, role labels stay out of the user UI, and the task does not broaden privacy/compliance product scope.

## 3. Carry-Over

- Existing Settings privacy/admin-awareness content remains reachable.
- `PermissionsView` capability rows continue to use capability token data attributes and `capabilityLabel` SSOT behavior.
- Production shell navigation to Settings remains through the existing Settings view; this task does not change sidebar/footer IA.

## 4. Reverse Checks

- `packages/client/src/__tests__/SettingsPage.test.tsx` proves Settings renders the standalone permission surface and keeps the existing privacy tab behavior.
- Current docs describe Settings as including user-owned capability visibility without treating it as an admin audit/compliance surface.
- Grep review confirms this task does not touch ArtifactComments or sidebar/footer implementation files.
