---
version: v1.1
accepted: 2026-05-18
prev: v1.0
---

# Client Shape — 一份 SPA + 三个分发壳

> Borgee 客户端的目标设计。一份代码交付到三个运行形态，符合前 10 轮的所有核心决策。
> 状态：v1.1 promotion (2026-05-18，新增 §5 CT-1 + §6 IA-1)。前置阅读：所有其他设计文档。

## 0. 一句话定义

> **Web SPA 是主要协作界面；Tauri 壳运行 Helper；Mobile PWA 是离开桌面后的团队状态通道。一份代码，三个交付形态，职责不重叠。**

---

## 1. 目标态（Should-be）— 四条约定

### 1.1 一份 SPA + 三个分发壳

> 一份 Web SPA 代码，交付到三个运行形态：浏览器 / Tauri / Mobile PWA。每个运行形态承载**不同场景下的同一份产品**，不是三套独立 app。

| 壳 | 用户 / 场景 | 包含的能力 |
|----|------------|------------|
| **浏览器** | 主要协作界面，默认入口，所有用户 | 纯 SPA |
| **Tauri 桌面壳** | 桌面用户（开发场景） | SPA + **Borgee Helper（host-bridge）随桌面壳一起安装**——系统托盘 + 自启 + 卸载向导 |
| **Mobile PWA** | 离开桌面后的"团队感知"通道 | 安装到主屏幕 + **Web Push**（VAPID）+ standalone display |

#### 三个壳不重叠

- 桌面用户看到引导："**装 Borgee Helper**"（Tauri 壳）
- 移动用户看到引导："**加到主屏**"（PWA 安装）
- 浏览器用户看到完整 SPA，不被打扰

#### Mobile PWA 的产品理由（建军 push back 后修订）

PWA 不是冗余——它是 [agent-lifecycle](agent-lifecycle.md) "agent = 同事"产品定义在移动端的体现：

| PWA 能力 | 产品价值 |
|---------|---------|
| **Install + 主屏入口** | 一键回到 Borgee，比 Safari 标签更稳定 |
| **Web Push 通知** | "@你"、"agent 完成长任务"——AI 团队异步协作的核心 UX，**没推送 = AI 团队像后台脚本不像同事** |
| **Standalone display** | 全屏体验，去掉浏览器界面控件 |

#### PWA 范围克制

