# AL-2b spec brief — `agent_config_update` BPP frame + ack 路径 (跟 BPP-3 同 PR 合)

> 烈马 · 2026-04-29 · ≤200 行 spec lock (跟 #452 acceptance v1 同源, 实施视角拆 PR 由战马待派, 跟 BPP-3 同 PR 合规划)
> **蓝图锚**: [`plugin-protocol.md`](../../blueprint/current/plugin-protocol.md) §1.4 (Borgee=SSOT 字段划界) + §1.5 (热更新分级 — 字段下发 + 幂等 reload + runtime 不缓存) + §2.1 (`agent_config_update` 控制面帧 server→plugin) + [`agent-lifecycle.md`](../../blueprint/current/agent-lifecycle.md) §2.1 (用户改完 PATCH → plugin 立即收 → 下条消息渲染就用新值)
> **关联**: AL-2a #264/#447 (SSOT 表 v=20 + REST PATCH /api/v1/agents/:id/config + 轮询 reload 临时路径) + AL-2a content-lock #454 + AL-2a stance #454 + #452 烈马 acceptance v1 (12 项 4 段); BPP-1 ✅ #304 (envelope CI lint reflect 自动覆盖 frame 字段顺序); 跟 BPP-2 spec (#460) §1.3 + §1.6 字面对齐
> **章程闸**: Phase 4 入口 — 跟 BPP-3 同 PR 合, BPP-3 plugin 启停 + AL-2b config 下发 共 hub.cursors sequence (drift 跨 PR review 抓出)

> ⚠️ 锚说明: AL-2a 4.1.d 轮询 reload 临时路径 (`config_poll`/`pollAgentConfig`) AL-2b 落地后下线 — 反约束 drift 防 BPP frame + 轮询双轨并存; AL-2a 不裂 BPP frame (R3 决议字面承袭) → AL-2b 跟 BPP-3 同 PR 入

## 0. 关键约束 (3 条立场, 蓝图字面 + AL-2a/BPP-1/BPP-2 边界对齐)

1. **`agent_config_update` 走 BPP envelope 单源, 跟 5-frame 共序 type/cursor 头位 + reflect 自动覆盖** (BPP-1 #304 envelope CI lint 锚 + 蓝图 §2.1): 7 字段 byte-identical `{type, cursor, agent_id, schema_version, blob, idempotency_key, created_at}` 跟 RT-1=7 / AnchorComment=10 / MentionPushed=8 / IterationStateChanged=9 + AgentConfigAck=7 / **AL-1b BPP-2 task_started=7 / task_finished=7** 同模式 type/cursor 头位; cursor 走 hub.cursors 单调发号; **反约束**: 不另起 plugin-only 推送通道 (跟 RT-1 反约束同源); 不带 ack/retry (best-effort, plugin 重连后 cursor replay 兜底, 跟 RT-0 立场承袭)
2. **幂等 reload + schema_version stale 不缓存 + in-flight 不打断** (蓝图 §1.5 字面 "幂等 reload" + "runtime 不缓存"): 同 `idempotency_key` 重发 N 次 → plugin reload 行为只触发 1 次 (server 端不去重发, plugin 端去重接收); ack `status='applied'` (首次) / `'stale'` (重发覆盖); plugin 收 `schema_version < 当前` → ack `'stale'` + 主动拉最新 (走 GET /agents/:id/config endpoint, 不复用 frame); **反约束**: in-flight inference 用旧 blob 跑完不打断 (蓝图 §1.5 "字段分类生效" — name/avatar/能力开关立即, prompt/model 下次 inference)
3. **AgentConfigAck plugin→server direction 锁 + status enum CHECK + 反人工伪造** (BPP-1 #304 direction 锁同模式 + AL-2a stance #454 ④): ack 7 字段 `{type, cursor, agent_id, schema_version, status, reason, applied_at}` 锁 plugin→server (反向断言 `direction='server_to_plugin'` 不在此 frame); status CHECK ('applied','rejected','stale') reject 'unknown' 枚举外值; cross-owner agent_id 调 frame 入站 → server-side 拒不下发 (REG-INV-002 fail-closed 扫描器复用); **反约束**: admin god-mode 不下发 frame (admin 不入业务路径, ADM-0 §1.3 红线 + AL-3 #303 ⑦ + AL-4 #379 v2 同模式)

## 1. 拆段实施 (单 PR 跟 BPP-3 合, 3 文件 + 2 测试)

| 文件 | 范围 |
|---|---|
| `internal/ws/agent_config_update_frame.go` (新) | `AgentConfigUpdateFrame` 7 字段 struct + JSON tag byte-identical 跟 acceptance §1.1 字面 + `Hub.PushAgentConfigUpdate(agentID string, schemaVersion int, blob json.RawMessage, idempotencyKey string, createdAt int64) (cursor int64, sent bool)` 单推目标 plugin (跟 MentionPushed 单推 BroadcastToUser 同模式, 反约束: 不抄送 owner / 不广播 channel); cursor 走 hub.cursors.NextCursor() 跟 5-frame 共一根 sequence; BPP-1 #304 envelope CI lint reflect 自动闸位 |
| `internal/ws/agent_config_ack_frame.go` (新) | `AgentConfigAckFrame` 7 字段 struct + status const ('applied'/'rejected'/'stale') + dispatcher 入站 handler (走 BPP-2 spec 同模式 frame dispatcher 路径) + cross-owner reject (REG-INV-002 fail-closed 扫描器复用) + admin god-mode 不下发 (反向 grep `admin.*PushAgentConfigUpdate\|admin.*AgentConfig.*ack` 0 hit) |
| `internal/api/agents.go` (改) | `PATCH /api/v1/agents/:id/config` handler (AL-2a #447 既有) 加 fanout 步骤: 写库后 `Hub.PushAgentConfigUpdate(...)` 单推目标 plugin; idempotency_key UUID 生成 (server 端, 客户端 PATCH body 不需带); reload 计数 mock 单测 N=5 → plugin 端 count==1; AL-2a 轮询 reload 路径下线 (移除 `config_poll`/`pollAgentConfig` — 反向 grep 0 hit drift 防双轨并存) |

**owner**: 战马 (跟 BPP-3 plugin 启停 frame 同 PR 合, drift 跨 PR review 抓出 — BPP-3 + AL-2b 共 hub.cursors sequence)

## 2. 与 AL-2a/BPP-1/BPP-2/BPP-3/RT-1/ADM-0/AL-1b 留账冲突点

- **AL-2a #264/#447 SSOT 表 + REST PATCH** (核心承袭): AL-2a `agent_configs` 表 v=20 + PATCH endpoint 复用; AL-2b 跟 AL-2a 同 PATCH 后置 fanout, 不开新写路径; 轮询 reload (4.1.d) AL-2b 落地后下线 (REG-AL2A 4.1.d 行同步降 ⛔ broken/由 AL-2b 替代承担)
- **BPP-1 ✅ #304 envelope CI lint** (核心): 7 字段 byte-identical 跟 reflect 自动覆盖; 5-frame 共序锁: RT-1=7 / AnchorComment=10 / MentionPushed=8 / IterationStateChanged=9 / **AgentConfigUpdate=7 + AgentConfigAck=7**; type/cursor 头位锁 byte-identical 不漂
- **BPP-2 spec #460 §1.3+§1.6** (字面对齐, 跟 BPP-2 task_started/task_finished 7 字段同 envelope 模式): AL-2b frame schema 跟 BPP-2 frame 同 hub.cursors sequence 单调发号; AL-2b 不引入第 6 frame schema (BPP-1 已锁 5 frame, AgentConfigUpdate + AgentConfigAck 是 control plane 6+7 frame, 跟 BPP-2 task_started/finished 8+9 frame 同模式新增, 不裂 namespace)
- **BPP-3 (待) plugin 启停** (同 PR 合): BPP-3 plugin 启停 frame + AL-2b config 下发 frame 共 hub.cursors sequence; direction 锁 (server→plugin / plugin→server) byte-identical; drift 跨 PR review 抓出 (一 PR 双段不混翻)
- **RT-1 cursor 单调** (核心): hub.cursors atomic int64 + CAS 重启从 MAX seed; agent_config 跟 artifact/mention/anchor/iterate/task_started/task_finished 共一根 sequence (反约束: 不另起 plugin-only 通道)
- **ADM-0 §1.3 admin god-mode 红线**: admin 不下发 agent_config frame; 反向 grep `admin.*PushAgentConfigUpdate\|admin.*AgentConfig.*ack` 0 hit 闸位
- **AL-1b #453/#457 task_started/task_finished**: AL-1b BPP-2 frame 跟 AL-2b config 下发 frame 路径独立但 cursor 共序; AL-1b plugin reload 不依赖 AL-2b (AL-1b 是 task-level state, AL-2b 是 config-level update 不冲突)
- **REG-INV-002 fail-closed 扫描器复用**: cross-owner reject + runtime-only 字段 (api_key/temperature/token_limit/retry_policy) 不在 blob (跟 #447 TestAL2A1_NoDomainBleed 同源)

## 3. 反查 grep 锚 (Phase 4 验收 + AL-2b 实施 PR 必跑)

```
git grep -nE 'AgentConfigUpdateFrame\{|AgentConfigAckFrame\{' packages/server-go/internal/ws/   # ≥ 1 hit (frame struct 字面)
git grep -nE 'PushAgentConfigUpdate'                            packages/server-go/internal/    # ≥ 1 hit (Hub method + handler 调用)
git grep -nE 'IdempotencyKey|idempotency_key'                  packages/server-go/internal/ws/ packages/server-go/internal/api/   # ≥ 1 hit (幂等 reload key 字面)
git grep -nE "case\\s+['\"]agent_config_update['\"]"            packages/server-go/internal/ws/ # ≥ 1 hit (frame dispatcher)
git grep -nE "case\\s+['\"]agent_config_ack['\"]"               packages/server-go/internal/ws/ # ≥ 1 hit (ack 入站 handler)
# 反约束 (5 条 0 hit)
git grep -nE 'config_poll|pollAgentConfig|GET.*\\/config.*polling' packages/server-go/internal/ packages/client/src/   # 0 hit (AL-2a 轮询路径下线 drift 防双轨)
git grep -nE 'agent_config.*timestamp|sort.*AgentConfig.*time|client.*sort.*ack\\.cursor' packages/server-go/ packages/client/   # 0 hit (cursor 唯一可信序, 反 timestamp 漂)
git grep -nE 'admin.*PushAgentConfigUpdate|admin.*AgentConfig.*ack' packages/server-go/internal/api/admin*.go   # 0 hit (ADM-0 §1.3 红线)
git grep -nE 'AgentConfigUpdate.*broadcast|PushAgentConfigUpdate.*channel|fanout.*config' packages/server-go/internal/ws/   # 0 hit (反约束 单推 plugin 不广播)
git grep -nE "blob.*['\"](api_key|temperature|token_limit|retry_policy)['\"]" packages/server-go/internal/   # 0 hit (SSOT 立场承袭 #447 NoDomainBleed)
```

任一 0 hit (除反约束行) → CI fail.

## 4. 不在本轮范围 (反约束)

- ❌ ack/retry 机制 (best-effort, plugin 重连后 cursor replay 兜底, 跟 RT-0 立场承袭)
- ❌ admin god-mode 看 agent_config frame (ADM-0 §1.3 红线, 字段白名单不含)
- ❌ 跨 owner agent_id 下发 frame (REG-INV-002 fail-closed 扫描器拦)
- ❌ blob 包含 runtime-only 字段 (api_key/temperature/token_limit/retry_policy 反向 grep 0 hit, SSOT 字段划界永久锁)
- ❌ in-flight inference 打断 (蓝图 §1.5 字面 "字段分类生效" — prompt/model 下次 inference)
- ❌ AL-2a 轮询路径双轨并存 (drift 防, AL-2b 落地即下线)
- ❌ 第 6+ schema frame (BPP-1 #304 envelope CI lint 锁 5 frame; AgentConfigUpdate + AgentConfigAck 是 6+7 frame schema 新增, 不裂 namespace)
- ❌ multi-agent batch config update (一 frame = 一 agent_id; batch 留 v3+)

## 5. Test plan (实施 PR 各自带, 此 spec 不带)

- frame schema: `internal/ws/agent_config_update_frame_test.go::TestAgentConfigUpdateFrameFieldOrder` JSON byte-equality pin 7 字段 + `agent_config_ack_frame_test.go::TestAgentConfigAckFrameDirection` direction 锁反断 + status enum CHECK reject 'unknown'
- 行为: server PATCH /config 后 ≤1s plugin 收 frame (clock fixture 真路径 ws fixture 跟 RT-0 #239 stopwatch 同模式) + 同 idempotency_key 重发 N=5 → reload count==1 + schema_version stale → ack 'stale' + 反向断言 cross-owner 拒
- 反约束: 反向 grep 5 锚 0 hit (轮询路径下线 + cursor timestamp 漂 + admin god-mode + 广播 plugin / fanout config + blob runtime field)
- 跟 BPP-3 同 PR 合 — BPP-3 plugin 启停 frame 测试 + AL-2b config 下发 frame 测试共 hub.cursors sequence smoke (跟 BPP-1 #304 lint reflect 自动闸位)
