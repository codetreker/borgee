# BPP-3.1 permission_denied frame (server→plugin) — 烈马 (QA acceptance) signoff

> **状态**: ✅ **SIGNED** (烈马 acceptance 代签, 2026-04-29, post-#494 9aab11a)
> **范围**: BPP-3.1 milestone — Phase 4 plugin-protocol 第二段 — `PermissionDeniedFrame` (server→plugin) 8 字段 byte-identical 跟蓝图 §4.1 row + AP-1 abac.go 403 body 跨 PR drift 守; envelope 12→13 扩 (BPP-1 reflect lint 自动覆盖, control 6→7); PushPermissionDenied hub method + PermissionDeniedPusher interface seam (api 包不 import ws, AP-1 wiring 1-line follow-up)
> **关联**: BPP-3.1 #494 (zhanma-c 9aab11a) 整 milestone 一 PR (跟 ADM-2 #484 + BPP-2 #485 + AL-1 #492 + AL-2a #480 + AL-2b #481 + AL-1b #482 + CM-5 #476 + AP-1 #493 同模式); 前置: BPP-1 #304 envelope CI lint ✅ + BPP-2 #485 dispatcher + task lifecycle + agent_config_update ✅ + BPP-3 #489 plugin 上行 dispatcher ✅; AP-1 #493 abac.go::HasCapability false 路径 deferred wiring (interface seam ready, AP-1 merge 后 1-line follow-up); REG-BPP31-001..006 6 🟢; 全套 server test 18s PASS (含 BPP-1 reflect lint count 12→13)
> **方法**: 跟 #403 G3.3 + #449 G3.1+G3.2+G3.4 + #459 G4.1 + G4.2 + G4.3 + G4.4 + G4.5 + AL-1 + AP-1 烈马代签机制承袭 — 真单测实施证据 + 立场反查 + acceptance template 闭锁 + 跨 milestone byte-identical 链承袭 + 烈马代签 (BPP-3.1 工程内部 plugin-protocol 第二段, 跟 cm-4 / adm-0 / adm-2 / al-1 / ap-1 deferred 同模式不进野马 G4 流)

---

## 1. 验收清单 (烈马 acceptance 视角 5 项, 跟 acceptance bpp-3.1.md 6 项 ✅ byte-identical, 三立场对照)

| # | 验收项 (立场对照) | 立场锚 | 结果 | 实施证据 (PR/SHA + 测试名 byte-identical) |
|---|--------|--------|------|------|
| ① | **立场 ① frame schema 单源 — envelope 12→13 扩** (BPP-1 9 + AL-2b ack +1 + BPP-2.2 task +2 + BPP-3.1 permission_denied +1) + PermissionDeniedFrame 8 字段 byte-identical 跟蓝图 §4.1 row + AP-1 abac.go 403 body (`type/cursor/agent_id/request_id/attempted_action/required_capability/current_scope/denied_at`) + cursor 共序 (BPP-3.1 push 跟 RT-1+AL-2b 共一根 sequence, 反约束: 不另起 plugin-only 通道) | spec §1 立场 ① + 蓝图 §4.1 row + RT-1 立场 cursor 唯一可信序 + BPP-1 #304 reflect lint | ✅ pass | `internal/bpp/frame_schemas_test.go::TestBPPEnvelopeFrameWhitelist` count 12→13 + control 6→7 + AllBPPEnvelopes 加 PermissionDeniedFrame{} + `internal/bpp/envelope.go` PermissionDeniedFrame 字段定义 + json tag 字面锁 + `permission_denied_frame_test.go::TestBPP31_PushPermissionDenied_Basic` round-trip + `TestBPP31_PushPermissionDenied_SharedSequence` (RT-1 c1 < BPP-3.1 c2 < AL-2b c3 三连递增) PASS — REG-BPP31-001 + 003 + 004 + acceptance §1.1+§1.2+§1.3 (3 项) |
| ② | **立场 ② direction lock server→plugin** — `bppEnvelopeWhitelist[FrameTypeBPPPermissionDenied] == DirectionServerToPlugin` + plugin offline → sent=false, cursor still allocated (frame 丢, sequence 不留洞, 跟 PushAgentConfigUpdate 同模式) + PermissionDeniedPusher interface seam (*Hub 实现, api 包通过此接口注入, AP-1 wiring 不 import ws — 跟 ArtifactPusher / IterationStatePusher / AgentConfigPusher / ActionHandler 同精神依赖反转) | spec §1 立场 ② + plugin-protocol §2.1 control plane (server→plugin) + 接口 seam 依赖反转跨 milestone 同精神 | ✅ pass | `TestBPP31_DirectionLock_ServerToPlugin` (whitelist + 实例方法双闸断) + `TestBPP31_PushPermissionDenied_PluginOffline` (sent=false, cursor allocated 不留洞) + `TestBPP31_HubImplementsPermissionDeniedPusher` (var _ ws.PermissionDeniedPusher = hub 编译期断) PASS — REG-BPP31-002 + 003 + 006 + acceptance §2.1+§2.2+§2.3 (3 项) |
| ③ | **立场 ③ payload byte-identical 跟 AP-1 abac.go 403 body** — 字段名 `required_capability` + `current_scope` byte-identical 跟 AP-1 abac.go 403 body (跨 PR drift 守, 改 = 改三处: BPP-3.1 envelope.go + AP-1 abac.go + 蓝图 §4.1 row) + 8 字段全序列化 (反 omitempty) — zero-tail 仍 8 keys + ':' count ≥ 8 双 snapshot | spec §1 立场 ③ + AP-1 acceptance §5.1 BPP 路由字段 + 跨 PR byte-identical 链锚 | ✅ pass | `envelope.go::PermissionDeniedFrame` json tag 字面锁 + `TestBPP31_PushPermissionDenied_Basic` wire JSON byte-identical + `TestBPP31_PushPermissionDenied_FieldByteIdentity` (filled + zero-tail 双 snapshot, ':' count ≥ 8) PASS — REG-BPP31-005 + acceptance §3.1+§3.2 (2 项) |
| ④ | **反约束 grep — direction reflect 双闸 + docstring 立场反查** — plugin→server direction 无 permission_denied (data plane count 6 不变, BPP-3.1 不进 data plane) + admin god-mode 不消费此 frame (ADM-0 §1.3 红线, docstring 立场反查 + ADM-0.2 admin RequireAdmin mw 不动) | spec §2 反约束 + ADM-0 §1.3 红线 + RT-1 cursor 唯一序立场承袭 | ✅ pass | `frame_schemas_test.go::TestBPPEnvelopeDirectionLock` data plane count 6 不变 (BPP-3.1 不进 data plane); `permission_denied_frame.go` 包级 docstring 立场反查 §; ADM-0.2 admin RequireAdmin mw 不动 — acceptance §4.1+§4.2 (2 项) + 留 §4.3 follow-up CI lint (attempted_action 字面 hardcode + HTTP 403 旁路 + drift 双向 grep) post AP-1 merge |
| ⑤ | **现网回归不破 + interface seam ready + AP-1 wiring deferred 1-line follow-up** — 全套 server test 18s PASS (含 BPP-1 reflect lint count 12→13); PushPermissionDenied hub method 跟 PushAgentConfigUpdate 同模式 (offline fail-graceful + cursor 不留洞); AP-1 abac.go::HasCapability false 路径 → server 调 PushPermissionDenied 真 wire 留 follow-up commit (interface seam ready, AP-1 #493 merge 后 1-line follow-up) | spec §3 + 退出条件 + AP-1 deferred wiring | ✅ pass | 全套 server test 18s PASS (含 BPP-1 reflect lint count 12→13 自动覆盖); PushPermissionDenied hub 方法跟 PushAgentConfigUpdate 同模式 (跟 AL-2b #481 hub.PushAgentConfigUpdate impl 同精神) — 退出条件 + AP-1 wiring follow-up 留账 |

**总体**: 5/5 通过 (覆盖 acceptance 6 项 ✅ 含 §1+§2+§3+§4 全节, frame schema + direction + payload + 反约束 + 现网) → ✅ **SIGNED**, BPP-3.1 permission_denied frame 闸通过.

---

## 2. 反向断言 (核心立场守门 byte-identical)

BPP-3.1 三处反向断言全 PASS:

- **PermissionDeniedFrame plugin→server direction 0 hit**: `bppEnvelopeWhitelist[FrameTypeBPPPermissionDenied] == DirectionServerToPlugin` whitelist + 实例 FrameDirection 双闸断; data plane count 6 不变 (BPP-3.1 不进 data plane, 走 control plane 6→7); 反向 plugin 永不发 (跟 BPP-3 #489 plugin_frame_dispatcher_test PanicsOnServerToPluginFrame 同模式 — defense-in-depth 双向锁)
- **cursor 共序三连递增** (BPP-3.1 跟 RT-1+AL-2b 共一根 sequence): `TestBPP31_PushPermissionDenied_SharedSequence` (RT-1 c1 < BPP-3.1 c2 < AL-2b c3 三连递增) — 反约束 不另起 plugin-only 通道, 跟 RT-1 立场 cursor 唯一可信序 + BPP-2 12-frame envelope 共序 + AL-2b ack hub.cursors 同源
- **8 字段反 omitempty (zero-tail 仍 8 keys)**: `TestBPP31_PushPermissionDenied_FieldByteIdentity` filled + zero-tail 双 snapshot, ':' count ≥ 8 — 反向 omitempty 漂移防御; 跟 BPP-2 TaskFinishedFrame 7 字段 byte-identical + AL-2b agent_config_ack 7 字段 byte-identical 同模式守
- **plugin offline → cursor allocated 不留洞**: `TestBPP31_PushPermissionDenied_PluginOffline` sent=false, cursor still allocated; 跟 PushAgentConfigUpdate (AL-2b #481) + PushIterationStateChanged (cv-4.2 #409) 同模式 fail-graceful (best-effort, plugin 重连后 cursor replay 兜底)

---

## 3. 跨 milestone byte-identical 链验 (BPP-3.1 是 AP-1 BPP 推送闭环锚)

BPP-3.1 兑现/承袭多源 byte-identical:

- **BPP-3.1 frame 字段跟 AP-1 abac.go 403 body byte-identical (改 = 改三处)**: BPP-3.1 envelope.go PermissionDeniedFrame `required_capability` + `current_scope` json tag ↔ AP-1 abac.go 403 body 字段 ↔ 蓝图 §4.1 row 字面 — 任一处漂 = byte-identical drift = lint fail; 跟 reason 八处单测锁链 (AL-1a #249 + ... + AL-2b ack + BPP-2 task_failed) + AL-2a allowedConfigKeys ↔ client ALLOWED_CONFIG_KEYS 跨层锁 + ADM-2 system DM 5 模板 byte-identical 同模式跨 PR 锚
- **envelope whitelist 12→13 扩**: BPP-1 9 + AL-2b ack +1 + BPP-2.2 task +2 + BPP-3.1 permission_denied +1 = 13; control 6→7 自动覆盖; BPP-1 #304 reflect lint TestBPPEnvelopeFrameWhitelist count + TestBPPEnvelopeDirectionLock control 字面同步 (跟 G4.3 BPP-2 envelope 9→12 扩同模式)
- **cursor 共序三连递增 (RT-1 + BPP-3.1 + AL-2b)**: 跟 RT-1 hub.cursors atomic int64 单调发号 + 12-frame envelope 共序 + BPP-1 #304 reflect lint 自动覆盖同模式; 反约束 不另起 plugin-only 通道
- **PermissionDeniedPusher interface seam 跟 ActionHandler/Pusher seam 同精神**: api 包不 import ws (依赖反转), 跟 BPP-2 ActionHandler / AL-2a AgentConfigPusher / AL-1 AppendAgentStateTransition / AP-1 HasCapability 同模式 (单 entry + interface seam + 依赖反转), 防绕过路径
- **PushPermissionDenied hub method 跟 PushAgentConfigUpdate (AL-2b #481) + PushIterationStateChanged (cv-4.2 #409) 同模式**: offline fail-graceful + cursor allocated 不留洞 + plugin 重连后 cursor replay 兜底
- **ADM-0 §1.3 admin god-mode 红线承袭**: admin 不消费 BPP-3.1 frame (admin 不入业务路径) — 跟 AL-3 #303 ⑦ + AL-4 #379 v2 + AL-2b #471 §2.4 + ADM-2 #484 + BPP-2 #485 + AL-1 #492 + AP-1 #493 同模式

---

## 4. 留账 (BPP-3.1 闭闸不阻, Phase 5 / follow-up — 跟 spec §5 字面承袭)

- ⏸️ **AP-1 abac.go::HasCapability false 路径真 wire** — server 调 PushPermissionDenied 真 wire (interface seam ready, AP-1 #493 merge 后 1-line follow-up commit 接); 跟 AL-2a → AL-2b → BPP-3 Pusher seam wire 跨 PR 链同模式
- ⏸️ **BPP-3.2 plugin 端 UX** — plugin 端收 frame → owner DM 推审批通知 + 一键 grant UI (Phase 5); retry 提示 owner 加权后 plugin 自动重试 (BPP-3.2 跟 owner DM 同期)
- ⏸️ **AP-3 cross-org owner-only 强制** — 后续 milestone (admin handleGrantPermission 已 admin only); 跟 AP-1 留账承袭
- ⏸️ **spec §2 #1+#4+#5 反向 grep CI lint follow-up** — attempted_action 字面 hardcode + HTTP 403 旁路 + drift 双向 grep (野马 stance checklist + AP-1 merge 后双向 grep 落 follow-up patch)

---

## 5. 解封路径 + Registry 数学验

**Phase 4 BPP-3.1 闸通过** (BPP-3.1 是 AP-1 BPP 推送闭环 follow-up):
- ✅ **G4.1 ADM-1**: 野马 ✅ #459
- ✅ **G4.2 ADM-2**: 烈马 ✅ #484 6cf5240
- ✅ **G4.3 BPP-2**: 烈马 ✅ G4 batch
- ✅ **G4.4 CM-5**: 烈马 ✅ G4 batch
- ✅ **G4.5 AL-2a + AL-2b + AL-4 联签**: 烈马 ✅ G4 batch
- ✅ **AL-1 状态四态 wrapper**: 烈马 ✅ #492
- ✅ **BPP-3 plugin 上行 dispatcher**: ✅ #489
- ✅ **AP-1 ABAC SSOT + 严格 403**: 烈马 ✅ #493 d6625b2
- ✅ **BPP-3.1 permission_denied frame**: 烈马 acceptance ✅ 本 signoff (5/5 验收 + REG-BPP31-001..006 6🟢 + envelope 12→13 + 全套 server test 18s PASS)

**Registry 数学验 (post-#494 9aab11a)**:
- 总计 238 → **244** (+6 行 BPP-3.1 全 🟢)
- active 213 → **219** (+6 净)
- pending **25** → **25** (BPP-3.1 6 行全 🟢 不增 pending)
- 跟 #475 spec brief / G4.audit 飞马 row + AL-1 5🟢 + BPP-2 17🟢 + CM-5 5🟢 + AL-2a 7🟢 + AL-1b 6🟢 baseline 累加链

后续:
- ⏸️ **AP-1 abac.go HasCapability false 路径 1-line follow-up wiring** — interface seam ready, AP-1 #493 merge 后接
- ⏸️ **G4.audit** Phase 4 代码债 audit (软 gate 飞马职责) — 含 BPP-3.1 follow-up + AP-1 8 项 + AL-1 4 项 + AL-4.2/4.3 5⚪
- ⏸️ **Phase 4 closure announcement** (Phase 4 entry 8/8 全签 ✅ + BPP-3.1 闭环 + G4.audit 飞马软 gate 链入)

---

## 6. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 烈马 | v0 — BPP-3.1 permission_denied frame (server→plugin) ✅ SIGNED post-#494 9aab11a (zhanma-c). 5/5 验收通过 covers acceptance bpp-3.1.md 6 项 ✅: 立场 ① envelope 12→13 扩 + PermissionDeniedFrame 8 字段 byte-identical 跟蓝图 §4.1 row + AP-1 abac.go 403 body + cursor 共序三连递增 / 立场 ② direction lock server→plugin + plugin offline cursor 不留洞 + PermissionDeniedPusher interface seam (api 包不 import ws 依赖反转) / 立场 ③ 字段名 byte-identical 跟 AP-1 + 8 字段反 omitempty zero-tail 双 snapshot / 反约束 plugin→server direction reflect 双闸 + admin god-mode 不消费 + docstring 立场反查 / 现网回归不破 + interface seam ready + AP-1 wiring 1-line follow-up. 跟 #403 G3.3 / G4.1-G4.5 / AL-1 / AP-1 烈马代签机制同模式 (BPP-3.1 工程内部 plugin-protocol 第二段不进野马 G4 流). 反向断言三处全过 (PermissionDeniedFrame plugin→server direction 0 hit + cursor 共序三连递增 + 8 字段反 omitempty + plugin offline cursor allocated 不留洞). 跨 milestone 链全锚 (BPP-3.1 frame 字段 跟 AP-1 abac.go 403 body byte-identical 改三处 + envelope whitelist 12→13 扩 BPP-1 reflect lint 自动覆盖 + cursor 共序 RT-1+AL-2b+BPP-3.1 三连递增 + PermissionDeniedPusher interface seam 跟 ActionHandler/Pusher 同精神 + PushPermissionDenied 跟 PushAgentConfigUpdate 同模式 + ADM-0 §1.3 红线承袭). 留账 4 项 ⏸️ deferred (AP-1 abac.go HasCapability false 真 wire interface seam ready 1-line follow-up + BPP-3.2 plugin 端 UX owner DM + 一键 grant UI + AP-3 cross-org owner-only + spec §2 #1+#4+#5 反向 grep CI lint). registry 数学: 238 → 244 (+6 全 🟢), active 213 → 219 (+6 净), pending 25 → 25. BPP-1 reflect lint count 12→13 自动覆盖. 跟 BPP-2 #485 / AL-2b #481 / AP-1 #493 同 "一 milestone 一 worktree 一 PR" 模式 (worktree `.worktrees/bpp-3.1`). |
