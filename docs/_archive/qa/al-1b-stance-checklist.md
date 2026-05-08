# AL-1b 立场反查表 (busy/idle 状态扩展, BPP 同期)

> **状态**: v0 (野马, 2026-04-29)
> **目的**: AL-1b 实施 PR 直接吃此表为 acceptance; 战马C #453 spec brief / 烈马 acceptance template `al-1b.md` / 实施 review 拿此表反查立场漂移. 一句话立场 + §X.Y 锚 + 反约束 (X 是, Y 不是) + v0/v1.
> **关联**: 蓝图 `agent-lifecycle.md` §2.3 (5-state, 2026-04-28 4 人 review #5 决议 busy/idle 跟 BPP 同期 Phase 4) + §11 文案守 (沉默胜于假活物感); 战马C spec #453 §0 ①②③ (拆三路径 / BPP 单源 / 文案三态合并); AL-1a #249 三态机源 (online/offline/error + 6 reason); AL-3 #310 PresenceTracker (session-level 心跳); AL-4 #398 agent_runtimes (process-level status); BPP-2 留账 task_started/task_finished frame; 野马 #al-1b-content-lock.md 7 处字面锁; 跨 milestone reason 八处单测锁.
> **依赖**: AL-1a ✅ #249 三态机 (online/offline/error 不动); AL-3 ✅ #310/#317/#324 presence_sessions 不动; AL-4 ✅ #398 agent_runtimes 不动; BPP-1 ✅ #304 envelope CI lint (不裂 frame namespace); BPP-2 待落 task_started/task_finished frame (AL-1b.2 server endpoint 真接管前 stub).
> **#338 cross-grep 反模式遵守**: AL-1b 是新表 (agent_status, v=21), 既有 AL-1a/AL-3/AL-4 表 + lib/agent-state.ts REASON_LABELS 字面已稳定, 立场跟既有 byte-identical 引用不臆想新词.

---

## 1. AL-1b 立场反查表 (5-state busy/idle 扩展)

| # | 立场锚 | 一句话立场 | 反约束 (X 是, Y 不是) | v0 / v1 |
|---|--------|----------|----------------------|---------|
| ① | 战马C spec #453 §0 ① + 蓝图 §2.3 + AL-1a/AL-3/AL-4 拆死 | **三路径互不污染** — AL-1b busy/idle 跟 AL-1a online/offline/error + AL-3 presence_sessions + AL-4 agent_runtimes 三表三路径拆死, 表无 cursor / is_online / last_error_reason / endpoint_url / process_kind 字段 (反域漏) | **是** `agent_status` 单表 (`agent_id PK / state CHECK ('busy','idle') / last_task_id nullable / last_task_started_at + last_task_finished_at Unix ms / created_at + updated_at`) 仅锁 task in-flight 真值; **不是** 共享 AL-1a state 列 (反 AL-1a 三态字面破); **不是** 写 presence_sessions (反 AL-3 hub 心跳混用); **不是** 写 agent_runtimes.status (反 AL-4 process status 混用); **改 = 改 4 个 spec** (AL-1a #249 + AL-3 #303 + AL-4 #379 v2 + 本 stance) | v0/v1 永久锁 — 三路径拆死是产品定位红线 |
| ② | 战马C spec #453 §0 ② + 蓝图 §2.3 决议字面 | **BPP 单源 — busy/idle source 必须是 plugin 上行 `task_started`/`task_finished` frame, 没 BPP 不 stub** | **是** schema 先落占号 (AL-1b.1 v=21), server endpoint 仅暴露 GET 不暴露 PATCH 直接改 state (避免人工伪造); state machine 转移由 BPP frame 三路径触发 (`task_started` → busy / `task_finished` → idle / 5min IdleThreshold → idle); **不是** stub 假数据 (蓝图 §2.3 决议字面 "stub 一旦上 v1 要拆掉 = 白写"); **不是** server 自动合成 busy/idle (除 5min IdleThreshold const); **不是** client/admin 直接 PATCH (反人工伪造); BPP-2 落地后 frame 真接管, BPP-2 未落 — server endpoint 返默认空状态 (跟 AL-4 stub `runtime_not_registered` 同模式) | v0: schema + GET-only; v1: BPP-2 落地后 PATCH internal 路径走 frame handler (非 admin 路径) |
| ③ | 战马C spec #453 §0 ③ + 蓝图 §11 + AL-1a/AL-3 文案锁 | **client UI 见 5-state 合并显示**, 但 schema 仅 2 态 — 客户端 `describeAgentState()` 优先级 `error > busy > idle > online > offline` 八处单测锁 | **是** schema 仅 `('busy','idle')` 2 态 byte-identical (反约束枚举外 reject); client 合并 5-state 走 `lib/agent-state.ts::describeAgentState()` 单源 (跟 AL-1a #249 + AL-3 #305 字面对齐); 改优先级 = 改八处单测锁 (#453 spec §0 ③ + acceptance §2.1 + 本 stance + 野马 #al-1b-content-lock ②); **不是** server 端合成 5-state (那是 GET endpoint 返复合 JSON, 但 UI 单字面渲染走 client 函数); **不是** 多 dot UI 一 agent (跟 AL-3 #303 ⑤ + 野马 #al-1b-content-lock ② "一 agent 一 dot" 同源) | v0: 5-state client 合并; v1 同 |
| ④ | 战马C spec #453 §1 + AL-1a #249 6 reason byte-identical | **error reason 走 REASON_LABELS 八处单测锁 (跨 milestone byte-identical)** | **是** error tooltip `"故障 ({reason_label})"` byte-identical 跟 AL-3 #305 ③ + AL-1a #249 6 reason codes (`api_key_invalid|quota_exceeded|network_unreachable|runtime_crashed|runtime_timeout|unknown`); **八处单测锁** (改 reason = 改八处): AL-1a `agent/state.go` Reason* + AL-3 `lib/agent-state.ts` REASON_LABELS + CV-4 #380 ③ + AL-2a #454 ④ + AL-1b 本 content-lock ⑤ + AL-4 #387/#461; **不是** 自造 busy/idle 专属 reason (busy/idle 不带 reason, 跟 AL-1a 三态 reason 拆死); **不是** raw error.message 显示 (隐私 + UX); **不是** error reason 漂出枚举 (枚举锁) | v0: reason 走 REASON_LABELS 八处单测锁; v1 同 (AL-4 落地后切真路径 reason 不再 'runtime_not_registered') |
| ⑤ | 战马C spec #453 §0 ② + ADM-0 §1.3 红线 + AL-3 ⑦ + AL-4 ② + AL-2a ④ 四源同模式 | **admin god-mode reject PATCH busy/idle, server 不挂 admin 写路径** | **是** server admin god-mode endpoint **不挂** PATCH `/admin-api/v1/agents/:id/status` (跟 ADM-0 §1.3 红线 + AL-3 #303 ⑦ + AL-4 #379 v2 §3 + AL-2a #454 ④ **四源同模式**); admin GET /admin/agents/:id/status 仅返元数据白名单 (state + last_task_id) 不返 raw `intent_text` 类敏感字段 (跟 CV-4 #365 立场 ⑦ + AL-2a #454 ⑦ 同模式); **不是** admin 可改 (反人工伪造, 跟 #11 silent default + 立场 ② BPP single source 同源); **不是** owner perm 跟 admin role 混用 (RBAC 拆死) | v0/v1 永久锁 — admin 不入 busy/idle 写路径 |
| ⑥ | 战马C spec #453 §1 AL-1b.3 + AL-2a stance ⑤ + RT-1 4 frame 已锁 | **AL-1b 不裂 frame namespace, 走 GET pull 不上 push** | **是** RT-1 4 frame (ArtifactUpdated 7 / AnchorComment 10 / MentionPushed 8 / IterationStateChanged 9) + BPP-1 9 frame 已锁; AL-1b 不引入第 5 个 RT-frame (跟 AL-4 #379 v2 立场 ⑥ + AL-2a #454 ⑤ + #382 立场 ① "不引入新 frame" 同精神); 状态变化由 BPP `task_started`/`task_finished` frame 自然推 (BPP-2 落地后), AL-1b client SPA 走 GET `/api/v1/agents/:id/status` SWR debounce 5s 拉; **不是** `AgentStatusChangedFrame` push (反向 grep `AgentStatusChangedFrame|StatusUpdatedFrame` count==0); **不是** SPA 1s polling spam (反 DDoS 自家 server, 跟 AL-3 #303 ⑤ "5s 节流" 同源); **不是** AL-1b 阻塞 BPP-2 (schema 先落占号, BPP-2 落地后真接管) | v0: GET pull + 5s debounce; v1: BPP-2 落地后 frame 自然推 (BPP-1 既有 frame 携带, 不裂新 frame) |
| ⑦ | 蓝图 §11 + AL-3 #305 silent default + AL-2a #454 ⑦ + #382 立场 ⑤ 同精神 | **agent 状态变化 silent default, 不发 system message broadcast** (#11 沉默胜于假活物感) | **是** busy/idle 状态变化 — 不发 system message / 不 fanout / 不污染 channel chat 流 (跟 AL-3 #305 ③ "agent join silent" + AL-2a #454 ⑦ + #382 立场 ⑤ + 蓝图 #11 同精神); 仅 owner SPA 在 agent settings / sidebar dot 看到 — UI 单点单源 (跟 AL-3 dot UI 同模式); **不是** "{agent_name} 进入工作状态" system message (跟 #11 silent default 永久锁); **不是** fanout 给 channel members (这是 owner 自己的事); **不是** SPA badge 红点提示状态变化 (反 UI 噪声 — dot 已经够直接, 跟 AL-3 dot 同精神) | v0/v1 永久锁 — agent silent default 是用户感知 #11 立场红线 |

---

## 2. 黑名单 grep — AL-1b 实施 PR merge 后跑, 全部预期 0 命中 (除标 ≥1)

```bash
# 立场 ① — agent_status 表三路径拆死, 表无域漏字段
grep -rnE 'agent_status.*(is_online|cursor|last_error_reason|endpoint_url|process_kind)|ALTER TABLE agent_status.*ADD.*(is_online|cursor|last_error_reason)' packages/server-go/internal/migrations/ | grep -v _test.go
# 立场 ② — schema CHECK 仅 2 态 (预期 ≥1)
grep -rnE "state.*CHECK.*\\('busy','idle'\\)" packages/server-go/internal/migrations/al_1b_*.go 2>/dev/null | grep -v _test.go  # 预期 ≥1
# 立场 ② — server 不开 PATCH /status 路径 (反人工伪造)
grep -rnE 'PATCH.*\\/api\\/v1\\/agents\\/.*\\/status|admin.*PATCH.*agent.*status' packages/server-go/internal/api/ | grep -v _test.go
# 立场 ③ — describeAgentState 5-state 合并函数 (预期 ≥1, 跟 AL-3 #305 同模式)
grep -rn 'describeAgentState' packages/client/src/lib/agent-state.ts 2>/dev/null | grep -v _test  # 预期 ≥1
# 立场 ③ — 多 dot UI 反约束 (一 agent 一 dot)
grep -rnE '<AgentStatusDot.*<AgentStatusDot|agent-status-dot.*agent-status-dot' packages/client/src/components/ 2>/dev/null | grep -v _test
# 立场 ④ — error reason 走 REASON_LABELS (预期 ≥1)
grep -rn 'REASON_LABELS\[' packages/client/src/lib/agent-state.ts 2>/dev/null | grep -v _test  # 预期 ≥1
# 立场 ④ — busy/idle 不带 reason 反约束
grep -rnE 'busy.*reason|idle.*reason|state.*=.*busy.*reason' packages/server-go/internal/api/agent_status*.go 2>/dev/null | grep -v _test.go
# 立场 ⑤ — admin god-mode 不挂 PATCH /status
grep -rnE 'admin.*PATCH.*agent.*status|/admin-api.*PATCH.*status|GodMode.*agent_status.*PATCH' packages/server-go/internal/api/admin*.go | grep -v _test.go
# 立场 ⑥ — 不裂 frame namespace (RT-1 4 frame + BPP-1 9 frame 已锁)
grep -rnE 'AgentStatusChangedFrame|StatusUpdatedFrame|TaskStartedFrame.*new|TaskFinishedFrame.*new' packages/server-go/internal/ws/ | grep -v _test.go
# 立场 ⑥ — polling spam 防御 (debounce 5s 锁, 反 1s spam)
grep -rnE 'setInterval.*1000.*status|polling.*1s.*status|fetch.*status.*every.*1s' packages/client/src/ 2>/dev/null | grep -v _test
# 立场 ⑦ — agent silent default — 反 system message broadcast 状态变化
grep -rnE "['\"]\\{agent_name\\} 进入工作状态['\"]|agent_status.*system.*message.*broadcast|state.*change.*fanout" packages/server-go/internal/api/ | grep -v _test.go
# 立场 ⑦ — SPA badge 红点提示反约束 (UI 噪声)
grep -rnE 'AgentStatusBadge.*pulse|status.*change.*notify' packages/client/src/components/AgentStatus*.tsx 2>/dev/null | grep -v _test
```

---

## 3. 不在 AL-1b 范围 (避免 PR 膨胀, 跟 #453 spec + acceptance + 野马 #al-1b-content-lock 同源)

- ❌ stub 假 busy/idle 数据 (蓝图 §2.3 决议字面 "stub 一旦上 v1 要拆掉 = 白写", 立场 ② BPP single source 永久锁)
- ❌ client/admin 直接 PATCH busy/idle (反人工伪造, 立场 ②⑤ 永久锁)
- ❌ AgentStatusChangedFrame WS push (RT-1 4 frame + BPP-1 9 frame 已锁不裂, 立场 ⑥)
- ❌ 多 dot UI / 多面板拆 5-state (一 agent 一 dot, 立场 ③ + AL-3 #303 ⑤ 同源)
- ❌ idle 时间戳 UI / 历史 timeline (跟 AL-3 ⑥ "不显 last_heartbeat_at" + #11 反约束同源, 留 v3+)
- ❌ busy 任务进度条 / 百分比 (agent 不报百分比, 跟 CV-4 #380 ③ "running 进度条无具体百分比" 同精神)
- ❌ system message broadcast 状态变化 (跟 #11 silent default + AL-3/AL-2a 同精神, 立场 ⑦ 永久锁)
- ❌ admin SPA 改 busy/idle (admin 不入业务路径, ADM-0 §1.3 红线 + 立场 ⑤)
- ❌ AL-1b 阻塞 BPP-2 (schema 先落占号, 跟 AL-4 stub `runtime_not_registered` 同模式 — 立场 ②)
- ❌ busy/idle 自造 reason 枚举 (busy/idle 不带 reason, 跟 AL-1a 三态 reason 拆死, 立场 ④)

---

## 4. 验收挂钩

- AL-1b.1 schema PR (v=21): 立场 ①②③ — `agent_status` 表 (`agent_id PK / state CHECK ('busy','idle') / last_task_*`) + 反向断言 9 列 NoDomainBleed (跟 #453 spec §1.3 + acceptance §1.3 同源) + state CHECK 2 态 + idempotent + `idx_agent_status_state` + NoCascadeDelete
- AL-1b.2 server PR: 立场 ②④⑤⑥⑦ — GET `/api/v1/agents/:id/status` 5-state 合并 (`error > busy > idle > online > offline`) + admin god-mode reject PATCH (四源同模式) + state machine BPP `task_started/task_finished` + 5min IdleThreshold const + 反向断言无 system message broadcast + 不裂 frame namespace
- AL-1b.3 client PR: 立场 ③④ — `describeAgentState()` 5-state 合并八处单测锁 + dot DOM `data-presence` + `data-task-state` + tooltip 中文文案锁 + REASON_LABELS 八处单测锁 + SWR debounce 5s polling
- AL-1b entry 闸: 立场 ①-⑦ 全锚 + §2 黑名单 grep 全 0 (除标 ≥1) + 跨 milestone byte-identical (reason 八处单测锁: AL-1a #249 + AL-3 #305 + CV-4 #380 + AL-2a #454 + 本 + AL-4 #387/#461) + RT-1 4 frame + BPP-1 9 frame 锁守 + BPP-2 留账锁

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 野马 | v0, 7 立场 (三路径拆死改=改 4 spec / BPP 单源不 stub / 5-state client 合并八处单测锁 / error reason 五处单测锁 / admin god-mode reject PATCH 四源同模式 / 不裂 frame namespace 走 GET pull debounce 5s / agent silent default 跟 #11 + AL-3/AL-2a 同精神) 承袭战马C #453 spec 3 立场拆细 + 跨 milestone byte-identical 锁 (reason 五处 + admin 元数据 only 四源 + frame 不裂 + silent default 三源); 13 行反向 grep (含 3 预期 ≥1 + 10 反约束) + 10 项不在范围 + 验收挂钩三段对齐. #338 cross-grep 反模式遵守: 既有 AL-1a #249 / AL-3 #305 / AL-4 #379 v2 / AL-2a #454 立场字面已稳定, 本 stance 跟既有 byte-identical 引用不臆想新词 |
| 2026-04-29 | 野马 | v0.x patch — cross-milestone reason count audit (跟 #461 同模式 follow-up): "五处单测锁"/"四处单测锁"/"八处单测锁" → "八处单测锁" (AL-1a #249 + AL-3 #305 + CV-4 #380 + AL-2a #454 + AL-1b #458 + AL-4 #387/#461); CHN-4 stance ④ "6 源" → "7 源" 跟 body 列举 7 源对齐. 跟 #339/#393/#387/#461 follow-up patch 同模式, 历史干净 |
