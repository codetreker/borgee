# Design: Production Surface E2E Reverse Proof

## 1. Data Flow

1. Playwright starts the existing server-go and Vite e2e web servers.
2. Test setup logs in through the admin API, mints invites, registers user accounts, and creates private channels through user REST APIs.
3. Browser contexts receive the signed-in user's `borgee_token` cookie and drive the production app shell.
4. Artifact tests navigate sidebar -> channel -> Canvas -> ArtifactPanel and create artifacts through the UI.
5. Settings tests navigate app shell -> Settings and observe the production `PermissionsView` mount.

No new product endpoint, client route, or test-only production hook is introduced.

## 2. Server-Backed Denials

ArtifactComments forbidden proof fulfills the browser's comment list request with a response fetched from the real server under a different unauthorized user's API context. The test records the server's `403` and `comment.cross_channel_reject` code, then asserts the UI renders only the generic forbidden copy.

ArtifactPanel archived-channel proof archives setup data through `PUT /api/v1/channels/{channelId}` because `packages/client/src/lib/api.ts::archiveChannel` delegates to `updateChannel`, and server-go registers the same `PUT` route.

## 3. Test Surface

- `packages/e2e/tests/production-surface-reverse-proof.spec.ts` owns the focused reverse proof.
- Component baselines remain in existing jsdom tests for `ArtifactPanel`, `ArtifactComments`, `PermissionsView`, and `SettingsPage`.
- Current docs record that this e2e spec is a focused product-surface proof, not the general e2e quality platform.

## 4. Edge Cases

- Empty comment lists must render as empty state, not forbidden or unavailable.
- Comment denial must hide the composer.
- Archived-channel artifact creation must not render the rejected title or server archive reason.
- Settings forbidden/error payloads must not leak injected sensitive strings.

## 5. Design Review

| Role | Decision | Notes |
|---|---|---|
| Architect | LGTM | Uses existing server/client contracts and keeps production code unchanged. |
| PM | LGTM | Proves the named surfaces without broadening the quality backlog. |
| Security | LGTM | Server remains the authority; UI assertions lock non-leaky forbidden states. |
| QA | LGTM | Focused e2e plus component baseline gives recoverable failure signals. |

