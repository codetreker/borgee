# Admin SPA Details

This document expands the admin SPA browser architecture: entry, auth/session provider, API client, routes/pages, user rail isolation, and frontend-visible safety boundaries. Server endpoint implementation is outside this document and belongs to `server-rail.md`.

## Module Overview

```text
admin.html
  -> src/admin/main.tsx
    -> AdminAuthProvider
      -> AdminApp
        -> BrowserRouter
          -> /admin login or dashboard redirect
          -> /admin/* protected AdminLayout
            -> admin pages call admin/api.ts
```

The admin SPA is a separate React entry in the same Vite build. `admin.html` points to `/src/admin/main.tsx`; Vite builds it as the `admin` Rollup input next to the user `main` input (`packages/client/admin.html`, `packages/client/vite.config.ts`).

## Responsibilities

This module is responsible for the frontend admin session, protected route tree, route-to-page mapping, admin layout/navigation, admin API client shape, and frontend-visible rail isolation (`packages/client/src/admin/main.tsx`, `packages/client/src/admin/auth.ts`, `packages/client/src/admin/AdminApp.tsx`, `packages/client/src/admin/api.ts`, `packages/client/src/admin/pages/*`).

This module is not responsible for server-side admin auth, cookie creation, endpoint ACLs, audit storage, data sanitization, or backend privacy enforcement. It documents only the calls and rendering behavior visible in client source; server details belong to the sibling `server-rail.md` (`packages/client/src/admin/api.ts`).

This module is not responsible for the user SPA's `/api/v1` client, app reducer, `/ws` hook, or feature surfaces. The admin SPA does not mount those modules (`packages/client/src/admin/main.tsx`, `packages/client/src/admin/AdminApp.tsx`, `packages/client/src/App.tsx`, `packages/client/src/context/AppContext.tsx`, `packages/client/src/hooks/useWebSocket.ts`).

## Entry And Providers

`packages/client/src/admin/main.tsx` imports `../index.css`, wraps `<AdminApp />` with `<AdminAuthProvider>`, and renders under `React.StrictMode`. It does not register `/sw.js` and does not import the user `App` (`packages/client/src/admin/main.tsx`, `packages/client/src/main.tsx`).

`AdminAuthProvider` is the sole admin session provider. It stores `session: AdminSession | null` and `checked: boolean`, runs `refresh()` on mount, and exposes `login`, `logout`, and `refresh` through `useAdminAuth()` (`packages/client/src/admin/auth.ts`, `packages/client/src/admin/api.ts`).

The admin session shape is `{ id, login }`. The admin API client comments explicitly lock this to the server `/auth/me` shape and do not model role, username, token, or expiry fields; the browser carries the admin cookie by using `credentials: 'include'` (`packages/client/src/admin/api.ts`, `packages/client/src/admin/auth.ts`).

```text
LoginPage
  -> useAdminAuth().login(login, password)
  -> POST /admin-api/v1/auth/login
  -> GET  /admin-api/v1/auth/me
  -> session { id, login }
  -> navigate /admin/dashboard
```

Logout calls `adminLogout()` and clears local session in a `finally` block, so the browser session state is cleared even if the logout request errors (`packages/client/src/admin/auth.ts`, `packages/client/src/admin/pages/SettingsPage.tsx`, `packages/client/src/admin/AdminApp.tsx`).

## Admin API Client

`packages/client/src/admin/api.ts` is the admin API boundary. It sets `BASE = '/admin-api/v1'`, includes cookies, sets JSON `Content-Type` for non-`FormData` bodies, parses non-2xx JSON error bodies when possible, and throws `AdminApiError` (`packages/client/src/admin/api.ts`).

