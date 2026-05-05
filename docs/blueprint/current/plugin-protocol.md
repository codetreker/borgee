# Plugin Protocol — BPP (Borgee Plugin Protocol)

> Plugin 是 agent 接入 Borgee 的**唯一**通道。本文规范 BPP 协议的目标态——v1 由 OpenClaw plugin 作 reference implementation，v2+ 接其它 runtime 不必重写。
> 状态：建军 + 飞马 + 野马 对齐（2026-04-27）。前置阅读：[`agent-lifecycle.md`](agent-lifecycle.md)。

## 0. 一句话定义

> **BPP 是 Borgee 与 runtime（OpenClaw / Hermes / 自建）之间的协议契约。Borgee 是 agent 的控制面 SSOT，runtime 是执行面。**

---

## 1. 目标态（Should-be）— 六条立场

### 1.1 Plugin 实例 = runtime 实例（B 主 + A 兼容）

- **默认（B）**：一条 plugin connection = 一个 runtime 实例（如一个 OpenClaw 进程），该 runtime 上跑的多个 agent **共享**这一条 connection。
- **兼容（A）**：保留"一 plugin = 一 agent"作为 power user 简单场景。
- ⚠️ **UI 永远不暴露 "plugin" 概念**——用户只看到 agents，plugin 是底层实现细节。

> 协议要求：plugin 在握手时 `runtime_schema_advertise`，并以多个 agent 身份注册到 Borgee。

### 1.2 BPP 是中立通用协议（不是 OpenClaw 私有）

- **v1 只有 OpenClaw 实现，但协议本身中立**——任何 runtime 实现 BPP 即可接入。
- **20% 工程溢价的回报**：
  - 早期种子用户不会觉得 Borgee 是"OpenClaw 套壳"
  - v2 接 Hermes 时**不必重写** plugin 协议
  - 公开 BPP 规范文档作为平台战略——欢迎第三方 runtime 实现
- **OpenClaw plugin 作为 reference implementation**，长期维护者对照规范实现。

### 1.3 Plugin 调 Borgee：抽象语义层（C），不直对 REST

Plugin **不**直接调用 Borgee 的 REST endpoint，而是调用**协作语义动作（semantic actions）**。

#### v1 必须的语义动作

```
create_artifact          # 创建产物（PRD / 代码 / 设计稿等）
update_artifact          # 修改产物内容（生成新版本）
reply_in_thread          # 在某条消息线程下回复
mention_user             # @ 某个 user/agent
request_agent_join       # 请求邀请其他 agent 进 channel（触发审批）
read_channel_history     # 读 channel 消息历史（分页）
read_artifact            # 读 workspace 中的 artifact
```

#### v2+ 的协作意图动作

```
propose_artifact_change  # 提议改动（diff/PR-style，等 owner review）
request_owner_review     # 请求 owner 检查产物
request_clarification    # 跟某个 user 索要更多信息
```

> **关键洞察**：动作集就是 Borgee 的**协作姿态**。未来加什么动作，就是在告诉世界"Borgee 推崇什么样的 AI 协作模式"。

#### 实现细节

- 每个语义动作内部 **dispatch 到白名单 REST API**
- server 统一在 dispatch 层做权限检查（依据 [concept-model](concept-model.md) 的 user/agent permission 系统）
- 不允许 plugin "下穿"语义层直调 REST——这是协议红线

### 1.4 Borgee 是 agent 配置面的 SSOT

**按"用户做选择 vs 系统调优"划界**：

| 归 Borgee 管（用户选择项） | 归 Runtime 管（系统调优） |
|--------------------------|--------------------------|
| `name`（agent 显示名） | `temperature` |
| `avatar` | `token 上限` |
| `prompt`（system / role prompt） | 限速 |
| **`model`**（可选值由 runtime 上报 schema） | retry 策略 |
| 能力开关（哪些 tool 启用） | 记忆策略实现 |
| 启用/禁用状态 | 模型 API key |
| `memory_ref`（**只存引用，不存内容**） | — |

#### 关键设计：runtime 上报 model schema

- runtime 通过 `runtime_schema_advertise` 上报：
  - 它支持哪些 model（list）
  - 每个 model 的 metadata（context window、cost tier、default temperature）
  - runtime 自己的私有 opaque blob 字段（Borgee 不解读，只透明传递）
- Borgee UI 通用渲染 model 选择器，**不写死** OpenClaw / Hermes 的具体模型列表

#### Memory 边界（v1 不踩坑）

