# Acceptance Template — RT-0: /ws push 顶住 BPP

> 蓝图: `docs/blueprint/current/realtime.md` §2.3 (R3 已固化 push frame schema)
> Implementation: `docs/architecture/realtime.md` RT-0
> R3 决议: 4 人 review 立场冲突 #4 (飞马硬约束 + 野马硬条件, 2026-04-28)
> 依赖: **INFRA-2 (Playwright scaffold) 必须前置** (烈马 R3, latency ≤ 3s vitest 跑不了)

## 验收清单

### 数据契约 (字段固化, BPP-1 时必须 byte-identical 复制)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| ws event `agent_invitation_pending` 字段固化 (invitation_id / requester_user_id / agent_id / channel_id / created_at / expires_at) 顺序与蓝图 §2.3 表 1:1 | unit + CI grep | 飞马 / 烈马 | _(待填)_ |
| ws event `agent_invitation_decided` 字段固化 (invitation_id / state / decided_at) | unit | 飞马 / 烈马 | _(待填)_ |
| `internal/ws/event_schemas.go` 单文件存在 + godoc 注释 "BPP-1 必须 byte-identical 复制" | 人眼 (PR review) | 飞马 | _(待填)_ |

### 行为不变量

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| `POST /api/v1/agent_invitations` 成功后, `hub.SendToUser(invitee_owner_id, agent_invitation_pending)` 被调用 1 次 | unit (mock hub) | 战马 / 烈马 | _(待填)_ |
| `PATCH /api/v1/agent_invitations/{id}` 成功后, `hub.Broadcast(agent_invitation_decided)` 被调用 1 次 | unit | 战马 / 烈马 | _(待填)_ |
| client 收到 ws frame 后, InvitationsInbox state 自动更新 (取代 60s polling) | unit (vitest, mock ws) | 战马 / 烈马 | _(待填)_ |
| polling fallback 不删: ws disconnect 60s 后回退 polling (烈马 #189 review 加补) | E2E | 烈马 | _(待填, INFRA-2 后)_ |

### G2.6 schema lint (烈马 #189 review 调整)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| ~~CI lint `bpp/frame_schemas.go` 与 `ws/event_schemas.go` byte-identical~~ → 改成: ws schema 单文件 + 注释 "BPP-1 启用 byte-identical lint", BPP-1 落地时启用 | 人眼 (PR review) + CI grep (注释存在) | 飞马 / 烈马 | _(待填)_ |

### 野马 G2.4 硬条件 (Playwright stopwatch)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 邀请发出 → owner 端 ws frame 抵达 → InvitationsInbox 出现 latency **≤ 3s** | E2E (Playwright stopwatch) | 烈马 (跑) / 野马 (签) | _(待填, INFRA-2 后)_ |
| Playwright 截屏含 stopwatch (G2.4 5 张关键截屏之一) | E2E + 人眼 | 野马 | _(待填)_ |

### 蓝图行为对照 (闸 2)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| realtime §2.3 frame 字段表与 `ws/event_schemas.go` 一致 (字段名 / 顺序) | unit + CI grep | 飞马 / 烈马 | _(待填)_ |
| §2.3 "Phase 2 用 /ws hub 顶住, BPP 仍 Phase 4 接管" 立场在代码注释引用 | 人眼 | 飞马 | _(待填)_ |

### 退出条件

- 上表 12 项全绿
- INFRA-2 已 merged (前置)
- CM-4.2 (#186) 既有 vitest 9 项**回归全绿**
- G2.4 野马 demo 签字 + stopwatch 截屏入 `docs/evidence/cm-4/`
