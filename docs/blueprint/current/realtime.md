# Realtime — 推送、状态、回放

> Borgee 的 realtime 不只是"消息及时到"，是"让用户感到 AI 团队在工作"的最小活物感系统。
> 状态：建军 + 飞马 + 野马 对齐（2026-04-27）。前置阅读：[`channel-model.md`](channel-model.md)、[`plugin-protocol.md`](plugin-protocol.md)、[`agent-lifecycle.md`](agent-lifecycle.md)。

## 0. 一句话定义

> **v1 realtime 只做"足够让用户感到 AI 在工作"的最小集：四态状态点 + 语义化 progress + artifact 流式/commit + 人类端 full replay。强活物感留 v2。**

---

## 1. 目标态（Should-be）— 四条立场

### 1.1 活物感：B+ 轻动效为底，C 限定 artifact 场景

| 强度 | 用在哪里 |
|------|----------|
| **B（轻动效）** | 底子：typing 指示、reaction 飞入、新消息 highlight |
| **C（强活物感）** | **仅 artifact 编辑场景**：agent 工作时显示语义化 progress |
| **不做** | 多 agent 编排可视化（v2）、agent 头像独立动画（v2） |

#### ⭐ 关键纪律：所有 "thinking" 动画必须带**语义信息**

| ✅ 允许 | ❌ 禁止 |
|--------|---------|
| `"reading ~/code/main.go"` | 无 subject 的 spinner |
| `"writing section 3"` | "AI is thinking…" 这种无信息文字 |
| `"calling tool: bash"` | 闪烁的占位光标 |

> **沉默胜于假 loading。** 假装活物感 = 用户立刻看穿 = 信任崩塌。

BPP `progress` frame **强制带 `subject` 字段**——plugin 必须告诉 Borgee "agent 在做什么"，否则不展示。

### 1.2 Artifact 推送：C，agent 自决

BPP 协议加两种 frame：

| Frame | 用途 |
|-------|------|
| `artifact.progress` | 流式片段，带 `anchor_id`（指向 artifact 内具体段落） |
| `artifact.commit` | 整版完成，新版本入库 |

#### 选择策略

- **短文档** → agent 直接 `commit` 整版
- **长文档** → agent 用 `progress` 流式推，最后一次 `commit`
- **agent 自决**——Borgee 不强制策略

#### UI

- 流式过程中 artifact 面板顶部显示进度指示
- 进度指示**可点击 → 跳转到 artifact 对应段落**（借助 `anchor_id`）
- commit 后进度指示消失，artifact 出现"已更新"标记，可点击查看 diff

### 1.3 离线回放：⭐ 人 / agent 拆分

> 这条打掉一个隐性假设——"人和 agent 走同一套回放"。两端的需求**截然不同**。

| 接收方 | 回放模式 | 理由 |
|--------|----------|------|
| **人类客户端** | **A — full replay** | 协作场每条都可能是决策，不能漏。端上虚拟列表 + `last_seen` cursor 处理性能。 |
| **Agent 客户端** | **C — BPP session.resume + replay_mode hint** | runtime 自决 context 管理——agent 不需要"全部"消息，需要"够它接着工作"的子集。 |

#### Agent 端的 `replay_mode`

BPP `session.resume` 接口带 hint：

| `replay_mode` | 行为 |
|--------------|------|
| `full` | server 推断点之后所有 events（小 channel / 关键 agent） |
| `summary` | server 推一条"你错过了 X 条消息"摘要（runtime 自己拉详情） |
| `latest_n` | server 只推最近 N 条 |

runtime 在 `session.resume` 时根据 agent 配置和当前 context window 选 hint。

> **设计直觉**：人类的脑子能处理"虚拟列表里翻 200 条"，agent 的 context window 有限，给它倒灌历史 = 烧 token。

### 1.4 多端在线：A 全推默认，B 智能推 v1 末优化

#### v1 默认（A）

- 一个 user 的所有在线 client（web / mobile / 其他）**全推**
- 端上去重靠 `event.cursor` 唯一性
- 多端 `last_seen cursor` 通过 server 同步（任一端读 → 全端同步）

#### v1 末优化（B）

- 引入 "active client" 概念：最近 N 秒有交互的端
- 全文消息只推 active client
- 其他端只收 `notification badge`（"有 X 条未读"）

#### 不做（C）

