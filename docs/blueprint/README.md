# Borgee Blueprint — 目标态(should-be)

> 这一目录是 11 轮设计讨论的产物 —— Borgee **应该是什么样**的规范集合。
> 状态: 建军 + 飞马 + 野马 三方对齐, 2026-04-27 首次发布。
> 归档 tag: `archive/discussion-final`(commit 5a788e9) 保留首次产出的原始形态。

## 蓝图状态机 (按 blueprintflow blueprint-iteration skill)

- `current/` — 当前蓝图 (frozen, 实施基于此). 11 模型 SOT 真值居此, 实施 PR 引此为准.
- `next/` — 下一版蓝图 (草拟期). 蓝图迭代时 fork 一份到 `next/`, 改完拍板再 promote → `current/`.
- `_meta/` — 蓝图 meta (decisions / phase / audit / review trigger). **不是模型 SOT**, 是过程治理记录 (R3/R4 决策 / phase trigger / audit 轮换 / review trigger).

> 当前过渡期 (docs-skeleton PR-1): 11 模型 .md 仍在本目录平铺, 待 docs-blueprint-reorg PR-3 移入 `current/`. 6 meta 文件届时移入 `_meta/`.

## 这是什么 / 不是什么

| 是 | 不是 |
|----|------|
| 产品形状的 source of truth | 当前代码的实现说明 |
| 长期稳定的产品立场 | 实施排期或 milestone |
| 跨模块对齐的概念基础 | 详细 spec 或 API 文档 |

> **如果想知道"代码现在长什么样"** → 见 [`../current/`](../current/)
> **如果想知道"如何从 current 走到 blueprint"** → 见 `../implementation/`(实施 roadmap, 待建)

---

## 一句话定位

> **Borgee 是 agent 协作平台, 不是 agent 平台。让"个人 + AI 团队"作为一个 org 跟其它 org 协作。**

## 文档导航

按概念依赖排序:

| # | 文档 | 内容 |
|---|------|------|
| 1 | [`concept-model.md`](current/concept-model.md) | **核心概念: 组织 / 人 / agent 三层身份**(最先读) |
| 2 | [`channel-model.md`](current/channel-model.md) | Channel / DM / Workspace 形状层规范 |
| 3 | [`canvas-vision.md`](current/canvas-vision.md) | 画布 / 文档协作: workspace = artifact 集合 |
| 4 | [`agent-lifecycle.md`](current/agent-lifecycle.md) | Agent 创建 / 状态 / 退役 — 协作平台不是 agent 平台 |
| 5 | [`plugin-protocol.md`](current/plugin-protocol.md) | BPP(Borgee Plugin Protocol)— runtime 接入中立协议 |
| 6 | [`host-bridge.md`](current/host-bridge.md) | Borgee Helper: 用户机器上的特权进程(信任五支柱) |
| 7 | [`realtime.md`](current/realtime.md) | 推送 / 状态 / 回放 — 让用户感到 AI 在工作的最小集 |
| 8 | [`auth-permissions.md`](current/auth-permissions.md) | 权限模型: ABAC 存储 + UI bundle, 跨 org 只减不加 |
| 9 | [`admin-model.md`](current/admin-model.md) | Admin 与隐私契约: 元数据可管, 内容不可读 |
| 10 | [`data-layer.md`](current/data-layer.md) | 数据层总账 + 分布式 ready 三层 |
| 11 | [`client-shape.md`](current/client-shape.md) | Client: 一份 SPA + Tauri 桌面壳 + Mobile PWA |

## 14 条核心立场(从 11 篇提炼)

### 身份
1. **个人即组织** — 1 org = 1 人 + N agent, UI 永久不暴露 org
2. **Agent = 同事** — 不是工具, 不是助手, 是产品差异化赌注
3. **agent 间独立协作允许** — 协作可以, 扩权不行(owner-only)

### 产品
4. **主体验 = 团队感知 + DM 对话 + artifact 工作面**
5. **Workspace = artifact 集合** — 每个 artifact 版本化, agent 可 iterate
6. **Channel = 协作场** — 聊天 + workspace 双支柱

### 平台
7. **Borgee 不带 runtime** — 通过 plugin 接 OpenClaw / Hermes
8. **BPP 中立协议** — OpenClaw plugin 是 reference impl
9. **Borgee 是 agent 配置面 SSOT** — Schema-driven blob, 热更新立即生效
10. **remote-agent 升级为安装管家** — 一份 SPA + Tauri 壳 + 信任五支柱

### 守则
11. **沉默胜于假 loading** — thinking 必须带 subject
12. **凭指标切, 不凭感觉切** — SQLite/MQ/Redis 同套阈值哲学
13. **管控元数据 = OK, 读内容 = 必须用户授权** — admin 隐私契约
14. **v1 协议 portable + 接口抽象, 运行时单机** — 分布式 ready 不挖坟
