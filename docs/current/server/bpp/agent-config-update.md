# BPP-2.3 `agent_config_update` — implementation note

> BPP-2.3 (#485) · Phase 4 plugin-protocol 主线 · 蓝图 [`plugin-protocol.md`](../../../blueprint/current/plugin-protocol.md) §1.5 (配置热更新按字段分类生效 + 幂等 reload) + §1.4 表 (Borgee 管 vs Runtime 管 字段划界).

## 1. 设计原则

配置热更新单源 server→plugin. `AgentConfigUpdateFrame` payload 携带配置 delta, plugin 必须支持幂等 reload (runtime 不缓存 agent 定义 — 每次 inference 前读最新 config).

## 2. 6 字段白名单

`internal/bpp/agent_config_update.go::ValidConfigFields` byte-identical 跟蓝图 §1.4 表左列字面 "归 Borgee 管 (用户选择项)":

```
name / avatar / prompt / model / capabilities / enabled
```

字典外字段 (含 runtime 调优字段 `api_key` / `temperature` / `token_limit` / `retry_policy` / `latency_budget_ms` 等蓝图 §1.4 右列字面) → reject + 错误码 `bpp.config_field_disallowed`.

## 3. 幂等 reload (`ConfigRevTracker`)

`ShouldApply(agentID, configRev) bool` 守门:
- `configRev > last_seen_rev` → true + 记 rev. Caller 调 plugin reload.
- `configRev <= last_seen_rev` → false (idempotent retry / network double-send). Caller drop frame, log debug.

线程模型: `ConfigRevTracker` **不** goroutine-safe. BPP 单 plugin 连接是 single-reader 串行化 (BPP-1 不变量), 跟 AL-4.1 #398 schema `UNIQUE(agent_id)` "one runtime per agent" 设计一致 — 跨 plugin 连接同 agent 上行本身已是协议违反.

## 4. 反约束 (grep 守门 CI lint count==0)

- runtime 调优字段不入 frame payload — 字段白名单严闭.
- config 单源 server→plugin (plugin 不上行 config) — direction 锁 `server_to_plugin`.
- 不另起 `bpp_v2` namespace — 复用 BPP-1 envelope 不裂.

## 5. 相关参考

- spec brief: [`docs/implementation/modules/bpp-2-spec.md`](../../../implementation/modules/bpp-2-spec.md) §1 BPP-2.3
- acceptance: [`docs/qa/acceptance-templates/bpp-2.md`](../../../qa/acceptance-templates/bpp-2.md) §3
- content lock: [`docs/qa/bpp-2-content-lock.md`](../../../qa/bpp-2-content-lock.md) §1 ② 6 字段白名单
- 实施: `internal/bpp/agent_config_update.go` + `agent_config_update_test.go` (8 tests)
- 跟 AL-2b #481 关联: AL-2b 用同 `AgentConfigUpdateFrame` 字段名做 server→plugin push (PATCH /api/v1/agents/:id/config 后 fanout); BPP-2 worktree 跟 AL-2b worktree 当前字段集略有 drift, merge 时由 teamlead 排序处理.