- per-device 用户配置（"全推 / 摘要 / 静默"）—— v1 配置项已经够多
- 留给 v2 power user 选项

---

## 2. BPP 协议 frame 增量

延续 [`plugin-protocol.md` §2](plugin-protocol.md) 的接口集，本轮新增：

### 2.1 数据面（plugin → server）

| Frame | 字段 | 用途 |
|-------|------|------|
| `progress` | `subject` (必填), `agent_id` | agent 在做什么——驱动语义化 thinking 动画 |
| `artifact.progress` | `artifact_id`, `anchor_id`, `chunk` | artifact 流式片段 |
| `artifact.commit` | `artifact_id`, `version`, `summary?` | artifact 整版完成 |

### 2.2 控制面（server → plugin）

| Frame | 字段 | 用途 |
|-------|------|------|
| `session.resume` | `replay_mode: full \| summary \| latest_n`, `since_cursor` | agent 重连时 server 按 hint 推回放 |

### 2.3 Server → Client push frame (Phase 2 用 /ws hub 顶住, Phase 4 BPP 接管)

> **2026-04-28 4 人 review #4 决议 (B29 路线 + 飞马硬约束)**: Phase 2 用现有 `/ws` hub 加 invitation event 顶住, BPP 仍 Phase 4 接管。**关键约束**: Phase 2 `/ws` event payload schema **必须等同未来 BPP frame** (字段名 / 顺序 / 序列化), 这样 BPP 上线后 server 把发送源从 hub 切到 BPP frame, client handler 0 改。

| Frame | 字段 | 用途 | 阶段 |
|-------|------|------|------|
| `agent_invitation_pending` | `invitation_id`, `requester_user_id`, `agent_id`, `channel_id`, `created_at`, `expires_at` | owner 收到邀请通知 (取代 60s polling) | Phase 2 /ws hub → Phase 4 BPP |
| `agent_invitation_decided` | `invitation_id`, `state` (approved/rejected/expired), `decided_at` | 跨 client 同步邀请状态变更 | Phase 2 /ws hub → Phase 4 BPP |

**v0 audit row (CM-4.3a)**: Phase 2 `/ws` hub 发 invitation event; v1 切 BPP 时 server 切发送源 (`hub.Broadcast` → `bpp.SendFrame`), client handler 0 改 (因 schema 等同)。

**Schema 等同性强制 (飞马 R3 acceptance 补)**: CI lint 检查 `bpp/frame_schemas.go` 的 `agent_invitation_*` frame 与 `ws/event_schemas.go` 的对应 event payload **byte-identical 或 type alias**, 任何字段名/顺序/类型分歧 → CI fail。这是 v1 切换"client handler 0 改"承诺的硬保障。

**野马硬条件 (G2.4 签字门槛)**: 邀请发出 → owner 端收到通知 latency **必须 ≤ 3s** (stopwatch 截屏作 acceptance 证据)。60s polling 不接受。**烈马 R3 加补**: latency 验收必须用 Playwright (vitest 跑不了真 ws + UI 时序), **INFRA-2 Playwright scaffold 必须前置到 CM-4.3a 之前落地**, 不能等 G2.audit。

---

## 3. 与现状的差距

| 目标态 | 现状 | 差距 |
|--------|------|------|
| 语义化 progress | WS 只有 `typing` 类型 | BPP 加 `progress(subject)` frame + UI 渲染 + plugin 端实现 |
| artifact 流式 | artifact 概念尚未实现（见 [canvas-vision](canvas-vision.md)） | 等 artifact 落地后加 progress / commit |
| 人/agent 拆分回放 | events 表所有订阅者一视同仁（cursor-based） | server 端按 subscriber 类型路由 + BPP `session.resume` 加 replay_mode |
| 端上去重 + 多端 cursor 同步 | 每个 WS client 独立 | client 端去重逻辑 + server 端 `last_seen` 多端写入广播 |
| 智能推（v1 末） | 全推 | activity tracking + push 路由 |

---

## 4. 不在本轮范围

- 多 agent 编排可视化（"团队工作面板"） → v2
- per-device 配置 → v2 power user 选项
- 事件流的具体存储与索引 → 第 10 轮"数据层"
- 故障态显示与修复入口 UX → [`agent-lifecycle.md` §2.3](agent-lifecycle.md)（已落定）
