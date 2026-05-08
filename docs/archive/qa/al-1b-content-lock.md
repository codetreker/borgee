# AL-1b 状态 dot UI 文案锁 (野马 G4.x demo 预备)

> **状态**: v0 (野马, 2026-04-29)
> **目的**: AL-1b.x client UI 实施前锁 5-state 状态 dot 文案 + DOM 字面 + 反约束 — 跟 AL-3 #305 dot 文案锁同模式 (但 AL-3 锁 3 态, 本锁补 busy/idle 2 态合 5 态), 防 AL-1b 实施时把 busy/idle 漂同义词 / dot 颜色错对应 / 5-state 优先级混乱.
> **关联**: 蓝图 `agent-lifecycle.md` §2.3 (5-state, 2026-04-28 4 人 review #5 决议: busy/idle 跟 BPP 同期 Phase 4) + §11 文案守 (沉默胜于假活物感 — 状态明确不准模糊); 战马C #453 spec brief §0 ① 拆三路径 / §0 ② BPP 单源 / §0 ③ 文案三态合并; AL-3 #305 presence dot 文案锁 (`online`/`offline`/`error` 已锁); AL-1a #249 6 reason codes byte-identical (error reason 同源); CV-1 #347 line 251 kindBadge 二元 byte-identical (跨 milestone 锁字面源).
> **#338 cross-grep 反模式遵守**: 既有 AL-3 #305 dot 文案锁 + AL-1a `agent/state.go` Reason* + REASON_LABELS 字面已稳定, 本锁字面跟既有 byte-identical 引用 (改一处 = 改三处), 不臆想新词。

---

## 1. 7 处文案 + DOM 字面锁

| # | 场景 | 字面锁 (byte-identical) | 反约束 |
|---|------|-----|------|
| ① | **5-state dot 颜色 + tooltip 字面** (5 态优先级 error > busy > idle > online > offline) | DOM: `<span class="agent-status-dot" data-presence="{state}" data-task-state="{busy|idle|null}" title="{LABEL}">●</span>` byte-identical 跟 #453 spec §0 ③ 同源:<br>• `error` → 🔴 红 + tooltip `"故障 ({reason_label})"` byte-identical 跟 AL-3 #305 ③ 同源 (REASON_LABELS 走 #249 6 reason)<br>• `busy` → 🟡 黄 + tooltip `"在工作"` byte-identical (跟 acceptance al-1b.md §3 + 蓝图 §11 字面对齐, 反 active/working/forwarding 同义词)<br>• `idle` → ⚪ 灰白 + tooltip `"空闲"` byte-identical (反 idling/inactive/waiting 同义词)<br>• `online` → 🟢 绿 + tooltip `"在线"` byte-identical 跟 AL-3 #305 ① 同源<br>• `offline` → ⚫ 黑 + tooltip `"已离线"` byte-identical 跟 AL-3 #305 ② + 蓝图 §11 字面同源 | ❌ 不准 "Busy/Idle/Active/Online/Offline/Working" 英文同义词漂移 (中文 byte-identical 永久锁); ❌ 不准 dot 颜色错对应 (5 颜色精神锁: 黄/灰白/绿/黑/红 byte-identical, 反"统一灰点"#11 字面禁); ❌ 不准 "在忙" / "工作中" / "处理中" / "Pending" busy 同义词; ❌ 不准 "闲置" / "暂无活动" / "待机" idle 同义词 |
| ② | **5-state 合并优先级** (client `describeAgentState()` 三处单测锁源) | 优先级 byte-identical 跟 #453 spec §0 ③ + acceptance §2.1 + 本锁 **三处单测锁** (改 = 改三处): `error > busy > idle > online > offline` (5 态合并函数返单 dot 文案); 实现走 `lib/agent-state.ts::describeAgentState()` (跟 AL-3 #305 ③ + AL-1a #249 同模式); 5-state 合并不裂 multi-dot UI (只 1 个 dot, 显当前最高优先级态) | ❌ 不准多 dot UI (跟 AL-3 #303 立场 ⑤ "5s 节流 + 60s 心跳" 不裂同源 — 一 agent 一 dot); ❌ 不准 server 端合并 (client `describeAgentState()` 单源, server 仅返 5-state 字面); ❌ 不准跳过 error 优先级 (error > 一切, 跟 AL-3 #303 ⑤ 同精神 — 故障最显眼) |
| ③ | **busy state DOM data-task-state="busy" + last_task_id 显示** | DOM: `data-task-state="busy"` + 副字段 `data-last-task-id="{task_id}"` byte-identical (跟 schema agent_status.last_task_id 同源); tooltip hover 显 `"在工作 (任务 {task_id_short})"` byte-identical (`task_id_short` = task_id 头 8 字符截断, 防 UI 噪声跟 #314 fallback DM body_preview 80 字精神); 反约束: 不显示完整 raw task_id (隐私 + UX) | ❌ 不准 raw task_id 文本节点 (跟 ADM-0 #211 §1.1 raw UUID 隐私红线同源); ❌ 不准 task_id 为空时仍渲染 `data-last-task-id` attr (反约束 — 仅 last_task_id 非 null 才 render); ❌ 不准 server side render busy 时不带 task_id (server schema agent_status.last_task_id 跟 BPP frame.task_id byte-identical 同源, 漏 = 字面漂) |
| ④ | **idle state — last_task_finished_at 时间戳隐藏** | DOM: `data-task-state="idle"` + **不**渲染 `data-last-task-finished-at` (UI 噪声防御 — idle = 当前空闲, 不展示历史时间戳); 跟 AL-3 #303 ⑥ 反约束 "不显 last_heartbeat_at 时间戳 ('最近活跃 X 秒前')" 同源 (#11 沉默胜于假 loading 反约束) | ❌ 不准 `"上次任务完成于 X 分钟前"` UI (#11 反约束); ❌ 不准 idle dot 闪烁 / 渐变动画 (视觉噪声 - dot 是状态标不是活跃指示); ❌ 不准 `data-task-state="idle"` 仍带 `data-last-task-id` (idle 是任务完成后态, last_task_id 留 schema 但 UI 不显) |
| ⑤ | **error state 文案模板 byte-identical 跟 AL-3 + #249 六处单测锁** | DOM `data-presence="error"` + tooltip `"故障 ({reason_label})"` byte-identical 跟 **AL-1a #249 6 reason codes + AL-3 #305 ③ + CV-4 #380 ③ "失败: {reason_label}" + AL-2a #454 ④ "保存失败 ({reason_label})" + AL-4 #387/#461 + 本锁 六处单测锁** (改 reason = 改六处); reason_label 走 `lib/agent-state.ts::REASON_LABELS` 同源 (跟 AL-1a #249 6 reason `api_key_invalid|quota_exceeded|network_unreachable|runtime_crashed|runtime_timeout|unknown` byte-identical) | ❌ 不准 "Error/Failed/出错" 同义词 ("故障" byte-identical 锁); ❌ 不准 reason raw 文本 (走 REASON_LABELS 枚举锁); ❌ 不准 error dot 跟 busy/idle dot 视觉混淆 (红色专属 error, 黄/灰白专属 busy/idle) |
| ⑥ | **状态变化反约束 — busy↔idle 必经 BPP frame, server 不合成** | 反约束: client 不能直接 PATCH 改 busy/idle (跟 AL-1b spec 立场 ② "BPP single source" 同源); server admin god-mode endpoint **reject** PATCH `/api/v1/agents/:id/status` (admin 只能看不能改, 跟 ADM-0 §1.3 红线 + AL-3 #303 ⑦ + AL-4 #379 v2 + AL-2a #454 ④ **四源同模式**); state 转移仅由 BPP `task_started` (→ busy) / `task_finished` (→ idle) / 5min 无 frame (→ idle) 三路径触发 | ❌ 不准 client SPA 写 PATCH /status (跟 AL-1b stance ④ "client 不直写" 同源); ❌ 不准 admin god-mode 改 state (反人工伪造, 跟立场 ② BPP 单源同源); ❌ 不准 server 自动 transition busy→idle (除 5min IdleThreshold const) — 必经 BPP frame |
| ⑦ | **agent silent default + 反 polling spam** | 反约束: agent 状态变化 — 不发 system message / 不 fanout / 不 push WS frame (走 GET /api/v1/agents/:id/status 客户端 pull, 跟 AL-2a #454 ⑦ + AL-3 #305 ③ "agent join silent" + 蓝图 §11 + #382 立场 ⑤ messages 流不污染 同精神); client 端 polling 走 SWR debounce 5s (反 polling spam, 跟 AL-3 #303 ⑤ "5s 节流" 同源); 状态变化进 BPP frame 自然推 (BPP-2 落地后), AL-1b 不裂 frame namespace | ❌ 不准 SPA 1s polling spam status endpoint (DDoS 自家 server); ❌ 不准 status push WS frame `AgentStatusChangedFrame` (RT-1 4 frame + BPP-1 9 frame 已锁不裂, 跟 AL-4 #379 v2 立场 ⑥ "不裂 frame namespace" 同源); ❌ 不准 system message broadcast 状态变化 (跟 #11 silent default 同精神) |

---

## 2. 反向 grep — AL-1b.x PR merge 后跑, 全部预期 0 命中 (除标 ≥1)

```bash
# ① 5-state dot tooltip 中文 byte-identical (预期 ≥5 — 5 态各 ≥1)
grep -rnE "['\"](在工作|空闲|在线|已离线|故障)['\"]" packages/client/src/lib/agent-state.ts 2>/dev/null | grep -v _test  # 预期 ≥5
# ① busy/idle 同义词漂移防御
grep -rnE "['\"](Busy|Idle|Active|Working|忙|工作中|处理中|闲置|待机|暂无活动)['\"]" packages/client/src/lib/agent-state.ts packages/client/src/components/AgentStatus*.tsx 2>/dev/null | grep -v _test
# ② describeAgentState 5-state 优先级 (预期 ≥1)
grep -rnE 'describeAgentState' packages/client/src/lib/agent-state.ts 2>/dev/null | grep -v _test  # 预期 ≥1
# ② 多 dot UI 反约束 (一 agent 一 dot)
grep -rnE 'agent-status-dot.*agent-status-dot|<AgentStatusDot.*<AgentStatusDot' packages/client/src/components/ 2>/dev/null | grep -v _test
# ③ data-task-state="busy" + data-last-task-id (预期 ≥1)
grep -rnE 'data-task-state=["'"'"']busy["'"'"']' packages/client/src/components/AgentStatus*.tsx 2>/dev/null | grep -v _test  # 预期 ≥1
grep -rnE 'data-last-task-id' packages/client/src/components/AgentStatus*.tsx 2>/dev/null | grep -v _test  # 预期 ≥1
# ④ idle 时间戳 leak 防御 (跟 AL-3 last_heartbeat_at 反约束同源)
grep -rnE "上次任务完成于|last_task_finished_at.*format|分钟前" packages/client/src/components/AgentStatus*.tsx 2>/dev/null | grep -v _test
# ⑤ error reason_label 跟 REASON_LABELS 同源 (预期 ≥1)
grep -rnE 'REASON_LABELS\[' packages/client/src/lib/agent-state.ts 2>/dev/null | grep -v _test  # 预期 ≥1
grep -rnE "['\"]故障 \\(\\$\\{.*reason.*\\}\\)['\"]|故障 \\(\\{.*\\}\\)" packages/client/src/lib/agent-state.ts 2>/dev/null | grep -v _test  # 预期 ≥1
# ⑥ client 不直写 PATCH /status (反 client 直写)
grep -rnE 'PATCH.*\/api\/v1\/agents\/.*\/status|fetch.*method.*PATCH.*status' packages/client/src/ 2>/dev/null | grep -v _test
# ⑥ admin god-mode 不改 state
grep -rnE 'admin.*PATCH.*agent.*status|admin.*update.*agent_status' packages/server-go/internal/api/admin*.go 2>/dev/null | grep -v _test.go
# ⑦ status push WS frame 反约束 (RT-1 4 frame + BPP-1 9 frame 已锁不裂)
grep -rnE 'AgentStatusChangedFrame|StatusUpdatedFrame' packages/server-go/internal/ws/ 2>/dev/null | grep -v _test.go
# ⑦ system message broadcast 状态变化反约束
grep -rnE "['\"]\\{agent_name\\} 进入工作状态['\"]|agent_status.*system.*message.*broadcast" packages/server-go/internal/api/ 2>/dev/null | grep -v _test.go
# ⑦ polling spam 防御 (debounce 5s 锁)
grep -rnE 'setInterval.*1000.*status|polling.*1s.*status' packages/client/src/ 2>/dev/null | grep -v _test
```

---

## 3. 验收挂钩 (AL-1b.x PR 必带)

- ① 5-state dot e2e: 各 5 态 DOM `data-presence` + `data-task-state` + tooltip 中文文案 byte-identical 跟 AL-3 #305 + 本锁同源
- ② 5-state 合并优先级 vitest table-driven: error > busy > idle > online > offline 三处单测锁 (#453 spec + acceptance §2.1 + 本锁)
- ③ busy state e2e: BPP `task_started` 触发 → DOM `data-task-state="busy"` + `data-last-task-id="{task_id}"` + tooltip 头 8 字符截断
- ④ idle state e2e: BPP `task_finished` / 5min IdleThreshold → DOM `data-task-state="idle"` + 反向断言无 `last-task-finished-at` UI 时间戳
- ⑤ error state e2e: reason 走 REASON_LABELS **六处单测锁** (AL-1a #249 + AL-3 #305 + CV-4 #380 + AL-2a #454 + 本锁 + AL-4 #387/#461) — 改 reason = 改六处
- ⑥ 反 client 直写 + admin god-mode 反约束 e2e: client SPA 反向断言无 PATCH /status 路径 + admin god-mode endpoint 返 405 (跟 AL-3 #303 ⑦ + AL-4 #379 v2 + AL-2a #454 ④ 同模式)
- ⑦ silent default + polling debounce e2e: 反向断言无 system message 状态变化 + 无 push frame + SWR debounce 5s (跟 AL-3 #303 ⑤ 节流模式同源)
- G4.x demo 截屏 4 张归档 (跟 #391 §1 截屏路径锁同源): `docs/qa/screenshots/g4.x-al1b-{busy,idle,error-with-reason,5-state-priority}.png` 撑 Phase 4 退出闸 demo

---

## 4. 不在范围

- ❌ busy/idle source 走 client 直写 / admin god-mode (跟立场 ⑥ + #453 spec 立场 ② BPP single source 同源)
- ❌ AgentStatusChangedFrame WS push (RT-1 4 frame + BPP-1 9 frame 已锁不裂, 跟 AL-4 #379 v2 立场 ⑥ + #382 立场 ① 同模式)
- ❌ 多 dot UI / 多面板拆开 5-state (一 agent 一 dot, 跟 AL-3 #303 立场 ⑤ + 立场 ② 单 describeAgentState 同源)
- ❌ idle 时间戳 UI / 历史 timeline (跟 AL-3 #303 ⑥ "不显 last_heartbeat_at" + #11 反约束同源, 留 v3+)
- ❌ busy 任务进度条 / 百分比 (agent 不报百分比, 跟 CV-4 #380 ③ "running 进度条无具体百分比" 同精神)
- ❌ admin SPA 改 busy/idle 状态 (admin 不入业务路径, ADM-0 §1.3 红线 + 立场 ⑥)
- ❌ system message broadcast 状态变化 (跟 #11 silent default + AL-3 #305 + AL-2a #454 ⑦ 同精神)
- ❌ 跟 BPP-2 落地强耦合 — schema (AL-1b.1) 先落占号, server endpoint (AL-1b.2) BPP frame 写入留 BPP-2 后真接管 (跟 AL-4 stub `runtime_not_registered` 同模式)

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 野马 | v0, 7 处文案锁 (5-state dot 颜色 + tooltip 中文 byte-identical 跟 AL-3 #305 同模式 — busy "在工作" / idle "空闲" / error "故障 ({reason_label})" + online "在线" + offline "已离线" / 5-state 合并优先级 三处单测锁 / busy data-task-state + last_task_id 头 8 字符截断 / idle 时间戳隐藏 跟 AL-3 ⑥ 同源 / error reason_label 五处单测锁 / busy↔idle 必经 BPP frame 反 client 直写 + admin god-mode 四源同模式 / agent silent default + polling debounce 5s) + 14 行反向 grep (含 6 预期 ≥1 + 8 反约束) + G4.x demo 截屏 4 张归档. #338 cross-grep 反模式遵守: 既有 AL-3 #305 dot 文案锁 + AL-1a state.go REASON_LABELS 字面已稳定, 本锁跟既有 byte-identical 引用不臆想新词 |
| 2026-04-29 | 野马 | v0.x patch — cross-milestone reason count audit (跟 #461 同模式 follow-up): "五处单测锁"/"四处单测锁"/"三处单测锁" → "六处单测锁" (AL-1a #249 + AL-3 #305 + CV-4 #380 + AL-2a #454 + AL-1b #458 + AL-4 #387/#461); CHN-4 stance ④ "6 源" → "7 源" 跟 body 列举 7 源对齐. 跟 #339/#393/#387/#461 follow-up patch 同模式, 历史干净 |
