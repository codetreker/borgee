# Dev Design: ACL Forbidden-State UX

## Approach

Use local component state on the three already-mounted surfaces. This keeps server ACL authority unchanged and avoids a global redirect that would hide which embedded surface failed.

## Data Flow

1. `ArtifactPanel` continues to load authoritative artifact head/version data through existing API helpers. If reload receives 401 or 403, it clears protected local state and renders `[data-artifact-forbidden]`.
2. `ArtifactComments` tracks `loading`, `ready`, `forbidden`, and `unavailable` list states around `listArtifactComments`. It only renders the empty state after a successful list response and hides the composer while denied or unavailable.
3. `PermissionsView` preserves its existing fetch path and capability rendering. A local fetch error wrapper carries only status, so 401/403 can render `[data-ap2-forbidden]` without parsing or displaying response bodies.

## Files

- `packages/client/src/components/ArtifactPanel.tsx`
- `packages/client/src/components/ArtifactComments.tsx`
- `packages/client/src/components/PermissionsView.tsx`
- `packages/client/src/index.css`
- Focused tests under `packages/client/src/__tests__/` for the three surfaces.
- Task four-piece, content lock, progress, acceptance, and current client docs.

## Alternatives Considered

- Global forbidden route: rejected because Task2 asks for local/in-surface states and the surfaces are embedded in production UI.
- Keep generic error only: rejected because a generic fetch failure does not distinguish denied access from transient unavailability.
- Render server error bodies: rejected because denied responses are not a safe source for user-facing protected context.

## Verification Plan

- Run the focused red/green Vitest set for ArtifactPanel, ArtifactComments, PermissionsView, and SettingsPage.
- Run client typecheck and build.
- Run `git diff --check`.
