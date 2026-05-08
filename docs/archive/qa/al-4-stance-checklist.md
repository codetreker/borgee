# AL-4 立场反查表 (Phase 4 第三波 — agent_runtimes registry)

> **状态**: v0 (野马, 2026-04-28)
> **目的**: Phase 4 第三波 milestone 立场锚点 — 战马 AL-4.x PR 直接吃此表为 acceptance; 飞马 / 烈马 review 拿此表反查漂移. 跟 #303 (AL-3) / #282 (CV-1) 同模板.
> **关联**: `agent-lifecycle.md` §2.2 (默认 remote-agent + power user 双路径, v1 only OpenClaw / Mac+Linux); `README.md` §1 立场 #7 (Borgee 不带 runtime); `concept-model.md` §0 (不调 LLM); ADM-0 立场 ⑦ (admin 元数据 only); AL-1a #249 三态机 + 6 reason codes; AL-3 #310 PresenceTracker; BPP-1 #304 envelope CI lint.
> **依赖**: #313 AL-4 spec brief merged (SHA a8f8483); AL-3.x 三轨 (#301/#302/#303/#305) + DM-2 三段 实施落地后 AL-4.1 schema 接力.

---

## 1. 7 项立场 — 锚 §X.Y + 反约束 + v0/v1

| # | 立场锚 | 一句话立场 | 反约束 (X 是, Y 不是) | v0 / v1 |
|---|--------|----------|----------------------|---------|
| ① | README §1 立场 #7 + #313 §0 立场 ① + concept-model §0 | **agent_runtime ≠ LLM runtime, 是 plugin process descriptor** — registry 存 endpoint_url / process_kind / status / heartbeat, 不存 LLM 调用本身 | **是** plugin 进程元数据 (`endpoint_url` + `process_kind` ∈ {openclaw} v1 + `status` 4 态); **不是** LLM 配置 (`llm_provider` / `model_name` / `api_key` / `prompt_template` 列禁); **不是** prompt 模板存储 (那是 plugin 内部事) | v0: `agent_runtimes` 表无 LLM 列; v1 同 |
| ② | #313 §0 立场 ② + ADM-0 立场 ⑦ + #211 红线 | **启停 owner-only**, admin god-mode 元数据 only — `RequirePermission('agent.runtime.control')` 默认仅 grant agent.owner_id; admin 不入 start/stop 写动作 | **是** owner 启 / 停 / 重启 (POST /agents/:id/runtime/start/stop); admin GET /admin/runtimes 仅看元数据列表 + status (跟 ADM-2 god-mode 同模式); **不是** admin 可启停 (反向 grep `admin.*runtime.*start` count==0); **不是** 任意 channel member 可启停 (跟 channel-model §1.4 owner-only 同模式) | v0: owner-only 闸 + admin 元数据 only; v1 同, 加 audit_log 行 (跟 ADM-2 #266 同 schema) |
| ③ | #313 §0 立场 ③ + AL-3 #310 拆死 | **runtime status ≠ presence** — process-level 持久态 (running/stopped) vs session-level 瞬时态 (online/offline) 双表双路径, 共存合法 | **是** `agent_runtimes.status` ∈ {registered, running, stopped, error} 持久化; AL-3 `presence_sessions` 由 WS hub lifecycle 管 (#310 实施); status='running' ∧ presence='offline' 共存合法 (runtime 在跑但 WS 断, 跟 AL-1a 故障态对齐); **不是** 单源 (反向 grep `agent_runtimes.*is_online` count==0); **不是** heartbeat 共享同一表 (AL-4 更 `agent_runtimes.last_heartbeat_at` process-level / AL-3 更 `presence_sessions.last_heartbeat_at` session-level) | v0: 双表双路径 byte-identical; v1 同 |
| ④ | AL-1a #249 6 reason codes + #313 §1 AL-4.2 | **last_error_reason 复用 #249 6 reason codes byte-identical** — 不另造 runtime-only reason 枚举 (**八处单测锁**: AL-1a `agent/state.go` Reason* + AL-3 client `lib/agent-state.ts` REASON_LABELS + CV-4 #380 ③ + AL-2a #454 ④ + AL-1b #458 ⑤ + 本 stance, 改 reason = 改八处) | **是** `agent_runtimes.last_error_reason` ∈ {`api_key_invalid`, `quota_exceeded`, `network_unreachable`, `runtime_crashed`, `runtime_timeout`, `unknown`} 字面同 `agent/state.go` Reason*; client SPA error badge 跟 `lib/agent-state.ts` REASON_LABELS 同源 (跟 AL-3 #305 ③ error 文案 + CV-4 #380 ③ "失败: {reason_label}" + AL-2a #454 ④ "保存失败 ({reason_label})" + AL-1b #458 ⑤ "故障 ({reason_label})" + #379 v2 §0 全景同源); **不是** 自造 runtime-specific reason (`startup_failed` / `port_in_use` 等 — 都映射到 `runtime_crashed` 或 `unknown`); **不是** 加 reason 枚举 (改 = 改八处 — 单测锁); **新增子项** (跨 milestone byte-identical): AL-4 stub fail-closed 时 reason='runtime_not_registered' 走 CV-4 #365 §2 stub 接口路径 (跟 #379 v2 §2 + §3 grep `runtime_not_registered` ≥1 hit + CV-4 #380 反约束 ② "AL-4 stub via direct owner commit walkaround" 同源); AL-4 落地后 status='running' 路径 reason 不再 'runtime_not_registered', 切真 runtime 真接管 commit 路径 | v0: 6 reason + stub fail-closed 'runtime_not_registered'; v1 同, AL-4 落地后真接管 |
| ⑤ | ADM-0 §1.4 红线 ③ + AL-3 #305 ③ | **runtime 状态变化触发 system message** — owner 用户感知签字 ("{agent_name} 已启动 / 停止 / 出错: {reason}"), 跟 DM-2 fallback / soft delete 同模式 | **是** runtime status 进入 running / stopped / error → 推 system DM 给 agent owner (kind='system' + sender_id='system' + 文案 byte-identical); error 文案带 reason label (#249 同源); 跟 AL-3 presence dot 4 状态文案锁同精神 (用户感知签字); **不是** silently 写表 (#11 沉默胜于假 loading 反约束: 状态变化 = 用户得知道, 走 DM 不走 toast); **不是** fanout 全 channel (仅 owner DM, runtime 是 owner 的事不污染 channel) | v0: 3 处 system message 文案锁 (start/stop/error); v1 同, 加 owner 静音偏好 |
| ⑥ | agent-lifecycle.md §2.2 v1 边界 + #313 §4 反约束 | **multi-runtime 不在 v0/v1** — 一个 agent 一个 runtime, schema 锁 UNIQUE(agent_id) | **是** `agent_runtimes` 表 `UNIQUE(agent_id)` 单 runtime per agent; 第二个 runtime register 同 agent → 409 conflict (跟 CV-1 ② 锁 last-writer-wins 同模式但更严, 这里直接拒); **不是** 多 runtime 并行 (蓝图 §2.2 v1 边界 "不优化多 runtime 并行" 字面); **不是** runtime pool / 负载均衡 (留 v2+); **不是** failover (单 runtime 挂了走 status='error' + system DM, 不自动切备份) | v0: UNIQUE(agent_id); v1 同, v2+ 加 runtime_replicas 列 |
| ⑦ | agent-lifecycle.md §4 留账 + #313 §4 反约束 | **远程 runtime 安全模型不在 AL-4** — registry + 启停信号 only, 二进制下载 / 沙箱 / 资源限制 / auto-update / uninstall 留第 6 轮 (Remote-agent / Host bridge) | **是** AL-4 仅 plugin process descriptor + start/stop API + heartbeat 信号; **不是** remote-agent 二进制下载执行 (留第 6 轮 §4); **不是** 沙箱隔离 / 资源 quota 调度 (留第 6 轮 + Phase 5+); **不是** plugin 自动升级 / 撤销 (留第 6 轮); **不是** Hermes / Windows 接入 (CHECK process_kind='openclaw' v1 字面锁, Hermes 留 v2+; OS 蓝图 §2.2 only Mac+Linux v1) | v0: registry + 启停信号; v1 同; 第 6 轮 接 remote-agent 安全模型 |

