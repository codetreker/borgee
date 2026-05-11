# Web Push Client Subscribe (DL-4.5) — implementation note

> DL-4.5 (#490) · Phase 4 · 蓝图 [`client-shape.md`](../../blueprint/current/client-shape.md) L22 (Mobile PWA + Web Push VAPID) + L37 ("没推送 = AI 团队像后台脚本不像同事") + L46 (实现路径).

## 1. 设计

PWA install 三件套客户端实现包括 service worker 注册 push handler、browser PushManager 订阅、POST 到 server。

| 设计点 | 约束 |
|---|---|
| VAPID key 持有方 | client 持公钥；server env 持私钥 |
| browser subscription 输出 | browser 生成 endpoint+p256dh+auth 三个公开字段 |
| 退订单一来源 | PushManager.unsubscribe + server DELETE 双侧同步；与蓝图 L22 字面一致 |

## 2. service worker (`packages/client/public/sw.js`)

service worker 保留 3 类 event listener，其中 cache shell 为 RT-1 既有逻辑，push 相关为 DL-4.5 新增逻辑。

| Event | 行为 |
|---|---|
| `install` / `activate` / `fetch` | RT-1 既有 cache shell，不改动 |
| `push` | 解析 `e.data.json()` payload → 渲染通知 (`self.registration.showNotification`)；payload shape 与 `internal/push/mention_notifier.go` byte-identical；`mention` kind 渲 `${from} mentioned you in #${channel}` + body；`agent_task` kind 渲 busy/idle 状态，busy 必带 subject 蓝图 §1.1 ⭐；未知 kind drop silently |
| `notificationclick` | 关闭通知；优先 focus 既有 SPA tab (clients.matchAll → focus)，否则 `openWindow('/')` 进入 SPA 路由 |

**反向约束**: sw.js 不存 secret / token；payload 由 SW 渲染，主线程不直接处理 push；该约束与 visibility-based dedup 蓝图 §1.4 的隐私边界同源。

## 3. pushSubscribe.ts helper (`packages/client/src/lib/pushSubscribe.ts`)

4 export + 1 internal helper:

| Export | 签名 | 行为 |
|---|---|---|
| `isPushSupported()` | `(): boolean` | feature detect；jsdom 返回 false，浏览器返回 true |
| `getCurrentSubscriptionState()` | `(): 'granted' \| 'denied' \| 'default' \| 'unsupported'` | observability；`Notification.permission` 4-enum |
| `registerServiceWorker()` | `(): Promise<ServiceWorkerRegistration>` | idempotent register `/sw.js` |
| `getActiveSubscription()` | `(): Promise<PushSubscription \| null>` | 读当前 PushSubscription (无则 null) |
| `subscribeToPush(vapidPublicKey)` | `(string): Promise<PushSubscription>` | 完整订阅: permission prompt → PushManager.subscribe → POST server |
| `unsubscribeFromPush()` | `(): Promise<void>` | 完整退订: PushManager.unsubscribe + server DELETE |
| `urlBase64ToUint8Array(s)` | `(string): Uint8Array` | W3C VAPID applicationServerKey 编码；- → +, _ → /, padding fix |

POST/DELETE 路径走 raw `fetch`，不依赖 `api.ts request<T>`。原因是 push registration runs early in main.tsx before SPA bootstraps，因此该 helper 必须保持为自包含独立模块。

## 4. ⚠️ 命名真分清 — DL-4 vs HB-1 #491

DL-4 PWA endpoint `/api/v1/pwa/manifest` 与 HB-1 #491 `/api/v1/plugin-manifest` 字面真分清。

| endpoint | 用途 | client 约束 |
|---|---|---|
| `/api/v1/pwa/manifest` | 公开 install prompt 用 | DL-4 PWA 范围 |
| `/api/v1/plugin-manifest` | 双签 binary plugin manifest | HB-1 install-butler host-bridge 范围，不是 web SPA 范围 |

client 端绝不调用 `plugin-manifest` 字面。

## 5. 相关参考

- 实施: `packages/client/public/sw.js` (push event handler) + `packages/client/src/lib/pushSubscribe.ts` (8 export)
- 单测: `packages/client/src/__tests__/pushSubscribe.test.ts` 6 vitest (jsdom feature detect + W3C 编码 4 sub-case)
- e2e: `packages/e2e/tests/pwa-push-notification-subscribe.spec.ts` 3 case (manifest W3C real fetch + 命名真分清 + sw.js push handler text-scan)
- spec brief: [`docs/implementation/modules/dl-4-spec.md`](../../implementation/modules/dl-4-spec.md) §1 DL-4.5
- server 端: [`docs/current/server/push.md`](../server/push.md)
