# Acceptance Template — AL-2a: agent 配置 SSOT 表 + update API

> 蓝图: `docs/blueprint/current/agent-lifecycle.md` §2.1 (L28-39, "用户完全自主决定 agent 的 name/prompt/能力/model")
> 蓝图: `docs/blueprint/current/plugin-protocol.md` §1.4 (L63-91, Borgee=SSOT 字段划界) + §1.5 (L93-107, 热更新分级 — 字段下发, **AL-2a 不含 BPP frame**)
> Implementation: `docs/architecture/agent-lifecycle.md` §AL-2 (L41-52, AL-2a 拆段)
> R3 决议: AL-2 拆 a/b — AL-2a 只落 config 表 + REST update API; agent 端 reload 走轮询; BPP `agent_config_update` frame 留给 AL-2b 与 BPP-3 同合 (战马 D5 锁紧)
> 依赖: 无 (可并行 CM-*)
> Owner: 战马A 实施 / 烈马 验收

## 拆 PR 顺序 (单 PR, 不串行)

- **AL-2a**: `agent_configs(agent_id, schema_version, blob, updated_at)` 表 + `PATCH /api/v1/agents/:id/config` update API + 并发 update idempotent + agent 端轮询 reload (临时, 等 AL-2b 切 BPP frame)

## 验收清单

### 数据契约 (蓝图 §1.4 字段划界)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| `agent_configs` 表 schema 字段固化 (agent_id PK / schema_version int / blob JSON / updated_at), schema_version 单调递增 | unit + migration test | 战马A / 烈马 | ✅ #447 (13af413) — `internal/migrations/al_2a_1_agent_configs_test.go::TestAL2A1_CreatesAgentConfigsTable` + `TestAL2A1_PKEnforcesSingleRowPerAgent` + `TestAL2A1_AcceptsMonotonicSchemaVersion` PASS (REG-AL2A-001 + 003) |
| `blob` 仅含蓝图 §1.4 "归 Borgee 管" 字段 (name / avatar / prompt / model / 能力开关 / 启用状态 / memory_ref); CI grep 反向断言 blob schema 不含 `temperature`/`token_limit`/`api_key`/`retry_policy` (runtime-only 字段, fail-closed) | CI grep + JSON schema test | 战马A / 烈马 | ✅ #447 (13af413+1b69670) — `al_2a_1_agent_configs_test.go::TestAL2A1_NoDomainBleed` (8 列反向) PASS + server allowedConfigKeys whitelist 7 字段 byte-identical 跟 client ALLOWED_CONFIG_KEYS 同源 (REG-AL2A-002 + 007) |

### 行为不变量 (闸 2 — AL-2a 4.1)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1.a `PATCH /api/v1/agents/:id/config` 并发 2 写 → 末次胜出 + schema_version 严格递增 + 无丢失 (idempotent: 同 payload 重发不增 version) | unit (并发 goroutine) | 战马A / 烈马 | ✅ #447 (1b69670) — `internal/api/al_2a_2_agent_config_test.go::TestAL2A2_ConcurrentLastWriteWins` (10 goroutines, schema_version=10 monotonic 验) + `TestAL2A2_PatchAndGet` (二次 PATCH version 1→2 + blob 整体替换 model 字段消失) PASS (REG-AL2A-004) |
| 4.1.b 跨 agent owner 调用 `PATCH .../config` → 403 (反向断言, 非本人 agent 不可改) | unit | 烈马 | ✅ #447 (1b69670) — `al_2a_2_agent_config_test.go::TestAL2A2_CrossOwnerReject` (member token PATCH+GET owner agent → 403 双断) PASS (REG-AL2A-005) |
| 4.1.c response struct 反射扫描白名单: 不返回 `api_key`/`temperature`/`retry_policy` 等 runtime-only 字段 (fail-closed, REG-INV-002 同款扫描器) | unit (reflect scan) | 烈马 | ✅ #447 (1b69670) — `al_2a_2_agent_config_test.go::TestAL2A2_RuntimeFieldRejected` (4 子用例 api_key/temperature/token_limit/retry_policy 全 400 + code `agent_config.runtime_field_rejected` byte-identical) + server `allowedConfigKeys` whitelist 7 字段 fail-closed PASS (REG-AL2A-005) |
| 4.1.d agent 端轮询 reload: PATCH 后 ≤ 轮询周期内 GET `/api/v1/agents/:id/config` 返回新 blob + schema_version (drift test, 防止 cache 不刷) | unit (httptest + 时钟 mock) | 战马A / 烈马 | ✅ #447 (1b69670+fb69ca1) — `al_2a_2_agent_config_test.go::TestAL2A2_PatchAndGet` (PATCH 后立即 GET 返新 state, 二次 PATCH 后 GET model 字段消失断言 SSOT) + `TestAL2A2_GetCorruptBlob` (json.Unmarshal error 防御 → 500) + `TestAL2A2_HandlerNowInjection` (injectable clock fixture) PASS (REG-AL2A-006) |

### 蓝图行为对照 (闸 2)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| §1.5 BPP frame `agent_config_update` **不在** AL-2a 范围: `grep -rE 'agent_config_update' internal/ws/ internal/bpp/` count==0 (反向, 推到 AL-2b) | CI grep | 飞马 | ✅ #447 (8110797) — client `al-2a-content-lock.test.ts::反约束 不订阅 push frame` PASS (`'agent_config_update'` 单引号字面 0 hit, 仅 doc comment 出现说明立场) + server `agent_config.go` 无 hub.Broadcast 调用 + 走轮询 reload 立场字面 (REG-AL2A-007) |
| §1.4 SSOT 立场: `users` 表 agent 行不再持有 `prompt`/`model` 列 (若存量字段 → backfill 入 agent_configs.blob + drop 列; v0 forward-only) | unit + migration test | 战马A / 烈马 | ⚪ 留账 — v0 stance: agent_configs.blob SSOT 已落, users 表 prompt/model 列 (如有) backfill + drop 留 v3+ migration (forward-only, 不阻塞 AL-2a closure). 当前 users 表 schema 无 prompt/model 列 (`migrations.go` createSchema 反向断言), 此项无 backfill 工作量 — 等未来若加 SSOT 校准 PR 时翻 ✅ |
| §1.5 BPP envelope CI lint **真落** ✅ (G2.6 闸 — `bpp/frame_schemas.go` whitelist + 方向锁 + 字段顺序锁 + godoc 锚 + AST 反向覆盖, 反向断言: `grep -rEn 'replay_mode.*=.*"full".*default\|defaultReplayMode\|ResumeModeFull.*default' packages/server-go/internal/bpp/ --exclude='*_test.go'` count==0 + schema_equivalence dispatcher prefix byte-identical 于 RT-0 #237 / RT-1.1 #290 / RT-1.3 #296) | reflection + AST + grep | 飞马 / 烈马 | #304 (commit 4724efa) — `bpp/frame_schemas_test.go` 6 reflection subtest + `bpp/schema_equivalence_test.go` 2 dispatcher subtest, REG-BPP1-001..008 全 🟢 |

### 退出条件

- 上表 7 项**全绿** (一票否决式: 任何 4.1.x 红 → 不签字)
- 战马A 引用 review 同意 + 烈马 acceptance 跑完
- 登记 `docs/qa/regression-registry.md` REG-AL2A-001..007 (PR merge 后 24h 内翻 ⚪ → 🟢)
