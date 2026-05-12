# Build PWA Cache

This document covers Vite build configuration, dual HTML entries, package scripts, dev proxy, PWA manifest, service worker behavior, and cache boundaries for the client package.

## Module Overview

```text
packages/client/vite.config.ts
  inputs: index.html -> src/main.tsx -> user SPA
          admin.html -> src/admin/main.tsx -> admin SPA
  dev proxy: /api /admin-api /health /uploads /ws

User PWA:
  index.html links manifest
  src/main.tsx registers /sw.js
  public/sw.js caches shell and same-origin GET responses except /api and /ws
```

The client package is a Vite React app with two Rollup HTML inputs: `main: 'index.html'` and `admin: 'admin.html'`. Build output goes to `dist` with sourcemaps enabled (`packages/client/vite.config.ts`).

The package scripts are `dev`, `build`, `build:dev`, `preview`, `typecheck`, and `test`. `build` runs `tsc -b && vite build`; `typecheck` runs `tsc --noEmit`; tests run through Vitest (`packages/client/package.json`, `packages/client/vitest.config.ts`, `packages/client/tsconfig.json`).

## Responsibilities

This module is responsible for documenting how the frontend is built, how user and admin entries are separated at HTML/build level, how dev proxying works, and what the current service worker caches (`packages/client/vite.config.ts`, `packages/client/index.html`, `packages/client/admin.html`, `packages/client/public/sw.js`).

This module is not responsible for backend serving, deployment topology, CDN policy, or server cache headers. It only reflects the frontend source configuration in `packages/client` (`packages/client/vite.config.ts`, `packages/client/public/sw.js`).

This module is not responsible for admin SPA route/session details. The admin build entry is noted here because Vite builds it, but admin architecture is documented in `../admin/README.md` and `../admin/spa.md` (`packages/client/admin.html`, `packages/client/src/admin/main.tsx`, `packages/client/src/admin/AdminApp.tsx`).

## Vite Dual Entry

`packages/client/vite.config.ts` installs `@vitejs/plugin-react`, sets dev server port `5173`, and configures `rollupOptions.input` with both user and admin HTML files (`packages/client/vite.config.ts`).

`packages/client/index.html` loads `/src/main.tsx`, links `/manifest.json`, sets Apple mobile metadata, and references the app icons. This is the only entry that registers the service worker because service-worker registration is in `src/main.tsx` (`packages/client/index.html`, `packages/client/src/main.tsx`).

`packages/client/admin.html` loads `/src/admin/main.tsx`, has favicon/theme metadata, and does not link `/manifest.json`. The admin entry imports shared `../index.css` but mounts `AdminAuthProvider` and `AdminApp`, not the user providers (`packages/client/admin.html`, `packages/client/src/admin/main.tsx`).

## Dev Proxy

The Vite dev proxy target is `process.env.VITE_E2E_API_TARGET ?? 'http://localhost:4900'`. `/api`, `/admin-api`, `/health`, and `/uploads` proxy to that HTTP target; `/ws` proxies to the corresponding `ws:` target with WebSocket support (`packages/client/vite.config.ts`).

This proxy setup means the user REST client can keep `BASE = ''` and call `/api/v1/*`, while the admin REST client can keep `BASE = '/admin-api/v1'` (`packages/client/src/lib/api.ts`, `packages/client/src/admin/api.ts`, `packages/client/vite.config.ts`).

## PWA Manifest

The manifest declares `name` and `short_name` as `Borgee`, `start_url: '/'`, `display: 'standalone'`, background/theme colors, and SVG icons for 192, 512, and maskable favicon purposes (`packages/client/public/manifest.json`, `packages/client/index.html`).

Because `admin.html` does not link the manifest and `src/admin/main.tsx` does not register the service worker, the PWA install path is tied to the user entry in current code (`packages/client/admin.html`, `packages/client/src/admin/main.tsx`, `packages/client/src/main.tsx`).

## Service Worker Cache

`public/sw.js` uses cache name `borgee-v1` and precaches `['/', '/index.html']` on install. Activation deletes caches whose key is not `borgee-v1` and claims clients (`packages/client/public/sw.js`).

The fetch handler only handles `GET` requests. It skips paths starting with `/api` and `/ws`, performs a network-first fetch for other requests, caches successful same-origin responses, and falls back to a matching cache entry or `/index.html` when the network request fails (`packages/client/public/sw.js`).

The current skip list does not explicitly skip `/admin-api`. Because `/admin-api` starts with `/admin-api`, not `/api`, a same-origin `GET /admin-api/*` response could be intercepted and cached by this service worker if the admin page is controlled by it. This is a code-derived boundary to keep in mind when changing service-worker scope or admin entry behavior (`packages/client/public/sw.js`, `packages/client/admin.html`, `packages/client/src/main.tsx`).

## Push Notifications

The service worker handles push payloads with `kind: 'mention'` and `kind: 'agent_task'`. It renders notifications with icon `/icons/icon-192.svg`, badge `/favicon.svg`, and a `tag` equal to the payload kind so same-kind notifications collapse (`packages/client/public/sw.js`).

Notification clicks focus an existing window client when one exists; otherwise the service worker opens `/` (`packages/client/public/sw.js`).

## Cache And API Boundaries

| Area | Current behavior | Evidence |
| --- | --- | --- |
| User app shell | `src/main.tsx` registers `/sw.js`; service worker precaches `/` and `/index.html`. | `packages/client/src/main.tsx`, `packages/client/public/sw.js` |
| User REST | `/api` is skipped by the service worker and proxied in dev. | `packages/client/public/sw.js`, `packages/client/vite.config.ts` |
| WebSocket | `/ws` is skipped by the service worker and proxied as WebSocket in dev. | `packages/client/public/sw.js`, `packages/client/vite.config.ts` |
| Upload serving | `/uploads` is proxied in dev and is not in the service-worker skip list. | `packages/client/vite.config.ts`, `packages/client/public/sw.js` |
| Admin REST | `/admin-api` is proxied in dev but not explicitly skipped by the service worker. | `packages/client/vite.config.ts`, `packages/client/public/sw.js`, `packages/client/src/admin/api.ts` |
| Admin entry | `admin.html` is a Vite input but has no manifest link and no SW registration in its entry. | `packages/client/admin.html`, `packages/client/src/admin/main.tsx`, `packages/client/vite.config.ts` |

## Interfaces To Other Modules

The build module interfaces with the user SPA through `index.html` and `src/main.tsx`, with the admin SPA through `admin.html` and `src/admin/main.tsx`, with backend rails through Vite proxy paths, and with browser offline/push behavior through `public/sw.js` and `public/manifest.json` (`packages/client/vite.config.ts`, `packages/client/index.html`, `packages/client/admin.html`, `packages/client/public/sw.js`, `packages/client/public/manifest.json`).
