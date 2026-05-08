# BPP-3.1 spec brief — `permission_denied` BPP frame 推送 (server → plugin)

> 战马C · 2026-04-29 · ≤80 行 · Phase 4 plugin-protocol 第二段 (BPP-2 ✅ #485 之后, 接 AP-1 #493 留账闭环)
> 关联: 蓝图 [`auth-permissions.md`](../../blueprint/current/auth-permissions.md) §2 不变量 "Permission denied 走 BPP — 不靠 HTTP 错误码, 由协议层路由到 owner DM" + §4.1 字面 frame 字段 (`permission_denied` row: `attempted_action`, `required_capability`, `current_scope`, `reason`); [`plugin-protocol.md`](../../blueprint/current/plugin-protocol.md) §2.1 control plane (server→plugin); AP-1 #493 abac.go::HasCapability 403 body `{required_capability, current_scope}` byte-identical 同源 (跨 PR drift 防御)
> Owner: 战马C 一 milestone 一 PR

---

## 1. 范围 (3 立场)

### 立场 ① — frame schema 单源, 不裂 envelope
- BPP envelope whitelist 12→13 扩 (BPP-1 9 + AL-2b ack +1 + BPP-2.2 task ×2 + **BPP-3.1 permission_denied +1 = 13**)
- 不另起 control-plane channel — 复用 `bppEnvelopeWhitelist` + reflect lint reflect 自动覆盖
- 字段顺序锁 byte-identical 跟蓝图 §4.1 row + AP-1 abac.go 403 body: `{type, cursor, agent_id, request_id, attempted_action, required_capability, current_scope, denied_at}`

### 立场 ② — direction lock server→plugin (反约束)
- `permission_denied` 是 server 通知 plugin 的 deny 信号 — direction = `DirectionServerToPlugin`
- 反约束: plugin 永不发 `permission_denied` (反向断言: `bppEnvelopeWhitelist` direction map + reflect lint + reverse grep 守 plugin→server direction 无此 frame type)
- 跟 ADM-0 §1.3 "admin god-mode 不入业务路径" 同精神 — admin 不消费 deny frame (admin 走 /admin-api/*)

### 立场 ③ — payload 字段跟 AP-1 abac.go 403 body byte-identical (跨 PR drift 防御)
- 字段名锁: `required_capability` + `current_scope` byte-identical 跟 AP-1 abac.go 403 body (AP-1 #493 `internal/api/artifacts.go::handleCommit` 已落)
- 双向 grep CI lint 守 future drift: 改 `required_capability` = 改三处 (蓝图 §4.1 + AP-1 abac.go body + BPP-3.1 frame field)
- `attempted_action` ∈ BPP-2.1 7 op 白名单 (复用 `ValidSemanticOps`, 反约束: 'list_users' 等枚举外值 reject)
- `request_id`: AP-1 调用方 (commits handler 等) 生成的 trace UUID, plugin 端按此 key 关联 owner DM 推审批通知 + retry

---

## 2. 反约束 (5 grep 锁, count==0)

```bash
# 1) frame 字段名 drift (改 required_capability/current_scope = 改三处)
git grep -nE 'permission_denied.*required_capability|required_capability.*permission_denied' \
  packages/server-go/internal/{bpp,ws,api}/  # 期望 ≥3 hit (frame + push + AP-1 abac.go body)

# 2) plugin→server direction 走 permission_denied (反约束)
git grep -nE 'DirectionPluginToServer.*permission_denied|permission_denied.*DirectionPluginToServer' \
  packages/server-go/internal/bpp/  # 0 hit

# 3) admin god-mode 推 deny (反约束: admin 不入业务)
git grep -nE 'admin.*permission_denied|permission_denied.*admin' \
  packages/server-go/internal/{bpp,ws}/  # 0 hit

# 4) raw HTTP 401/403 旁路 (反约束: deny 走 BPP frame 不走 HTTP error)
git grep -nE 'permission_denied.*StatusForbidden|StatusForbidden.*permission_denied' \
  packages/server-go/internal/api/  # 0 hit (HTTP 403 是 fallback, frame 是 primary)

# 5) attempted_action 字面 hardcode (走 BPP-2.1 op const 单源)
git grep -nE 'attempted_action.*"[a-z_]+"' packages/server-go/internal/  # 0 hit (走 SemanticOp* const)
```

---

## 3. 文件清单 (≤6 文件)

| 文件 | 范围 |
|---|---|
| `internal/bpp/envelope.go` | 加 `PermissionDeniedFrame` 8 字段 + `FrameTypeBPPPermissionDenied` const + whitelist 12→13 |
| `internal/ws/permission_denied_frame.go` | hub 方法 `PushPermissionDenied(agentID, requestID, attemptedAction, requiredCapability, currentScope, deniedAt)` (跟 PushAgentConfigUpdate 同模式) + `PermissionDeniedPusher` interface seam (api 包不 import ws) |
| `internal/bpp/frame_schemas_test.go` | count 12→13 + control 6+1 / data plane 6 + direction lock + reflect 自动覆盖新 frame |
| `internal/ws/permission_denied_frame_test.go` | 5 unit (frame schema lock / direction enforce / hub push 路径 / nil-safe / cursor 共序) |
| `docs/qa/acceptance-templates/bpp-3.1.md` | 烈马 acceptance ≤30 行 + 5 反约束 grep + REG-BPP31-001..N |
| `docs/qa/regression-registry.md` | REG-BPP31-001..N + §5 总计 sync + §6 changelog |

---

## 4. 验收挂钩

- AP-1 #493 留账闭环: 蓝图 §2 "Permission denied 走 BPP" 不变量真落 frame 通道
- BPP-1 envelope CI lint reflect 自动扫 13 frame whitelist 全过
- 跨 PR drift 守: AP-1 abac.go body 字段 + 蓝图 §4.1 row + BPP-3.1 frame 字段 三处 byte-identical (双向 grep)
- e2e: agent 触发 commit_artifact 无权 → AP-1 HasCapability false → server 调 `PushPermissionDenied` → plugin 端收 frame (本 PR test 用 fake plugin conn 收, 真 plugin 集成留 follow-up)

---

## 5. 不在范围 (留账)

- AP-1 abac.go::HasCapability false 路径 → server 调 PushPermissionDenied 真 wire (AP-1 #493 未 merge, deferred wiring via interface seam — AP-1 merge 后 1-line follow-up commit 接)
- plugin 端收 frame → owner DM 推审批通知 + 一键 grant UI (BPP-3.2 Phase 5)
- retry 提示 owner 加权后 plugin 自动重试 (BPP-3.2 跟 owner DM 同期落)
- cross-org owner-only AP-3 强制 (后续 milestone)

---

## 6. 跨 milestone byte-identical 锁

- 跟 BPP-1 #304 envelope CI lint reflect 同模式 (frame_schemas_test.go count + direction lock 自动覆盖)
- 跟 AL-2b #481 PushAgentConfigUpdate hub method 同模式 (cursor 共序 + direction lock + plugin 离线 frame 丢)
- 跟 AP-1 #493 abac.go 403 body 字面双向 grep 同源 (改一处 = 改三处)

---
