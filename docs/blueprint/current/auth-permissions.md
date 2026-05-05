# Auth & Permissions — 权限模型

> 权限是 Borgee 把 "agent = 同事" 落到操作层面的载体。本文规范权限的目标态。
> 状态：建军 + 飞马 + 野马 对齐（2026-04-27）。前置阅读：[`concept-model.md`](concept-model.md)、[`agent-lifecycle.md`](agent-lifecycle.md)、[`plugin-protocol.md`](plugin-protocol.md)。

## 0. 一句话定义

> **存储层是 ABAC（capability list），UI 层用 bundle 简化。Agent 默认最小权限，主入口是动态请求 + 一键 grant。跨 org 只能减权，不能加权。**

---

## 1. 目标态（Should-be）— 四条立场

### 1.1 颗粒度：C 混合（存储 ABAC + UI bundle）

| 层 | 形态 |
|---|------|
| **存储层** | ABAC——`user_permissions(user_id, permission, scope)` 是 source of truth |
| **UI 层** | capability bundle——勾选预设组合，对用户友好 |
| **不入库** | role 不存为枚举字段——未来加能力零迁移 |

#### 为什么不是纯 RBAC

- 角色（PM/Dev/QA）= 套人格 → 与 [agent-lifecycle §2.1](agent-lifecycle.md) "Borgee 不定义角色"立场冲突
- 角色固定包 → 加新能力时所有角色都要改

#### 为什么不是纯 ABAC

- 让用户对着 30 个 capability 勾选 → 不可用
- bundle 是必要的 UI 抽象

#### Bundle 是 UI 糖，不是数据

- bundle 内容由 client 端定义，server 不识别
- "勾选 Workspace bundle" = 一次性 grant 多个 capability
- 后续加新 capability 到 bundle 不影响已 grant 的 agent（除非用户重勾）

### 1.2 Scope 层级：v1 三层

| Scope | v1 是否做 | 用途 |
|-------|----------|------|
| `*` | ✅ | 全局（admin 用） |
| `channel:<id>` | ✅ | per-channel 授权（已有） |
| `artifact:<id>` | ✅ | per-artifact 授权（agent 只能改某一份 PRD） |
| `workspace:<channel_id>` | ❌ | channel 级足够，workspace 是 channel 的下属概念 |
| `org:<id>` | ❌ | v1 "组织"对用户隐藏，scope 不暴露 |
| `expires_at` 列 | ✅ schema 保留, UI 不做 | 协议升级位，让未来加时间窗权限零迁移 |

### 1.3 授权 UX：B 主推 + A' 快速 bundle（无角色名）

#### 创建 agent 时

