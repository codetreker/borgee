# Implementation · Concept Model

> 蓝图: [`../../blueprint/current/concept-model.md`](../../blueprint/current/concept-model.md)
> 现状: [`../../current/`](../../current/) — 当前 `users` 表只有 `id / role / owner_id`, 无 `organizations` 概念
> 阶段: ⚡ v0 (允许删库重建)
> 所属 Phase: Phase 1 (CM-1, CM-3) + Phase 2 (CM-4) — 见 [`execution-plan.md`](../00-foundation/execution-plan.md)

## 1. 现状 → 目标 概览

**现状**: organizations 概念**完全不存在**于代码——`users.owner_id` 隐式承担"agent 归属人类"的语义, 但跨多个 query 都是 join `owner_id`。

**目标**: blueprint §1.1 + §2 + §1.2 + §5.1/§5.2 — organizations 是数据层一等公民 (UI 不暴露), agent = 同事, 跨 org 协作离线 fallback 与邀请审批可见。

**主要差距**:
1. 缺 `organizations` 表
2. 缺 `users.org_id` 列 (以及主要业务表的 `org_id` 索引)
3. 注册流程缺"自动建 org"逻辑
4. 跨 org 协作 (blueprint §5.1, §5.2): 离线 fallback + 邀请审批 — 完全没实现

> **CM-2 (默认权限注册回填) 已挪到 [`auth-permissions.md`](auth-permissions.md) 的 AP-0**, 不在本模块范围。

## 2. Milestones

---

### CM-1: organizations 表落地

- **目标**: blueprint §1.1 + §2 — organizations 数据层一等公民, 1 person = 1 org, UI 不暴露。
- **范围**:
  - 新建 `organizations(id TEXT PK, name TEXT, created_at INTEGER)` 表
  - `users` 加 `org_id TEXT NOT NULL` 列 + 索引
  - 主要业务表 (`channels`, `messages`, `workspace_files`, `remote_nodes`) 加 `org_id` 索引
  - 注册流程: 新 human user 自动建一个 org, agent user 继承 owner 的 org
  - admin stats endpoint 切换到 `GROUP BY org_id`
  - **CM-1.4 visibility checkpoint**: admin 调试页显示当前用户 `org_id + 成员数` (士气可见信号, 不是 acceptance)
- **不在范围**:
  - UI 暴露 org_id ❌(blueprint §1.1 永远不暴露)
  - 多 user 共享 org ❌(blueprint §1.1)
  - 老数据 backfill ❌(v0 阶段, 删库重建 — v0 代码债 audit 已登记)
  - 默认权限回填 ❌(挪到 auth-permissions / AP-0)
- **依赖**: INFRA-1 (schema_migrations 框架, Phase 0 退出 gate)
- **预估**: ⚡ v0 阶段 3-5 天

#### PR 拆分

| PR | 内容 | Acceptance |
|----|------|-----------|
| CM-1.1 | schema: organizations 表 + users.org_id 列 + 索引 | 数据契约: 表/列/索引存在 |
| CM-1.2 | 注册流程: 自动建 org | E2E: 新注册 user 走完后, `organizations` 多一行, user 的 `org_id` 指向它 |
| CM-1.3 | admin stats: GROUP BY org_id | 蓝图行为对照: blueprint §2 "数据层一等公民"的查询路径 |
| CM-1.4 | admin 调试页: 显示当前 user 的 org_id + 成员数 | **visibility checkpoint** (非 acceptance, 早期可见信号) |

#### Acceptance spec (CM-1 整体)

- ✅ 启动后 SQLite 存在 `organizations` 表 + `users.org_id` 列 + 索引
- ✅ 新 user 注册 (human) 后, `organizations` 多一行, user.org_id = 该 org.id
- ✅ 通过 admin API 创建 agent 后, agent.org_id = owner.org_id
- ✅ `/admin-api/v1/stats` 返回值能看到"按 org 聚合"的结构
- ✅ 任何 user-facing API 响应中**不**出现 `org_id` 字段
- 📺 (visibility) admin 调试页能显示自己的 org_id + 成员数 (CM-1.4)

---

### CM-3: 资源归属切 org_id 直查 (Phase 1 后置)

