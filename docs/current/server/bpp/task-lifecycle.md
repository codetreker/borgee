# BPP-2.2 `task_started` / `task_finished` Task Lifecycle — implementation note

> BPP-2.2 (#485) · Phase 4 plugin-protocol 主线 · 蓝图 [`plugin-protocol.md`](../../../blueprint/current/plugin-protocol.md) §1.6 (失联与故障状态) + [`agent-lifecycle.md`](../../../blueprint/current/agent-lifecycle.md) §2.3 字面: "busy/idle source 必须 plugin 上行 frame, 不准 stub".

## 1. 原则 — busy/idle 单源

`busy` 状态由 `task_started` / `task_finished` frame 单源驱动. **不开** PATCH `/api/v1/agents/:id/state`. `online = session-level` 走 WS conn lifecycle, 跟 task-level (busy) 正交. 跟 AL-1b #482 BPP single source 一致 (蓝图 §2.3 R3).

AL-1b client busy/idle UI 走**派生** push: server 在收到 task lifecycle frame 后, 直接复用既有 RT-* AgentRosterUpdated / presence push 通道把派生 state 推给 client. 不另起独立的 `AgentTaskStateChangedFrame` (frame 数量少一个 不冗余, busy/idle 是 task lifecycle 的算法结果不是独立信号).

## 2. Frame schema (envelope.go #304 byte-identical)

```
TaskStartedFrame  (6 字段): {type, task_id, agent_id, channel_id, subject, started_at}
TaskFinishedFrame (7 字段): {type, task_id, agent_id, channel_id, outcome, reason, finished_at}
```

Direction 锁 `plugin_to_server`. `bppEnvelopeWhitelist` 11 frame (control 6 + data 5).

## 3. 校验规则 (`task_lifecycle.go::Validate*`)

- `TaskStartedFrame.Subject`: `strings.TrimSpace` 后非空; 空 → 错误码 `bpp.task_subject_empty` (野马 §11 文案守, 反对默认值 fallback).
- `TaskFinishedFrame.Outcome`: ∈ 3-enum `{completed, failed, cancelled}`; 中间态 (`partial`/`paused`/`pending`/`starting`) 一律 reject → `bpp.task_outcome_unknown`.
- `outcome=='failed'` 时 `Reason` 必填且 ∈ AL-1a 6 字典 (api_key_invalid / quota_exceeded / network_unreachable / runtime_crashed / runtime_timeout / unknown); 空 → `bpp.task_finished_no_reason`, 字典外 → `bpp.task_reason_unknown`.
- `outcome ∈ {completed, cancelled}` 时 `Reason` 必须为空 (反字典污染).

## 4. Reason 字典六处单测锁

`validAL1aReasons` byte-identical 跟 `internal/agent/state.go::Reason*` SSOT 同源. **改 = 改六处单测锁**: AL-1a #249 + AL-3 #305 + CV-4 #380 + AL-2a #454 + AL-1b #458 + AL-4 #387/#461 (BPP-2.2 是第七跟链处, 不另起字典).

## 5. 相关参考

- spec brief: [`docs/implementation/modules/bpp-2-spec.md`](../../../implementation/modules/bpp-2-spec.md) §1 BPP-2.2
- acceptance: [`docs/qa/acceptance-templates/bpp-2.md`](../../../qa/acceptance-templates/bpp-2.md) §2
- 实施: `internal/bpp/task_lifecycle.go` + `task_lifecycle_test.go` (8 tests)
