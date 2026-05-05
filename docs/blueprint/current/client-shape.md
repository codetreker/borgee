# Client Shape — 一份 SPA + 三个分发壳

> Borgee 客户端的目标态。一份代码三个壳，配合前 10 轮的所有内核决策。
> 状态：建军 + 飞马 + 野马 对齐（2026-04-27）。前置阅读：所有其它 design 文档。

## 0. 一句话定义

> **Web SPA 是协作主战场；Tauri 壳是 Helper 的家；Mobile PWA 是离桌面后的团队感知通道。一份代码，三个分发，不重叠。**

---

## 1. 目标态（Should-be）— 四条立场

### 1.1 一份 SPA + 三个分发壳

> 一份 Web SPA 代码，分发到三个壳：浏览器 / Tauri / Mobile PWA。每个壳承担**不同场景下的同一份产品**，不是三套独立 app。

| 壳 | 用户 / 场景 | 包含的能力 |
|----|------------|------------|
| **浏览器** | 主战场，默认入口，所有用户 | 纯 SPA |
| **Tauri 桌面壳** | 桌面用户（开发场景） | SPA + **Borgee Helper（host-bridge）同捆**——系统托盘 + 自启 + 卸载向导 |
| **Mobile PWA** | 离桌面后的"团队感知"通道 | install 主屏 + **Web Push**（VAPID）+ standalone display |

#### 三个壳不重叠

- 桌面用户看到引导："**装 Borgee Helper**"（Tauri 壳）
- 移动用户看到引导："**加到主屏**"（PWA install）
- 浏览器用户看到完整 SPA，不被打扰

#### Mobile PWA 的产品理由（建军 push back 后修订）

PWA 不是冗余——是 [agent-lifecycle](agent-lifecycle.md) "agent = 同事"立场的 mobile 体现：

| PWA 能力 | 产品价值 |
|---------|---------|
| **Install + 主屏入口** | 一键回 Borgee，比 Safari 标签强 |
| **Web Push 通知** | "@你"、"agent 完成长任务"——AI 团队异步协作的核心 UX，**没推送 = AI 团队像后台脚本不像同事** |
| **Standalone display** | 全屏体验，去掉浏览器 chrome |

#### PWA 范围克制

