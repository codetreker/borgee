# Implementation · Client Shape

> 蓝图: [`../../blueprint/current/client-shape.md`](../../blueprint/current/client-shape.md)
> 现状: 当前 SPA 在 `/src/client/`, Tauri 壳已有 (remote-agent → Borgee Helper); Mobile 完全没有
> 阶段: ⚡ v0
> 所属 Phase: Phase 4

## 1. 现状 → 目标 概览

**现状**: SPA + Tauri 壳; 主界面三栏雏形; 故障 UX 简单; 无本地缓存; 无 Mobile。
**目标**: blueprint 四条立场 — 一份 SPA + 三个分发壳 (Browser / Tauri / Mobile PWA), 主界面三栏 + 顶部团队栏 + Artifact 分级展开, 故障三态 (online/error/offline), 本地乐观缓存 (B)。

## 2. Milestones

### CS-1: 主界面三栏 + Artifact 分级展开

- **目标**: blueprint §1.2 — 主界面三栏 + 顶部团队栏, Artifact 分级展开 (列表 / 预览 / 详情)。
- **Owner**: 野马 (UX 立场) / 战马 / 飞马 / 烈马
- **范围**: 三栏布局 (channel 列表 / 主区 / 侧栏); 顶部团队栏 (org 隐式 — UI 永不显 org_id); artifact 分级展开
- **依赖**: CHN-1 (workspace), CV-1 (artifact)
- **预估**: ⚡ v0 1-2 周
- **Acceptance**: E2E + 用户感知签字 (野马: "找东西不绕路")

### CS-2: 故障三态 + 本地乐观缓存

- **目标**: blueprint §1.3 + §1.4 — 故障 UX 三态 (online/error/offline), 本地乐观缓存 (B 不全 cache)。
- **Owner**: 飞马 / 战马 / 野马 / 烈马
- **范围**: 三态指示器 (顶部); message 发送乐观 UI; 网络断 → offline 状态; reconcile 策略
- **依赖**: RT-2 (回放)
- **预估**: ⚡ v0 1 周
- **Acceptance**: 行为不变量 (乐观发送的 message 在 reconcile 后跟服务端一致, 单测) + E2E

### CS-3: Mobile PWA (install + Web Push)

- **目标**: blueprint §1.1 — 三个分发壳之一 (Browser / Tauri / Mobile PWA)。
- **Owner**: 飞马 / 战马 / 野马 / 烈马
- **范围**: PWA manifest; service worker (offline shell); Web Push 通知; iOS/Android 可装
- **不在范围**: native app ❌ (永不做)
- **依赖**: CS-1, **DL-4 (server 端 Web Push gateway 提供 VAPID 签名推送)**
- **预估**: ⚡ v0 1 周
- **Acceptance**: E2E (iPhone Safari 装到桌面 → 收到 push)

## 3. 不在 client-shape 范围

- 桌面 native UI (Tauri 已有壳) 内部自定义化 → 由 host-bridge 负责
- 内容渲染 (画布 / artifact) → canvas-vision

## 4. Blueprint 反查表

| Milestone | §X.Y | 立场一句话 |
|-----------|------|-----------|
| CS-1 | client-shape §1.2 | 三栏 + 顶部团队栏, artifact 分级 |
| CS-2 | client-shape §1.3 + §1.4 | 三态 + 乐观缓存 |
| CS-3 | client-shape §1.1 | 三个分发壳之一: Mobile PWA |