| API group | Functions | Frontend boundary | Evidence |
| --- | --- | --- | --- |
| Auth/session | `adminLogin`, `adminLogout`, `fetchAdminMe` | Cookie-backed admin login, logout, and session refresh. | `packages/client/src/admin/api.ts`, `packages/client/src/admin/auth.ts`, `packages/client/src/admin/pages/LoginPage.tsx` |
| Stats | `fetchStats` | Dashboard count cards and optional org debug rows. | `packages/client/src/admin/api.ts`, `packages/client/src/admin/pages/DashboardPage.tsx` |
| Users | `fetchUsers`, `createUser`, `patchUser`, `deleteUser`, `fetchUserAgents` | User list, create member, disable/enable, delete, detail page, password/role/agent rows. | `packages/client/src/admin/api.ts`, `packages/client/src/admin/pages/UsersPage.tsx`, `packages/client/src/admin/pages/UserDetailPage.tsx` |
| Permissions | `fetchUserPermissions`, `grantUserPermission`, `revokeUserPermission` | Capability grant/revoke table on user detail. | `packages/client/src/admin/api.ts`, `packages/client/src/admin/pages/UserDetailPage.tsx` |
| Channels | `fetchChannels`, `forceDeleteChannel` | Channel metadata table and force delete for eligible rows. | `packages/client/src/admin/api.ts`, `packages/client/src/admin/pages/ChannelsPage.tsx` |
| Invites | `fetchInvites`, `createInvite`, `deleteInvite` | Invite code list/create/revoke. | `packages/client/src/admin/api.ts`, `packages/client/src/admin/pages/InvitesPage.tsx` |
| Audit log | `fetchAdminAuditLog` | Filterable admin action rows with metadata string. | `packages/client/src/admin/api.ts`, `packages/client/src/admin/pages/AdminAuditLogPage.tsx` |
| Multi-source audit | `fetchMultiSourceAudit` | Source-filtered audit rows from server/plugin/host_bridge/agent. | `packages/client/src/admin/api.ts`, `packages/client/src/admin/pages/MultiSourceAuditPage.tsx` |
| Runtimes | `fetchAdminRuntimes` | Read-only runtime metadata table. | `packages/client/src/admin/api.ts`, `packages/client/src/admin/pages/RuntimesPage.tsx` |
| Heartbeat lag | `fetchAdminHeartbeatLag` | Read-only lag snapshot. | `packages/client/src/admin/api.ts`, `packages/client/src/admin/pages/HeartbeatLagPage.tsx` |
| Archived channels | `fetchAdminArchivedChannels` | Read-only archived channel list. | `packages/client/src/admin/api.ts`, `packages/client/src/admin/pages/ArchivedChannelsPage.tsx` |
| Description history | `fetchAdminChannelDescriptionHistory` | Read-only channel description history rows. | `packages/client/src/admin/api.ts`, `packages/client/src/admin/pages/ChannelDescriptionHistoryPage.tsx` |

## Routes And Pages

`AdminApp` creates a `BrowserRouter`. `/admin` renders `LoginPage` when there is no session and redirects to `/admin/dashboard` when authenticated. `/admin/*` renders `AdminLayout` only when a session exists; otherwise it redirects to `/admin` (`packages/client/src/admin/AdminApp.tsx`, `packages/client/src/admin/pages/LoginPage.tsx`).

`AdminLayout` owns the admin sidebar and nested page routes. It displays the current `session.login`, provides a logout button, and does not reuse the user `Sidebar` component or the user `mainView` model (`packages/client/src/admin/AdminApp.tsx`, `packages/client/src/components/Sidebar.tsx`, `packages/client/src/lib/mainView.ts`).

