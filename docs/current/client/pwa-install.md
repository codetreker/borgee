# CS-3 PWA install + Web Push UI (client)

> Source: `docs/blueprint/current/client-shape.md` §1.1 (PWA primary surface) + §1.4 (Web Push) + `docs/implementation/modules/cs-3-spec.md` v0
> Landing plan: phased implementation (one milestone per PR, no server production changes + no schema changes)

## PWA Install Prompt Single Source (lib/cs3-install-prompt.ts)

```ts
export type InstallState = 'installable' | 'installed' | 'unavailable';

export function useInstallPrompt(): {
  state: InstallState;
  prompt: () => Promise<'accepted' | 'dismissed' | 'unavailable'>;
};
```

- intercept `beforeinstallprompt` event (preventDefault) + cache deferredPrompt
- `appinstalled` event listener → state='installed'
- display-mode: standalone matchMedia → state='installed' (PWA 已安装)
- `prompt()` **must be triggered by a user click handler** (Chrome/Edge abuse guard: browser rejects auto-prompt)

## Push permission labels (lib/cs3-permission-labels.ts)

```ts
export const PUSH_PERMISSION_LABELS: Record<PushPermissionState, string> = {
  granted: '已开启通知',
  denied: '通知已被浏览器拒绝, 请到浏览器设置开启',
  default: '开启通知',
  unsupported: '', // 不渲染 (避免显示误导性的可用状态)
};

export const INSTALL_BUTTON_LABEL = '安装 Borgee 桌面应用';
```

These labels match DL-4 #485 PushPermissionState 4-enum + blueprint §1.1+§1.4 literals.
**Changing this requires updating two places + content-lock §1**.

## UI Components

| Component | DOM source | Trigger | Prohibited behavior |
|---|---|---|---|
| `InstallPromptButton.tsx` | `<button data-cs3-install-button data-install-state>{INSTALL_BUTTON_LABEL}</button>` | click → useInstallPrompt.prompt() (user-gesture only) | return null when installed/unavailable; do not replace this with disabled styling |
| `PushSubscribeToggle.tsx` | `<button data-cs3-push-toggle data-push-state aria-pressed>{label}</button>` | click → DL-4 `subscribeToPush()` (default) / `unsubscribeFromPush()` (granted) | return null when unsupported; disabled when denied because the browser permanently rejected it and click has no effect; do not auto-call requestPermission at mount time, use the DL-4 entry point |

## Reverse Constraint Checks (same source as cs-3-stance-checklist §2 + content-lock §4)

```bash
# ① auto-prompt 反向 (Chrome 红线)
git grep -nE 'prompt\(\)\.then|auto.*install|silent.*install' packages/client/src/lib/cs3-install-prompt.ts packages/client/src/components/InstallPromptButton.tsx  # 0 hit
# ② mount-time auto requestPermission 反向 (滥用红线)
git grep -nE 'Notification\.requestPermission\(\)' packages/client/src/components/PushSubscribeToggle.tsx  # 0 hit (走 DL-4)
# ③ 不复用 DL-4 之外 helper
git grep -nE 'cs3.*pushSubscribe.*new|CS3PushHelper' packages/client/src/  # 0 hit
# ④ 同义词反向
git grep -nE '下载客户端|装个 app|接收推送|订阅通知|权限被拒' packages/client/src/lib/cs3-permission-labels.ts  # 0 hit
# ⑤ admin god-mode 不挂 (ADM-0 §1.3)
git grep -nE 'admin.*pwa-install|admin.*PushSubscribeToggle' packages/client/src/  # 0 hit
# ⑥ 0 server 改
git diff origin/main -- packages/server-go/ | grep -c '^\+'  # 0
# ⑦ DL-4 lib byte-identical (CS-3 仅 import)
git diff origin/main -- packages/client/src/lib/pushSubscribe.ts  # 0 行
```

## Cross-Milestone Byte-Identical Locks

- DL-4 #485 pushSubscribe.ts must remain unchanged (CS-3 imports it only)
- Existing DL-4 manifest.json + sw.js stay unchanged
- PushPermissionState 4-enum must match DL-4 (granted/denied/default/unsupported)
- Copy must keep blueprint client-shape.md §1.1+§1.4 literals
- ADM-0 §1.3 admin surface must not mount this entry point

## Out of Scope

- Tauri desktop shell (HB stack Go re-review decision abandoned it)
- IndexedDB optimistic cache (left for CS-4)
- Web Notifications API custom rendering (uses the sw.js DL-4 path)
- background sync (matches blueprint §1.1 literal)
- iOS Safari beforeinstallprompt actual support (left for v2)
- per-device multi-client management UI (left for v1)
- Permanent prohibition: do not mount or expose an ADM-0 §1.3 admin PWA install / push management entry point
