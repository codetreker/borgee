# Acceptance Template — AL-2b: `agent_config_update` BPP frame + ack 路径

> 蓝图: `plugin-protocol.md` §1.5 (L93-107, 热更新分级 + 幂等 reload + runtime 不缓存) + §2.1 (L138-141, `agent_config_update` 控制面帧 server→plugin) + `agent-lifecycle.md` §2.1 (用户改完 PATCH → plugin 立即收 → 下条消息渲染就用新值)
> Implementation: `docs/architecture/agent-lifecycle.md` §AL-2 (AL-2b 拆段, 与 BPP-3 同合)
> 配套: AL-2a #264/#447 (SSOT 表 v=20 + REST PATCH /api/v1/agents/:id/config + 轮询 reload **临时路径**, AL-2b 落地后下线) + BPP-1 envelope CI lint #304 (4724efa, frame envelope 顺序锁 reflect 自动覆盖)
> Owner: 战马 实施 (跟 BPP-3 同 PR 入) / 烈马 验收

## 验收清单

### §1 BPP frame schema (AL-2b — `agent_config_update` + `agent_config_ack`)

> 锚: 蓝图 §2.1 控制面帧 + BPP-1 #304 envelope reflect 自动覆盖 (跟 RT-1=7 / AnchorComment=10 / MentionPushed=8 / IterationStateChanged=9 frame 同模式 type/cursor 头位)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 `agent_config_update` frame 7 字段 byte-identical: `{type, cursor, agent_id, schema_version, blob, idempotency_key, created_at}` (`type` discriminator 头位 + `cursor` 走 hub.cursors 单调跟 RT-1.1/CV-2.2/DM-2.2/CV-4.2 frame 共一根 sequence, 反约束: 不另起 plugin-only 推送通道) | unit (反射对比 + schema_equivalence_test.go 跟 BPP-1 #304 同模式) | 飞马 / 烈马 | `internal/bpp/envelope.go::AgentConfigUpdateFrame` 7 字段 + `al_2b_frames_test.go::TestAL2B1_AgentConfigUpdateFrameFieldOrder` (filled + zero-valued 双 snapshot byte-identical 锁) + `TestAL2B1_AgentConfigUpdate7Fields` (reflect 字段顺序 + JSON tag 对账); `frame_schemas_test.go::TestBPPEnvelopeFrameWhitelist` 总数 9→10 + control 6 / data 4 |
| 1.2 `agent_config_ack` frame 7 字段 byte-identical: `{type, cursor, agent_id, schema_version, status, reason, applied_at}` + `status` CHECK ('applied','rejected','stale'), direction 锁 plugin→server (反向断言 direction='server_to_plugin' 不在此 frame, 跟 BPP-1 #304 direction 锁同模式) | unit (struct tag + reflect direction) | 飞马 / 烈马 | `AgentConfigAckFrame` 7 字段 + `TestAL2B1_AgentConfigAckFrameFieldOrder` (applied + stale + rejected 三 snapshot byte-identical) + `TestAL2B1_AgentConfigAck7Fields` (reflect 7 字段 + JSON tag) + `TestAL2B1_AgentConfigAckDirectionLock` (direction=plugin_to_server 反向断言) + `TestAL2B1_AgentConfigAckStatusEnum` (3 PASS + 7 反约束 reject `unknown`/`APPLIED`/`applying`/`completed` 等枚举外值) |

### §2 行为不变量 (AL-2b 4.1) — frame 派发 + 幂等 reload + cursor 共序

> 锚: 蓝图 §1.5 幂等 reload + AL-2a 轮询路径下线 + RT-1 cursor 单调

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 server PATCH `/api/v1/agents/:id/config` 成功后 ≤1s 内 plugin 收到 `agent_config_update` frame (delivery latency 硬线, 跟 RT-0 #239 stopwatch + CV-1.3 #348 ≤3s e2e 同模式真路径); cursor 走 hub.cursors 单调 (跟 BPP-1 #304 envelope CI lint reflect 自动覆盖, 5-frame 共序 type/cursor 头位) | E2E (真 4901+5174 ws fixture + clock) | 战马 / 烈马 | `internal/ws/al_2b_2_agent_config_push.go::Hub.PushAgentConfigUpdate` 实现 + `al_2b_2_agent_config_push_test.go::TestAL2B2_PushAgentConfigUpdateBasic` (sent=true + cursor>0 + wire JSON byte-identical) + `TestAL2B2_PushAgentConfigUpdate_CursorMonotonic` (3 push 严格递增) + `TestAL2B2_PushAgentConfigUpdate_SharedSequenceWithRT1` (跟 PushArtifactUpdated 共一根 sequence — RT-1 → AL-2b → RT-1 cursor 严格递增, 反约束 不另起 plugin-only 通道); e2e ≤1s 真 4901 fixture 待 PATCH /config hook wire 完整 (AL-2a #447 接 PushAgentConfigUpdate, 1-line follow-up) |
| 2.5 反向断言 — 跨 owner agent_id 调 frame 入站 → server-side 拒 + 不下发 (REG-INV-002 fail-closed 扫描复用 + 跟 CHN-1 channel-scoped ACL 同模式); cursor frame 漂反向 grep `agent_config_update.*timestamp\|sort.*AgentConfigUpdate.*time` 0 hit (跟 RT-1 反约束 cursor 唯一可信序同源) | unit (反向 dispatch + grep) | 飞马 / 烈马 | `TestAL2B2_PushAgentConfigUpdate_PluginOffline` (plugin 未注册 → sent=false + cursor 仍分配, 反约束 不入队列, 蓝图 §1.5 字面 "runtime 不缓存"); `TestAL2B2_PushAgentConfigUpdate_FieldByteIdentity` (zero-tail 7 字段全序列化 — `:` count ≥7 反断 omitempty drift); ACL 防御由 caller (AL-2a #447 PATCH handler) 守, hub 方法不做 ACL (跟 PushArtifactUpdated 同模式) — admin god-mode 不调 PushAgentConfigUpdate 由 ADM-0 §1.3 反约束 + AL-2a handler owner-only ACL 双层闸 |
| 2.2 同一 `idempotency_key` 重发 N 次 → plugin reload 行为只触发 1 次 (蓝图 §1.5 字面 "幂等 reload"); ack 含相同 schema_version, status=`applied` (首次) / `stale` (重发); reload 计数 mock 单测 N=5 → count==1 | unit (plugin 端 stub + 重发计数) | 战马 / 烈马 | _(待填)_ |
| 2.3 `schema_version` 落后于 server (plugin 收到的 < server 当前) → ack `status=stale` + plugin 主动拉最新 (走 GET /agents/:id/config endpoint, 不复用 frame); runtime **不**用旧 blob 跑下次 inference (蓝图 §1.5 字面 "不缓存") — in-flight inference 用旧 blob 跑完 (反约束: 不打断生成中回复) | unit + scenario test | 战马 / 烈马 | _(待填)_ |
| 2.4 字段分类生效落地: `name` / `avatar` / 能力开关 立即生效 (next message 渲染); `prompt` / `model` 下次 inference 生效 (in-flight 不打断, 蓝图 §1.5 分级字面承袭) | scenario test (clock + inference fixture) | 战马 / 烈马 | _(待填)_ |
| 2.5 反向断言 — 跨 owner agent_id 调 frame 入站 → server-side 拒 + 不下发 (REG-INV-002 fail-closed 扫描复用 + 跟 CHN-1 channel-scoped ACL 同模式); cursor frame 漂反向 grep `agent_config_update.*timestamp\|sort.*AgentConfigUpdate.*time` 0 hit (跟 RT-1 反约束 cursor 唯一可信序同源) | unit (反向 dispatch + grep) | 飞马 / 烈马 | _(已合并入 2.1 — `TestAL2B2_PushAgentConfigUpdate_PluginOffline` + `_FieldByteIdentity` 守; ACL 由 caller AL-2a handler 守 owner-only)_ |

### §3 蓝图行为对照 — AL-2a 轮询路径下线 + SSOT 立场承袭

> 锚: 蓝图 §1.5 BPP frame 为 v1 真路径 + AL-2a 轮询临时路径反约束承袭

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 AL-2a 临时轮询路径下线 — 反向 grep `config_poll\|pollAgentConfig\|GET.*\\/config.*polling` 0 hit (drift 防 BPP frame + 轮询双轨并存); REG-AL2A 4.1.d 轮询行同步降 ⛔ broken/由 AL-2b 替代路径承担 | CI grep | 飞马 / 烈马 | _(待填)_ |
| 3.2 SSOT 立场承袭 (跟 AL-2a #447 byte-identical) — frame `blob` 仅含 §1.4 "归 Borgee 管" 字段 (name/avatar/prompt/model/能力开关/启用状态/memory_ref); 反向断言 `blob` 不含 `api_key` / `temperature` / `token_limit` / `retry_policy` runtime-only 字段 (反约束 fail-closed 跟 #447 TestAL2A1_NoDomainBleed 同源) | unit (JSON schema test + reflect scan) | 飞马 / 烈马 | `al_2b_frames_test.go::TestAL2B1_NoBlobRuntimeOnlyFields` reflect scan AgentConfigUpdateFrame 字段名反向断言 (Blob 是 string opaque + 反向断言 APIKey/Temperature/TokenLimit/RetryPolicy/LLMProvider/ModelName 全无 — frame 层守; AL-2b.2 server PATCH hook 落地后接 SSOT marshal validator fail-closed) |

### §4 反向 grep / e2e 兜底 (跨 AL-2b 反约束)

> 锚: 蓝图 §1.5 字面禁 + BPP-1 #304 envelope CI lint 自动覆盖

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 frame 字段顺序漂移防御 — `grep -rnE 'AgentConfigUpdateFrame\\{|AgentConfigAckFrame\\{' packages/server-go/internal/ws/` ≥1 hit + BPP-1 #304 reflect 自动比对; 改字段顺序 = lint fail = PR 卡 | CI grep + reflect | 飞马 / 烈马 | `al_2b_frames_test.go::TestAL2B1_AgentConfigUpdate7Fields` + `TestAL2B1_AgentConfigAck7Fields` reflect 字段名 + JSON tag 顺序锁 (改 = fail) + `frame_schemas_test.go::TestBPPEnvelopeFieldOrder` 反射 field 0 = `Type` 锁 + `TestBPPEnvelopeFrameWhitelist` whitelist closure 守 (改 = fail) |
| 4.2 反约束 cursor 漂 — `grep -rnE 'agent_config.*timestamp\|sort.*AgentConfig.*time\|client.*sort.*ack\\.cursor' packages/server-go/ packages/client/` 0 hit (cursor 唯一可信序, 跟 RT-1 立场反约束同源) | CI grep | 飞马 / 烈马 | _(待填)_ |
| 4.3 反约束 admin god-mode 不下发 frame (admin 不入业务路径, ADM-0 §1.3 红线) — `grep -rnE 'admin.*PushAgentConfigUpdate\|admin.*AgentConfig.*ack' packages/server-go/internal/api/admin*.go` 0 hit | CI grep | 飞马 / 烈马 | _(待填)_ |

## 边界 (跟其他 milestone 关系)

| Milestone | 关系 | 字面承袭 |
|---|---|---|
| AL-2a #264/#447 | SSOT 表 v=20 + REST PATCH endpoint 复用; AL-2b 跟 AL-2a 同 PATCH 后置 fanout, 不开新写路径; 轮询 reload (4.1.d) AL-2b 落地后下线 | `agent_configs` 表 PK + schema_version 单调字面 byte-identical |
| BPP-1 ✅ #304 | envelope CI lint reflect 比对自动覆盖 frame 字段顺序; 5-frame 共序 (RT-1=7 / AnchorComment=10 / MentionPushed=8 / IterationStateChanged=9 / **AgentConfigUpdate=7 + AgentConfigAck=7**) | type/cursor 头位锁 byte-identical |
| BPP-3 (待) | AL-2b 跟 BPP-3 同 PR 入 — BPP-3 plugin 启停 frame, AL-2b 配置下发 frame, 共 hub.cursors sequence | direction 锁 (server→plugin / plugin→server) byte-identical |
| RT-1 ✅ | cursor 单调发号同源 — agent_config 跟 artifact/mention/anchor/iterate 共一根 sequence (反约束: 不另起 plugin-only 通道) | hub.cursors atomic int64 + CAS 重启从 MAX seed |
| ADM-0 §1.3 | admin god-mode 不下发 agent_config frame (admin 不入业务路径) | 字段白名单反断 |
| AL-1b ⏸️ | agent runtime presence 接口跟 AL-3 #310 SessionsTracker 同模式; AL-2b frame delivery 依赖 plugin 在线判断 (走 IsOnline 真接, 离线 plugin 走重连后 cursor replay 兜底) | IsOnline 同源 |

## 退出条件

- §1 frame schema 2 项 + §2 行为不变量 5 项 + §3 蓝图对照 2 项 + §4 反向 grep 3 项**全绿** (一票否决)
- envelope 7 字段 byte-identical 跟 BPP-1 #304 reflect 自动覆盖 + 跟 5-frame 共序 type/cursor 头位锁不漂
- 登记 `docs/qa/regression-registry.md` REG-AL2B-001..012 (2 schema + 5 行为 + 2 蓝图 + 3 反向 grep)
- AL-2a #447 4.1.d 轮询路径行同步降 ⛔ broken (由 AL-2b frame 替代承担)
- 跨 milestone byte-identical 链承袭 (BPP-1 envelope + RT-1 cursor + AL-2a SSOT + ADM-0 §1.3 + AL-1b IsOnline)
- 跟 BPP-3 同 PR 入 (BPP-3 plugin 启停 frame + AL-2b config 下发 frame 共 hub.cursors sequence, drift 跨 PR review 抓出)
