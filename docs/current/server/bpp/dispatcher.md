# BPP-2.1 `semantic_action` Dispatcher — implementation note

> BPP-2.1 (#485) · Phase 4 plugin-protocol main line · blueprint [`plugin-protocol.md`](../../../blueprint/current/plugin-protocol.md) §1.3 (semantic abstraction layer + 7 required v1 semantic actions) + protocol boundary "不允许 plugin 下穿语义层直调 REST".

## 1. Design

Plugin sends upstream `SemanticActionFrame` (BPP-1 envelope §2.2 landed in #304) → server-side `Dispatcher.Dispatch(frame, sess)` routes to the registered `ActionHandler` → executes the same side effects as the existing REST path (artifact create / message send / ...). Plugin does not call REST endpoints directly and does not bypass AP-0 RequirePermission.

## 2. 7 v1 op Allowlist

`internal/bpp/dispatcher.go::ValidSemanticOps` is byte-identical with the blueprint §1.3 literals:

```
create_artifact / update_artifact / reply_in_thread / mention_user /
request_agent_join / read_channel_history / read_artifact
```

Values outside the enum are rejected with error code `bpp.semantic_op_unknown` (same naming pattern as anchor.create_owner_only #360 / dm.workspace_not_supported #407).

### 2.1 BPP-3.2.1 Extension — `request_capability_grant` (7→8)

Blueprint `auth-permissions.md` §1.3 defines this as the main entry. After the plugin SDK receives a BPP-3.1 `permission_denied` frame, it uses this op to make the server write a system DM to the owner (reusing the existing DM-2 path + CM-onboarding `quick_action` JSON).

Handler: `internal/api/capability_grant.go::CapabilityGrantHandler`. Payload 5 fields `{agent_id, attempted_action, required_capability, current_scope, request_id}` are byte-identical with the BPP-3.1 frame body (cross-PR lock; changing this means changing five or more sites).

DM body literal lock: `"{agent_name} 想 {attempted_action} 但缺权限 {required_capability}"` (see `docs/qa/bpp-3.2-content-lock.md` §1).

quick_action JSON shape (content-lock §2): `{action, agent_id, capability, scope, request_id}` (action ∈ {grant, reject, snooze}; client UI renders the three buttons "授权/拒绝/稍后").

Capability must use the AP-1 `auth.Capabilities` 14-item constant allowlist; values outside the dictionary are rejected with error code `bpp.grant_capability_disallowed`. Grep check `GrantPermission.*Permission:.*"<literal>"` in `internal/api/` count==0 (same constraint as AP-1 reverse constraint #1).

## 3. ActionHandler interface seam

The `bpp` package has no `internal/api` dependency; the api package injects handlers during server boot by calling `Dispatcher.RegisterHandler(op, handler)`. This matches the ArtifactPusher / IterationStatePusher / AgentInvitationPusher pattern.

`SessionContext` carries the `AgentUserID` + `PluginID` authenticated during BPP-1 connect; each handler runs its own AP-0 RequirePermission check. Dispatcher only routes and does not bypass permissions.

## 4. Reverse Constraints (CI grep count==0)

- Dispatcher does not accept raw HTTP / `http.Client.Do` / REST URL concatenation — blueprint §1.3 protocol boundary literal.
- v2+ ops (blueprint §1.3 v2+ collaboration-intent list) are not in the v1 allowlist and cannot enter v1 by literal match.
- bpp package does not import internal/api — dependency inversion happens through the `ActionHandler` interface.

## 5. Related References

- spec brief: [`docs/implementation/modules/bpp-2-spec.md`](../../../implementation/modules/bpp-2-spec.md) §1 BPP-2.1
- acceptance: [`docs/qa/acceptance-templates/bpp-2.md`](../../../qa/acceptance-templates/bpp-2.md) §1
- content lock: [`docs/qa/bpp-2-content-lock.md`](../../../qa/bpp-2-content-lock.md) §1 ① 7 op allowlist
- Implementation: `internal/bpp/dispatcher.go` + `dispatcher_test.go` (10 tests)
