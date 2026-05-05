# B29 (PRD) vs Blueprint — 6 条立场冲突对照表

作者: 战马 (zhanma) · 2026-04-28 · 用途: 4 人 review 加速参考。
范围: 建军提的 6 条 (P0×3 / P1×3)；引用行号截至当前 main commit `acf141e`。

> 阅读顺序: 每行 = 一个冲突点。「Blueprint 立场」「B29/PRD 立场」「现状代码」「冲突类型」「最小决议建议 (战马观点)」五栏。决议栏只是抛砖，不是结论。

---

## P0

### #1 — agent 默认权限 `message.send` vs `message.send + message.read`

| 维度 | 内容 |
|---|---|
| **Blueprint 立场** | `auth-permissions.md` §1.3 / §2 / §3: agent 默认**只**有 `message.send`；`message.read` 这个 capability **不存在**于 v1 清单 (§3 Messaging 只列 send/edit_own/delete_own/mention.user)。读 channel 走"频道成员"语义，不是 capability gate。 |
| **B29/PRD 立场** | B29 §需求3 已确认决策第 1 条 + 全局验收: agent 默认 `message.send + message.read`；并且 v1 capability 集合需要新增 `message.read` 以便 owner 控制 agent 看不看消息。 |
| **现状代码** | `store/queries.go:369` agent 默认 `[]string{"message.send"}`；`server.go:76` 只有 `RequirePermission("message.send", ...)` 一个 gate；**没有**任何 `message.read` 检查 → 任何 channel member (含 agent) 都能读。AP-0 (#177) 按 blueprint 落的。 |
| **冲突类型** | 🟡 **Delta 而非真冲突** — blueprint 当前没规定 read 的 capability 形态；B29 是在 blueprint v1 集合上**新增**一项。但行为后果有冲突: B29 要求"agent 不被显式 grant 就读不到", blueprint 默认"是 channel member 就能读"。 |
| **最小决议建议** | 二选一:<br>(a) 收进 blueprint: §3 Messaging 加 `message.read`，§1.3 agent 默认改成 `[message.send, message.read]`，AP-0 升级 1 行 + 1 个 backfill (现网 agent 都加 `message.read`)。<br>(b) 否决 B29 这条: 维持 channel 成员即可读，B29 改文。<br>战马倾向 (a) — owner 想"agent 不偷看 channel 历史"是合理需求，且代码代价小 (字符串数组多一个元素 + middleware 加一条 RequirePermission，只 gate `GET /channels/:id/messages`)。 |

### #2 — `users.role` 是否保留 `'admin'` 枚举

| 维度 | 内容 |
|---|---|
| **Blueprint 立场** | `concept-model.md` §6 (line 156-157) 明确写 `Human user: users 行, role IN ('member','admin')`；`Agent: users 行, role='agent'`。`admin-model.md` §3.1 line 31 `users.role = 'admin'` 标记 + `admin_grants` 表 promote 流程。 |
| **B29/PRD 立场** | B29 全局验收第 1 条: "系统中不存在 admin role (用户表只有 `member` 和 `agent`)"；admin 改成完全独立身份 (env bootstrap, 不在 users 表)。 |
| **现状代码** | `users.role` 仍是 enum-like string，"admin"/"member"/"agent" 都用；`testutil/server.go:58/73` 显式造 `role=admin` 的 user；`api/admin.go` 走 admin SPA 但底层仍是 user 行；`auth.go` 的 cookie session 不区分两套体系。`docs/current/server/data-model.md:29` 注册路径假设 admin 也是 user 行。 |
| **冲突类型** | 🔴 **真冲突** — concept-model 的 `admin_grants` 表 + role enum 路线 ≠ B29 的"完全独立身份" 路线。两个落地形态完全不同 (前者改 promote 流程, 后者要拆 admin 表 + auth 路径分叉)。 |
| **最小决议建议** | 必须先选一条主路线**再继续 Phase 2**，否则 CM-3 (org_id 直查) 会写到 `users.role='admin'` 的分支处理上，B29 路线一翻就全废。<br>战马观点: B29 路线代价大 (要新表 + 新 auth path + admin SPA 重接) 但模型干净；blueprint 路线代价小但 admin-as-user 的连锁矛盾 (`admin-model.md:20` 自己列了) 一直存在。建议**正面承担 B29 路线**，把 blueprint `concept-model.md` §6 + `admin-model.md` §3 改写。 |

### #3 — admin 能否创建 agent

| 维度 | 内容 |
|---|---|
| **Blueprint 立场** | `admin-model.md` §1.3 line 19: "admin **没有** agent、没有 channel、不在 user 表的 role 里走 product 流程"。`concept-model.md` 把 agent 严格挂到 owner (人类 user) 的 org 下；admin 不在任何 org，理论上自动不能创建 agent。但 blueprint **没有显式禁令**说 "admin 调 `POST /api/v1/agents` 必须 403"。 |
| **B29/PRD 立场** | B29 §需求3 决策: "Agent 不能创建其他 agent"；B27 §需求2 验收: "用户管理只能创建 user 类型账号" → admin 后台**只能造 user, 不能造 agent**。 |
| **现状代码** | `api/agents.go` `POST /api/v1/agents` 是 user-facing endpoint, 走 cookie session, 任意 user 都能调；admin SPA 用 `/admin-api/v1/users` (admin.go:196) 造 user 但**没有** create-agent endpoint。所以现状默认就是"admin 走 admin-api 不会造 agent"，但**如果 admin 用普通 cookie 登录普通 SPA**(因为 #2 现状 admin 也是 user 行) 就能用 user-api 造 agent。 |
| **冲突类型** | 🟡 **派生于 #2** — 只要 #2 决了, #3 自然跟着。如果走 B29 路线 admin 是独立身份, 就不可能走 user-api, 自然 enforce；如果维持 blueprint, 需要在 user-api `POST /agents` 加一条 `if user.Role == "admin": 403`。 |
| **最小决议建议** | 跟 #2 一起决, 不要单独决议否则 churn。 |

---

## P1

### #4 — BPP 在 Phase 4 落地, CM-4 用 stub 桥接

| 维度 | 内容 |
|---|---|
| **Blueprint 立场** | `auth-permissions.md` §4 + `plugin-protocol.md` 把 `permission_denied` / `capability_granted` / 跨 org 邀请通知都列为 BPP frame；`concept-model.md` §4.1/4.2 离线检测 + 邀请审批 system message 都依赖 BPP push。Phase 排期把 BPP 放 Phase 4, Phase 2 用 polling/stub 顶。 |
| **B29/PRD 立场** | B29 没直接谈 BPP, 但建军 P1 #4 指出: 没有真 BPP, owner 收不到真 push, 邀请通知体验是假的, 野马 (PM 验收角色) 不会签字。 |
| **现状代码** | CM-4.2 (#186) 客户端 60s 轮询 `listAgentInvitations('owner')` 当 push 替代品 (`Sidebar.tsx`); inbox 是真功能但 latency 60s, agent 离线检测完全没接 (`concept-model.md` §4.1 提的 system message 还没实现)。 |
| **冲突类型** | 🟡 **优先级冲突** — 不是逻辑冲突，是工作量/验收 gate 冲突。 |
| **最小决议建议** | 三选一:<br>(a) Phase 重排, 把 BPP 提到 Phase 2 后段, CM-4.3 直接走 push frame, 不做 polling 升级。<br>(b) 维持 polling 但接受 v0 验收降级 (野马签 v0/v1 两阶段)。<br>(c) Phase 2 做最小 SSE/WS push (复用现有 `/ws` hub) 顶住 BPP 缺失, BPP 仍然 Phase 4 完整化。<br>战马倾向 (c): 现网已经有 `/ws` channel push, 加一个 `agent_invitation_pending` event type 是 ≤ 200 行小活, 不挡 Phase 2 验收, 也不污染未来 BPP 设计。 |

### #5 — Phase 2 三态缺 busy + §11 thinking subject 来源

| 维度 | 内容 |
|---|---|
| **Blueprint 立场** | `agent-lifecycle.md` §2.3 (line 72-90) 四态 `online/busy/idle/error`；blueprint 没明确 busy 的 source-of-truth (写的是"runtime 报告活动 / message 待 ack" — 两条触发都假设 BPP/ack 已就位)。Phase 排期 §11 (thinking) 暂未在 repo 定位到对应文档段, 建军提的 "thinking subject 来源缺" 指的是 `chat_message` ack 后没有"agent 正在思考"的 subject 字段。 |
| **B29/PRD 立场** | B29 没列状态, 但 P1 #5 指出 Phase 2 验收清单里"在线列表展示 agent 状态"无法分辨 online vs busy (没有数据来源)。 |
| **现状代码** | Hub (`internal/hub`) 只 track online/offline；message ack 在 `messages.go` 的 chat_message 走 `message_ack` WS frame, 但没在 user/agent 上贴"busy" 标记。前端 `Sidebar.tsx` 直接用 `state.onlineUserIds` set, 没有 busy/idle 维度。 |
| **冲突类型** | 🟡 **数据缺失而非立场冲突** — blueprint 有目标态, 但 Phase 2 验收清单引用了这个目标态而 source 还没接。 |
| **最小决议建议** | 把 busy/idle 砍出 Phase 2 验收清单, 挪到跟 BPP 同期 (一旦 plugin 上行 `task_started`/`task_finished`, busy 自然就有 source)。Phase 2 只承诺 online/offline 二态 + error 旁路。这个砍法对 v0 体验影响小, 对工期影响大 (避免造一套 stub busy)。 |

### #6 — 缺 "新用户第一分钟" 端到端旅程

| 维度 | 内容 |
|---|---|
| **Blueprint 立场** | `agent-lifecycle.md` §2.1 "默认路径(一键 onboarding)" + `host-bridge.md` §1.3 "装时轻, 用时问" 都暗示 onboarding 是关键体验, 但**没有**端到端文档把 "注册 → 进首个 channel → 收到第一条 system 提示 → 创建 agent → 看到 agent 上线" 串起来。 |
| **B29/PRD 立场** | B29 §需求2 验收: "User 登录后所有功能立即可用, 不存在缺权限"；PRD-v3.md §邀请注册一节列了注册流但同样没串到"第一分钟体验"。 |
| **现状代码** | `RegisterPage` → `LoginPage` → `App.tsx` 的 `init()` 串行 load (auth.go:151 GrantDefaultPermissions(member, "*") + auto-select first channel)；没有 onboarding tour, 没有空状态引导, 新 org 无 channel 时 `App.tsx` line 209 只显示"👈 选择一个频道开始聊天"。 |
| **冲突类型** | 🟢 **盲区而非冲突** — 没有立场互相打架, 是没人写过这条旅程。 |
| **最小决议建议** | 野马 (PM) 出一份 1 页 onboarding journey doc (注册 → 首个 channel → 默认 system message → 创建 agent → agent 上线), 列出每一步的 success/empty/error 状态; 战马/飞马按这份 doc 反推 server/client 还缺哪些 surface (估计: 默认 system channel 自动加入 + 空状态 CTA, 不需要新表)。建军 sign off 后才进 Phase 2 验收。 |

---

## 决议依赖图 (战马观点)

```
#2 (admin role)  ─┬─→ #3 (admin 创 agent)
                  └─→ 影响所有 Phase 2 后续 (CM-3 / 权限中间件)
#1 (message.read) ──→ AP-0 backfill 1 次 (≤ 50 行)
#4 (BPP / push)   ──→ Phase 排期, 不挡逻辑
#5 (busy)         ──→ Phase 2 验收范围裁剪
#6 (onboarding)   ──→ 野马补 doc 即可
```

**最高优先级**: #2 必须最先决 (其它都派生或独立)。
**最快决**: #1 / #5 / #6 各 ≤ 30min 讨论可决。
**最贵决**: #4 (Phase 重排) + #2 (admin 拆表) 各预留 2h+。
