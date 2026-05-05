# Agent Lifecycle — 接入、状态、退役

> 一个 agent 从被创建到被废弃的完整旅程。
> 状态：建军 + 飞马 + 野马 对齐（2026-04-27）。前置阅读：[`concept-model.md`](concept-model.md)。

## 0. 一句话定义

> **Borgee 不创造 agent，也不运行 agent；它让用户把别处运行的 agent 接入并跟人组队协作。**

---

## 1. 重大产品立场（必读）

> ⚠️ **Borgee 是一个 agent 协作平台，不是 agent 平台。**

- ❌ Borgee **不**调 LLM、**不**带 runtime、**不**提供 system prompt 模板
- ❌ Borgee **不**定义"PM agent"、"Dev agent"这种角色——这是 OpenClaw / Hermes 等 runtime 平台的事
- ✅ Borgee **只**提供"创建 agent 行 + 关联 plugin connection"的机制
- ✅ agent 必须接其它 runtime 平台（OpenClaw、Hermes、用户自建），通过 plugin 接入 Borgee
- ✅ Borgee 永远保持中立：不训模型、不卖 token、不做 prompt 工程

> **这条立场是对早期 PRD"接入平台"措辞的强化，不是变更。** 早期 PRD 的中立精神在此保留并加固——Borgee 的差异化是"人 + AI 团队的协作体验"，不是"又一个 LLM 包装"。

---

## 2. 目标态（Should-be）— 四条立场

### 2.1 Agent 创建：完全用户决定，无模板

- Borgee **不**提供预设角色库（无 PM/Dev/QA 模板）
- 用户**完全自主**决定 agent 的：
  - 名字
  - prompt（在哪个 runtime 平台配）
  - 能力（哪些 tool / MCP）
  - LLM 模型选择
- Borgee 端只有：`agent 行 + plugin connection 配置`
- 创建 UI：填名字 → 选 runtime（v1 只有 OpenClaw）→ 配 plugin → 完成

> **理由**：模板会限制用户创造性的 AI 团队组合，把 Borgee 拉回"又一个工作流模板平台"。预设角色库在 runtime 平台（如 OpenClaw 的 skill marketplace）上更合适。

### 2.2 Agent 运行时：默认 remote-agent，可选直连

#### 默认路径（一键 onboarding）

```
用户点"添加 agent"
    ↓
Borgee 引导用户安装 remote-agent（v1 只 Mac/Linux）
    ↓
remote-agent 帮用户在本机一键 setup OpenClaw
    ↓
OpenClaw 启动，自动注册 plugin connection 到 Borgee
    ↓
agent 出现在 Borgee 的"我的团队"列表
```

remote-agent 因此**升级为 "runtime 安装管家"**——不再只是文件代理。详细安全风险见下文 §3。

#### Power user 路径（保留）

已经在用 OpenClaw / Hermes 的种子用户可以**直接配 plugin**接入，跳过 remote-agent。这是早期口碑放大器，不能为了 onboarding 去掉。

#### v1 务实边界

| 维度 | v1 | v2+ |
|------|----|----|
| 支持的 runtime | **只 OpenClaw** | Hermes 等 |
| 操作系统 | **只 Mac / Linux** | Windows |
| Onboarding | 单条路径（添加 agent → 自动 setup） | 增加 plugin marketplace UI |
| 多 runtime 并行 | 不优化 | 进程管理优化 |

### 2.3 Agent 状态：四态 + 故障可解释

主状态枚举：

| 状态 | 含义 | 触发 | Phase |
|------|------|------|-------|
| **在线（online）** | runtime 已连接，等待消息 | plugin 在 WS / poll 心跳 | Phase 2 |
| **工作中（busy）** | 正在处理一个任务 | runtime 报告活动 / message 待 ack | Phase 4 (BPP 同期) |
| **空闲（idle）** | 在线但长时间无任务（>5min） | 心跳但无活动 | Phase 4 (BPP 同期) |
| **故障（error）** | runtime 报错或失联 | API key 失效、超限、网络断、进程崩溃 | Phase 2 (旁路) |