> 顺序说明: CM-3 在 **CM-4 + ADM-0 之后** 做 (R3 2026-04-28: ADM-0 admin 拆表会改 admin user fixture, CM-3 写路径 acceptance 用同 fixture 必须等 ADM-0 落)。理由: CM-3 是查询路径优化 (蓝图行为对照), demo 不依赖, 不应阻塞 CM-4 的产品标志性 demo。

- **目标**: blueprint §2 — agent 创建资源归 owner, 查询不绕 owner_id JOIN。
- **范围**: 拆 2 个 PR, 各自独立可合 main:
  - **CM-3.1 写路径**: `messages.org_id`, `channels.org_id`, `workspace_files.org_id`, `remote_nodes.org_id` 在创建时填好 (sender 的 org_id)
  - **CM-3.2 读路径**: admin stats 之外的"按 owner 聚合"查询切到 `WHERE org_id = ?`
- **不在范围**:
  - 删 `owner_id` 字段 ❌ (v0 阶段也保留, 它是不同语义: agent 归属哪个 user)
- **依赖**: CM-1 (org_id 列存在), CM-4 (Phase 2 已退出, 协作闭环验证完毕), **ADM-0 (admin 拆表完成, R3 2026-04-28)**
- **预估**: ⚡ v0 阶段 1 周 (CM-3.1 + CM-3.2 各 3-4 天)

#### Acceptance spec
- ✅ CM-3.1 (数据契约): 创建 message/channel/file 时, `org_id` 自动填上, NOT NULL 约束跑得过
- ✅ CM-3.2 (蓝图行为对照): 主要 "我的 channel/我的文件" 查询走 `WHERE org_id = ?`, 不再 JOIN `owner_id` (grep 代码可证)
- ✅ 行为对照: blueprint §2 直查
- ✅ G1.4 闭合 (Phase 1 退出 gate): SQL EXPLAIN 走 idx_*_org_id + 黑名单 grep `JOIN.*owner_id` 命中 0

---

### CM-onboarding: Welcome channel + 默认 system message (R3 新增, 2026-04-28)

> **2026-04-28 4 人 review #6 决议** (野马 + 飞马盲点 B): 蓝图 concept-model §10 已固化 onboarding 硬产出。注册路径必须 auto-create #welcome channel, 否则业主第一分钟空屏, §1.4 团队感知主体验体感断档。

- **目标**: blueprint §10 (R3 已固化) — 业主注册第一分钟看到非空屏。
- **Owner**: 战马 / 飞马 / 野马 (立场签字 — 文案审过) / 烈马
- **范围**:
  - migration v=N: 注册流程 (`auth.handleRegister` + `admin.handleCreateUser` 两处) auto-create 1 个 system channel `#welcome` (业主 owner + member)
  - 同事务写 1 条 system message: "欢迎! 试试创建你的第一个 agent 协作伙伴 →" (野马审过文案)
  - `App.tsx` 注册成功后 default selected channel = #welcome (取代当前空屏 "👈 选择频道")
  - 旧 user backfill: migration 加现网"无 channel" 的 user 自动建 #welcome
- **不在范围**:
  - 完整 onboarding journey UI (看野马 `00-foundation/onboarding-journey.md` 1 周内出, 反推后续 surface)
  - agent 创建引导 / agent 上线 subject 文案 (留给 AL-1b)
- **依赖**: CM-1.2 (注册自动建 org) ✅
- **预估**: ⚡ v0 1-1.5 天 (含 system message kind 扩展 quick action button +0.5 天 + App.tsx 空状态降级 +0.2 天 — 详见 `00-foundation/onboarding-journey.md §5`)
- **Acceptance**:
  - 数据契约: 新 user 注册后 `channels` 表多 1 行 (kind=system, name=welcome), `channel_members` 多 1 行, `messages` 多 1 行 system
  - E2E (Playwright): 注册 → 进 app → 第一眼看到 #welcome 已 selected + 见 system 欢迎消息
  - 行为不变量 4.1: 旧 user backfill migration up → 无 channel 的 user 都有 #welcome (单测覆盖)

---

### CM-4: agent 同事感首秀 ⭐

> 这是 concept-model 模块的**产品标志性 milestone** (Phase 2 整个 Phase)——前面 CM-1 / CM-3 用户无感, CM-4 一次性把"agent 同事感"演示出来。
> **关闭前必须**: 野马跑 demo + 签字 + 留 3-5 张关键截屏 (闸 4, AI 团队不录视频)。