- ✅ manifest + install prompt + Web Push + standalone
- ❌ **完整离线缓存**（[Q11.4](#14-本地持久化乐观缓存) 守住）
- ❌ background sync

底层实现：service worker 已注册（main.tsx），增量是 `manifest.json` + push subscription endpoint + VAPID key 生成 + server-go 一个 push 通道接 [data-layer §3.4](data-layer.md) global_events fan-out。预估 1-2 天工程量。

### 1.2 主界面：三栏 + 顶部团队栏 + Artifact 分级展开

```
┌──────────────────────────────────────────────────┐
│ [顶部团队栏：agent 头像 + 状态]                  │ ← 永久首屏感知
├──────────┬──────────────────┬───────────────────┤
│  侧栏    │  主区（聊天默认） │ artifact          │
│ channel  │                  │（触发分级展开）   │
│  + DM    │                  │                   │
└──────────┴──────────────────┴───────────────────┘
```

#### 顶部团队栏

- 永久存在（[concept-model §1.4](concept-model.md) 团队感知首屏）
- 横排 agent 头像 + 状态色环（[§1.3 三态](#13-故障-ux分层呈现--三态-v1)）
- 故障中心入口

#### 侧栏

- channel 列表（按 [channel-model §1.4](channel-model.md) 作者定义 group + 个人折叠/排序）
- DM 单独一组，**视觉分化**（channel-model §1.2 不让用户混淆"私聊 vs 协作"）

#### 主区 + Artifact 分级展开

避免"自动劈开屏幕"——artifact 触发分两级：

| 操作 | 行为 |
|------|------|
| 首次点击 artifact 引用 | 右侧**抽屉**展开（轻量预览） |
| 显式动作（拖拽 / 二次点击） | 升级为 **split view**（聊天 + artifact 并存） |

**移动浏览器降级**：顶部团队栏折叠为 drawer。

### 1.3 故障 UX：分层呈现 + 三态（v1）

#### v1 状态枚举：**三态**（野马 push back，建军采纳）

| 状态 | 含义 |
|------|------|
| **在线** | runtime 已连接 |
| **故障** | API key 失效 / 超限 / 进程崩溃 / 网络断 |
| **离线** | disable / 用户主动关 |

> "**工作中 / 空闲**"等 BPP `progress` frame 真的能精确上报 busy 再加第四态。
> 跟 [realtime §1.1](realtime.md) "沉默胜于假 loading"一致——无可靠数据不画状态。
> [agent-lifecycle §2.3](agent-lifecycle.md) 的"四态目标态"作为**长期保留**，v1 实施三态。

#### 故障 UX 四层呈现

| 层 | 形态 | 触发 |
|----|------|------|
| **头像角标** | 故障小红点 | 任意 agent 进入故障态 |
| **点头像 → 浮层** | 显示原因 + **inline 修复**（重连 / 重填 key / 查日志） | 用户主动点击 |
| **顶部 banner** | 全屏宽通知 | "全部故障" 或 "核心 agent 故障 > 5min" |
| **故障中心** | 团队栏按钮，聚合多 agent | 多 agent 故障时展开 |

#### plain language 错误文案

跟 [host-bridge §1.3](host-bridge.md) 对齐——错误信息说人话：

| ✅ 用户语言 | ❌ 不可接受 |
|-----------|-----------|
| "DevAgent 跟 OpenClaw 失联" | `connection refused: openclaw://localhost:9100` |
| "API key 已失效，需要重新填写" | `401 Unauthorized: invalid_token` |

错误码 → 用户语言由 client 端映射表维护。

#### **inline 修复，不跳设置页**

- 故障浮层里直接出现 "**重连**" / "**重填 API key**" / "**查日志**" 按钮
- 修复成功后浮层关闭，agent 状态自动更新

### 1.4 本地持久化：乐观缓存（B）

#### 什么存在哪

| 存储 | 内容 |
|------|------|
| **localStorage** | token + 用户偏好（团队栏顺序、布局） |
| **IndexedDB** | 最近 N 个 channel 消息 + agent 状态 + `last_read_at` + 当前 artifact 草稿 |
| **不缓存** | typing / presence 等真正实时数据——这些必须从 server 实时拉 |
| **不做** | 完整离线 / background sync |

#### 离线为什么不做

> Borgee 是协作平台，**离线没意义**——AI 团队、其它 org 在线，你才有可协作的对象。要"装"走 Tauri 壳。

#### 缓存非权威

- 用户多设备切换 → server cursor 增量同步是真相
- IndexedDB 只是 first paint 加速，不能取代 server
- 跟 [data-layer §4.A.2](data-layer.md) cursor opaque 协议天然契合

---

## 2. 一句话总结

> **Web SPA 是协作主战场；Tauri 壳是 Helper 的家；Mobile PWA 是离桌面后的团队感知通道（install + push，不离线）；团队栏是首屏感知；artifact 可 split 不强切；故障原地修复；缓存乐观但不离线化。**

---

## 3. 与现状的差距

| 目标态 | 现状 | 差距 |
|--------|------|------|
| Tauri 桌面壳 | 无 | 全新加 Tauri 项目，SPA 复用 |
| Mobile PWA + Web Push | service worker 已注册（`main.tsx`），但无 push 实现 | manifest.json + push subscription 表 + VAPID + push 通道（~1-2 天） |
| 顶部团队栏 + 三态 | 无团队感知视图 | 新组件 + 状态来源接 BPP |
| Artifact 分级展开 | 无 artifact 概念 | 等 [canvas-vision](canvas-vision.md) 落地 |
| 故障三层 + inline 修复 | 仅 online/offline 文字 | 角标 + 浮层 + banner + 故障中心 |
| Plain language 错误 | 技术堆栈直出 | 错误码 → 用户语言映射表 |
| IndexedDB 乐观缓存 | 每次刷新拉全量 | 加缓存层 + cursor 同步 |
| DM vs channel 视觉分化 | UI 同样 | 设计层重做 DM 入口 |

---

## 4. 不在本轮范围

- Tauri 壳的具体打包 / 签名 / 自动更新流程 → 第 6 轮 [host-bridge §1.2](host-bridge.md) 已定原则
- 推送通知的具体内容文案库 → 实施时定
- IndexedDB schema 与同步算法 → 实施时定
- 国际化（i18n）/ 主题切换 → 实施细节，不影响形状
- A11y / 键盘快捷键 → 实施细节
