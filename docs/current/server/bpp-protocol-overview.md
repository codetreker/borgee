# BPP-1 Protocol Overview — implementation note

> 战马 A · Phase 4 BPP-1 接活前 30 秒速读卡, 给战马B / 飞马参考. 蓝图见 [`docs/blueprint/bpp-protocol.md`](../../blueprint/bpp-protocol.md) (R3-4 已敲定), 现状骨架已落 RT-0 子集 (PR #237).

**协议范围** — agent runtime ↔ server 实时事件. frame 跟现 RT-0 同级别 byte-identical, 不再走 REST polling.

**已落 (RT-0 子集, PR #237)**:
- `agent_invitation_pending` — server → owner client, 邀请待决.
- `agent_invitation_decided` — server → owner + invitee, 决议落地 (accept/decline).

**Phase 4 加 (BPP-1 主体)**:
- `session.resume` / `session.resume_ack` — RT-1.3 (#293), runtime 重连后的 replay 握手. 三 mode `incremental` (default) / `none` (cold start) / `full` (agent 显式), server **不 default full** (反向约束). 详见 [`bpp/session-resume.md`](./bpp/session-resume.md).
- `agent_runtime_state` — 复用 #249 enum (`online/offline/error`) + 6 reason codes; 替代当前 GET 内联 `state` 字段的 poll 模式.
- `agent_config_update` / `agent_config_ack` — **AL-2b** (#452 acceptance + 本 PR 实施), owner 改 agent 配置后 server → plugin 推送 + plugin → server ack 路径. AgentConfigUpdateFrame 7 字段 `{type, cursor, agent_id, schema_version, blob, idempotency_key, created_at}` server→plugin direction 锁定 (蓝图 §1.5 热更新 + §2.1 控制面). AgentConfigAckFrame 7 字段 `{type, cursor, agent_id, schema_version, status, reason, applied_at}` plugin→server direction 锁定 + status CHECK ('applied','rejected','stale') fail-closed (反向约束 reject 'unknown'/同义词脱节). cursor 走 hub.cursors 单调跟 RT-1/CV-2/DM-2/CV-4 5-frame 共 sequence (反向约束: 不另起 plugin-only 通道). idempotency_key 蓝图 §1.5 字面 "幂等 reload" — 同 key 重发 reload 仅 1 次. 单一来源反向约束 (跟 AL-2a #447 同源): blob 不含 api_key/temperature/token_limit/retry_policy runtime-only 字段.
- `agent_busy_started` / `agent_idle_started` — AL-1b, runtime → server, 触发 #249 deferred 的 busy/idle 子态 (4 人 review #5 决议: 没 BPP 不准 stub).
- `agent_disable` / `agent_resume` — AL-4, owner 操作 → server → runtime, runtime 收到 disable 立即停接消息 (蓝图 §2.4).

**协议要点**:
- WebSocket 单向 server→client (`/ws` 已有, `/ws/plugin` runtime 端).
- Reverse channel: client→server 走 `POST /ws/upstream` (REST shim, BPP-2 升 WS bidirectional).
- frame schema 锁定: byte-identical between blueprint §schemas + `internal/ws/event_schemas.go` + `packages/client/src/types/ws-frames.ts`. CI lint G2.6 (Phase 4 加) catch drift.

**不带 migration** — BPP 是协议层, 不动 schema. AL-3 落表是分开的 task (state 持久化 hook 已在 #249 Tracker 接口形参化).

**接活动作清单 (Phase 4 战马A/B)**:
1. 蓝图 §schemas 字面拷到 `event_schemas.go` const, 加 lint vet.
2. `ws.Hub` 加 frame dispatch (按 `type` 字段), 别 ad-hoc map[string]any.
3. client `types/ws-frames.ts` mirror, 加 vitest snapshot 锁定.
4. 三处同 PR (server schema + client schema + 蓝图行号), 飞马 byte-diff 卡通过即可。
