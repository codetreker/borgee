# CS-3 立场反查表 (PWA install + Web Push UI)

> **状态**: v0 (野马 / 飞马, 2026-04-30)
> **目的**: CS-3 实施 PR 直接吃此表为 acceptance — 战马D 实施 + 烈马 acceptance + 飞马 spec brief 反查立场漂移.
> **关联**: 蓝图 `client-shape.md` §1.1 (PWA 主战场 + 三个分发壳) + §1.4 (Web Push 数据通路); DL-4 #485 pushSubscribe.ts ✅ merged + manifest.json + sw.js 既有 byte-identical 不动.
> **依赖**: 0 server / 0 schema (Wrapper 选项 C 同 CS-1/CS-2 模式承袭).

---

## 1. CS-3 立场反查表 (3 立场)

| # | 立场锚 | 一句话立场 | 反约束 (X 是, Y 不是) | v0 / v1 |
|---|--------|----------|----------------------|---------|
| ① | client-shape §1.1 PWA 主战场 + Chrome/Edge `beforeinstallprompt` 防滥用红线 | **PWA install prompt user-gesture only — 必 click handler 触发, 不准 mount auto-prompt** | **是** `lib/cs3-install-prompt.ts` 单源 — `beforeinstallprompt` event 拦截 + cache deferredPrompt + 提供 `useInstallPrompt()` hook 返 `{state, prompt}`; `prompt()` 必由 user click `安装 Borgee 桌面应用` button 触发 (Chrome/Edge 反滥用红线 — auto-prompt 浏览器会 reject); 三态 enum: `installable` / `installed` / `unavailable`; **不是** mount/effect 内调 `prompt()` (反向 grep `prompt\(\)\.then` 在 useEffect 0 hit); **不是** `auto-install` / `silent install` (反向 grep `auto.*install\|silent.*install` 0 hit); **不是** PWA banner native 强弹 (`beforeinstallprompt` event preventDefault 拦截) | v0/v1 永久锁 — user-gesture 是浏览器红线 |
| ② | client-shape §1.4 + DL-4 #485 pushSubscribe.ts byte-identical | **Web Push 三态权限 byte-identical 跟 DL-4 (granted/denied/default), UI 必 click 触发, 复用 DL-4 lib 单源** | **是** `components/PushSubscribeToggle.tsx` 走 `pushSubscribe.subscribeToPush()` byte-identical 不改 lib; UI 三态文案 byte-identical: `granted` → `已开启通知` / `denied` → `通知已被浏览器拒绝, 请到浏览器设置开启` / `default` → `开启通知`; `unsupported` (iOS Safari) 时不渲染; **不是** mount-time 自动 `Notification.requestPermission()` (反滥用红线; 反向 grep 在 useEffect 0 hit); **不是** 走 `cs3-pushSubscribe` 等并行 helper (走 DL-4 单源, 反向 grep `cs3.*pushSubscribe.*new\|CS3PushHelper` 0 hit); **不是** UI 直接调 `Notification.requestPermission()` (必走 DL-4 `subscribeToPush()` 入口, 内部已封装 `requestPermission`) | v0: subscribe/unsubscribe UI; v1: per-device 多端管理 (复用 DL-4 既有 endpoint) |
| ③ | 0-server-prod 选项 C + 蓝图字面文案 byte-identical | **0 server prod + 0 schema 改 + 复用 DL-4 lib byte-identical + 文案 byte-identical 跟蓝图字面** | **是** server diff 0 行 (`git diff origin/main -- packages/server-go/` count==0); 不引入 cs_3 命名 server file (反向 grep `migrations/cs_3\|cs3.*api\|cs3.*server` count==0); pushSubscribe.ts byte-identical 不动 (CS-3 仅 import 不改); 文案 byte-identical 跟蓝图 §1.1 `安装 Borgee 桌面应用` / §1.4 `开启通知`; **不是** 同义词漂 (`下载客户端` / `装个 app` / `接收推送` / `订阅通知` 在 user-visible 0 hit); **不是** Tauri 桌面壳 install path (放弃, 跟 HB stack Go 重审决策同精神); **不是** admin god-mode PWA install / push 管理 (永久不挂 ADM-0 §1.3 红线) | v0/v1 永久锁 — 0-server-prod + plain language byte-identical 是 wrapper 立场 |

