# Content Lock: Settings PermissionsView Reachability

## UI Literals

- Existing Settings tab label remains byte-identical: `隐私`.
- Existing `PermissionsView` state literals remain byte-identical when rendered through Settings:
  - Loading: `加载中`
  - Error: `加载失败`
  - Empty: `暂无授权`
  - Wildcard permission: `完整能力`

## DOM Anchors

- Settings root remains `[data-page="settings"]`.
- Privacy tab remains `[data-tab="privacy"]` with active class and `aria-current="page"`.
- `PermissionsView` anchors remain owned by `PermissionsView`:
  - `[data-ap2-permissions-view]`
  - `[data-ap2-capability-row]`
  - `[data-ap2-capability-token]`
  - `[data-ap2-scope]`
  - `[data-ap2-known]`
  - `[data-ap2-loading]`
  - `[data-ap2-error]`
  - `[data-ap2-empty]`

## Locked Absences

- Do not introduce new Settings text for `GDPR`, `DPA`, `compliance center`, `audit viewer`, or `privacy dashboard`.
- Do not add role labels as the user-facing permission model.
