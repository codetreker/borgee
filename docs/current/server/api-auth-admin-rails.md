# API Auth Admin Rails

## Role

The server exposes multiple authority rails because the actors are not interchangeable. A browser user, an admin operator, an agent plugin, and a remote node all interact with the same server process, but they carry different credentials, scopes, session lifetimes, and privacy expectations.

This document describes those rails as architecture boundaries. It is not an endpoint manual. The important design point is isolation: a credential accepted on one rail must not silently become authority on another rail.

```mermaid
flowchart TB
  user[User rail]
  admin[Admin rail]
  plugin[Plugin rail]
  remote[Remote rail]
  caps[Capability and ownership checks]
  adminSession[Admin session lookup]
  pluginState[Plugin session and BPP dispatch]
  remoteState[Remote node binding]
  store[(Server store)]

  user --> caps --> store
  admin --> adminSession --> store
  plugin --> pluginState --> store
  remote --> remoteState --> store
  pluginState --> caps
```

## Boundary

The user rail authenticates users and agents as product actors. It authorizes actions with user permission rows, ownership checks, channel membership, organization boundaries, and handler-specific predicates.

The admin rail authenticates operational admins through separate admin sessions. It is not a privileged user session. Admin surfaces are mounted explicitly and are biased toward metadata, audit, and operational controls rather than content access.

The plugin rail authenticates an agent plugin by API key and then treats the connection as an agent-owned runtime channel. It can proxy API requests and send BPP frames, but accepted plugin events still pass through protocol validation and owner checks.

The remote rail authenticates remote nodes by connection token. It represents a user-owned machine connection, not a user browser session and not an admin session.

## Collaborators

The user auth subsystem resolves user identity from session cookies, bearer API keys, and development-only bypasses. It attaches a user object to request context for the application layer.

The capability subsystem reads permission rows and resolves resource scopes. It is the shared policy layer for capability-shaped decisions, while some resources also require owner or membership checks inside their domain handlers.

The admin subsystem owns admin login, session creation, session resolution, logout, and admin context. It deliberately avoids depending on the user auth subsystem for admin identity. Admin bootstrap still requires a bcrypt hash with cost at least 10; the only fast path is the explicit Playwright-only `BORGEE_TEST_FAST_ADMIN_PASSWORD` hook, which bypasses repeated e2e compare cost without changing production/default login behavior.

The REST application layer is the consumer of rail identity. Handlers decide whether an operation is user-owned, member-scoped, permission-scoped, admin-scoped, or read-only metadata.

The realtime hub owns plugin and remote connection registration. It validates credentials at socket entry and gives the rest of the server a live transport for fanout or proxy requests.

## Internal Architecture

User authentication accepts a product session cookie or an API key. The resulting identity is a row from the user aggregate, including both humans and agents. Disabled or deleted users are rejected before handler logic runs.

User authorization has two layers. Coarse capability checks compare requested permission and scope against permission rows, with wildcard handling and organization-aware scope resolution. Domain handlers then add ownership, channel membership, private-channel visibility, artifact ownership, or agent-owner rules where those semantics are not expressible as a simple capability row.

Channel management is one of those domain-handler authority layers. User-rail channel mutation routes do not treat a wildcard or scoped permission row as sufficient by itself: creators cannot leave their own channels, non-members cannot leave or manage channels, delete/archive require the authenticated user to be the channel creator, member management cannot remove the channel creator, and cross-org management attempts fail closed before mutating membership or ownership state.

Admin authentication uses an opaque admin session cookie. The cookie value is a session token that must resolve to a live admin session row and then an admin row. This is intentionally separate from product user JWTs and API keys.

Admin authorization is route-based. If a request reaches an admin route, admin session middleware establishes admin identity; the handler surface itself determines what an admin can do. The admin rail does not become a product user context.

Plugin authentication is socket-specific. The plugin uses an agent API key to establish a live connection, then sends RPC envelope frames or BPP event frames. RPC proxying can call the HTTP application surface, while BPP frames are dispatched through protocol handlers with owner validation.

Remote authentication is token-specific to a remote node. A live remote connection is associated with a registered node and its owning user, and later REST calls can use that live connection to proxy node operations.

## Key Flows

User request flow: a browser or API client presents a product session or API key, the server resolves a user, capability or domain checks validate the action, and the handler reads or mutates product state. Side effects may include audit rows, realtime fanout, push, or event publication.

Admin request flow: an admin client presents the admin session cookie, the server resolves the admin session, and the admin handler returns metadata or performs an explicitly mounted operational action. Admin audit surfaces and user-facing audit surfaces are different projections.

Plugin connection flow: an agent plugin presents an API key, the hub registers the plugin connection, RPC frames can be proxied into the application surface, and BPP frames are routed through typed dispatchers. Task lifecycle frames can update live agent task state and fan out to clients.

Remote connection flow: a remote node presents a connection token, the hub registers it under the remote-node identity, and user-owned remote routes can proxy requests to that connection when it is online. The node create route (`POST /api/v1/remote/nodes`) surfaces that connection token exactly once, as a sibling `connection_token` field on the create response; node list/status responses never carry it (the `RemoteNode.ConnectionToken` column is `json:"-"`), so it cannot be re-read from a stored row. The operator pastes it into `npx @codetreker/borgee-remote-agent install --server <wss://host> --token <token> --dirs <dirs>`.

Impersonation/audit flow: user-facing grant state lives on the user rail, while admin-facing audit views live on the admin rail. This split prevents a user credential from reading global audit state and prevents admin context from masquerading as the user rail without an explicit grant model.

## Invariants

- User sessions and admin sessions use different cookies, tables, and context values.
- User permission checks do not have an admin-role shortcut.
- Admin routes are mounted on the admin rail; legacy user-rail admin mounts are not part of the active architecture.
- Agent wildcard capability is narrower than human wildcard behavior.
- Plugin frames are not trusted merely because the socket is connected; protocol validation and owner checks still apply.
- Remote-node tokens authenticate machines, not browser users or admins.
- Admin metadata views must avoid content-bearing fields unless a route explicitly owns that disclosure.
- Every JSON request-body decode (including the unauthenticated register / login / admin-login rails) is bounded to 1 MiB via `http.MaxBytesReader`; an over-limit body is rejected with 413 before it is buffered, so an unauthenticated caller cannot drive memory exhaustion with a giant body. This is the input-side DoS boundary — the app layer is the only enforcement point, as there is no edge proxy body limit.

## Non-Goals

Rails do not define every endpoint, replace domain-specific authorization, or merge admin authority into user authority.

## Implementation Anchors

- `packages/server-go/internal/server/server.go`
- `packages/server-go/internal/auth/middleware.go`
- `packages/server-go/internal/auth/permissions.go`
- `packages/server-go/internal/auth/abac.go`
- `packages/server-go/internal/admin/auth.go`
- `packages/server-go/internal/api/request_helpers.go`
- `packages/server-go/internal/admin/middleware.go`
- `packages/server-go/internal/api/admin.go`
- `packages/server-go/internal/api/admin_endpoints.go`
- `packages/server-go/internal/api/admin_audit_query.go`
- `packages/server-go/internal/api/runtimes.go`
- `packages/server-go/internal/ws/client.go`
- `packages/server-go/internal/ws/plugin.go`
- `packages/server-go/internal/ws/remote.go`
- `packages/server-go/internal/bpp/plugin_frame_dispatcher.go`
- `packages/server-go/internal/bpp/envelope.go`
- `packages/server-go/internal/store/queries.go`
- `packages/server-go/internal/store/admin_actions.go`
- `admin.Handler`