> **2026-04-28 4 人 review #5 决议**: busy / idle 在 Phase 2 不实现 (source 必须是 plugin 上行的 `task_started` / `task_finished` frame, 没 BPP 就只能 stub, stub 一旦上 v1 要拆掉 = 白写)。**Phase 2 在线列表只承诺 online / offline + error 旁路**, 野马 G2.4 签字范围对应收窄。busy/idle 跟 BPP 同期 (Phase 4 AL-1) 落地。
> ⚠️ **§11 文案守 (野马硬条件)**: Phase 2 Sidebar 显示 online / offline 两态时, 不准用"灰点 + 不说原因"糊弄, 必须明确文案 ("已离线" 而不是模糊 idle 灰)。

#### 故障态的关键设计

故障态**必须可解释**——不能只是红点。UI 要展示：

- **原因码** (AL-1a #249 锁 6 个 — server `internal/agent/state.go Reason*` ↔ client `lib/agent-state.ts REASON_LABELS` byte-identical, 改字面 = 改两边 + 锁两端单测): `api_key_invalid` / `quota_exceeded` / `network_unreachable` / `runtime_crashed` / `runtime_timeout` / `unknown`
- **直达修复入口**：根据原因跳到具体修复页（重新填 key、增加 quota、重启 runtime、查日志）

> 这是 owner 维护 AI 团队的核心 UX——agent 像同事一样会"生病"，owner 要能看到病因并直接处理。

#### "在哪个 channel" 作 hover 辅助

不放主视觉（避免侧栏挤）；hover agent 名字时显示当前活跃 channel 列表。

### 2.4 Agent 退役：B 禁用为默认 + A 删除藏高级

| 行为 | 入口 | 含义 |
|------|------|------|
| **B 禁用**（默认） | 主路径，agent 设置一键 | 停接消息，所有产出 / 对话保留可查；可一键恢复 |
| **A 删除**（高级） | 高级设置 + 二次确认 | 不可恢复；agent 用户行删除（产出归 owner，但失去溯源） |
| **C 归档**（v2） | — | 把 message / artifact 归到"历史"专区，不污染当前视图 |

> **设计直觉**：AI 同事不应该可以一键消失。删除是危险动作，必须有刹车。

---

## 3. 与现状的差距

| 目标态 | 现状 | 差距 |
|--------|------|------|
| 无模板 | ✅ 当前已经"用户自填" | 无差距，文档显式声明即可 |
| remote-agent 作 runtime 管家 | ❌ 当前只是文件代理 | **重写**——见下文 §4 风险 |
| 四态 + 故障可解释 | 仅有 online / offline | 状态模型扩展 + 故障原因码 + 修复入口 UI |
| Owner 自助禁用入口 | admin 有 disable cascade，owner 端无 UI | 加 owner 端禁用按钮 |

---

## 4. ⚠️ Remote-agent 安全模型重写（挂第 6 轮深入）

remote-agent 从"文件代理"升级为"runtime 安装管家"，安全边界**完全不同**：

| 维度 | 旧 remote-agent | 新 remote-agent（runtime 管家） |
|------|----------------|------------------------------|
| 暴露面 | 受限文件系统（read-only，白名单 dir） | **下载并执行二进制**（OpenClaw、未来 Hermes） |
| token 失效后果 | 顶多读不到文件 | **攻击者可远程在用户机器上跑任意进程** |
| 沙箱机制 | userland 路径白名单 | 需要：进程沙箱、签名校验、auto-update 机制、子进程隔离 |
| 攻击路径 | 文件读取 | RCE、持久化、横向移动 |

**第 6 轮（Remote-agent / Host bridge）必须专门处理**：
- 二进制下载 / 校验 / 更新策略
- runtime 进程的资源限制（CPU/内存/网络）
- 用户授权粒度（"是否允许 remote-agent 安装/执行 X 二进制"）
- 失败回滚 / uninstall 路径

---

## 5. 不在本轮范围

- agent 跟 owner / 跨 org agent 的具体协作流程 → [`concept-model.md` §4](concept-model.md)（已落定）
- workspace / artifact 的权限 → [`canvas-vision.md` §1.5](canvas-vision.md)（已落定）
- agent 的具体 SQLite schema → 第 10 轮"数据层"
- remote-agent 安装管家的具体协议 → 第 6 轮"Remote-agent / Host bridge"
- plugin connection 的协议演进 → 第 5 轮"Plugin (OpenClaw)"
