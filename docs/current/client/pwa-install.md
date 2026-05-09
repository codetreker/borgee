# CS-3 PWA install + Web Push UI (client)

> 出处: `docs/blueprint/current/client-shape.md` §1.1 (PWA 主战场) + §1.4 (Web Push) + `docs/implementation/modules/cs-3-spec.md` v0
> 落点: 战马D + 飞马 + 烈马 + 野马 (一个 milestone 一个 PR, 0 server prod + 0 schema)

## PWA install prompt 单一来源 (lib/cs3-install-prompt.ts)

```ts
export type InstallState = 'installable' | 'installed' | 'unavailable';

export function useInstallPrompt(): {
  state: InstallState;
  prompt: () => Promise<'accepted' | 'dismissed' | 'unavailable'>;
};
```

- `beforeinstallprompt` event 拦截 (preventDefault) + cache deferredPrompt
- `appinstalled` event listener → state='installed'
- display-mode: standalone matchMedia → state='installed' (PWA 已安装)
- `prompt()` **必由 user click handler 触发** (Chrome/Edge 防滥用红线 — auto-prompt 浏览器会 reject)

## Push permission labels (lib/cs3-permission-labels.ts)

```ts
export const PUSH_PERMISSION_LABELS: Record<PushPermissionState, string> = {
  granted: '已开启通知',
  denied: '通知已被浏览器拒绝, 请到浏览器设置开启',
  default: '开启通知',
  unsupported: '', // 不渲染 (沉默胜于假活物感)
};

export const INSTALL_BUTTON_LABEL = '安装 Borgee 桌面应用';
```

byte-identical 跟 DL-4 #485 PushPermissionState 4-enum + 蓝图 §1.1+§1.4 字面.
**改 = 改两处 + content-lock §1**.

## UI 组件

| 组件 | DOM 出处 | 触发 | 反向约束 |
|---|---|---|---|
| `InstallPromptButton.tsx` | `<button data-cs3-install-button data-install-state>{INSTALL_BUTTON_LABEL}</button>` | click → useInstallPrompt.prompt() (user-gesture only) | installed/unavailable 时 return null (不准 disabled style 替代) |
| `PushSubscribeToggle.tsx` | `<button data-cs3-push-toggle data-push-state aria-pressed>{label}</button>` | click → DL-4 `subscribeToPush()` (default) / `unsubscribeFromPush()` (granted) | unsupported 时 return null; denied 时 disabled (浏览器永久拒绝, click 无效); 不准 mount-time auto requestPermission (走 DL-4 入口) |

## 反向约束守门 (跟 cs-3-stance-checklist §2 + content-lock §4 同源)

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

## 跨 milestone byte-identical 锁定

- DL-4 #485 pushSubscribe.ts byte-identical 不动 (CS-3 仅 import 不改)
- DL-4 既有 manifest.json + sw.js 不动
- PushPermissionState 4-enum byte-identical 跟 DL-4 (granted/denied/default/unsupported)
- 文案 byte-identical 跟蓝图 client-shape.md §1.1+§1.4 字面
- ADM-0 §1.3 admin god-mode 不挂

## 不在范围

- Tauri 桌面壳 (HB stack Go 重审决策放弃)
- IndexedDB 乐观缓存 (留 CS-4)
- Web Notifications API 自定义渲染 (走 sw.js DL-4 路径)
- background sync (跟蓝图 §1.1 字面一致)
- iOS Safari beforeinstallprompt 真支持 (留 v2)
- per-device 多端管理 UI (留 v1)
- admin god-mode PWA install / push 管理 (永久不挂 ADM-0 §1.3)
