# CS-3 spec brief — PWA install + Web Push UI (≤80 行)

> 飞马 + 战马D · 2026-04-30 · Phase 4+ Client Shape 第三段 (蓝图 client-shape.md §1.1+§1.4)
> **蓝图锚**: [`client-shape.md`](../../blueprint/current/client-shape.md) §1.1 (一份 SPA + 三个分发壳, **PWA 主战场**) + §1.4 (Web Push 数据通路) + §0 (Web SPA 协作主战场)
> **关联**: DL-4 #485 PWA Web Push gateway server-side ✅ merged (`pushSubscribe.ts` + `/api/v1/push/subscribe` POST/DELETE 既有 byte-identical 不动) + DL-4 manifest.json + sw.js 已落 + CS-1 spec §3 留账承袭 (CS-3 = PWA install + Web Push, CS-4 = IndexedDB 乐观缓存)
> **命名**: CS-3 = Client Shape 第三段 — PWA install + Web Push UI (CS-2 = 故障 UX, CS-4 = IndexedDB)

> ⚠️ Wrapper milestone — 复用 DL-4 #485 既有 pushSubscribe lib + manifest.json + sw.js,
> 仅落 client-only **PWA install prompt UI + Web Push 订阅 UI + 三态权限文案**.
> **0 server prod + 0 schema 改 + 0 新 endpoint** — 真符合三选项决策树选项 C (跟 CS-1/CS-2 / CV-9..14 / DM-5..6 / DM-9 / CHN-11..12 同精神).

## 0. 关键约束 (3 条立场)

1. **PWA install prompt UI byte-identical 跟蓝图 §1.1 PWA 主战场** (`beforeinstallprompt` event 拦截 + 用户 explicit 触发 install): `lib/cs3-install-prompt.ts` 单源 — `window.addEventListener('beforeinstallprompt', ...)` 拦截 native banner, `prompt()` 由用户点 `安装 Borgee 桌面应用` button 触发 (蓝图字面 byte-identical); 反约束: 不允许 auto-prompt (Chrome/Edge 防滥用红线 — 必 user-gesture); 反向 grep `prompt\(\)\.then` 在 mount/effect 0 hit (必 click handler 内调); 反向 grep `auto.*install\|silent.*install` 在 cs-3-* 0 hit. 三态状态: `installable` (event 拦截到 + 未安装) / `installed` (匹配 `appinstalled` event OR `display-mode: standalone`) / `unavailable` (浏览器不支持 OR iOS Safari).

2. **Web Push 订阅 UI byte-identical 跟蓝图 §1.4 + DL-4 既有 pushSubscribe lib** (三态权限 + 用户 explicit 触发): `components/PushSubscribeToggle.tsx` 走 DL-4 `pushSubscribe.ts::subscribeToPush()` byte-identical 不动; UI 三态文案 byte-identical — `granted` → `已开启通知` (toggle on) / `denied` → `通知已被浏览器拒绝, 请到浏览器设置开启` / `default` → `开启通知` (toggle off); 反约束: 不允许 mount-time 自动 `Notification.requestPermission()` (反滥用红线); 必 click toggle 触发 `pushSubscribe.subscribeToPush()` (走 DL-4 既有路径); 反向 grep `Notification\.requestPermission\(\)` 在 useEffect/mount 0 hit (必 click handler 内调).

3. **0 server prod + 0 schema + 复用 DL-4 lib byte-identical** (Wrapper 立场 同 CS-1/CS-2): server diff 0 行 (`git diff origin/main -- packages/server-go/` count==0); 不引入 cs_3 命名 server file (反向 grep `migrations/cs_3\|cs3.*api\|cs3.*server` count==0); pushSubscribe.ts byte-identical 不动 (CS-3 仅 import 不改); 反向 grep `cs3.*pushSubscribe.*new\|CS3PushHelper` count==0 (走 DL-4 单源); 文案禁同义词漂 (`下载客户端 / 装个 app / 接收推送` 在 user-visible 0 hit, 跟蓝图字面 `安装 Borgee 桌面应用` / `开启通知` byte-identical).

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| **CS-3.1** PWA install prompt SSOT + 三态 hook | `packages/client/src/lib/cs3-install-prompt.ts` (新, ≤80 行) — `BeforeInstallPromptEvent` 拦截 + 三态 `InstallState` enum + `useInstallPrompt()` React hook (返 `{state, prompt}` — `prompt` 走 user click handler 不准 auto); `lib/cs3-permission-labels.ts` (新, ≤40 行) — 三态权限文案 byte-identical (granted/denied/default/unsupported 4-enum 跟 DL-4 PushPermissionState 同源); 8 vitest (TestCS31_InstallStateEnum + InstallPromptHook + AutoPromptForbidden + PermissionLabelsByteIdentical + 4 反向 grep) | 战马D |
| **CS-3.2** Install button + Push toggle UI 组件 | `components/InstallPromptButton.tsx` (新, ≤60 行) — `installable` 时渲染 `安装 Borgee 桌面应用` button + click → useInstallPrompt.prompt(); `components/PushSubscribeToggle.tsx` (新, ≤80 行) — Switch toggle 三态文案 + click → DL-4 `pushSubscribe.subscribeToPush()` / `unsubscribeFromPush()` byte-identical 复用; `installed` / `unsupported` 时不渲染 button (return null); 12 vitest (4 组件各 3 case) | 战马D |
| **CS-3.3** closure | REG-CS3-001..006 + acceptance + content-lock + PROGRESS [x] CS-3 + 4 件套 + docs/current sync (`docs/current/client/pwa-install.md` ≤80 行 — install prompt 三态字面 + Push 三态文案 byte-identical + DL-4 单源链锚) + e2e (`packages/e2e/tests/cs-3-pwa-install.spec.ts` 4 case: installable 渲染 button / installed 时 button 隐藏 / push toggle granted 文案 / push toggle denied 文案) | 战马D / 烈马 |