- ✅ manifest + install prompt + Web Push + standalone
- ❌ **完整离线缓存**（保持 [Q11.4](#14-本地持久化乐观缓存) 的边界）
- ❌ background sync

底层实现：service worker 已注册（main.tsx），需要新增 `manifest.json` + push subscription endpoint + VAPID key 生成 + server-go push 通道，并接入 [data-layer §3.4](data-layer.md) `global_events` 分发。预估 1-2 天工程量。

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

- 始终显示（[concept-model §1.4](concept-model.md) 团队感知首屏）
- 横排 agent 头像 + 状态色环（[§1.3 三态](#13-故障-ux分层呈现--三态-v1)）
- 故障中心入口

#### 侧栏

- channel 列表（按 [channel-model §1.4](channel-model.md) 作者定义的分组 + 个人折叠/排序）
- DM 单独一组，**视觉上区分**（channel-model §1.2 不让用户混淆"私聊 vs 协作"）

#### 主区 + Artifact 分级展开

避免"自动拆分屏幕"——artifact 触发分两级：

| 操作 | 行为 |
|------|------|
| 首次点击 artifact 引用 | 右侧**抽屉**展开（轻量预览） |
| 明确操作（拖拽 / 二次点击） | 升级为 **split view**（聊天 + artifact 并存） |

**移动浏览器布局**：顶部团队栏折叠为抽屉。

### 1.3 故障 UX：分层呈现 + 三态（v1）

#### v1 状态枚举：**三态**（野马 push back，建军采纳）

| 状态 | 含义 |
|------|------|
| **在线** | runtime 已连接 |
| **故障** | API key 失效 / 超限 / 进程崩溃 / 网络断 |
| **离线** | disable / 用户主动关 |

> "**工作中 / 空闲**"等状态只有在 BPP `progress` frame 能可靠上报 busy 时才增加为第四态。
> 跟 [realtime §1.1](realtime.md) "沉默胜于假 loading"一致——没有可靠数据时不显示状态。
> [agent-lifecycle §2.3](agent-lifecycle.md) 的"四态目标设计"作为**长期保留**，v1 实施三态。

#### 故障 UX 四层呈现

| 层 | 形态 | 触发 |
|----|------|------|
| **头像角标** | 故障小红点 | 任意 agent 进入故障态 |
| **点头像 → 浮层** | 显示原因 + **原地修复**（重连 / 重填 key / 查日志） | 用户主动点击 |
| **顶部 banner** | 横跨页面宽度的通知 | "全部故障" 或 "核心 agent 故障 > 5min" |
| **故障中心** | 团队栏按钮，聚合多个 agent | 多个 agent 故障时展开 |

#### plain language 错误文案

跟 [host-bridge §1.3](host-bridge.md) 对齐——错误信息使用用户能理解的语言：

| ✅ 用户语言 | ❌ 不可接受 |
|-----------|-----------|
| "DevAgent 跟 OpenClaw 失联" | `connection refused: openclaw://localhost:9100` |
| "API key 已失效，需要重新填写" | `401 Unauthorized: invalid_token` |

错误码 → 用户语言由客户端映射表维护。

#### **inline 修复，不跳设置页**

- 故障浮层里直接显示 "**重连**" / "**重填 API key**" / "**查日志**" 按钮
- 修复成功后浮层关闭，agent 状态自动更新

### 1.4 本地持久化：乐观缓存（B）

#### 什么存在哪

| 存储 | 内容 |
|------|------|
| **localStorage** | token + 用户偏好（团队栏顺序、布局） |
| **IndexedDB** | 最近 N 个 channel 消息 + agent 状态 + `last_read_at` + 当前 artifact 草稿 |
| **不缓存** | typing / presence 等实时数据——这些必须从 server 实时获取 |
| **不做** | 完整离线 / background sync |

#### 离线为什么不做

> Borgee 是协作平台，**离线状态下无法完成协作**——AI 团队、其他 org 在线时，用户才有可协作的对象。需要本地安装能力时使用 Tauri 壳。

#### 缓存非权威

- 用户多设备切换 → server cursor 增量同步是权威来源
- IndexedDB 只是 first paint 加速，不能取代 server
- 跟 [data-layer §4.A.2](data-layer.md) cursor opaque 协议直接匹配

---

## 2. 一句话总结

> **Web SPA 是主要协作界面；Tauri 壳运行 Helper；Mobile PWA 是离开桌面后的团队感知通道（install + push，不离线）；团队栏在首屏显示团队状态；artifact 支持 split view 但不会自动强制拆分；故障在当前位置修复；缓存用于乐观加速但不提供离线模式。**

---

## 3. 与现状的差距

| 目标态 | 现状 | 差距 |
|--------|------|------|
| Tauri 桌面壳 | 无 | 全新加 Tauri 项目，SPA 复用 |
| Mobile PWA + Web Push | service worker 已注册（`main.tsx`），但无 push 实现 | manifest.json + push subscription 表 + VAPID + push 通道（~1-2 天） |
| 顶部团队栏 + 三态 | 无团队感知视图 | 新组件 + 状态来源接入 BPP |
| Artifact 分级展开 | 无 artifact 概念 | 等 [canvas-vision](canvas-vision.md) 实现 |
| 故障四层 + 原地修复 | 仅 online/offline 文字 | 角标 + 浮层 + banner + 故障中心 |
| Plain language 错误 | 直接显示技术错误信息 | 错误码 → 用户语言映射表 |
| IndexedDB 乐观缓存 | 每次刷新拉全量 | 加缓存层 + cursor 同步 |
| DM vs channel 视觉上区分 | UI 没有区分 | 设计层重做 DM 入口 |

---

## 4. 不在本轮范围

- Tauri 壳的具体打包 / 签名 / 自动更新流程 → 第 6 轮 [host-bridge §1.2](host-bridge.md) 已确定规则
- 推送通知的具体内容文案库 → 实施时确定
- IndexedDB schema 与同步算法 → 实施时确定
- 国际化（i18n）/ 主题切换 → 实施细节，不影响客户端形态
- A11y / 键盘快捷键 → 实施细节

---

## 5. v1.1 CT-1：Client 真实性

### 5.1 已宣称的 surface 必须生产可达

- 蓝图层宣称存在的 surface（如 ArtifactComments / ArtifactPanel / Settings PermissionsView）**必须**在生产 UI 实际能从用户能到的入口被打开。
- 不能"代码里有、生产没挂"。这条不是 e2e 平台扩张，而是反向 e2e 证据：每个已宣称的 surface 至少有一个浏览器可达的真路径用例。

### 5.2 Forbidden 状态不可泄漏

- 无权访问 channel / DM / artifact / 文件 / 消息时，client 必须显示明确 forbidden 状态，不能只白屏或 loader。
- Forbidden / empty / redirect 状态在 server ACL 通过之前**不得**泄漏私有 channel / artifact / message / file 的 name、body、metadata、附件名。
- Forbidden UI 本身**不是**授权——server ACL 始终是唯一授权判断。
- 默认形态：local / in-surface forbidden state；只有当 task 显式证明 redirect 或 full-page state 更合适时才切换。

### 5.3 Security / permission AP bundle

- Settings 内 PermissionsView / 安全相关 AP bundle UI 是已宣称 surface，必须能从 Settings 正常入口到达；这一条与 PS-1 不冲突（PS-1 只禁用户侧 privacy/compliance 产品面扩张，不禁后端安全 surface 的可达性）。

---

## 6. v1.1 IA-1：Sidebar footer 与 Account 入口

### 6.1 Sidebar footer 紧凑

- Sidebar footer 只暴露少量主要入口；默认候选：avatar / Agents / Workspace / Settings。
- 其他历史 footer 入口（Invitations / Remote Nodes / logout 等）下沉到二级入口。

### 6.2 Avatar = Account 入口

- 头像作为 account 入口；logout 移入 account 面板。
- Account 面板 v1 = account summary + logout；账户设置扩张需要 task 显式 scope。

### 6.3 Remote Nodes / Helper 入口可移，但 rail 不合

- Remote Nodes / Helper / Host Bridge 入口可以挪到 Settings 或专门的 runtime-management surface。
- **IA 移动绝不合并 rails**：Remote Agent 和 Helper 的 credentials / grants / 强制执行 rail 仍各自独立建模（PS-1 + [`host-bridge.md` §1.2](host-bridge.md) #6 支柱）。
