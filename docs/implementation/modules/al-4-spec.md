# AL-4 spec brief — agent_runtime registry (plugin process descriptor 启停)

> 飞马 · 2026-04-28 v0 / 2026-04-29 v1 + v2 · ≤80 行 spec lock (实施视角拆 PR 由战马待派, Phase 4 入口前置)
> **蓝图锚**: [`agent-lifecycle.md`](../../blueprint/current/agent-lifecycle.md) §2.2 (默认 remote-agent + power user 直配 plugin 双路径) + §2.2 v1 务实边界表 (v1 only OpenClaw / Mac+Linux / 不优化多 runtime 并行) + §4 (remote-agent 安全模型 — 二进制下载/沙箱/资源限制留第 6 轮); [`README.md`](../../blueprint/README.md) §1 立场 #7 (Borgee 不带 runtime — 走 plugin 接) + [`concept-model.md`](../../blueprint/current/concept-model.md) §0 (不调 LLM / 不带 runtime / 不定义角色模板)
> **关联 (v2 全景同步)**: AL-1a #249 三态机 + AL-3 #310 PresenceTracker (session 在线 — DM-2.2 #372 IsOnline 真接同源) + BPP-1 #304 envelope CI lint (跟 5-frame 同模式 type/cursor 头位: RT-1=7 / AnchorCommentAdded=10 / MentionPushed=8 / IterationStateChanged=9); **CV-4 #365 iterate runtime 接口** (AL-4 未落 stub fail-closed reason='runtime_not_registered' 同源) + CHN-4 #374/#375 协作场骨架 demo (runtime 启停 demo 路径之一) + ADM-0 立场 ⑦ admin 元数据 only

> ⚠️ 锚说明: 蓝图 agent-lifecycle.md 章节 §2.2 字面 "runtime 安装管家" 是 remote-agent 角色, 落地到 server 侧表现为 `agent_runtimes` registry 表; 此 spec 锁的是 **registry 元数据 + 启停 API + UI**, 不锁 remote-agent 二进制下载/沙箱 (留第 6 轮 §4). 跟立场 #7 "Borgee 不带 runtime" 不冲突 — registry 存的是 plugin process descriptor (endpoint / status / 心跳), 不存 LLM 调用本身.

## 0. 关键约束 (3 条立场, 蓝图字面 + ADM-0/AL-1/AL-3 接力)

