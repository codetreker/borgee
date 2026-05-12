# BPP-2.2 `task_started` / `task_finished` Task Lifecycle — implementation note

> BPP-2.2 (#485) · Phase 4 plugin-protocol main line · blueprint [`plugin-protocol.md`](../../../blueprint/current/plugin-protocol.md) §1.6 (disconnection and failure state) + [`agent-lifecycle.md`](../../../blueprint/current/agent-lifecycle.md) §2.3 literal: "busy/idle source 必须 plugin 上行 frame, 不准 stub".

## 1. Principle — busy/idle single source

`busy` state is driven only by `task_started` / `task_finished` frames. **Do not add** PATCH `/api/v1/agents/:id/state`. `online = session-level` follows the WS connection lifecycle and stays orthogonal to task-level busy state. This matches the AL-1b #482 BPP single source (blueprint §2.3 R3).

AL-1b client busy/idle UI uses a **derived** push: after the server receives a task lifecycle frame, it reuses the existing RT-* AgentRosterUpdated / presence push channel to send derived state to the client. Do not add a separate `AgentTaskStateChangedFrame`; busy/idle is computed from task lifecycle rather than being an independent signal.

## 2. Frame schema (envelope.go #304 byte-identical)

```
TaskStartedFrame  (6 字段): {type, task_id, agent_id, channel_id, subject, started_at}
TaskFinishedFrame (7 字段): {type, task_id, agent_id, channel_id, outcome, reason, finished_at}
```

Direction is locked to `plugin_to_server`. `bppEnvelopeWhitelist` has 11 frames (control 6 + data 5).

## 3. Validation Rules (`task_lifecycle.go::Validate*`)

- `TaskStartedFrame.Subject`: non-empty after `strings.TrimSpace`; empty → error code `bpp.task_subject_empty` (copy guard; do not fall back to a default value).
- `TaskFinishedFrame.Outcome`: must be in 3-enum `{completed, failed, cancelled}`; intermediate states (`partial`/`paused`/`pending`/`starting`) are rejected → `bpp.task_outcome_unknown`.
- When `outcome=='failed'`, `Reason` is required and must be in the AL-1a 6-value dictionary (api_key_invalid / quota_exceeded / network_unreachable / runtime_crashed / runtime_timeout / unknown); empty → `bpp.task_finished_no_reason`, outside dictionary → `bpp.task_reason_unknown`.
- When `outcome ∈ {completed, cancelled}`, `Reason` must be empty to prevent dictionary pollution.

## 4. Reason Dictionary Six-Test Lock

`validAL1aReasons` is byte-identical with the `internal/agent/state.go::Reason*` single source. **Changing it requires updating six test locks**: AL-1a #249 + AL-3 #305 + CV-4 #380 + AL-2a #454 + AL-1b #458 + AL-4 #387/#461 (BPP-2.2 is the seventh linked site; do not create another dictionary).

## 5. Related References

- spec brief: [`docs/implementation/modules/bpp-2-spec.md`](../../../implementation/modules/bpp-2-spec.md) §1 BPP-2.2
- acceptance: [`docs/qa/acceptance-templates/bpp-2.md`](../../../qa/acceptance-templates/bpp-2.md) §2
- Implementation: `internal/bpp/task_lifecycle.go` + `task_lifecycle_test.go` (8 tests)
