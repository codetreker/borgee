# Acceptance Template — CS-3: PWA install + Web Push UI

> Spec: `docs/implementation/modules/cs-3-spec.md` (飞马 + 战马D v0)
> 蓝图: `docs/blueprint/current/client-shape.md` §1.1 (PWA 主战场) + §1.4 (Web Push 数据通路)
> Stance: `docs/qa/cs-3-stance-checklist.md` (野马 / 飞马 v0)
> 前置: DL-4 #485 pushSubscribe.ts ✅ + manifest.json + sw.js 既有 ✅
> Owner: 战马D (主战) + 飞马 (spec) + 烈马 (acceptance) + 野马 (文案)

## 验收清单

### 立场 ① — PWA install user-gesture only (Chrome/Edge 防滥用红线)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 `lib/cs3-install-prompt.ts` 拦截 `beforeinstallprompt` event + cache deferredPrompt + `useInstallPrompt()` hook 返 `{state, prompt}` | unit | 战马D | `cs3-install-prompt.test.ts::TestCS31_InstallPromptHookCachesEvent` |
| 1.2 三态 enum `InstallState = 'installable' | 'installed' | 'unavailable'` byte-identical | unit | 战马D | `TestCS31_InstallStateEnum` |
| 1.3 `prompt()` 必由 user click handler 触发 — 反向断 mount/effect 内调 (`prompt\(\)\.then` 在 useEffect 0 hit) | unit | 战马D | `TestCS31_AutoPromptForbidden` (filepath.Walk + regex) |
| 1.4 `installed` 检测: `appinstalled` event OR `display-mode: standalone` matchMedia 任一 true | unit | 战马D | `TestCS31_InstalledStateDetection` |

### 立场 ② — Web Push 三态权限 byte-identical 跟 DL-4 + click-only 触发

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 `lib/cs3-permission-labels.ts::PUSH_PERMISSION_LABELS` 4-enum byte-identical (granted/denied/default/unsupported) — 文案 byte-identical 跟蓝图字面 | unit | 战马D | `cs3-permission-labels.test.ts::TestCS31_PermissionLabels_4DictByteIdentical` |
| 2.2 `components/PushSubscribeToggle.tsx` 走 DL-4 `pushSubscribe.subscribeToPush()` + `unsubscribeFromPush()` byte-identical (CS-3 不改 lib) | vitest | 战马D | `PushSubscribeToggle.test.tsx::TestCS32_DelegatesToDL4` (mock pushSubscribe + 验 call) |
| 2.3 三态文案渲染 — granted `已开启通知` (toggle on) / denied `通知已被浏览器拒绝, 请到浏览器设置开启` / default `开启通知` (toggle off) | vitest | 战马D | `PushSubscribeToggle.test.tsx::TestCS32_GrantedDeniedDefault_LabelsByteIdentical` |
| 2.4 unsupported 时不渲染 (return null) | vitest | 战马D | `PushSubscribeToggle.test.tsx::TestCS32_UnsupportedReturnsNull` |
| 2.5 反向断 mount-time auto requestPermission (`Notification\.requestPermission\(\)` 在 PushSubscribeToggle.tsx count==0, 必走 DL-4 入口) | unit | 战马D | `TestCS32_NoMountTimeAutoPermission` |

