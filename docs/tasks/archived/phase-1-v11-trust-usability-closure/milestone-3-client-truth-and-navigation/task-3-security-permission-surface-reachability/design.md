# Design: Settings PermissionsView Reachability

## 1. Data Flow

1. User opens Settings through the existing app shell Settings view.
2. `SettingsPage` renders existing privacy/admin-awareness sections.
3. `SettingsPage` also renders the standalone `PermissionsView` component.
4. `PermissionsView` fetches `/api/v1/me/permissions` with included credentials unless tests inject entries.
5. `PermissionsView` renders loading, error, empty, or capability rows using its existing AP-2 DOM anchors and capability-label helper.

No new server endpoint, route, or shell navigation state is introduced.

## 2. Data Model

No database, server response, or client state model changes. The existing `PermissionEntry` shape from `hooks/usePermissions` remains the rendered data model:

- `permission`: capability token or `*`.
- `scope`: server-provided capability scope.
- `granted_by` and `granted_at`: retained in the data shape, not expanded into a Settings audit product.

## 3. API Contract

The task uses the existing `PermissionsView` default fetch path: `/api/v1/me/permissions`.

- Success: `PermissionsView` consumes `details` and renders rows or empty state.
- Failure: `PermissionsView` renders its existing non-leaky `加载失败` alert.
- Authorization remains server-owned. Settings does not interpret the permission payload as an authorization decision.

## 4. Edge Cases

- Empty permission details render `暂无授权` inside Settings.
- Fetch failure remains an in-surface error state.
- Unknown capability tokens keep forward-compatible rendering through `PermissionsView`.
- Settings privacy/admin-awareness content remains mounted with the permissions surface.
- No role label or compliance copy is introduced in Settings.

## 5. Options Considered

### Option A: Inline `PermissionsView` in the existing privacy tab

Chosen. It is the smallest production-reachability change, keeps Settings as a single-tab user surface, and avoids sidebar/footer or route churn.

### Option B: Add a second Settings tab for permissions

Rejected for this task. It would expand Settings IA and tab state while the task only needs production reachability of an existing surface.

### Option C: Route permissions through AgentManager

Rejected. The task explicitly keeps AgentManager permission-card behavior intact and targets standalone user Settings reachability.

## 6. Integration Points

- `packages/client/src/components/Settings/SettingsPage.tsx`: mount the component without changing app shell navigation.
- `packages/client/src/components/PermissionsView.tsx`: reused as-is.
- `packages/client/src/__tests__/SettingsPage.test.tsx`: coverage for Settings reachability and existing privacy-tab locks.
- `docs/current/client/ui/settings.md`: current behavior update for capability visibility.

## Sensitive-Task Threat Model

- Risk: exposing admin/compliance semantics as a user-facing product. Mitigation: reuse capability surface only and lock absence of compliance/audit-dashboard copy.
- Risk: client-side permission display being mistaken for authorization. Mitigation: no new authorization checks in Settings; server remains authoritative.
- Risk: role/RBAC label bleed. Mitigation: `PermissionsView` continues capability-token rendering and tests keep role labels out of permission rows.

## Design Review

| Role | Decision | Notes |
|---|---|---|
| Architect | LGTM | Reuses existing component and endpoint; no new route/state model. |
| PM | LGTM | Delivers claimed surface reachability without expanding privacy/compliance scope. |
| Security | LGTM | No new authority path; server remains authoritative; no admin SPA mount. |
| QA | LGTM | Testable with Settings jsdom coverage plus existing `PermissionsView` tests. |
