# Acceptance Template — AL-4: agent_runtime registry (plugin process descriptor 启停)

> 蓝图: `docs/blueprint/current/agent-lifecycle.md` §2.2 (默认 remote-agent + power user 直配 plugin 双路径) + §2.2 v1 务实边界 (v1 only OpenClaw / Mac+Linux / 不优化多 runtime 并行) + §4 (remote-agent 安全模型留第 6 轮); `README.md` §1 立场 #7 (Borgee 不带 runtime — 走 plugin 接); `concept-model.md` §0
> Implementation: `docs/implementation/modules/al-4-spec.md` (飞马 #313, 3 立场 + 3 拆段 + 7 grep 反查 含 4 反约束 + 7 反约束)
> 关联: AL-1a #249 三态机 + AL-3 #310 PresenceTracker + BPP-1 #304 envelope CI lint + ADM-0 立场 ⑦
> 拆 PR: **AL-4.1** schema migration v=15 (`agent_runtimes` 表) — TBD / **AL-4.2** server registry + start/stop API + heartbeat — TBD / **AL-4.3** client SPA agent settings 启停 UI — TBD
> Owner: 战马 (待派) 实施 / 烈马 验收

> ⚠️ skeleton 状态: 反约束 §3 立即机器化可跑; 正向 §1 + §2 + §3 测试函数路径留 _(待 AL-4.x PR)_, 跟 #287 chn-1 / #298 rt-1 / #315 al-3.1 翻牌同模式 — AL-4.1 PR merge 后回填 ✅ 实测路径。

## 验收清单

### §1 schema (AL-4.1) — agent_runtimes 数据契约 (跟 AL-3.1 #310 / DM-2.1 #316 三轴模板同)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 表 schema 三轴: `id` PK + `agent_id` NOT NULL FK `agents.id` UNIQUE (单 runtime per agent, 立场 ① 蓝图 §2.2 v1 边界) + `endpoint_url` TEXT NOT NULL + `process_kind` TEXT NOT NULL + `status` TEXT NOT NULL + `last_error_reason` nullable + `last_heartbeat_at` INTEGER nullable + `created_at` + `updated_at` NOT NULL; pragma assert `PRAGMA table_info` 字面 9 列 | migration drift test | 战马 / 烈马 | _(待 AL-4.1 PR)_ — `internal/migrations/al_4_1_agent_runtimes_test.go::TestAL41_CreatesAgentRuntimesTable` (跟 #310 `TestAL31_CreatesPresenceSessionsTable` 同模式) |
| 1.2 CHECK 约束: `process_kind` ∈ ('openclaw','hermes') (v1 仅 'openclaw' 蓝图 §2.2 v1 边界字面, 'hermes' 占号 v2+); `status` ∈ ('registered','running','stopped','error'); 反向断言: INSERT `process_kind='unknown'` reject + INSERT `status='busy'` reject (枚举外值) | migration drift test | 战马 / 烈马 | _(待 AL-4.1 PR)_ — `al_4_1_agent_runtimes_test.go::TestAL41_RejectsInvalidProcessKind` + `TestAL41_RejectsInvalidStatus` |
| 1.3 UNIQUE(agent_id) — 同 agent 二次 INSERT runtime reject (立场 ① v1 不优化多 runtime 并行); INDEX `idx_agent_runtimes_agent_id` (lookup 热路径) | migration drift test | 战马 / 烈马 | _(待 AL-4.1 PR)_ — `al_4_1_agent_runtimes_test.go::TestAL41_RejectsDuplicateRuntimePerAgent` + `TestAL41_HasAgentIDIndex` |
| 1.4 migration v=14 → v=15 串行号 + idempotent rerun no-op; `registry.go` v=15 字面锁 (DM-2.1 #316 v=14 后 forward-only) | migration drift test | 战马 / 烈马 | _(待 AL-4.1 PR)_ — `al_4_1_agent_runtimes_test.go::TestAL41_Idempotent` + `grep -n "v=15\|15:" packages/server-go/internal/migrations/registry.go` count==1 |
| 1.5 反约束 — 表无 `llm_provider` / `model_name` / `api_key` / `prompt_template` 列 (立场 ① Borgee 不带 runtime, 立场 #7 字面); 表无 `is_online` 列 (立场 ③ 跟 AL-3 presence 拆死) | migration drift + CI grep | 飞马 / 烈马 | _(待 AL-4.1 PR)_ — `al_4_1_agent_runtimes_test.go::TestAL41_NoLLMOrPresenceColumns` (反向 column list) + `grep -nE 'llm_provider\|model_name\|api_key\|prompt_template\|agent_runtimes.*is_online' packages/server-go/internal/store/agent_runtimes*` count==0 |

### §2 server registry + start/stop API + heartbeat (AL-4.2)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 `POST /api/v1/agents/:id/runtime/start` owner-only (RequirePermission `agent.runtime.control`); 非 owner 403; admin god-mode 不入写 (admin token → 401, 跟 ADM-0 双轨闸同模式) | unit | 战马 / 烈马 | _(待 AL-4.2 PR)_ — `internal/api/runtimes_test.go::TestRuntimeStart_OwnerOnly_403_ForOthers` + `TestRuntimeStart_AdminTokenRejected_401` |
| 2.2 `POST /agents/:id/runtime/stop` 同 2.1 owner-only; status 写 'stopped' + 通知 plugin 关闭路径 (BPP-1 frame); 反向: 重复 stop idempotent | unit | 战马 / 烈马 | _(待 AL-4.2 PR)_ — `runtimes_test.go::TestRuntimeStop_OwnerOnly_Idempotent` |
| 2.3 start 触发 `agent_register` BPP frame 走 #304 envelope whitelist (frame schema 已注册, 不裂 namespace); 反向: 不出现 `runtime.start` / `runtime.stop` 自造 frame type | unit + golden JSON | 飞马 / 烈马 | _(待 AL-4.2 PR)_ — `internal/bpp/runtime_register_test.go::TestRuntimeRegisterUsesExistingFrame` + `grep -rEn "type:.*'runtime\\." packages/server-go/internal/ws/` count==0 |
| 2.4 heartbeat 周期: plugin → server → 更 `agent_runtimes.last_heartbeat_at` (process-level), **不写** `presence_sessions.last_heartbeat_at` (那是 AL-3 hub WS lifecycle 路径, 立场 ③ 拆死); clock fixture (跟 G2.3 节流 + AL-3 同模式) | unit (clock fixture) | 战马 / 烈马 | _(待 AL-4.2 PR)_ — `internal/api/runtimes_heartbeat_test.go::TestHeartbeatUpdatesRuntimeLastHeartbeatNotPresence` (反向断言两表两路径) |
| 2.5 error 回填 `status='error'` + `last_error_reason` 复用 AL-1a #249 6 reason 枚举字面 byte-identical (`api_key_invalid`/`quota_exceeded`/`network_unreachable`/`runtime_crashed`/`runtime_timeout`/`unknown`, 跟 `agent/state.go` Reason* + AL-3 #305 ③ + `lib/agent-state.ts` REASON_LABELS 三处一致 — 改 = 改八处, #319 立场 ④) | unit | 战马 / 烈马 | _(待 AL-4.2 PR)_ — `runtimes_test.go::TestRuntimeErrorReasonsMatchAL1aEnum` (跟 #249 字面对) |
| 2.6 `GET /admin-api/v1/runtimes` admin god-mode 元数据白名单 — 返回 `{id, agent_id, endpoint_url, process_kind, status, last_heartbeat_at}`, **不返回** `last_error_reason` raw 文本 (隐私 立场 ⑦, 复用 ADM-0 反射扫描器) | unit (reflect scan) | 战马 / 烈马 | _(待 AL-4.2 PR)_ — `internal/api/admin_runtimes_test.go::TestAdminGodModeOmitsErrorReason` (跟 REG-ADM0-003 同模式) |
| 2.7 status 变化触发 system DM 文案锁 (#321 文案锁 byte-identical, 跟 AL-3 #305 / DM-2 #314 同模式) — start (→running): `"{agent_name} 已启动"` / stop (→stopped): `"{agent_name} 已停止"` (区分 AL-3 `"已离线"` process vs session) / error (→error): `"{agent_name} 出错: {reason}"` ({reason} 跟 2.5 6 枚举同源); recipient = `agent.owner_id` only, kind='system' + sender_id='system'; 反向断言: channel fanout count==0 + 不发 toast (沉默胜于假 loading §11) + payload 不含 raw `runtime_id`/`pid`/`endpoint_url` | unit + grep | 战马 / 烈马 | _(待 AL-4.2 PR)_ — `runtimes_test.go::TestRuntimeStatusChangeTriggersSystemDM_OwnerOnly` (3 子 case start/stop/error + 反向 channel sniff) + `grep -nE '已启动\|已停止\|出错:' packages/server-go/internal/api/runtimes.go` count≥3 + 反向 grep `已下线\|已退出\|崩溃\|启动中` count==0 (#321 §3 同义词漂防御) |

### §3 client SPA agent settings 启停 UI (AL-4.3)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 agent 详情页 `Runtime` 卡片 DOM 字面锁 (#321 §2): `<div data-runtime-status="{running|stopped|error}">` 3 态严闭 (registered 是 server-internal, 未启动不展示给 owner; v0 不允许 `starting`/`stopping`/`restarting` 中间态 — 反向 grep 防御); status badge 颜色字面对齐 AL-1a #249 REASON_LABELS 模板; `process_kind` 显, `endpoint_url`/`last_heartbeat_at` 原始时间戳 **不显** (#321 §2 反约束) | e2e + grep | 战马 / 烈马 | _(待 AL-4.3 PR)_ — `packages/e2e/tests/al-4-3-runtime-card.spec.ts::立场 ① data-runtime-status 3 态 DOM lock` + `grep -rnE 'data-runtime-status=["\\\(starting\|stopping\|restarting\)"]' packages/client/src/` count==0 |
| 3.2 start/stop 按钮 owner-only DOM gate (#321 §2 反约束: 非 owner DOM **omit**, 不仅是 disabled — 跟 CV-1 ⑦ rollback 同模式); 反向 e2e: `[data-testid="runtime-start-btn"]` 在非 owner 视图 count==0 + 反向 grep `disabled.*!isOwner` count==0 (不准 disabled leak owner 信息) | e2e + grep | 战马 / 烈马 | _(待 AL-4.3 PR)_ — `al-4-3-runtime-card.spec.ts::立场 ② owner-only btn DOM omit 反向断言` |
| 3.3 error 状态显 reason badge `<span data-error-reason="{reason}">{REASON_LABELS[reason]}</span>` 字面跟 `lib/agent-state.ts` REASON_LABELS 同源 #249 (跟 AL-3 #305 / AL-1a #249 三处 byte-identical, 改 = 改八处) + "查看日志" 直达入口 (蓝图 §2.3 故障可解释字面) | e2e | 战马 / 烈马 | _(待 AL-4.3 PR)_ — `al-4-3-runtime-card.spec.ts::立场 ③ error reason badge + 日志入口跳转` |
| 3.4 owner inbox 收 system DM 3 态 (#321 §1 文案锁): start `"{agent_name} 已启动"` / stop `"{agent_name} 已停止"` / error `"{agent_name} 出错: {reason}"`; 反向: 不发 toast (沉默胜于假 loading §11, 反向 grep `toast.*runtime\|RuntimeToast` count==0); G2.7 demo 截屏 `docs/qa/screenshots/g2.7-runtime-{start,stop,error}.png` 三张归档 (跟 G2.5 / G2.6 同模式) | e2e + screenshot | 战马 / 烈马 | _(待 AL-4.3 PR)_ — `al-4-3-runtime-card.spec.ts::立场 ④ system DM 3 态 owner inbox 渲染` + `page.screenshot()` 主动归档 3 张 |

### §4 蓝图行为对照 (反查锚, 每 PR 必带, **现在可立即机器化跑**)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 `git grep -nE 'llm_provider\|model_name\|api_key\|prompt_template' packages/server-go/internal/store/agent_runtimes*` count==0 (反约束 立场 ① Borgee 不带 runtime) | CI grep | 飞马 / 烈马 | spec lint job, AL-4.1 PR 必跑 |
| 4.2 `git grep -nE 'agent_runtimes.*is_online\|is_online.*agent_runtimes' packages/server-go/` count==0 (反约束 立场 ③ 跟 AL-3 presence 拆死) | CI grep | 飞马 / 烈马 | spec lint job, 每 PR 必跑 |
| 4.3 `git grep -nE 'admin.*runtime.*start\|admin.*runtime.*stop\|/admin/runtimes/.*start' packages/server-go/internal/api/admin*.go` count==0 (反约束 立场 ② admin 元数据 only, 复用 ADM-0 红线) | CI grep | 飞马 / 烈马 | spec lint job, 每 PR 必跑 |
| 4.4 `git grep -nE "type:.*'runtime\\." packages/server-go/internal/ws/` count==0 (反约束 走 BPP-1 既有 frame, 不裂 namespace) | CI grep | 飞马 / 烈马 | spec lint job, 每 PR 必跑 |
| 4.5 (正向锚, 等 AL-4.1 真落) `git grep -n 'agent_runtimes' packages/server-go/internal/migrations/` count≥1 + `git grep -nE 'process_kind.*CHECK.*openclaw' packages/server-go/internal/migrations/` count≥1 (v1 边界字面) | CI grep | 飞马 / 烈马 | _(待 AL-4.1 PR)_ |
| 4.6 (正向锚, 等 AL-4.2 真落) `git grep -n 'RequirePermission..agent\\.runtime\\.control' packages/server-go/internal/api/runtimes.go` count≥2 (start + stop owner-only 闸) | CI grep | 飞马 / 烈马 | _(待 AL-4.2 PR)_ |

## 退出条件

- §1 schema (1.1-1.5) + §2 server (2.1-2.6) + §3 client (3.1-3.3) **全绿** (一票否决)
- §4 反查锚 4.1-4.4 反约束**现在已可机器化跑** (AL-4 spec brief 阶段即生效, 防 AL-4.x 实施时立场漂移); 4.5-4.6 正向锚等 AL-4.1/4.2 真落
- 跨模块协同: AL-1a #249 三态 (反约束不写 sidebar dot) + AL-3 #310 presence (反约束 last_heartbeat_at 两表) + BPP-1 #304 envelope (反约束不裂 frame namespace) + ADM-0 立场 ⑦ (admin 元数据 only)
- 第 6 轮 remote-agent 安全 (蓝图 §4 二进制下载/沙箱) **不在 AL-4 范围**, 不挡闭合
- 登记 `docs/qa/regression-registry.md` REG-AL4-001..010 (待战马实施 PR 落后开号回填, 跟 #315 AL-3.1 同模式)
