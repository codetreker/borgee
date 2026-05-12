// CS-3.2 — PushSubscribeToggle (蓝图 client-shape.md §1.4 + DL-4 #485 复用).
//
// DOM 字面锁 (cs-3-content-lock §2):
//   <button data-cs3-push-toggle data-push-state="{granted|denied|default}"
//           aria-pressed="{true/false}">{label}</button>
//
// 走 DL-4 pushSubscribe.subscribeToPush() 单源 (不另起 helper).
// unsupported 时 return null.
//
// Negative constraint: mount-time code must not call Notification.requestPermission()
// (DL-4 subscribeToPush 内部已封装, 走 click → DL-4 入口).
import React, { useEffect, useState, useCallback } from 'react';
import {
  isPushSupported,
  getCurrentSubscriptionState,
  subscribeToPush,
  unsubscribeFromPush,
  type PushPermissionState,
} from '../lib/pushSubscribe';
import { PUSH_PERMISSION_LABELS } from '../lib/cs3-permission-labels';

export interface PushSubscribeToggleProps {
  /** VAPID public key (from server config or env). */
  vapidPublicKey: string;
  /** Optional state-change callback. */
  onStateChange?: (state: PushPermissionState) => void;
}

export default function PushSubscribeToggle({
  vapidPublicKey,
  onStateChange,
}: PushSubscribeToggleProps) {
  const [state, setState] = useState<PushPermissionState>(() =>
    typeof window === 'undefined' ? 'unsupported' : getCurrentSubscriptionState(),
  );

  useEffect(() => {
    // 仅 mount 时 sync state — 不调 requestPermission; permission prompt stays click-gated.
    setState(getCurrentSubscriptionState());
  }, []);

  const onClick = useCallback(async () => {
    if (state === 'denied') return; // 浏览器锁死, click 无效
    if (state === 'granted') {
      await unsubscribeFromPush();
    } else {
      // default → 走 DL-4 subscribeToPush (内部 requestPermission)
      await subscribeToPush(vapidPublicKey);
    }
    const next = getCurrentSubscriptionState();
    setState(next);
    onStateChange?.(next);
  }, [state, vapidPublicKey, onStateChange]);

  if (state === 'unsupported' || !isPushSupported()) return null;

  const label = PUSH_PERMISSION_LABELS[state];
  return (
    <button
      type="button"
      className={`cs3-push-toggle cs3-push-toggle-${state}`}
      data-cs3-push-toggle
      data-push-state={state}
      aria-pressed={state === 'granted'}
      onClick={onClick}
      disabled={state === 'denied'}
    >
      {label}
    </button>
  );
}
