# BPP-2.1 `semantic_action` Dispatcher — implementation note

> BPP-2.1 (#485) · Phase 4 plugin-protocol 主线 · 蓝图 [`plugin-protocol.md`](../../../blueprint/current/plugin-protocol.md) §1.3 (抽象语义层 + 7 v1 必须语义动作) + 协议红线 "不允许 plugin 下穿语义层直调 REST".

## 1. 设计

Plugin 上行 `SemanticActionFrame` (BPP-1 envelope §2.2 已落 #304) → server-side `Dispatcher.Dispatch(frame, sess)` 路由到注册的 `ActionHandler` → 执行既有 REST 同等副作用 (artifact create / message send / ...). Plugin 不直对 REST endpoint, 不绕 AP-0 RequirePermission.

## 2. 7 v1 op 白名单

`internal/bpp/dispatcher.go::ValidSemanticOps` byte-identical 跟蓝图 §1.3 字面:

```
create_artifact / update_artifact / reply_in_thread / mention_user /
request_agent_join / read_channel_history / read_artifact
```

枚举外值 reject + 错误码 `bpp.semantic_op_unknown` (跟 anchor.create_owner_only #360 / dm.workspace_not_supported #407 命名同模式).

### 2.1 BPP-3.2.1 扩展 — `request_capability_grant` (7→8)

蓝图 `auth-permissions.md` §1.3 主入口字面对接. plugin SDK 收 BPP-3.1 `permission_denied` frame 后通过此 op 触发 server 给 owner 写 system DM (复用 DM-2 既有 path + CM-onboarding `quick_action` JSON).

Handler: `internal/api/capability_grant.go::CapabilityGrantHandler`. Payload 5 字段 `{agent_id, attempted_action, required_capability, current_scope, request_id}` byte-identical 跟 BPP-3.1 frame body 同源 (跨 PR drift 守, 改 = 改五处+).

DM body 字面锁: `"{agent_name} 想 {attempted_action} 但缺权限 {required_capability}"` (见 `docs/qa/bpp-3.2-content-lock.md` §1).

quick_action JSON shape (content-lock §2): `{action, agent_id, capability, scope, request_id}` (action ∈ {grant, reject, snooze}; client UI 渲染三按钮 "授权/拒绝/稍后").

Capability 必走 AP-1 `auth.Capabilities` 14 项 const 白名单; 字典外值 reject + 错误码 `bpp.grant_capability_disallowed`. grep 检查 `GrantPermission.*Permission:.*"<literal>"` 在 `internal/api/` count==0 (跟 AP-1 反约束 #1 同源).

## 3. ActionHandler interface seam

`bpp` 包零 `internal/api` 依赖 — api 包 server boot 时调 `Dispatcher.RegisterHandler(op, handler)` 注入. 跟 ArtifactPusher / IterationStatePusher / AgentInvitationPusher 同模式.

`SessionContext` 携带 BPP-1 connect 时已认证的 `AgentUserID` + `PluginID`; AP-0 RequirePermission 由 handler 自行调闸 — dispatcher 只路由不绕权限.

## 4. 反约束 (grep 守门 CI lint count==0)

- Dispatcher 不接 raw HTTP / `http.Client.Do` / REST URL 拼接 — 蓝图 §1.3 协议红线字面.
- v2+ ops (蓝图 §1.3 v2+ 协作意图列表) 不在 v1 白名单, 字面禁 v1 进.
- bpp 包不 import internal/api — 依赖反转 via `ActionHandler` interface.

## 5. 相关参考

- spec brief: [`docs/implementation/modules/bpp-2-spec.md`](../../../implementation/modules/bpp-2-spec.md) §1 BPP-2.1
- acceptance: [`docs/qa/acceptance-templates/bpp-2.md`](../../../qa/acceptance-templates/bpp-2.md) §1
- content lock: [`docs/qa/bpp-2-content-lock.md`](../../../qa/bpp-2-content-lock.md) §1 ① 7 op 白名单
- 实施: `internal/bpp/dispatcher.go` + `dispatcher_test.go` (10 tests)