---

## 2. 黑名单 grep — CS-3 实施 PR merge 后跑, 全部预期 0 命中

```bash
# 立场 ① — auto-prompt 反向 (Chrome 红线)
git grep -nE 'prompt\(\)\.then|auto.*install|silent.*install' packages/client/src/lib/cs3-install-prompt.ts packages/client/src/components/InstallPromptButton.tsx  # 0 hit
# 立场 ② — mount-time auto requestPermission 反向 (滥用红线)
git grep -nE 'Notification\.requestPermission\(\)' packages/client/src/components/PushSubscribeToggle.tsx  # 0 hit (走 DL-4 subscribeToPush 入口)
# 立场 ② — 不复用 DL-4 之外 helper
git grep -nE 'cs3.*pushSubscribe.*new|CS3PushHelper' packages/client/src/  # 0 hit
# 立场 ③ — 0 server 改
git diff origin/main -- packages/server-go/ | grep -c '^\+'  # 0 production lines
# 立场 ③ — 0 schema 改
git grep -nE 'migrations/cs_3|cs3.*api|cs3.*server' packages/server-go/internal/  # 0 hit
# 立场 ③ — 同义词反向
git grep -nE '下载客户端|装个 app|接收推送|订阅通知' packages/client/src/lib/cs3-permission-labels.ts  # 0 hit
# 立场 ③ — admin god-mode 不挂 (ADM-0 §1.3 红线)
git grep -nE 'admin.*pwa-install|admin.*PushSubscribeToggle' packages/client/src/  # 0 hit
```

---

## 3. 不在 CS-3 范围 (避免 PR 膨胀, 跟 spec §3 同源)

- ❌ Tauri 桌面壳 (放弃, HB stack Go 重审决策已对齐)
- ❌ IndexedDB 乐观缓存 (留 **CS-4**)
- ❌ Web Notifications API 自定义渲染 (sw.js push handler 已落 DL-4, CS-3 仅订阅 UI)
- ❌ background sync (蓝图 §1.1 字面承袭 "完整离线 / background sync 不做")
- ❌ admin god-mode PWA install / push 管理 (永久不挂 ADM-0 §1.3)
- ❌ iOS Safari `beforeinstallprompt` 真支持 (Safari 不支持, 走 share sheet 引导留 v2)
- ❌ per-device 多端管理 UI (留 v1 复用 DL-4 既有 endpoint)

---

## 4. 验收挂钩

- CS-3.1 PR: 立场 ①③ — `useInstallPrompt` hook + 三态 enum + permission labels 4-enum byte-identical + 8 vitest (TestCS31_*)
- CS-3.2 PR: 立场 ② — `InstallPromptButton.tsx` + `PushSubscribeToggle.tsx` + 12 vitest (TestCS32_*)
- CS-3.3 entry 闸: 立场 ①-③ 全锚 + §2 黑名单 grep 全 0 + 跨 milestone byte-identical (DL-4 pushSubscribe.ts byte-identical + manifest.json + sw.js + ADM-0 §1.3 红线) + REG-CS3-001..006 全 🟢 + e2e 4 case PASS

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-30 | 野马 / 飞马 | v0, 3 立场 (PWA install user-gesture only / Web Push 三态 byte-identical + DL-4 单源 / 0-server-prod 选项 C + plain language byte-identical 跟蓝图字面) 承袭蓝图 §1.1 PWA 主战场 + §1.4 Web Push + DL-4 #485 pushSubscribe.ts byte-identical. 7 行反向 grep (含 admin god-mode 第 7 锚) + 7 项不在范围 + 验收挂钩三段对齐. 命名澄清: CS-3 = §1.1+§1.4 PWA install + Web Push UI (CS-1=三栏 / CS-2=故障 UX / CS-4=IndexedDB), 跟 CS-1 spec §3 + CS-2 spec §3 留账 byte-identical. 0 server / 0 schema wrapper 模式同 CS-1/CS-2 / CV-9..14 / DM-5..6 / DM-9. |