### 立场 ③ — 0 server prod + 0 schema + 文案 byte-identical + admin god-mode 不挂

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 0 server diff (`git diff origin/main -- packages/server-go/` count==0 production lines) | unit | 战马D | `cs3_no_server_diff_test.ts` (filepath.Walk server-go/) |
| 3.2 0 schema 改 (反向 grep `migrations/cs_3\|cs3.*api\|cs3.*server` 在 server-go/internal/ count==0) | unit | 战马D | `TestCS31_NoSchemaChange` |
| 3.3 同义词反向 (`下载客户端 / 装个 app / 接收推送 / 订阅通知` 在 cs3-permission-labels.ts count==0) | unit | 战马D | `TestCS31_NoSynonymDrift` |
| 3.4 admin god-mode 不挂 (反向 grep `admin.*pwa-install\|admin.*PushSubscribeToggle` count==0) | unit | 战马D | `TestCS32_NoAdminPWAManagement` |
| 3.5 InstallPromptButton 字面 `安装 Borgee 桌面应用` byte-identical 跟蓝图 §1.1 | vitest | 战马D | `InstallPromptButton.test.tsx::TestCS32_LabelByteIdentical` |
| 3.6 `installed` / `unavailable` 时 InstallPromptButton 不渲染 (return null) | vitest | 战马D | `InstallPromptButton.test.tsx::TestCS32_HiddenWhenInstalled + WhenUnavailable` |

### 既有 DL-4 pushSubscribe.ts / manifest.json / sw.js 不破

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 DL-4 pushSubscribe.ts 字面 byte-identical 不动 (本 PR 仅 import 不改) | unit | 战马D | git diff `packages/client/src/lib/pushSubscribe.ts` 0 行 |
| 4.2 manifest.json + sw.js 字面 byte-identical 不动 | unit | 战马D | git diff `packages/client/public/{manifest.json,sw.js}` 0 行 |

### e2e (cs-3-pwa-install.spec.ts 4 case)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 5.1 installable 状态 → InstallPromptButton 渲染 + `安装 Borgee 桌面应用` DOM | playwright | 战马D / 烈马 | `cs-3-pwa-install.spec.ts::TestCS3E2E_InstallButtonRenders` |
| 5.2 installed 状态 → InstallPromptButton 隐藏 (display-mode: standalone) | playwright | 战马D / 烈马 | `TestCS3E2E_InstallButtonHiddenWhenInstalled` |
| 5.3 push toggle granted → `已开启通知` 文案 + toggle on DOM | playwright | 战马D / 烈马 | `TestCS3E2E_PushToggleGrantedLabel` |
| 5.4 push toggle denied → `通知已被浏览器拒绝, 请到浏览器设置开启` 文案 | playwright | 战马D / 烈马 | `TestCS3E2E_PushToggleDeniedLabel` |

## 不在本轮范围 (spec §3 字面承袭)

- ❌ Tauri 桌面壳 (放弃, HB stack Go 重审决策已对齐)
- ❌ IndexedDB 乐观缓存 (留 CS-4)
- ❌ Web Notifications API 自定义渲染 (sw.js push handler 已落 DL-4)
- ❌ background sync (蓝图 §1.1 字面承袭)
- ❌ admin god-mode PWA install / push 管理 (永久不挂 ADM-0 §1.3)
- ❌ iOS Safari `beforeinstallprompt` 真支持 (留 v2)
- ❌ per-device 多端管理 UI (留 v1)

## 退出条件

- 立场 ① 1.1-1.4 (install hook + 三态 + auto-prompt 反向 + installed 检测) ✅
- 立场 ② 2.1-2.5 (push labels + toggle delegate DL-4 + 三态文案 + unsupported null + auto requestPermission 反向) ✅
- 立场 ③ 3.1-3.6 (0 server / 0 schema / 同义词反向 / admin god-mode 不挂 / install button label / 隐藏条件) ✅
- 既有 4.1-4.2 (DL-4 pushSubscribe / manifest / sw.js 不动) ✅
- e2e 5.1-5.4 全 PASS ✅
- REG-CS3-001..006 = **6 行 🟢**

## 更新日志

- 2026-04-30 — 战马D / 飞马 / 烈马 / 野马 v0: CS-3 4 件套 acceptance template, 跟 spec 3 立场 + stance §2 黑名单 grep + 跨 milestone byte-identical (DL-4 pushSubscribe.ts byte-identical + manifest.json + sw.js + ADM-0 §1.3 红线) 三段对齐. 0 server prod + 0 schema 改 — wrapper milestone 选项 C 同 CS-1 / CS-2 / CV-9..14 / DM-5..6 / DM-9 模式承袭.
