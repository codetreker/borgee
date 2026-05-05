# Implementation · Realtime

> 蓝图: [`../../blueprint/current/realtime.md`](../../blueprint/current/realtime.md)
> 现状: 当前已有 messages 推送, 但没有"活物感"动效, 没有 artifact 推送, 离线回放未拆人/agent
> 阶段: ⚡ v0
> 所属 Phase: **RT-1 在 Phase 3 (CV-4 demo 必需), RT-2/RT-3 在 Phase 4**

## 1. 现状 → 目标 概览

**现状**: 推送框架有, "agent 在工作"活物感缺失; artifact 推送无; 离线回放未拆人/agent; thinking 无 subject 反约束未实施。
**目标**: blueprint §1.1~§1.4 + 核心 §11 — 活物感 B+ 轻动效, artifact 推送 agent 自决, 离线回放人/agent 拆, 多端 A 全推, **沉默胜于假 loading (thinking 必带 subject, 无 subject 不渲染)**。

## 2. Milestones

### RT-0: /ws push 顶住 BPP (Phase 2 R3 新增, 2026-04-28)

> **2026-04-28 4 人 review #4 决议** (飞马硬约束 + 野马硬条件): 蓝图 realtime §2.3 已固化 `agent_invitation_pending` push frame schema, 必须等同未来 BPP frame。Phase 2 用现有 `/ws` hub 顶住, BPP 仍 Phase 4 接管。
> **野马硬条件 G2.4**: 邀请发出 → owner 端通知 latency ≤ 3s (Playwright stopwatch 截屏作 acceptance 证据)。60s polling 不签字。

- **目标**: blueprint realtime §2.3 — Phase 2 用 /ws hub 实时推 invitation 通知, 取代 60s polling; schema 等同未来 BPP frame, v1 切换 client 0 改。
- **Owner**: 飞马 (主, schema 设计) / 战马 (实现) / 野马 (latency 签字) / 烈马 (Playwright + lint)
- **范围**:
  - **Hub 加 `SendToUser(userID, frame)` API** (~50 行, 战马 R3 估)
  - WS event `agent_invitation_pending` (字段: invitation_id / requester_user_id / agent_id / channel_id / created_at / expires_at)
  - WS event `agent_invitation_decided` (字段: invitation_id / state / decided_at)
  - server 端 `POST /api/v1/agent_invitations` 发出后立即 `hub.SendToUser(invitee_owner_id, frame)`
  - server 端 `PATCH /api/v1/agent_invitations/{id}` decided 后广播 `agent_invitation_decided` (跨 client 同步)
  - **client 端**: 接 ws frame 直接更新 InvitationsInbox state (取代 60s polling)
  - **CI lint** 强制 `bpp/frame_schemas.go` 与 `ws/event_schemas.go` byte-identical 或 type alias (蓝图 realtime §2.3, 飞马 R3)
- **不在范围**:
  - polling fallback 不删 (烈马 R3 留作降级)
  - artifact 推送 (RT-1 Phase 3)
  - 离线回放 (RT-2)
  - 完整 BPP frame (Phase 4 BPP-1 接管)
- **依赖**: **INFRA-2 (Playwright scaffold, 必须前置)**, CM-4.1 (#185 邀请 API) ✅
- **预估**: ⚡ v0 1.5-2 天 + INFRA-2 1-2 天 = 共 2-4 天
- **Acceptance** (G2.1 + G2.4 提前预演):
  - 数据契约: ws event schema (字段名 / 顺序 / 类型) 锁定文件存在
  - 行为不变量 4.1: CI lint 检查 bpp/ ↔ ws/ byte-identical, 任何分歧 fail
  - E2E (Playwright, INFRA-2 前置后): 邀请发出 → owner 端 ws frame 抵达 → InvitationsInbox 自动出现, **stopwatch ≤ 3s**
  - 用户感知签字 4.2: 野马跑 demo, 截屏含 stopwatch 证据 (这条进 G2.4)

### RT-1: artifact 推送 (Phase 3, CV-4 demo 必需)

- **目标**: blueprint §1.2 — artifact 推送 agent 自决, 非 server 强推。
- **Owner**: 飞马 / 战马 / 野马 / 烈马
- **范围**: BPP 数据面 frame `ArtifactUpdated` (agent 主动发); server 转发到 workspace 在线 client
- **依赖**: BPP-1, CV-1
- **预估**: ⚡ v0 4-5 天
- **Acceptance**: E2E (agent push artifact → 用户秒看到, 无需轮询)

### RT-2: 离线回放 (人/agent 拆分)

> 野马 review: 取消 ⭐ 标志性 — 离线回放是健壮性, RT-3 升 ⭐ 替代。

- **目标**: blueprint §1.3 — 人/agent 回放策略不同 (人重时间序, agent 重补漏)。
- **Owner**: 飞马 / 战马 / 野马 (立场关键) / 烈马
- **范围**: 回放 cursor (events 双流); 人拉时间序; agent 重连拉 missing
- **依赖**: BPP-1, DL-2
- **预估**: ⚡ v0 1 周
- **Acceptance**: 行为不变量 4.1 (agent 不漏 frame, 单测) + E2E (用户离线 1h 后 inbox 顺序正确)

### RT-3: 多端全推 + 活物感 + thinking subject 反约束 ⭐

> 野马 review: 升 ⭐ — 活物感比离线回放更产品立场 (核心 §11)。

- **目标**: blueprint §1.4 + §1.1 + 核心 §11 — 多端 A 全推, 主界面 B+ 轻动效, **沉默胜于假 loading**。
- **Owner**: 野马 (主, demo+签字) / 战马 / 飞马 / 烈马
- **范围**:
  - 多端 socket 全推 (v1 末优化智能推)
  - thinking indicator / typing dots
  - **thinking subject 反约束**: agent frame 缺 subject → server reject + UI 不渲染 (核心 §11 立场)
- **预估**: ⚡ v0 1 周
- **Acceptance** (⭐ 标志性, 4.1+4.2 双挂):
  - 行为不变量 4.1: nil subject 的 typing/thinking frame → server reject + UI 不渲染 (单测; BPP-1 已有 grep 校验, 这里加端到端单测)
  - 用户感知签字 4.2 (野马跑 demo 截 3 张):
    - thinking 带 subject (例: "正在搜索 docs...")
    - 多端同步 (两个 client 同时看到)
    - **反约束截屏**: agent 试图发 nil subject → UI 空白, 不出现裸 spinner (验立场底线)

## 3. 不在 realtime 范围

- 智能推 (per-device 优化) → v1 末
- BPP 协议本身 → plugin-protocol

## 4. Blueprint 反查表

| Milestone | §X.Y | 立场一句话 |
|-----------|------|-----------|
| RT-1 | realtime §1.2 | artifact 推送 agent 自决 |
| RT-2 | realtime §1.3 | 人/agent 回放策略拆, 不漏不重 |
| RT-3 | realtime §1.4 + §1.1 + 核心 §11 | 多端全推 + 轻动效活物感 + thinking 必带 subject |