## 2. 反向 grep 锚 (5 反约束, count==0)

```bash
# 1) 0 server 改 (Wrapper 立场 ③)
git diff origin/main -- packages/server-go/ | grep -c '^\+'  # 0 production lines
# 2) 不允许 auto-prompt (Chrome 防滥用红线, 立场 ①)
git grep -nE 'prompt\(\)\.then|auto.*install|silent.*install' packages/client/src/lib/cs3-install-prompt.ts packages/client/src/components/InstallPromptButton.tsx  # 0 hit (must be in click handler)
# 3) 不允许 mount-time auto Notification.requestPermission (立场 ②)
git grep -nE 'Notification\.requestPermission\(\)' packages/client/src/components/PushSubscribeToggle.tsx  # 0 hit (must call via DL-4 pushSubscribe.subscribeToPush)
# 4) 不复用 DL-4 之外的 push helper (立场 ③)
git grep -nE 'cs3.*pushSubscribe.*new|CS3PushHelper' packages/client/src/  # 0 hit
# 5) 文案 byte-identical (蓝图字面 vs 同义词漂禁)
git grep -nE '下载客户端|装个 app|接收推送' packages/client/src/lib/cs3-permission-labels.ts  # 0 hit
```

## 3. 不在范围 (留账)

- ❌ Tauri 桌面壳 (路径已弃, 蓝图 §1.1 PWA 主战场, 跟 HB stack Go 重审决策同精神)
- ❌ IndexedDB 乐观缓存 (蓝图 §1.4 — 留 **CS-4**)
- ❌ Web Notifications API 自定义渲染 (走 sw.js push handler 既有 DL-4 路径, CS-3 仅订阅 UI)
- ❌ background sync (蓝图 §1.1 "完整离线 / background sync 不做" 字面承袭)
- ❌ admin god-mode PWA install (永久不挂, ADM-0 §1.3 红线)
- ❌ iOS Safari beforeinstallprompt 真支持 (Safari 不支持此 event, 走 iOS share sheet 引导 — 留 v2)

## 4. 跨 milestone byte-identical 锁

- 复用 DL-4 #485 `pushSubscribe.ts::subscribeToPush()` + `unsubscribeFromPush()` byte-identical (CS-3 仅 UI wrapper, 不改 lib)
- 复用 DL-4 既有 `manifest.json` + `sw.js` (CS-3 不改 PWA 配置)
- PushPermissionState 4-enum byte-identical 跟 DL-4 (granted/denied/default/unsupported)
- 文案 byte-identical 跟蓝图 client-shape.md §1.1 + §1.4 字面
- ADM-0 §1.3 admin god-mode 不挂 (CS-3 仅 client 用户视角)
- 0-server-prod 系列模式承袭 (CV-9..14 / DM-5..6 / DM-9 / CHN-11..12 / CS-1 / CS-2 / CS-3 第 15 处)

## 5. 验收挂钩

- REG-CS3-001..006 (5 反向 grep + install state SSOT 单测 + push toggle 三态 vitest + e2e 4 case)
- 既有 DL-4 pushSubscribe.ts unit tests 全 PASS (Wrapper 复用 不破)
- vitest cs3-install-prompt.test.ts + cs3-permission-labels.test.ts + InstallPromptButton.test.tsx + PushSubscribeToggle.test.tsx (≥20 case 全闭)
