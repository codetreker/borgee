// pushSubscribe — DL-4.5 client Web Push subscription helper.
//
// Blueprint: docs/blueprint/current/client-shape.md L22 (Mobile PWA + Web Push
// VAPID) + L37 ("without push, the AI team feels like a background script,
// not a coworker") + L42 (manifest + install prompt + Web Push + standalone).
// Spec: docs/implementation/modules/dl-4-spec.md §1 DL-4.5.
//
// What this module does:
//   1. registerServiceWorker() — navigator.serviceWorker.register('/sw.js')
//      idempotent (browsers dedup by scope).
//   2. subscribeToPush(vapidPublicKey) — prompt user for notification
//      permission (if not granted), then PushManager.subscribe with
//      userVisibleOnly:true + applicationServerKey, POST to
//      /api/v1/push/subscribe with {endpoint, p256dh, auth, user_agent}.
//   3. unsubscribeFromPush() — PushManager.unsubscribe + DELETE
//      /api/v1/push/subscribe?endpoint=...
//   4. getCurrentSubscriptionState() — query browser current state
//      ('granted' | 'denied' | 'default' | 'unsupported').
//
// Design follows DL-4 spec §0:
//   - Client holds the VAPID public key; server env is the single source for the private key.
//   - Browser generates the subscription object (endpoint + p256dh + auth).
//   - Unsubscribe has two synchronized sides: PushManager.unsubscribe + server DELETE.

const PUSH_SUBSCRIBE_URL = '/api/v1/push/subscribe';

export type PushPermissionState = 'granted' | 'denied' | 'default' | 'unsupported';

/**
 * isPushSupported — feature detection. Browsers without ServiceWorker
 * + PushManager (older Safari, lockdown mode) cannot subscribe.
 */
export function isPushSupported(): boolean {
  return (
    typeof window !== 'undefined' &&
    'serviceWorker' in navigator &&
    'PushManager' in window
  );
}

/**
 * getCurrentSubscriptionState — observability helper. Returns the
 * Notification.permission value cast to our 4-enum, or 'unsupported'
 * if the browser has no Push API.
 *
 * Note: this is the *permission* state, not the subscription registration
 * state. To check active subscription, call getActiveSubscription().
 */
export function getCurrentSubscriptionState(): PushPermissionState {
  if (!isPushSupported()) return 'unsupported';
  if (typeof Notification === 'undefined') return 'unsupported';
  const p = Notification.permission;
  if (p === 'granted' || p === 'denied' || p === 'default') return p;
  return 'unsupported';
}

/**
 * registerServiceWorker — idempotent registration of /sw.js. Returns
 * the registration on success; throws on transient failure (caller
 * should catch + log).
 */
export async function registerServiceWorker(): Promise<ServiceWorkerRegistration> {
  if (!isPushSupported()) {
    throw new Error('push.unsupported: ServiceWorker / PushManager unavailable');
  }
  return navigator.serviceWorker.register('/sw.js');
}

/**
 * getActiveSubscription — read the current PushSubscription if one
 * exists. Returns null when no subscription registered (user never
 * subscribed OR previously unsubscribed). Throws on transient browser
 * error.
 */
export async function getActiveSubscription(): Promise<PushSubscription | null> {
  if (!isPushSupported()) return null;
  const reg = await navigator.serviceWorker.ready;
  return reg.pushManager.getSubscription();
}

/**
 * subscribeToPush — full subscribe flow:
 *   1. Permission check / request (browser prompt if 'default').
 *   2. PushManager.subscribe with userVisibleOnly:true +
 *      applicationServerKey (VAPID public key).
 *   3. POST /api/v1/push/subscribe with endpoint + p256dh + auth +
 *      user_agent (server stores in web_push_subscriptions table).
 *
 * Throws on permission denial or browser failure. Caller is responsible
 * for surfacing to user (PushPermissionToggle component does this).
 *
 * @param vapidPublicKey base64-url-encoded VAPID public key (from server env)
 */
export async function subscribeToPush(vapidPublicKey: string): Promise<PushSubscription> {
  if (!isPushSupported()) {
    throw new Error('push.unsupported: ServiceWorker / PushManager unavailable');
  }

  const permission = await Notification.requestPermission();
  if (permission !== 'granted') {
    throw new Error(`push.permission_denied: ${permission}`);
  }

  const reg = await navigator.serviceWorker.ready;

  // Avoid double-subscribe — return existing subscription if present.
  let sub = await reg.pushManager.getSubscription();
  if (!sub) {
    // TS strict: applicationServerKey expects BufferSource | string | null.
    // urlBase64ToUint8Array returns Uint8Array<ArrayBufferLike>; cast .buffer
    // to ArrayBuffer (the underlying buffer is always ArrayBuffer in browser
    // context, never SharedArrayBuffer for VAPID keys).
    const vapidKey = urlBase64ToUint8Array(vapidPublicKey).buffer as ArrayBuffer;
    sub = await reg.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: vapidKey,
    });
  }

  await postSubscriptionToServer(sub);
  return sub;
}

/**
 * unsubscribeFromPush — full unsubscribe flow: PushManager.unsubscribe +
 * server DELETE. Idempotent: repeated unsubscribe is still OK because the server
 * returns 204 either way.
 */
export async function unsubscribeFromPush(): Promise<void> {
  if (!isPushSupported()) return;
  const reg = await navigator.serviceWorker.ready;
  const sub = await reg.pushManager.getSubscription();
  if (!sub) return;

  const endpoint = sub.endpoint;
  await sub.unsubscribe();
  await deleteSubscriptionFromServer(endpoint);
}

/**
 * postSubscriptionToServer — POST /api/v1/push/subscribe with the 4
 * client-side public fields, byte-identical with the server-side
 * pushSubscribeRequest shape: endpoint / p256dh / auth / user_agent.
 *
 * Uses raw fetch (no api.ts request<T> dep) to keep this module
 * self-contained — push registration runs early in main.tsx before the
 * full SPA bootstraps.
 */
async function postSubscriptionToServer(sub: PushSubscription): Promise<void> {
  const json = sub.toJSON();
  const body = {
    endpoint: sub.endpoint,
    p256dh: json.keys?.p256dh ?? '',
    auth: json.keys?.auth ?? '',
    user_agent: navigator.userAgent,
  };
  const res = await fetch(PUSH_SUBSCRIBE_URL, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    throw new Error(`push.subscribe_failed: ${res.status}`);
  }
}

/**
 * deleteSubscriptionFromServer — DELETE /api/v1/push/subscribe?endpoint=...
 * Idempotent on the server side, matching the layout DELETE pattern.
 */
async function deleteSubscriptionFromServer(endpoint: string): Promise<void> {
  const url = `${PUSH_SUBSCRIBE_URL}?endpoint=${encodeURIComponent(endpoint)}`;
  const res = await fetch(url, {
    method: 'DELETE',
    credentials: 'include',
  });
  if (!res.ok && res.status !== 204) {
    throw new Error(`push.unsubscribe_failed: ${res.status}`);
  }
}

/**
 * urlBase64ToUint8Array — VAPID public key encoding helper. Web Push
 * applicationServerKey expects a Uint8Array; the server hands out a
 * base64-url-encoded string.
 *
 * Standard impl per https://developer.mozilla.org/en-US/docs/Web/API/PushManager/subscribe.
 */
export function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
  const rawData = atob(base64);
  const outputArray = new Uint8Array(rawData.length);
  for (let i = 0; i < rawData.length; ++i) {
    outputArray[i] = rawData.charCodeAt(i);
  }
  return outputArray;
}
