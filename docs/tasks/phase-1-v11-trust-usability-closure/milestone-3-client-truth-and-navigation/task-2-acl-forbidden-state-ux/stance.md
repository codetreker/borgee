# PM Stance: ACL Forbidden-State UX

## Claim

Production client surfaces must distinguish authorized empty/loading states from denied or unavailable states. A blank list, fake empty state, or server error body is not acceptable when a server ACL check fails.

## Required Signals

- `ArtifactPanel` must clear already-rendered artifact content when a later authoritative artifact reload returns 401 or 403.
- `ArtifactComments` must show loading before the comment list succeeds, and must show a local forbidden state when the comment list or composer is denied.
- Settings `PermissionsView` must show a local forbidden state for denied capability visibility without treating the client display as authorization.
- Forbidden copy may reveal that the current user lacks access to the named surface. It must not include channel names, artifact titles, message bodies, file names, permission grant internals, or server error bodies from the denied response.

## Boundaries

- Server ACL, permissions, artifact, and comment APIs remain authoritative.
- No redirect or full-page denial shell is introduced for these local surfaces.
- No new privacy dashboard, compliance center, audit viewer, GDPR/DPA workflow, sidebar IA change, account panel change, or Task4 e2e expansion is included.