- **目标**: blueprint §1.2 (agent = 同事) + §5.1 (离线 fallback) + §5.2 (跨 org 邀请审批)。
- **范围**:
  - **`agent_invitations` 表 + 状态机** (`pending/approved/rejected/expired`)
  - **跨 org 邀请 API**: 创建 / 同意 / 拒绝
  - **邀请通知 UI**: owner 在 inbox DM 收到带 quick action 的 system message
  - **接受邀请后 agent 自动加入 channel**
  - **Minimal in-process presence map**: 接口契约 `IsOnline(userID) bool` + agent 连接时 register / 断开时 unregister
  - **离线检测**: agent 无 active session 即"离线"
  - **离线 fallback**: 给 owner 写 system message 到内置 DM
  - **5 分钟节流**: 同 owner+agent+channel 三元组在节流窗口内只发 1 条 system message
- **不在范围**:
  - "Escape hatch (允许任何人邀请)" 开关 ❌(blueprint §5.2 power user, v1+)
  - 邀请通知的 push notification ❌(等 client-shape 模块)
  - 完整 presence (含状态推送 / 多端) ❌(等 agent-lifecycle / realtime 模块, 本 milestone 仅最小集)
- **Presence 接口契约 (锁死, 路径 `internal/presence/contract.go`, 烈马 R2 锁定)**:
  - `Presence.IsOnline(userID string) bool` — 单端在线查询入口
  - `Presence.Sessions(userID string) []SessionID` — 多端会话列表 (AL-3 完整 presence 不重做接口的关键, 飞马 N3)
  - `Presence.Register(userID, sessionID)` — 触发点 = BPP frame 建连 (Phase 2 用 stub 桥接, BPP-1 上线后切真 frame, 不算返工 — 飞马 R2)
  - `Presence.Unregister(userID, sessionID)` — 触发点 = BPP 连接断开 / heartbeat 超时
  - 实现: 进程内 `sync.Map`, 不持久化, 重启即清零 (v0 接受)
- **依赖**: CM-1 (org_id 落地)
- **预估**: ⚡ v0 阶段 3 周 (战马实测; CM-4.0 schema 前置 + CM-4.3 拆 a/b)

#### PR 拆分 (顺序: 邀请 → UI → 离线 → 节流; 共 6 个 PR)

| PR | 内容 | Acceptance 类型 |
|----|------|---------------|
| CM-4.0 | schema (agent_invitations + 节流表) + 状态机单测 (非法转移 reject) | 数据契约 + 行为不变量 4.1 |
| CM-4.1 | 邀请创建/同意/拒绝 API | 数据契约 + 行为不变量 (重复同意 idempotent) |
| CM-4.2 | 邀请通知 UI (inbox DM quick action) + 接受后自动 join channel | E2E: A 邀请 → B 同意 → agent 出现在 channel 成员列表 |
| CM-4.3a | minimal presence map (接口含 IsOnline + Sessions, 路径 `internal/presence/contract.go`) + 注册/注销 (Phase 2 用 stub 桥接) | 数据契约 (接口契约文件 + snapshot test) + 行为不变量 4.1 (**单端单测**; 多端去重压测留 AL-3 — 战马 R2) |
| CM-4.3b | 离线检测 + system message 写入 | E2E (离线 → owner 收到通知) |
| CM-4.4 | 5 分钟节流 + 端到端串通 | 行为不变量 4.1 (节流计数, fake clock 单测) + E2E |

> **CM-4.4 PR merge 与签字解耦 (战马 R2)**: PR 只挂 4.1 节流单测 + E2E, 即可 merge。**4.2 用户感知签字 + 5 张关键截屏走闸 4 独立流程**, 在 CM-4 milestone 关闭时进行 (Phase 2 退出 gate G2.4), 不阻塞 PR merge — 防止"demo 不过整个 PR 被堵"。

#### Acceptance spec (CM-4 整体, E2E)

