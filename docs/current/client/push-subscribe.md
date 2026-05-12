# Web Push Client Subscribe (DL-4.5) — implementation note

> DL-4.5 (#490) · Phase 4 · blueprint [`client-shape.md`](../../blueprint/current/client-shape.md) L22 (Mobile PWA + Web Push VAPID) + L37 ("没推送 = AI 团队像后台脚本不像同事") + L46 (implementation path).

## 1. 设计

The PWA install client implementation has three parts: service worker registration for the push handler, browser PushManager subscription, and POST to the server.

| 设计点 | 约束 |
|---|---|
| VAPID key holder | client holds the public key; server env holds the private key |
| Browser subscription output | browser generates the public endpoint+p256dh+auth fields |
| Unsubscribe handling | PushManager.unsubscribe + server DELETE keep both sides synchronized; matches blueprint L22 literally |

## 2. service worker (`packages/client/public/sw.js`)

The service worker keeps three categories of event listeners. The cache shell is existing RT-1 behavior; push handling is the DL-4.5 addition.

| Event | 行为 |
|---|---|
| `install` / `activate` / `fetch` | Existing RT-1 cache shell; unchanged |
| `push` | Parse `e.data.json()` payload → render notification (`self.registration.showNotification`); payload shape stays aligned with `internal/push/mention_notifier.go`; `mention` kind renders `${from} mentioned you in #${channel}` + body; `agent_task` kind renders busy/idle state, and busy must include subject per blueprint §1.1 ⭐; unknown kind is ignored without notifying |
| `notificationclick` | Close the notification; prefer focusing an existing SPA tab (clients.matchAll → focus), otherwise `openWindow('/')` enters the SPA route |

**Forbidden behavior**: sw.js must not store secrets or tokens; the service worker renders the payload, and the main thread does not handle push directly. This shares the same privacy boundary as the visibility-based dedup rule in blueprint §1.4.

## 3. pushSubscribe.ts helper (`packages/client/src/lib/pushSubscribe.ts`)

4 export + 1 internal helper:

| Export | 签名 | 行为 |
|---|---|---|
| `isPushSupported()` | `(): boolean` | feature detection; jsdom returns false, browser returns true |
| `getCurrentSubscriptionState()` | `(): 'granted' \| 'denied' \| 'default' \| 'unsupported'` | observability; `Notification.permission` 4-enum |
| `registerServiceWorker()` | `(): Promise<ServiceWorkerRegistration>` | idempotent register `/sw.js` |
| `getActiveSubscription()` | `(): Promise<PushSubscription \| null>` | reads the current PushSubscription, or null when absent |
| `subscribeToPush(vapidPublicKey)` | `(string): Promise<PushSubscription>` | full subscription: permission prompt → PushManager.subscribe → POST server |
| `unsubscribeFromPush()` | `(): Promise<void>` | full unsubscribe: PushManager.unsubscribe + server DELETE |
| `urlBase64ToUint8Array(s)` | `(string): Uint8Array` | W3C VAPID applicationServerKey encoding; - → +, _ → /, padding fix |

POST/DELETE use direct `fetch` instead of `api.ts request<T>`. Push registration runs early in main.tsx before the SPA bootstraps, so this helper must remain a self-contained module.

## 4. ⚠️ 命名边界 — DL-4 vs HB-1 #491

DL-4 PWA endpoint `/api/v1/pwa/manifest` 与 HB-1 #491 `/api/v1/plugin-manifest` 必须在命名和调用范围上保持区分。

| endpoint | 用途 | client 约束 |
|---|---|---|
| `/api/v1/pwa/manifest` | public install prompt | DL-4 PWA scope |
| `/api/v1/plugin-manifest` | dual-signed binary plugin manifest | HB-1 install-butler host-bridge scope, not web SPA scope |

The client side must never call the `plugin-manifest` literal.

## 5. 相关参考

- Implementation: `packages/client/public/sw.js` (push event handler) + `packages/client/src/lib/pushSubscribe.ts` (8 exports)
- Unit tests: `packages/client/src/__tests__/pushSubscribe.test.ts` 6 vitest cases (jsdom feature detection + W3C encoding 4 sub-cases)
- e2e: `packages/e2e/tests/pwa-push-notification-subscribe.spec.ts` 3 cases (manifest W3C real fetch + naming boundary + sw.js push handler text scan)
- spec brief: [`docs/implementation/modules/dl-4-spec.md`](../../implementation/modules/dl-4-spec.md) §1 DL-4.5
- Server side: [`docs/current/server/push.md`](../server/push.md)
