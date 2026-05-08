# Implementation · BPP-1 (Borgee Plugin Protocol Phase 3 启动)

> 蓝图: [`../../blueprint/current/plugin-protocol.md`](../../blueprint/current/plugin-protocol.md) · [`../../blueprint/current/realtime.md §2.3`](../../blueprint/current/realtime.md) · [`../../blueprint/_meta/r3-decisions.md §R3-4`](../../blueprint/_meta/r3-decisions.md)
> 现状: Phase 2 RT-0 (#218 client + #237 server) 已落 `/ws` hub + 2 frame (invitation_pending/decided), 字段顺序 byte-identical 锁
> 阶段: ⚡ v0 (Phase 3 解封后启动) · 所属 Phase: BPP-1.1~1.3 在 Phase 3; AL-1b 跟 BPP-1.2 同期 (owner 在 agent-lifecycle module)

## 1. 现状 → 目标 概览

**现状**: 单向 server→client push; `internal/ws/event_schemas.go` 锁 2 frame; TS 镜像 `packages/client/src/types/ws-frames.ts` 字段顺序 + JSON tag = TS field name 字面 byte-identical 由 CI 反向锁; cursor replay 兜底断线.
**目标**: BPP 协议骨架 (双向) + reverse channel (plugin→server frame) + reconnect/resume; cutover 时 server 发送源 `hub.Broadcast` → `bpp.SendFrame`, **client handler 0 改** (schema 等同).
**差距**: 单向→双向; hub-only→BPP frame envelope; 无 session resume → `session.resume` + replay_mode.

## 2. Milestones

### BPP-1.1: 协议骨架 + frame schema lock

- **目标**: plugin-protocol §2 — BPP frame envelope (type/version/payload) + `bpp/frame_schemas.go` byte-identical = `internal/ws/event_schemas.go`.
- **Owner**: 飞马 (schema review) / 战马A / 烈马
- **范围**: `internal/bpp/` 新 package; frame envelope; CI lint reverse-grep 锁两端字面相等; G2.6 留账行翻牌
- **依赖**: Phase 2 全过 + R4 输出锁 · **预估**: 5-7 天
- **Acceptance**: CI lint 翻牌 (改 ws schema 而 BPP 没同步 = CI 红); 2 frame 端到端走 BPP

### BPP-1.2: Reverse channel (plugin → server)

- **目标**: realtime §2.1 + plugin-protocol §2 — plugin 上行 `task_started` / `task_finished` / `progress(subject)`.
- **Owner**: 战马A / 飞马 / 野马 (subject 文案)
- **范围**: BPP endpoint accept plugin frame; `Hub.OnPluginFrame` adapter; subject 强制非空 (空则拒收 + 不渲染)
- **依赖**: BPP-1.1; AL-1b 同期 (busy/idle source = `task_started`/`task_finished`) · **预估**: 5-7 天
- **Acceptance**: agent 跑任务 → owner 看 busy + subject; 任务结束 → idle; subject 缺失 → 拒收 + log warn

### BPP-1.3: Reconnect / session resume

- **目标**: realtime §1+§2 — `session.resume(cursor, replay_mode)`; replay_mode = `human` (全) / `agent` (子集).
- **Owner**: 飞马 / 战马A / 烈马
- **范围**: events 表 cursor 接 BPP resume; replay_mode 路由; 断线 ≤ 30s 重连无丢 frame
- **依赖**: BPP-1.1 + BPP-1.2 · **预估**: 4-5 天
- **Acceptance**: 断线 30s 重连 → 0 丢; replay_mode=agent → 收子集 (规则 plugin-protocol §2 锁)

## 3. 不在 BPP-1 范围

- `agent_config_update` BPP 接口 → BPP-2 · 加密 / mTLS plugin auth → host-bridge module Phase 4+ · BPP 版本协商 → Phase 4+ · 多 runtime (Hermes) → Phase 5+; v1 只 OpenClaw reference impl

## 4. Blueprint 反查表

| Milestone | §X.Y | 立场一句话 |
|-----------|------|-----------|
| BPP-1.1 | plugin-protocol §2 + r3-decisions §R3-4 | BPP frame schema = /ws frame 字面 byte-identical, cutover client 0 改 |
| BPP-1.2 | realtime §2.1 + plugin-protocol §2 | reverse channel; progress 必带 subject 否则不渲染; busy/idle source = task_*/frame |
| BPP-1.3 | realtime §1+§2 + plugin-protocol §2 | session.resume + replay_mode hint (human 全 / agent 子集) |
| AL-1b 同期 | agent-lifecycle §2.3 | busy/idle 跟 BPP-1.2 同期, source 必须 plugin 上行 frame, 不准 stub |