---

## 2. 黑名单 grep — Phase 4 第三波反查 (PR merge 后跑, 全部预期 0 命中)

```bash
# AL-4 ①: agent_runtimes 表不应有 LLM 配置列 (Borgee 不带 runtime 立场 #7)
grep -rnE "llm_provider|model_name|api_key|prompt_template" packages/server-go/internal/store/agent_runtimes* packages/server-go/internal/migrations/ | grep -v _test
# AL-4 ②: admin 不应有 start/stop 写路径 (god-mode 元数据 only)
grep -rnE "admin.*runtime.*start|admin.*runtime.*stop|/admin/runtimes/.*/(start|stop)" packages/server-go/internal/api/admin*.go | grep -v _test
# AL-4 ③: agent_runtimes 不应挂 is_online 列 (跟 AL-3 presence 拆死)
grep -rnE "agent_runtimes.*is_online|is_online.*agent_runtimes" packages/server-go/ | grep -v _test
# AL-4 ④: last_error_reason 不应出现 #249 6 枚举之外的值
grep -rnE "last_error_reason.*=.*['\"](startup_failed|port_in_use|oom|disk_full)['\"]" packages/server-go/ | grep -v _test
# AL-4 ④ 新增 (v0.1 patch 跟 #379 v2 + CV-4 #380 同步): stub fail-closed reason byte-identical (预期 ≥1 — 跟 CV-4 stub 接口路径同源)
grep -rnE 'runtime_not_registered' packages/server-go/internal/api/ | grep -v _test  # 预期 ≥1 (跟 #379 v2 §3 + CV-4 #365 §2 stub 接口字面同源)
# AL-4 ⑤: runtime 状态变化必须走 system DM (反向: 不应有 toast 旁路)
grep -rnE "runtime.*status.*toast|RuntimeStatusToast" packages/client/src/ | grep -v _test
# AL-4 ⑥: 不应有 multi-runtime 并行支持 (UNIQUE(agent_id) 反向证)
grep -rnE "agent_runtimes.*replicas|runtime_pool|runtime_failover" packages/server-go/ | grep -v _test
# AL-4 ⑦: 不应有 remote-agent 二进制下载 / 沙箱执行 (留第 6 轮)
grep -rnE "remote_agent.*download|sandbox.*exec|auto_update.*runtime" packages/server-go/ packages/client/ | grep -v _test
```