> **可演示的端到端 (野马签字, 留关键截屏)**:
>
> 1. 用户 A 创建 channel `#foo`
> 2. 用户 A 想邀请用户 B 的 agent (B-bot) 加入
> 3. A 触发邀请 → 系统给 B 写一条 system message "A 想邀请 B-bot 进 #foo"
> 4. B 在他的 inbox DM 看到通知, 点【同意】
> 5. B-bot 自动加入 #foo, 出现在成员列表
> 6. 用户 A 在 #foo 中 @ B-bot, 但 B-bot 离线
> 7. 5 秒内, B 收到 system message "B-bot 当前离线, #foo 中有人 @ 了它"
> 8. A 短时间内再 @ B-bot 5 次, B 不会再收到通知 (5 分钟节流)
>
> 整个 E2E 跑过 = CM-4 完成。**5 张关键截屏** 留档 `docs/evidence/cm-4/`:
> - 邀请通知 (inbox quick action)
> - 接受后成员列表 (agent 在 #foo)
> - 离线通知 (owner 的 DM 收到 system message)
> - 节流第 6 次无通知 (5 分钟内只收 1 条, 验立场)
> - **左栏团队感知** (打开 app 第一眼看到"我 + N 个 agent"列表, **每个 agent 项必显 subject 文案"在做什么"** — 验证立场 §1.4 + §11 联动, 野马 R2 加条)
>
> **Demo 中口播一次"agent↔agent 协作 Phase 4 支持"** (§1.3 体感断档兜底, 野马 R2)。
>
> 同目录放 `blueprint-sha.txt` 记录当时蓝图 commit。

---

### CM-5: agent 间独立协作 (Phase 4)

> X2 冲突裁决: 不塞 CM-4, 单独 milestone 放 Phase 4。理由: CM-4 已覆盖"邀请+离线+节流+presence"4 件事, 再塞 agent↔agent 会污染标志性 demo; 而 §1.3 协作语义需先有蓝图最小可观测定义 (烈马要求), 单独节奏更清晰。

- **目标**: blueprint §1.3 — agent 间独立协作允许, 但有边界 (扩权仍 owner-only, AP-3 已覆盖)。蓝图 §1.3 已锁定**最小可观测协作语义**: "协作 = message + capability 调用 (留 audit), 不含 secret 共享" (烈马 R2)。
- **Owner**: 飞马 / 战马 / 野马 (立场: 协作允许 vs 扩权禁止) / 烈马
- **范围**:
  - 复用 message 路径: agent A 在同 channel 给 agent B 发 message, 不需 owner 介入
  - capability 调用: agent A 调 agent B 已 grant 的 capability, 留 audit log (供 ADM-2 分层透明用)
  - 边界单测: agent A 不能借 agent B 的 capability 扩自己的权 (走 AP-3 owner-only 审批)
- **不在范围**: agent 共享 secret / agent 互相绕过权限 ❌
- **依赖**: CM-4 (presence + 邀请就位), **AP-3 (跨 org owner-only 边界, 飞马 R2 加补)**
- **预估**: ⚡ v0 阶段 1 周
- **Acceptance**: E2E (agent A 在 #foo 调 agent B 的 message capability, 不需 owner 同意; B 收到并响应) + 行为不变量 4.1 (扩权请求被 owner-only 拒绝 — 单测)

---

## 3. 模块完成判定

CM-1 + CM-3 + CM-4 + CM-5 全部 acceptance spec 通过 → concept-model 模块可发版。

完成后:
- `../../current/` 中关于 user 身份模型的描述需要更新 (但本文档不维护 current, 由实施时 audit)
- blueprint/current/concept-model.md 中 §7 差距表中所有"❌"都变成"✅"

## 4. 不在 concept-model 范围

- 跨 org 邀请的 push notification → client-shape
- 默认权限注册回填 → [`auth-permissions.md`](auth-permissions.md) AP-0
- 权限的 UI bundle → auth-permissions
- agent 状态四态 / 故障态 / 完整 presence → agent-lifecycle
- artifact 归属 → canvas-vision

## 5. Blueprint 反查表 (闸 3)

| Milestone | Blueprint §X.Y | 立场一句话 |
|-----------|----------------|-----------|
| CM-1 | concept-model §1.1 + §2 | 1 person = 1 org, UI 永久不暴露; 数据层 org first-class |
| CM-3 | concept-model §2 | 资源归 org, 查询直查 org_id 不走 owner_id JOIN |
| CM-onboarding | concept-model §10 (R3 2026-04-28) | 业主注册第一分钟非空屏, Welcome channel + system message |
| CM-4 | concept-model §1.2 + §1.4 + §5.1 + §5.2 | agent 是同事不是工具; 团队感知主体验; 离线 fallback 给 owner; 跨 org 邀请 owner-only |
| CM-5 | concept-model §1.3 + auth-permissions §1.4 | agent 间协作允许, 扩权仍 owner-only |
