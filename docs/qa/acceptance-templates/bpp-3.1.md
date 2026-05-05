# Acceptance Template — BPP-3.1: `permission_denied` BPP frame 推送 (server→plugin)

> Spec: `docs/implementation/modules/bpp-3.1-spec.md` (战马C v0)
> 蓝图: `docs/blueprint/current/auth-permissions.md` §2 不变量 "Permission denied 走 BPP" + §4.1 row 字面 frame 字段; `docs/blueprint/current/plugin-protocol.md` §2.1 control plane (server→plugin)
> 前置: BPP-1 #304 envelope CI lint ✅ + BPP-2 #485 dispatcher + task lifecycle + agent_config_update ✅
> 关联: AP-1 #493 abac.go::HasCapability false 路径 deferred wiring (interface seam, AP-1 merge 后 1-line follow-up)
> Owner: 战马C 一 milestone 一 PR

## 验收清单

### 立场 ① — frame schema 单源

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 envelope 12→13 扩 (BPP-1 9 + AL-2b ack +1 + BPP-2.2 task +2 + BPP-3.1 permission_denied +1) | BPP-1 reflect lint | 战马C / 烈马 | `internal/bpp/frame_schemas_test.go::TestBPPEnvelopeFrameWhitelist` count 12→13 + control 6→7 + AllBPPEnvelopes 加 PermissionDeniedFrame{} |
| 1.2 PermissionDeniedFrame 8 字段 byte-identical 跟 spec §1 立场 ③ + 蓝图 §4.1 row + AP-1 abac.go 403 body | unit | 战马C / 烈马 | `internal/bpp/envelope.go` PermissionDeniedFrame 字段定义 + json tag 字面锁 + `permission_denied_frame_test.go::TestBPP31_PushPermissionDenied_Basic` round-trip |
| 1.3 cursor 共序 — BPP-3.1 push 跟 RT-1 + AL-2b 共一根 sequence (反约束: 不另起 plugin-only 通道) | unit | 战马C / 烈马 | `TestBPP31_PushPermissionDenied_SharedSequence` (RT-1 c1 < BPP-3.1 c2 < AL-2b c3 三连递增) |

### 立场 ② — direction lock server→plugin

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 `bppEnvelopeWhitelist[FrameTypeBPPPermissionDenied] == DirectionServerToPlugin` | unit | 战马C / 烈马 | `TestBPP31_DirectionLock_ServerToPlugin` (whitelist + 实例方法双闸断) |
| 2.2 plugin offline → sent=false, cursor still allocated (frame 丢, sequence 不留洞) | unit | 战马C / 烈马 | `TestBPP31_PushPermissionDenied_PluginOffline` |
| 2.3 PermissionDeniedPusher interface seam — *Hub 实现 (api 包通过此接口注入, AP-1 wiring 不 import ws) | unit | 战马C / 烈马 | `TestBPP31_HubImplementsPermissionDeniedPusher` (var _ ws.PermissionDeniedPusher = hub 编译期断) |

### 立场 ③ — payload byte-identical 跟 AP-1 abac.go 403 body

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 字段名 `required_capability` + `current_scope` byte-identical 跟 AP-1 abac.go 403 body (跨 PR drift 守) | unit | 战马C / 烈马 | `envelope.go::PermissionDeniedFrame` json tag 字面锁 + `TestBPP31_PushPermissionDenied_Basic` wire JSON byte-identical |
| 3.2 8 字段全序列化 (反 omitempty) — zero-tail 仍 8 keys | unit | 战马C / 烈马 | `TestBPP31_PushPermissionDenied_FieldByteIdentity` (filled + zero-tail 双 snapshot, ':' count ≥ 8) |

### 反约束 grep (spec §2)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 plugin→server direction 无 permission_denied (反约束) | reflect lint | 烈马 | `frame_schemas_test.go::TestBPPEnvelopeDirectionLock` data plane count 6 不变 (BPP-3.1 不进 data plane) |
| 4.2 admin god-mode 不消费此 frame (反约束 ADM-0 §1.3) | docstring 立场反查 + future grep | 烈马 | `permission_denied_frame.go` 包级 docstring 立场反查 §; ADM-0.2 admin RequireAdmin mw 不动 |
| 4.3 (留 follow-up) attempted_action 字面 hardcode / HTTP 403 旁路 / drift 双向 grep | follow-up CI lint | 烈马 / 飞马 | spec §2 #1+#4+#5 反向 grep 留 follow-up patch (野马 stance checklist + AP-1 merge 后双向 grep 落) |

## 不在本轮范围 (spec §5)

- AP-1 abac.go::HasCapability false 路径 → server 调 PushPermissionDenied 真 wire (interface seam ready, AP-1 #493 merge 后 1-line follow-up commit 接)
- plugin 端收 frame → owner DM 推审批通知 + 一键 grant UI → BPP-3.2 (Phase 5)
- retry 提示 owner 加权后 plugin 自动重试 → BPP-3.2 跟 owner DM 同期
- cross-org owner-only AP-3 强制 → 后续 milestone

## 退出条件

- 立场 ① 1.1-1.3 (envelope 12→13 + frame 8 字段 byte-identical + cursor 共序) ✅
- 立场 ② 2.1-2.3 (direction lock + offline 不留洞 + interface seam) ✅
- 立场 ③ 3.1-3.2 (payload byte-identical 跟 AP-1 + 反 omitempty) ✅
- 反约束 4.1-4.2 (direction reflect 双闸 + docstring 立场反查) ✅
- 现网回归不破: 全套 server test 18s PASS (含 BPP-1 reflect lint count 12→13)
- REG-BPP31-001..006 共 **6 行 🟢**

## 更新日志

- 2026-04-29 — 战马C v0 一 milestone 一 PR: PermissionDeniedFrame 8 字段 byte-identical 跟蓝图 §4.1 + AP-1 abac.go 403 body; envelope whitelist 12→13; PushPermissionDenied hub method (跟 PushAgentConfigUpdate 同模式) + PermissionDeniedPusher interface seam (api 包不 import ws); 6 unit + frame_schemas_test count 自动覆盖; AP-1 wiring deferred follow-up. 全套 server test 18s PASS.