- **默认 `[message.send, message.read]`** — 最小起步 + 默认能读 channel 历史 (B29 / 4 人 review #1 决议, 2026-04-28: owner 想"agent 不偷看"是合理需求, 但默认开, 关闭走 agent 配置)
- **不**勾选角色或预设包——保持 [agent-lifecycle §2.1](agent-lifecycle.md) "无角色库"立场
- 创建即就绪，能发消息 + 能读所在 channel

#### 主入口（B）：动态请求

```
agent 尝试做某事
    ↓
权限检查失败
    ↓
BPP `permission_denied` 上行（带原因 + 缺失的 capability）
    ↓
server 给 owner 写一条 system message 到内置 DM
    ↓
"飞马 想 create_artifact 但缺权限 workspace.create
 [✓ 一键 grant 限于此 channel]   [✗ 拒绝]   [⚙ 高级设置]"
```

owner 一键 grant，agent 自动重试动作。

#### 辅助入口（A'）：快速 bundle，无角色名

按**能力命名**，不用角色名：

| ✅ 允许的 bundle 名 | ❌ 禁止 |
|--------------------|---------|
| `Messaging`（基础消息收发） | `PM` |
| `Workspace`（artifact 读写） | `Dev` |
| `Channel Admin`（成员管理、topic 改动） | `QA` |
| `Researcher`（按工作类型描述） | `Designer`（角色） |

> **理由**：角色名 = 套人格，跟 [agent-lifecycle §2.1](agent-lifecycle.md) 自相矛盾；能力名 = 描述操作集合，自然延伸。

#### UX 语义跟 host-bridge 一致

[host-bridge §1.3](host-bridge.md)："**装时轻，用时问，问时有理由**"——这一条同样适用于 agent capability：

- 不在创建时强迫 owner 评估"agent 该有什么权限"
- 用到才问，问时附原因

### 1.4 跨 org：A，owner-only

> [concept-model §1.3](concept-model.md) "协作可以，扩权不行" 的强落地。

| 角色 | 能做 | 不能做 |
|------|------|--------|
| **agent owner**（建军） | grant / revoke 自己 agent 的所有 capability | — |
| **channel owner**（野马，agent 不在他 org） | mute 别人的 agent / 移除别人的 agent | **不能** grant 别人 agent 的 capability |
| 任何 user | — | 修改其它 agent 的权限 |

#### 设计直觉

- 野马拉建军的 PMagent 进自己 channel = "我请你的助手来帮忙"
- 野马**不能**给建军的 PMagent 加 channel 权限——那等于"替建军决定他的助手能干什么"
- 野马**可以** mute / 移除 = "在自己地盘上保留控制权"
- **跨 org 只能减权，不能加权**

---

## 2. 关键不变量

| 不变量 | 含义 |
|--------|------|
| Admin = `*` | 系统管理员永远拥有所有 capability，不可剥夺 |
| Agent 默认最小 | 创建即只有 `[message.send, message.read]` (能发能读 channel)，其它由 owner 显式 grant; owner 可在 agent 配置里关掉 `message.read` 阻止 agent 看 channel 历史 |
| Bundle 不是数据 | UI 层概念，server 端只看 capability list |
| 跨 org 只减不加 | channel owner 对外部 agent 只能 mute/kick，不能 grant |
| Permission denied 走 BPP | 不靠 HTTP 错误码，由协议层路由到 owner DM |

---

## 3. v1 capability 清单（最小集，可扩展）

> 这是 v1 起步所需的 capability，命名遵循 `<domain>.<verb>` 风格。

### Messaging

- `message.send`
- `message.read` — gate `GET /channels/:id/messages`; agent 默认有, owner 可在 agent 配置里关掉以阻止 agent 看 channel 历史 (B29 / 4 人 review #1 决议, 2026-04-28)
- `message.edit_own`
- `message.delete_own`
- `mention.user`

### Workspace

- `workspace.read`
- `artifact.create`
- `artifact.edit_content`
- `artifact.modify_structure`（rename / archive / delete）

### Channel

- `channel.create`
- `channel.invite_user`
- `channel.invite_agent`（受 [concept-model §1.3](concept-model.md) "扩权不行"约束）
- `channel.manage_members`
- `channel.set_topic`
- `channel.delete`

### Org / 系统

- `agent.manage`（owner 管理自己的 agent）
- `*`（admin only）

---

## 4. BPP 协议增量

延续 [`plugin-protocol.md` §2](plugin-protocol.md)，本轮新增：

### 4.1 数据面（plugin → server）

| Frame | 字段 | 用途 |
|-------|------|------|
| `permission_denied` | `attempted_action`, `required_capability`, `current_scope`, `reason` | agent 尝试动作被拒，server 据此向 owner 推审批通知 |

### 4.2 控制面（server → plugin）

| Frame | 字段 | 用途 |
|-------|------|------|
| `capability_granted` | `agent_id`, `capability`, `scope` | owner grant 后通知 plugin 重试动作 |

---

## 5. 与现状的差距

| 目标态 | 现状 | 差距 |
|--------|------|------|
| ABAC + UI bundle | 当前已是 ABAC 表 (`user_permissions`) | 加 UI bundle 渲染层（纯 client 端） |
| 三层 scope | 有 `*` 和 `channel:` | 等 artifact 落地后加 `artifact:<id>` 渲染逻辑 |
| `expires_at` 列 | 无 | 加列（schema 不破），暂不业务化 |
| `permission_denied` BPP frame | 错误以 HTTP 4xx 返回 | 协议层加 frame + server 路由到 owner DM + 一键 grant UI |
| 无角色名 bundle | 当前没有 bundle | 设计 v1 bundle 命名表（参考 §1.3） |
| Channel owner 对外部 agent 的 mute/kick | 仅 channel.manage_members（移除） | 加 mute；明确"不能改 capability" |

---

## 6. 不在本轮范围

- agent 创建/管理 UI → 第 11 轮"Client (web SPA)"
- bundle 在 UI 上的具体形态（modal / sidebar / inline） → 第 11 轮
- 权限审批通知的实时推送 → [`realtime.md`](realtime.md)（已落定）
- admin = `*` 的具体管理路径 → 第 9 轮"Admin / 管理面"
- `expires_at` 时间窗权限的具体语义 → v2+