| Route | Page | Responsibility | Evidence |
| --- | --- | --- | --- |
| `/admin/dashboard` | `DashboardPage` | Global stats and optional org debug rows. | `packages/client/src/admin/AdminApp.tsx`, `packages/client/src/admin/pages/DashboardPage.tsx` |
| `/admin/users` | `UsersPage` | User list, create member user, disable/enable, delete non-admin users, link to detail. | `packages/client/src/admin/AdminApp.tsx`, `packages/client/src/admin/pages/UsersPage.tsx` |
| `/admin/users/:id` | `UserDetailPage` | User detail, password reset, role change, disabled toggle, permissions, owned agents. | `packages/client/src/admin/AdminApp.tsx`, `packages/client/src/admin/pages/UserDetailPage.tsx` |
| `/admin/channels` | `ChannelsPage` | Channel metadata and force delete where UI allows. | `packages/client/src/admin/AdminApp.tsx`, `packages/client/src/admin/pages/ChannelsPage.tsx` |
| `/admin/channels-archived` | `ArchivedChannelsPage` | Read-only archived channels; links to description history. | `packages/client/src/admin/AdminApp.tsx`, `packages/client/src/admin/pages/ArchivedChannelsPage.tsx` |
| `/admin/channels/:id/description-history` | `ChannelDescriptionHistoryPage` | Read-only channel description edit history rows. | `packages/client/src/admin/AdminApp.tsx`, `packages/client/src/admin/pages/ChannelDescriptionHistoryPage.tsx` |
| `/admin/runtimes` | `RuntimesPage` | Read-only runtime metadata. | `packages/client/src/admin/AdminApp.tsx`, `packages/client/src/admin/pages/RuntimesPage.tsx` |
| `/admin/heartbeat-lag` | `HeartbeatLagPage` | Read-only heartbeat lag snapshot. | `packages/client/src/admin/AdminApp.tsx`, `packages/client/src/admin/pages/HeartbeatLagPage.tsx` |
| `/admin/invites` | `InvitesPage` | Invite code list/create/revoke. | `packages/client/src/admin/AdminApp.tsx`, `packages/client/src/admin/pages/InvitesPage.tsx` |
| `/admin/audit-log` | `AdminAuditLogPage` | Filterable admin action audit table with English enum actions. | `packages/client/src/admin/AdminApp.tsx`, `packages/client/src/admin/pages/AdminAuditLogPage.tsx` |
| `/admin/audit-multi-source` | `MultiSourceAuditPage` | Source-filtered multi-source audit table. | `packages/client/src/admin/AdminApp.tsx`, `packages/client/src/admin/pages/MultiSourceAuditPage.tsx` |
| `/admin/settings` | `SettingsPage` | Current admin session summary and logout. | `packages/client/src/admin/AdminApp.tsx`, `packages/client/src/admin/pages/SettingsPage.tsx` |

## User Rail Isolation

The admin rail is separated from the user rail by entry point, provider, API base, route tree, and realtime behavior.

| Boundary | Admin side | User side | Evidence |
| --- | --- | --- | --- |
| HTML entry | `admin.html` | `index.html` | `packages/client/admin.html`, `packages/client/index.html`, `packages/client/vite.config.ts` |
| React entry | `src/admin/main.tsx` | `src/main.tsx` | `packages/client/src/admin/main.tsx`, `packages/client/src/main.tsx` |
| Provider | `AdminAuthProvider` | `ThemeProvider`, `AppProvider`, `ToastProvider` | `packages/client/src/admin/main.tsx`, `packages/client/src/App.tsx`, `packages/client/src/context/AppContext.tsx` |
| API client | `src/admin/api.ts`, base `/admin-api/v1` | `src/lib/api.ts`, `/api/v1/*` paths | `packages/client/src/admin/api.ts`, `packages/client/src/lib/api.ts` |
| Routing/state | `BrowserRouter` under `/admin/*` | `mainView` plus selected channel state; no user router in `App.tsx` | `packages/client/src/admin/AdminApp.tsx`, `packages/client/src/App.tsx`, `packages/client/src/lib/mainView.ts` |
| Realtime | No admin WS hook mounted | `useWebSocket` mounted by user `AppInner` | `packages/client/src/admin/main.tsx`, `packages/client/src/admin/AdminApp.tsx`, `packages/client/src/App.tsx`, `packages/client/src/hooks/useWebSocket.ts` |

