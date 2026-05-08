# CS-3 文案锁 — PWA install + Web Push UI 字面 + DOM byte-identical

> **状态**: v0 (野马, 2026-04-30)
> **目的**: CS-3 实施 PR 落 client UI 前锁字面 — 跟 DL-4 #485 pushSubscribe.ts byte-identical + 蓝图 §1.1+§1.4 字面承袭.
> **关联**: spec `cs-3-spec.md` §0 + DL-4 #485 PushPermissionState 4-enum + 蓝图 client-shape.md §1.1+§1.4.

---

## 1. 文案 byte-identical 锁 (跟蓝图字面 + DL-4 4-enum)

| 场景 | 字面 byte-identical | 反约束 |
|---|------|------|
| Install button label (installable 状态) | `安装 Borgee 桌面应用` | 不准 `下载客户端` / `装个 app` / `安装到桌面` / `Add to Home Screen` |
| Push toggle (granted) | `已开启通知` | 不准 `已订阅` / `通知开` |
| Push toggle (denied) | `通知已被浏览器拒绝, 请到浏览器设置开启` | 不准 `权限被拒` / `请打开浏览器权限` |
| Push toggle (default) | `开启通知` | 不准 `订阅通知` / `接收推送` / `开通` |
| Push toggle (unsupported) | (return null, 不渲染) | 不准 fallback "请用最新浏览器" 提示 (沉默胜于假活物感) |

---

## 2. DOM 字面锁 (跟 vitest assertion + e2e selector byte-identical)

| 组件 | DOM 锚 | 反约束 |
|---|------|------|
| `InstallPromptButton` | `<button data-cs3-install-button data-install-state="installable">安装 Borgee 桌面应用</button>` | installed/unavailable 时 `return null` (不允许 disabled style 替代) |
| `PushSubscribeToggle` | `<button data-cs3-push-toggle data-push-state="{granted|denied|default}" aria-pressed="{true/false}">{label}</button>` | unsupported 时 `return null`; aria-pressed=true 仅 granted; toggle 必带 sibling label text (不准光按钮无 text) |

---

## 3. 三态枚举 byte-identical 锁 (跟 DL-4 PushPermissionState 同源)

| 维度 | enum | 来源 |
|---|---|---|
| InstallState (CS-3 新) | `installable` / `installed` / `unavailable` | spec §0 立场 ① |
| PushPermissionState (DL-4 既有) | `granted` / `denied` / `default` / `unsupported` | DL-4 #485 pushSubscribe.ts byte-identical 不动 |

**反约束**:
- ❌ install 第 4 态漂入 (`pending` / `dismissed` 0 hit; cache `deferredPrompt = null` 后回到 `unavailable`)
- ❌ push 第 5 态漂入 (跟 DL-4 4-enum 锁死)

---

## 4. 反向 grep 锚 (跟 stance §2 + spec §2 同源)

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
# ⑦ DL-4 pushSubscribe.ts 不改 (CS-3 仅 import)
git diff origin/main -- packages/client/src/lib/pushSubscribe.ts  # 0 行
```

---

## 5. 验收挂钩

- CS-3.1 PR: §1 文案 byte-identical + §3 三态 enum byte-identical + 单测 `TestCS31_PermissionLabels_4DictByteIdentical`
- CS-3.2 PR: §2 DOM 锚 + §1 toggle 三态文案 + vitest literal assert
- CS-3.3 entry 闸: §1+§2+§3+§4 全锚 + 跨 milestone byte-identical (DL-4 pushSubscribe.ts + manifest.json + sw.js + ADM-0 §1.3)

---

## 6. 不在范围

- ❌ Tauri 桌面壳文案 (放弃)
- ❌ iOS Safari install 引导 (留 v2 share sheet)
- ❌ 自定义 Notifications API 渲染文案 (走 sw.js DL-4)
- ❌ admin god-mode PWA UI (永久不挂)
- ❌ unsupported fallback 提示 (沉默胜于假活物感, 蓝图 §11)

---

## 7. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-30 | 野马 | v0 — CS-3 文案锁 (4 段: 5 文案 byte-identical 跟蓝图字面 + 2 DOM 锚 + 三态 + DL-4 4-enum byte-identical + 7 反向 grep 含 admin god-mode + DL-4 lib byte-identical 锁). 跟 DL-4 #485 / 蓝图 client-shape.md §1.1+§1.4 同源 byte-identical. 同义词漂禁 5 词 + admin god-mode 反向 + auto-prompt 反向 (Chrome 红线). |