- **memory_ref 在 Borgee**：用户在 Borgee UI 选择 agent 用哪份 memory（指针）
- **memory 内容在 runtime**：实际向量库 / RAG 索引由 runtime 维护
- v1 不让 Borgee 变向量库基础设施

> 这条直接让 [agent-lifecycle.md §2.1](agent-lifecycle.md) 的"用户完全自定义 agent" **明确**为：用户在 **Borgee UI** 一处全配，不去 OpenClaw 配。配置面与运行时分离，跟"Borgee 不带 runtime"不矛盾。

### 1.5 配置热更新：按字段分类生效

| 字段 | 生效时机 |
|------|----------|
| `name` / `avatar` / 能力开关 | **立即**——下条消息渲染就用新值 |
| `prompt` | **立即下发**，作用于"下一次 inference"，**不打断正在生成的回复** |
| `model` | 立即下发，下次 inference 切换 |
| 长任务边界 | 当前任务用旧 config 跑完，新 config 下次任务起作用 |

#### 协议接口

- `agent_config_update`：server → plugin 推送
- plugin **必须支持幂等 reload**（同一 update payload 重复推送不应有副作用）
- runtime **不缓存** agent 定义——每次 inference 前读最新 config

### 1.6 失联与故障状态

#### 长任务执行中失联

- Borgee **不**承担任务编排
- server **不**取消正在执行的任务
- plugin 重连后由 runtime 自己决定恢复 / 放弃

#### 状态显示

- plugin 失联 → agent 状态 = **"故障 (runtime 失联)"**
- "工作中" 状态需要 plugin **主动心跳上报**——缺心跳按"未知"
- runtime 崩溃 → agent **不消失**，显示"故障"
  - 这是 [concept-model §1.4](concept-model.md) "团队感知视图"对稳定成员实体的要求

#### 故障 UX 区分

| 类型 | 含义 | 提示 |
|------|------|------|
| `runtime_disconnected` | 平台问题（plugin 断线、进程崩溃） | "重连中…" |
| `agent_misconfigured` | 用户问题（API key 失效、模型超限、tool 配错） | "检查 OpenClaw 设置" + 直达修复入口 |

---

## 2. BPP 接口清单（v1 最小集）

### 2.1 控制面（Borgee → Plugin）

| 接口 | 用途 |
|------|------|
| `connect`（握手） | plugin 上线，发送 token、capabilities |
| `agent_register`（多 agent） | plugin 上报它 host 的 agents 列表 |
| `runtime_schema_advertise` | plugin 上报 runtime 的字段 schema（models、opaque blob 字段） |
| `agent_config_update` | server 推送 agent 配置变更（热更新） |
| `agent_disable / enable` | 暂停/恢复 agent 接收消息 |
| `inbound_message` | server 推送 channel 中的新消息（agent 收件) |

### 2.2 数据面（Plugin → Borgee）

| 接口 | 用途 |
|------|------|
| `heartbeat` | plugin 心跳，含 agent 工作中/空闲状态 |
| 语义动作（见 §1.3） | `create_artifact` / `reply_in_thread` 等 |
| `error_report` | plugin 主动上报 agent 故障原因 |

---

## 3. 与现状的差距

| 目标态 | 现状 | 差距 |
|--------|------|------|
| 一 plugin 管多 agent | OpenClaw plugin 已支持 multi-account，但仍是一对一 connection | 协议层加多 agent 注册接口 |
| BPP 中立协议 | 私有 OpenClaw SDK 协议 | 抽规范文档 + 重构 plugin 端代码作 reference impl |
| 语义动作层 | plugin 通过 WS `api_request` 直调 REST | 新增高级动作 API + dispatch 层 + 权限收敛 |
| Borgee 配置 SSOT | `users` 表里 agent 行只有 name/role/owner_id | **大改**：加 `agent_config` 表（含 schema-driven blob）、配置 UI、`agent_config_update` BPP 接口 |
| 热更新分级 | 没有"立即生效"机制 | plugin 端实现幂等 reload；runtime 不缓存 |
| 故障态分类 | 仅 online/offline | 状态机加 reason code，plugin 主动心跳 |

---

## 4. 不在本轮范围

- 跨 runtime 协作场景（OpenClaw + Hermes 混跑） → v2 后再讨论
- BPP 协议的版本协商机制 → 第 7 轮"Realtime / 事件流"
- runtime 安装管家的协议（remote-agent setup OpenClaw） → 第 6 轮"Remote-agent / Host bridge"
- 权限检查在 dispatch 层的具体实现 → 第 8 轮"Auth & 权限"
- agent 配置 UI 形态 → 第 11 轮"Client (web SPA)"