1. **agent_runtime ≠ LLM runtime, 是 plugin process descriptor**: registry 表存 `endpoint_url` (plugin WS/HTTP 入口) + `process_kind` CHECK in ('openclaw','hermes') (v1 仅 'openclaw' 蓝图 §2.2 v1 边界字面) + `status` (registered/running/stopped/error) + `last_heartbeat_at`; **反约束**: 不存 `llm_provider` / `model_name` / `api_key` / `prompt_template` 列 (那是 plugin 内部事, Borgee 不带 runtime 立场字面)
2. **启停 owner-only, admin 元数据 only (立场 ⑦ 复用 ADM-0 红线)**: `POST /agents/:id/runtime/start` 和 `/stop` 走 RequirePermission('agent.runtime.control') (默认 grant 给 agent.owner_id, admin 不 grant); admin god-mode 路径只能 GET /admin/runtimes 看元数据 (列表 + status), 不入 start/stop 写动作; **反约束**: `grep admin.*runtime.*start|admin.*runtime.*stop` count==0
3. **runtime status ≠ presence**: `agent_runtimes.status` 是 process-level 生命周期 (registered → running → stopped/error 持久化态); AL-3 `presence_sessions` 是 session-level 瞬时态 (WS 连上即 online, 断即 offline); 二者协同但不替代 — agent 可以 status='running' 但 presence='offline' (runtime 在跑但 WS 断, 跟 AL-1a 故障态对齐); **反约束**: `agent_runtimes` 不挂 `is_online` 列 (跟 #310 presence 拆死)

## 1. 拆段实施 (AL-4.1 / 4.2 / 4.3, ≤ 3 PR)

| 段 | 范围 | 闭锁 | owner |
|---|---|---|---|
| **AL-4.1** schema migration v=16 | `agent_runtimes` 表 (`id` PK / `agent_id` NOT NULL FK agents UNIQUE / `endpoint_url` TEXT NOT NULL / `process_kind` CHECK in ('openclaw','hermes') / `status` CHECK in ('registered','running','stopped','error') / `last_error_reason` nullable (复用 AL-1a 6 reason 枚举字面) / `last_heartbeat_at` INTEGER nullable / `created_at` / `updated_at`); 索引 `idx_agent_runtimes_agent_id` (lookup 热路径); migration v=15 → v=16 双向 (v=15 已被 DM-2.1 #361 抢号顺延; CV-3.1 拿 v=17) | 待 PR (战马待派) | TBD |
| **AL-4.2** server registry + start/stop API + heartbeat hook | `internal/api/runtimes.go` `POST /agents/:id/runtime/start` (校验 owner perm + UPDATE status='running' + emit BPP-1 `agent_register` frame, 走 #304 whitelist) / `/stop` (UPDATE status='stopped' + 通知 plugin 关闭) / `GET /admin/runtimes` (admin god-mode 元数据 only); heartbeat 周期更 `last_heartbeat_at` (跟 AL-3 hub lifecycle hook 同 internal/ws 路径); error 回填 `status='error'` + `last_error_reason` (复用 AL-1a #249 6 reason 字面 byte-identical) | 待 PR (战马待派) | TBD |
| **AL-4.3** client SPA agent settings 启停 UI | agent 详情页 `Runtime` 卡片: 显示 endpoint_url + process_kind + status badge (4 态颜色字面对齐 AL-1a #249 REASON_LABELS 模板); start/stop 按钮 owner-only (非 owner DOM 不渲染); error 状态显示 reason label (跟 #249 客户端 lib/agent-state.ts REASON_LABELS 同源) + "查看日志" 直达入口 (蓝图 §2.3 "故障可解释" 字面) | 待 PR (战马待派) | TBD |

## 2. 与 AL-1 / AL-3 / BPP-1 / ADM-0 留账冲突点

- **AL-1a #249 三态机**: AL-4 `agent_runtimes.status` 跟 AL-1 三态 (online/offline/error) **不重叠** — AL-1 是 agent 对外的 user-facing 态 (sidebar dot), AL-4 是 runtime process 内部态; 协同关系: AL-1 online ⊂ AL-4 status='running' ∧ AL-3 IsOnline==true; AL-1 error 可能源自 AL-4 status='error' (last_error_reason 复用同 6 枚举); **反约束**: 不在 AL-4 路径写 sidebar dot, 仍走 AL-1 旁路 (#249 字面)
- **AL-3 #310 presence**: AL-4.2 heartbeat 不直接写 `presence_sessions` (那是 WS hub lifecycle 事, 走 AL-3.2 #277 接口); AL-4 heartbeat 仅更 `agent_runtimes.last_heartbeat_at` (process-level), AL-3 hub 心跳更 `presence_sessions.last_heartbeat_at` (session-level), 两表两路径; AL-4.2 emit BPP-1 frame 时通过 #310 SessionsTracker.IsOnline(agent.id) 判 plugin 是否真接通, 不直接耦合
- **BPP-1 #304 envelope**: AL-4.2 start 触发 `agent_register` frame 走 #304 whitelist (frame schema 已在 BPP-1 envelope.go 注册), AL-4 不另起 runtime-only frame; **反约束**: `grep -E "type:.*'runtime\.\w+'" packages/server-go/internal/ws/` 0 hit (走 BPP-1 既有 6 control + 3 data, 不裂 namespace)
- **ADM-0 立场 ⑦**: admin god-mode `GET /admin/runtimes` 返回元数据白名单 (`{id, agent_id, endpoint_url, process_kind, status, last_heartbeat_at}`), **不返回** `last_error_reason` 字段 raw 文本 (隐私, 仅 owner 看 reason 详情); admin 不入 start/stop 写路径 (复用 ADM-0 红线)
- **CV-4 #365 iterate runtime 接口 (v2 加)**: CV-4.2 IterationStateChangedFrame 触发条件含 "AL-4 stub fail-closed reason='runtime_not_registered'" 字面 (CV-4 §2 留账); AL-4 落地后 — agent_runtimes.status='running' 是 CV-4 iterate 走真路径前置; AL-4 未落 — CV-4 走 stub 路径 fail-closed (state='failed', error_reason 跟 AL-1a 6 reason byte-identical 同源, 不另起 reason 枚举)
- **CHN-4 #374/#375 协作场骨架 demo (v2 加)**: AL-4.3 启停 UI 是 demo 路径之一 (agent settings 卡片 owner-only); CHN-4 e2e demo 含 invite agent 流, 真路径需 AL-4 落地后 agent_runtimes.status='running'; AL-4 未落 demo 走 mock runtime 走 happy path (CHN-4 立场 ③ "走真 4901+5174 不 mock" 跟 AL-4 stub 接口 fail-closed 路径不冲突 — mock 是 demo 截屏外的 runtime 占位, server-go + vite 仍走真)
- **v=14-19 sequencing 字面延续 (v2 全景同步)**: CV-2.1 v=14 ✅ #359 / DM-2.1 v=15 ✅ #361 / **AL-4.1 v=16 待 (本 spec)** / CV-3.1 v=17 / CV-4.1 v=18 / CHN-3.1 v=19 / CHN-4.1 v=20 占位无 schema 改
- **第 6 轮 remote-agent 安全 (蓝图 §4)**: AL-4 不前置 — 二进制下载/沙箱/资源限制/uninstall 留第 6 轮 (`agent-lifecycle.md` §4 字面挂着), AL-4 仅 registry + 启停信号, 不管 remote-agent 怎么真起进程

## 3. 反查 grep 锚 (Phase 4 验收)

```
git grep -n 'agent_runtimes'                    packages/server-go/internal/migrations/    # ≥ 1 hit (AL-4.1)
git grep -nE 'process_kind.*CHECK.*openclaw'    packages/server-go/internal/migrations/    # ≥ 1 hit (v1 边界字面, 蓝图 §2.2)
git grep -nE 'llm_provider|model_name|api_key|prompt_template' packages/server-go/internal/store/agent_runtimes* # 0 hit (反约束 立场 ① Borgee 不带 runtime)
git grep -nE 'agent_runtimes.*is_online|is_online.*agent_runtimes' packages/server-go/      # 0 hit (反约束 立场 ③ 跟 AL-3 presence 拆死)
git grep -nE 'admin.*runtime.*start|admin.*runtime.*stop|/admin/runtimes/.*start' packages/server-go/internal/api/admin*.go # 0 hit (反约束 立场 ② admin 元数据 only)
git grep -n  'RequirePermission..agent\.runtime\.control'  packages/server-go/internal/api/runtimes.go # ≥ 2 hit (start + stop owner-only 闸)
git grep -nE "type:.*'runtime\." packages/server-go/internal/ws/                            # 0 hit (反约束 走 BPP-1 既有 frame, 不裂 namespace)
# v2 加 — CV-4 stub 接口反查 + admin god-mode 字段白名单反断
git grep -nE 'runtime_not_registered'           packages/server-go/internal/api/                      # ≥ 1 hit (CV-4 stub fail-closed reason byte-identical 跟 AL-1a #249 6 reason 同源)
git grep -nE 'last_error_reason.*admin|admin.*last_error_reason' packages/server-go/internal/api/admin*.go # 0 hit (ADM-0 立场 ⑦ 字段白名单反断: admin 不返 reason raw 文本)
```

任一 0 hit (除反约束行) → CI fail, 视作蓝图立场 #7 "Borgee 不带 runtime" 被弱化 / AL-3 边界混淆 / ADM-0 立场 ⑦ 漂移.

## 4. 不在本轮范围 (反约束)

- ❌ LLM provider 配置 / api_key 持久化 / model_name 选择 (Borgee 不带 runtime 立场 #7 字面, 走 plugin 内部)
- ❌ Token quota / 用量计费 / rate limit per agent (留第 5 轮 plugin 协议 + 业主反馈)
- ❌ 多端同 agent runtime 并行 (蓝图 §2.2 v1 边界 "不优化多 runtime 并行" 字面; UNIQUE(agent_id) 列锁单 runtime per agent)
- ❌ GPU 调度 / 资源池 / runtime 优先级 (留第 6 轮 + Phase 5+)
- ❌ remote-agent 二进制下载 / 沙箱 / auto-update / uninstall 路径 (蓝图 §4 留第 6 轮 "Remote-agent / Host bridge")
- ❌ Hermes 接入 (v1 only OpenClaw, 蓝图 §2.2 v1 边界字面; CHECK 约束已锁单值 + Hermes 占号枚举留 v2+ migration 加列)
- ❌ Windows 支持 (v1 only Mac/Linux 蓝图 §2.2 v1 边界字面)

## 5. Test plan (实施 PR 各自带, 此 spec 不带)

- AL-4.1: migration v=15 → v=16 双向 + UNIQUE(agent_id) 反向 (重复 runtime per agent reject) + CHECK process_kind reject 'hermes' v1 (反向断言, v2+ flip) + CHECK status reject 'unknown' 枚举外值 + 反约束反射列名 grep `llm_provider|api_key` count==0
- AL-4.2: start owner-only (非 owner 403) + admin 401 (admin god-mode 不入写) + start 触发 BPP-1 `agent_register` frame (走 #304 whitelist, 反向 grep frame type 不裂 namespace) + heartbeat 周期 (clock fixture, 跟 G2.3/AL-3 同节流模式) + error 回填 last_error_reason 枚举对齐 AL-1a #249 6 reason byte-identical + admin god-mode `GET /admin/runtimes` 元数据白名单反向断言 (last_error_reason 字段不返回)
- AL-4.3: e2e owner-only 按钮 (非 owner DOM 不渲染) + 4 态 badge 颜色字面对齐 AL-1a #249 REASON_LABELS 模板 + error 状态 reason label 跟 client `lib/agent-state.ts` 同源 + "查看日志" 直达入口跳转
- **v2 加 — CV-4 stub 接口集成测试**: AL-4 未落时 CV-4.2 iterate 路径 fail-closed reason='runtime_not_registered' (跟 AL-1a 6 reason byte-identical 同源); AL-4 落地后 — agent_runtimes.status='running' 触发真路径 commit + IterationStateChangedFrame state='completed', 反向断言 reason 不再 'runtime_not_registered'
- **v2 加 — CHN-4 demo 路径集成测试**: AL-4.3 启停按钮在 CHN-4 demo agent settings 卡片 owner-only 渲染; CHN-4 e2e 真 4901+5174 路径下 invite agent → AL-4 start → status='running' → CHN-4 chat tab agent 显在线 (跟 AL-3 #310 IsOnline 同源, 跟 AL-1 sidebar dot 协同)
