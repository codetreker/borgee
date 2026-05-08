# AL-2b.2 server hook spec lite — PATCH agent_configs → push agent_config_update + AgentConfigAck handler

> 烈马 · 2026-04-29 · ≤100 行 spec lite (跟 #465 AL-2b spec brief 立场承袭, server hook 实施细化, 给 zhanma-c AL-2b.1 frame + 烈马 AL-2b.2 hook 起手锚)
> **依赖**: AL-2a.1+.2+.3 #447 (agent_configs 表 v=20 + PATCH /api/v1/agents/:id/config endpoint) ⏳ 待 merge / AL-2b.1 frame `agent_config_update_frame.go` + `agent_config_ack_frame.go` ⏳ 待 zhanma-c 实施
> **关联**: 跟 #465 AL-2b spec brief v0 + #452 acceptance v1 同源; 跟 cv-4.2 #409 IterationStateChanged push hook 同模式; BPP-1 ✅ #304 envelope CI lint reflect 自动覆盖

## 0. 实施 hook 路径 (跟 #465 立场 ①②③ 承袭)

PATCH `/api/v1/agents/:id/config` 成功路径 (AL-2a.2 #447 server) 加 fanout 步骤:

```
1. validate body + owner ACL (AL-2a.2 既有, 不动)
2. UPDATE agent_configs SET blob, schema_version+=1 (AL-2a.2 既有, 不动)
3. NEW: hub.PushAgentConfigUpdate(agentID, schemaVersion, blob, idempotencyKey, time.Now()) — 单推目标 plugin
4. NEW: 异步等 AgentConfigAck (best-effort, 不 block PATCH response, plugin 重连后 cursor replay 兜底)
5. response 200 跟 AL-2a.2 既有 byte-identical
```

跟 cv-4.2 #409 同模式: handler 调 hub.cursors.NextCursor() 单调发号 → BroadcastToUser(plugin_id) 单推 → SignalNewEvents 唤醒 long-poll.

## 1. 实施拆段 (单 PR, 3 文件改 + 1 测试新)

| 文件 | 范围 | 行数估 |
|---|---|---|
| `internal/api/agent_config.go` (改, AL-2a.2 既有) | PATCH handler 加 hub.PushAgentConfigUpdate 调用 (位置: UPDATE 成功后, response.Write 前); idempotencyKey 走 server 端 UUID 生成 (cmd/server crypto/rand 或 google/uuid 既有依赖); 失败仅 log+continue (best-effort, 跟 DM-2.2 #372 mention dispatch 失败不阻 message 落库同模式) | +15 |
| `internal/ws/hub.go` (改) | wire `PushAgentConfigUpdate` 接 `internal/bpp/envelope.go` AgentConfigUpdateFrame (跟 #472 真实落点 byte-identical sync — frame 在 bpp 包不在 ws); Hub 推 BPP frame 需新桥 (BPP frame 不实现 ws.Frame interface, ws.Hub ↔ BPP envelope routing) | +20 |
| `internal/bpp/agent_config_ack_dispatcher.go` (新, 跟 #472 frame 落点同包就近) | dispatcher case AgentConfigAckFrame.FrameType() == FrameTypeBPPAgentConfigAck → 校验 cross-owner (REG-INV-002 fail-closed) → 校验 schema_version 匹配 server 当前 (mismatch → 不写 ack 表, 仅 log warn) → status='applied' UPDATE agent_configs.last_applied_at (新列? 或 audit_log 行); admin god-mode 不下发反断 (走 dispatcher 入站层反向 grep 0 hit) | +80 |
| `internal/bpp/al_2b_2_server_hook_test.go` (新, 跟 #472 `al_2b_frames_test.go` 落点同包) | 5 test 跟派活字面: TestAL2B2_PushOnPatch (PATCH 成功 → frame 单推 plugin) / TestAL2B2_AckHandler (plugin reply ack → server 标记 applied) / TestAL2B2_StaleSchemaVersionReject (mismatch reject) / TestAL2B2_CrossOwnerReject (REG-INV-002) / TestAL2B2_RuntimeOnlyReject (blob 含 api_key/temperature 等 fail-closed) | +200 |

**owner**: 烈马 (实施 server hook + ack dispatcher) / zhanma-c (AL-2b.1 frame.go 字段定义) / 战马 (跟 BPP-3 同 PR 合规划承袭 #465 §1)

## 2. fail-closed 反约束 (5 处守门)

跟 #452 acceptance v1 §2.5 + §3.2 + §4 反向 grep byte-identical:

1. **schema_version mismatch** — ack `schema_version != server 当前` → 不更新 agent_configs.last_applied_at (跟 #452 §2.3 字面 "stale 不缓存" 同源)
2. **cross-owner reject** — frame 入站 agent_id 跟当前 plugin 的 owner 不匹配 → server-side 拒不写库 (REG-INV-002 fail-closed 扫描器复用 + 跟 CHN-1 channel-scoped ACL 同模式)
3. **runtime-only field reject** — PATCH body blob 含 api_key / temperature / token_limit / retry_policy → 400 `agent_config.runtime_field_rejected` (跟 #447 既有 NoDomainBleed reject 路径承袭, AL-2a.2 已实施)
4. **admin god-mode 不下发** — admin token 调 PATCH → 403 (AL-2a.2 既有 owner-only ACL); admin god-mode 不进 dispatcher fanout 路径 (反向 grep `admin.*PushAgentConfigUpdate` 0 hit, 跟 ADM-0 §1.3 红线 + AL-3 #303 ⑦ 同模式)
5. **idempotency_key 重发幂等** — 同 idempotency_key 重发 N 次 → server 不去重发 (plugin 端去重接收, 跟 #452 §2.2 字面承袭) — server hook 每次 PATCH 走新 UUID, 不复用旧 key

## 3. 测试设计 (跟派活字面 5 test 一一对应)

| Test | 验证 | 跟 acceptance 行 |
|---|---|---|
| TestAL2B2_PushOnPatch | PATCH 200 后 mockHub.Pushes count==1, frame 7 字段 byte-identical | #452 §2.1 delivery latency |
| TestAL2B2_AckHandler | plugin reply ack 'applied' → agent_configs.last_applied_at 更新 | #452 §2.2 idempotency_key 幂等 |
| TestAL2B2_StaleSchemaVersionReject | ack schema_version=N, server 当前=N+1 → 不更新 last_applied_at + log warn | #452 §2.3 stale 不缓存 |
| TestAL2B2_CrossOwnerReject | frame agent_id 跨 owner → dispatcher 拒不进 handler | #452 §2.5 cross-owner |
| TestAL2B2_RuntimeOnlyReject | PATCH body blob 含 api_key → 400 agent_config.runtime_field_rejected | #452 §3.2 SSOT 字段反断 (跟 #447 NoDomainBleed 同源) |

## 4. 跟 cv-4.2 #409 push hook 同模式 (实施参考锚)

cv-4.2 #409 IterationStateChanged hook 路径 (实施时 mirror):
```go
// internal/api/iterations.go:317-325 既有模式
cursor, sent := h.hub.PushIterationStateChanged(...)
if !sent {
    h.logger.Warn("iteration push skipped", ...)
}
```

AL-2b.2 server hook mirror:
```go
// internal/api/agent_config.go (AL-2a.2 既有 PATCH handler 加)
cursor, sent := h.hub.PushAgentConfigUpdate(agentID, schemaVersion, blob, idempotencyKey, time.Now().UnixMilli())
if !sent {
    h.logger.Warn("agent_config push skipped — plugin not online", "agent_id", agentID)
}
```

## 5. 不在本 spec 范围 (反约束承袭 #465 §4)

- ❌ 实施 frame 字段定义 (zhanma-c 职责, AL-2b.1)
- ❌ ack/retry 机制 (best-effort, plugin 重连 cursor replay 兜底)
- ❌ AL-2a 轮询路径下线 (drift 防双轨, 留 AL-2b.2 实施 PR 真去除)
- ❌ multi-agent batch push (一 frame = 一 agent_id)