Admin pages import from `../api`; user features import from `../lib/api` or `./lib/api`. This keeps admin-only calls out of the user API client and user-owned admin-awareness calls out of the admin client (`packages/client/src/admin/pages/*.tsx`, `packages/client/src/admin/api.ts`, `packages/client/src/lib/api.ts`).

## Metadata And Safety Boundaries Visible In Client Code

The strongest frontend-visible boundary is that admin pages render metadata tables and explicit admin operations, not the user chat/workspace/artifact surfaces. Admin audit rows render `metadata` as a JSON string; the admin API type comment states server omits body/content/text/artifact fields from that metadata (`packages/client/src/admin/api.ts`, `packages/client/src/admin/pages/AdminAuditLogPage.tsx`).

Runtime visibility is read-only metadata in the admin SPA. `AdminRuntime` includes id, agent id, endpoint URL, process kind, status, heartbeat, and timestamps; the type comment says `last_error_reason` is omitted, and `RuntimesPage` only renders the listed fields (`packages/client/src/admin/api.ts`, `packages/client/src/admin/pages/RuntimesPage.tsx`).

Archived channels and description history are read-only in the current admin UI. `ArchivedChannelsPage` lists archived rows and links to description history; `ChannelDescriptionHistoryPage` renders `old_content`, timestamp, and reason for description edits only (`packages/client/src/admin/pages/ArchivedChannelsPage.tsx`, `packages/client/src/admin/pages/ChannelDescriptionHistoryPage.tsx`, `packages/client/src/admin/api.ts`).

Powerful admin mutations are concentrated in explicit pages and explicit `/admin-api/v1` calls: user create/delete/disable/role/password/permission changes, channel force delete, and invite create/revoke (`packages/client/src/admin/api.ts`, `packages/client/src/admin/pages/UsersPage.tsx`, `packages/client/src/admin/pages/UserDetailPage.tsx`, `packages/client/src/admin/pages/ChannelsPage.tsx`, `packages/client/src/admin/pages/InvitesPage.tsx`).

User-facing admin-awareness is intentionally on the user rail. Settings loads `/api/v1/me/admin-actions` and `/api/v1/me/impersonation-grant`, while the admin SPA audit page uses `/admin-api/v1/audit-log`. The user privacy UI states that admins see metadata and not message/file/artifact contents unless the user grants temporary impersonation; keep that UI aligned with server behavior when endpoint shapes change (`packages/client/src/lib/api.ts`, `packages/client/src/components/Settings/PrivacyPromise.tsx`, `packages/client/src/components/Settings/SettingsPage.tsx`, `packages/client/src/admin/api.ts`, `packages/client/src/admin/pages/AdminAuditLogPage.tsx`).

The admin SPA code read here has no route or page for impersonation. The user rail has the visible grant banner and grant/revoke controls (`packages/client/src/admin/AdminApp.tsx`, `packages/client/src/components/Settings/BannerImpersonate.tsx`, `packages/client/src/components/Settings/ImpersonateGrantSection.tsx`, `packages/client/src/lib/api.ts`).

## Interfaces To Other Modules

| Interface | Contract | Evidence |
| --- | --- | --- |
| Vite build | Admin is a second HTML entry in the client package. | `packages/client/vite.config.ts`, `packages/client/admin.html` |
| Server admin rail | Admin pages call `/admin-api/v1`; server behavior belongs to the sibling `server-rail.md`. | `packages/client/src/admin/api.ts` |
| Shared CSS | Admin entry imports `../index.css`; component/provider logic stays separate. | `packages/client/src/admin/main.tsx`, `packages/client/src/index.css` |
| User privacy surface | User settings mirrors admin impact through user-owned endpoints, not admin API calls. | `packages/client/src/components/Settings/SettingsPage.tsx`, `packages/client/src/lib/api.ts`, `packages/client/src/admin/api.ts` |