---

## 3. 不在 AL-4 范围 (避免 PR 膨胀, 跟 #313 §4 一致)

- ❌ LLM provider 配置 / api_key 持久化 / model_name 选择 / prompt 模板 (立场 ① 锁, 走 plugin 内部)
- ❌ Token quota / 用量计费 / rate limit per agent (留第 5 轮 plugin 协议 + 业主反馈)
- ❌ 多端同 agent runtime 并行 (立场 ⑥ UNIQUE(agent_id) 锁; v2+ 加 replicas 列)
- ❌ GPU 调度 / 资源池 / runtime 优先级 (留第 6 轮 + Phase 5+)
- ❌ remote-agent 二进制下载 / 沙箱 / auto-update / uninstall (立场 ⑦ 留第 6 轮 §4)
- ❌ Hermes 接入 (v1 only OpenClaw, CHECK process_kind 单值锁; v2+ migration 加枚举)
- ❌ Windows 支持 (蓝图 §2.2 v1 边界 only Mac+Linux)
- ❌ admin start/stop 写动作 (立场 ② ADM-0 ⑦ 红线)
- ❌ runtime 自定义 reason 枚举 (立场 ④ 6 reason 字面锁, 改 = 改八处: AL-1a #249 + AL-3 #305 + CV-4 #380 + AL-2a #454 + AL-1b #458 + 本)

---

## 4. 验收挂钩

- AL-4.1 (schema): ① 表无 `llm_provider` / `api_key` 列; ⑥ `UNIQUE(agent_id)`; ④ `last_error_reason` CHECK 6 枚举字面同 #249; ③ 无 `is_online` 列; v=15 双向 migration
- AL-4.2 (handler): ② `RequirePermission('agent.runtime.control')` start/stop 双闸 + admin 401 (反向断言); ③ heartbeat 仅更 `agent_runtimes.last_heartbeat_at` (不写 presence_sessions); ⑤ status 变化触发 system DM 文案锁; ④ error 回填 reason byte-identical 跟 `agent/state.go`
- AL-4.3 (UI): ② owner-only 按钮 (非 owner DOM 不渲染); ④ error badge reason label 跟 `lib/agent-state.ts` REASON_LABELS 同源; 4 态颜色字面对齐 AL-1a #249 模板; ⑤ system DM owner inbox 渲染
- Phase 4 第三波闸 (野马): §1 7 项全锚 + §2 grep 0 + §3 不在范围 9 条对得上 → ✅ AL-4 解封

---

## 5. v0 → v1 切换条件 (立场补丁前置)

> v1 立场补丁 PR **不可早开** — 跟 RT-1 / CV-1 v1 同规则 (#295 §5 PR # 锁), 三条件齐全才解封.

| 项 | v0 当前 (本表锁) | v1 切换触发 | v1 立场补丁内容预留 |
|----|----------------|------------|------|
| 启停 ② | owner-only + admin 元数据 only, 无 audit_log | AL-4.2 merged + ADM-2 #266 audit_log schema 落地 | 加 audit_log 行 (action='runtime.start/stop/restart') |
| status ③ | 双表双路径 byte-identical | AL-3.x 三轨 (#301/#302/#303/#305) 全 merged + AL-4.2 落地 | 加跨表查询视图 `v_agent_full_status` (status + presence 联合) |
| reason ④ | 6 reason 字面锁 | AL-1a #249 + AL-4.1 落地 | 不动 (跟 AL-1a 同步; 加 reason = 改八处) |
| system DM ⑤ | 3 处文案锁 (start/stop/error) | DM-2 三段 + AL-4.2 落地 | 加 owner 静音偏好 (per-agent runtime 通知开关) |

**反约束**: v1 补丁 PR title 必须引 AL-3.x + DM-2 + AL-4.x 三 PR # (规则 6 留账闸编号锁同模式); 任一未落, v1 PR 不开.

---

## 6. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-28 | 野马 | v0, 7 项立场 (① runtime≠LLM / ② owner-only 启停 / ③ status≠presence / ④ 6 reason 复用 / ⑤ system DM 触发 / ⑥ multi-runtime 不在 v0 / ⑦ 远程 runtime 留第 6 轮) + 7 行黑名单 grep + 9 条不在范围 + 验收挂钩 + §5 v0/v1 切换条件 (跟 #295 §5 同模式), 跟 #313 spec 立场 ①②③ + ADM-0 ⑦ + AL-1a #249 + AL-3 #310 + 蓝图 §7 / §2.2 字面对齐 |
| 2026-04-29 | 野马 | v0.1 patch — 跟 AL-4 spec #379 v2 + CV-4 spec #365 + CV-4 文案锁 #380 + CV-4 stance #385 同步: ④ 立场加 **四处单测锁** 标注 (AL-1a `agent/state.go` + AL-3 client REASON_LABELS + CV-4 #380 ③ + 本 stance, 改 = 改四处) + **新增子项 stub fail-closed reason='runtime_not_registered'** 字面 (跟 CV-4 #365 §2 stub 接口 + #379 v2 §2/§3 + CV-4 #380 反约束 ② walkaround 同源, AL-4 落地后切真路径 reason 不再 'runtime_not_registered'); §2 黑名单 grep 加 1 行预期 ≥1 (`runtime_not_registered` 跟 #379 v2 §3 + CV-4 #365 §2 byte-identical 同源). 跟 #339 stance sweep follow-up 同模式 (历史干净, 不在原 v0 加 commit) |
| 2026-04-29 | 野马 | v0.2 patch (count audit) — reason 单测锁 count drift fix (post-#454 AL-2a + post-#458 AL-1b 加入跨 milestone 链): ④ 立场 "**四处单测锁**" → "**八处单测锁**" (加 AL-2a #454 ④ + AL-1b #458 ⑤); §3 不在范围 "改 = 改八处" → "改 = 改八处" 同步; 跟 #339 stance sweep / #393 DM-2 placeholder drift / #387 AL-4 stance v0.1 follow-up patch 同模式 (历史干净). 跨 milestone reason 链全闭: AL-1a #249 / AL-3 #305 / CV-4 #380 / AL-2a #454 / AL-1b #458 / AL-4 (本) — 改 reason = 改八处 byte-identical |
