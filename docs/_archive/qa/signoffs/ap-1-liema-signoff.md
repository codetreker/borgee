# AP-1 ABAC 单 SSOT + capability 白名单 + 严格 403 (Phase 4 entry 8/8) — 烈马 (QA acceptance) signoff

> **状态**: ✅ **SIGNED** (烈马 acceptance 代签, 2026-04-29, post-#493 rework 230d702)
> **范围**: AP-1 milestone — Phase 4 entry 8/8 收口闸 — channels 404→403 flip (REG-CHN1-007 ⏸️→🟢) + auth.HasCapability ABAC 单 SSOT + capabilities.go 14 项 const 白名单 + commits 端点 POC wired
> **关联**: AP-1 #493 (zhanma-c rework 230d702) 整 milestone 一 PR (跟 ADM-2 #484 + BPP-2 #485 + AL-1 #492 + AL-2a #480 + AL-2b #481 + AL-1b #482 + CM-5 #476 同模式 8/8 闭); 前置: AP-0 #177 ✅ + AP-0-bis #206 ✅ + ADM-0.2/0.3 cookie 拆 ✅ + Phase 4 ADM-1 #459 + ADM-2 #484 ✅; REG-AP1-001..007 unit 7🟢 + REG-AP1-101..104 e2e 4🟢 = **11 行 🟢** + REG-CHN1-007 ⏸️→🟢 flip; 全套 server test 18s PASS (含 CV-1.2 agent commit 路径 grant 补全)
> **方法**: 跟 #403 G3.3 + #449 G3.1+G3.2+G3.4 + #459 G4.1 + G4.2 + G4.3 + G4.4 + G4.5 + AL-1 烈马代签机制承袭 — 真单测+e2e 实施证据 + 立场反查 + acceptance template 闭锁 + 跨 milestone byte-identical 链承袭 + 烈马代签 (AP-1 是 Phase 4 entry 收口工程内部 milestone, 跟 cm-4 / adm-0 / adm-2 / al-1 deferred 同模式不进野马 G4 流)

---

## 1. 验收清单 (烈马 acceptance 视角 5 项, 跟 acceptance ap-1.md 11 项 ✅ byte-identical, 三立场对照)

| # | 验收项 (立场对照) | 立场锚 | 结果 | 实施证据 (PR/SHA + 测试名 byte-identical) |
|---|--------|--------|------|------|
| ① | **立场 ① 严格 403 flip** — `GET /api/v1/channels/:id` 非 member → 403 (不再 404 隐藏存在性, 跟 GitHub repo 私有路径同模式); 真不存在 → 404, 存在但无权 → 403 (区分两态) + 三处 e2e 断言同步 flip (channel_isolation + coverage_boost + permission_ws); REG-CHN1-007 ⏸️→🟢 兑现 (CHN-1 #178 deferred 行真翻) | spec §1 立场 ① + 蓝图 §1 + REG-CHN1-007 deferred 兑现 | ✅ pass | `internal/api/channels.go::handleGetChannel` 404→403 flip + `channel_isolation_test.go::TestP0PrivateChannelIsolation` + `coverage_boost_test.go::TestChannelMemberOperations/PrivateChannelAccessControl` + `internal/ws/permission_ws_test.go::TestP1WebSocketPermissionChanges` 三处 `status === 403` PASS — REG-CHN1-007 ⏸️→🟢 + acceptance §1.1+§1.2 (2 项) |
| ② | **立场 ② ABAC capability check 单 SSOT** — `auth.HasCapability(ctx, perm, scope) bool` 单 helper 落 internal/auth/abac.go + 正向通路 (agent 持显式 (commit_artifact, artifact:art-1) → true) + **反约束 agent 不享 (*,*) wildcard 短路** (owner 误 grant 仍拦, 蓝图 §1.4 字面承袭) + cross-scope 严格 (art-1 grant 不放过 art-2) + human owner 享 wildcard 短路 (立场 ④ 区分双向锁) + nil user → false defense + ArtifactScope(r) resolver 跟 channelScope 同模式 + ChannelScopeStr/ArtifactScopeStr 单源 builder | spec §1 立场 ② ABAC SSOT + 立场 ④ 区分 agent/human + 蓝图 §1.4 跨 org 只能减权 | ✅ pass | `auth/abac.go::HasCapability` + `abac_test.go` 7 PASS (TestHasCapability_AgentExplicitScope_Pass + AgentNoWildcardShortcut + AgentCrossScope_False + HumanWildcard_Pass + NilUser_False + TestArtifactScope_ResolvesPathValue + TestScopeStr_Builders) — REG-AP1-002..006 + acceptance §2.1-§2.7 (7 项) |
| ③ | **立场 ③ capability 字面白名单 14 项 const byte-identical** — `Capabilities` map 字面白名单 v1 14 项 (read_channel/write_channel/delete_channel/read_artifact/write_artifact/commit_artifact/iterate_artifact/rollback_artifact/mention_user/read_dm/send_dm/manage_members/invite_user/change_role) byte-identical 跟 spec §1 ③ + 蓝图 §1; admin 不入此白名单 (admin god-mode 走 /admin-api/* 单独 mw, ADM-0 §1.3 + ADM-0.2 既有 RequireAdmin 不动) | spec §1 立场 ③ + 蓝图 §1 capability list + ADM-0 §1.3 红线 | ✅ pass | `auth/capabilities.go` const + `Capabilities` map; `abac_test.go::TestCapabilities_WhitelistByteIdentical` (count + 字面 const 锁) PASS + 包级 docstring 立场反查 § + ADM-0.2 既有 RequireAdmin mw 反向不动 — REG-AP1-001 + acceptance §3.1+§3.2 (2 项) |
| ④ | **反约束 grep — HasCapability 字面 hardcode 0 hit** — `git grep -nE 'HasCapability\("[a-z_]+"' packages/server-go/internal/api/` count==0 (走 const, 反 hardcode 字面); CI lint 等价单测守 future drift (filepath.Walk + regex 自动扫 internal/api/*.go) | spec §2 反约束 #1 + capability const 立场守 | ✅ pass | `abac_test.go::TestReverseGrep_NoHardcodedPermissionLiteral` (filepath.Walk + regex 自动扫 internal/api/*.go) PASS — REG-AP1-007 + acceptance §4.1 (1 项) |
| ⑤ | **e2e 真路由 wired (POC: commits 端点)** — POST /api/v1/artifacts/{id}/commits 挂 `auth.HasCapability(ctx, auth.CommitArtifact, auth.ArtifactScopeStr(id))` 单 SSOT; agent 无 grant → 403 + body 含 `required_capability` + `current_scope` BPP 路由字段 (蓝图 §2 不变量); agent 显式 grant → 200; cross-artifact (art-other grant 不放过 art-target) → 403; human owner 享 wildcard 短路 → 200 (立场 ④ 区分) | acceptance §5 e2e + 蓝图 §2 不变量 (Permission denied 走 BPP 字段) | ✅ pass | `internal/api/ap_1_2_artifacts_e2e_test.go` 4 PASS (TestAP12_AgentNoGrant_403WithBPPRoutingHints + AgentWithExplicitGrant_200 + AgentCrossArtifactGrant_403 + HumanWildcardStillWorks_200) — REG-AP1-101..104 + acceptance §5.1-§5.4 (4 项) |

**总体**: 5/5 通过 (覆盖 acceptance 16 项 ✅ 含 §1+§2+§3+§4+§5 全节, 真路由 POC commits 端点) → ✅ **SIGNED**, AP-1 ABAC 单 SSOT + 严格 403 闸通过.

---

## 2. 反向断言 (核心立场守门 byte-identical)

AP-1 三处反向断言全 PASS:

- **HasCapability 字面 hardcode 0 hit (CI lint 单测守)**: `TestReverseGrep_NoHardcodedPermissionLiteral` 实跑 filepath.Walk + regex 扫 internal/api/*.go count==0 — 防 future drift; 跟 BPP-2 ActionHandler / cm5stance NoBypassEndpoint AST walk 立场守同模式
- **parallel impl 已删 (drift audit 重做)**: 上轮 c00f38e 4 drift 项 (RequireAgentStrict403 parallel 而非 abac.go SSOT / artifact.edit_content 字面而非 commit_artifact 白名单 / expires_at runtime check 越界 / REG-CHN1-007 未 flip), 重做 230d702 删 abac_artifact.go + abac_artifact_test.go (parallel impl) + 建 abac.go::HasCapability 单 SSOT + capabilities.go 14 项 const 白名单 byte-identical spec §1 ③; 反向 grep parallel impl 0 hit
- **expires_at runtime 不消费 (schema slot 留 v=24, runtime 删)**: 删 expires_at runtime check 2 处 + 2 test, schema slot v=24 migration 留 (`migrations/ap_1_1_user_permissions_expires.go`) 供 v2+ 业务化时 server 端消费; 蓝图 §1.2 + spec §5 字面 "schema 保留, UI/runtime 不做"
- **agent 不享 (*,*) wildcard 短路** (蓝图 §1.4 字面承袭): owner 误 grant agent (*,*) → HasCapability 仍 false; human owner 享 wildcard 短路 (立场 ④ 区分双向锁) — `TestHasCapability_AgentNoWildcardShortcut` + `TestHasCapability_HumanWildcard_Pass` 双向锁
- **403 body 含 BPP 路由字段** (蓝图 §2 不变量): `required_capability` + `current_scope` 字段供 future BPP-3 permission_denied frame 消费 (此 PR 落 server 端 403 body 字段, BPP frame 推送留 BPP-3 follow-up)

---

## 3. 跨 milestone byte-identical 链验 (AP-1 是 Phase 4 entry 8/8 收口锚)

AP-1 兑现/承袭多源 byte-identical:

- **REG-CHN1-007 ⏸️→🟢 flip 兑现 CHN-1 acceptance 闭环**: CHN-1 #178 deferred 行 (channels 404→403 flip + 三处 e2e 同步) AP-1 真翻 — channels.go::handleGetChannel 区分真不存在 (404) vs 存在但无权 (403), 跟 GitHub repo 私有路径同模式; 跟 ADM-1 #464 deferred 2 行 ADM-2 兑现 + AL-1b #482 deferred 1 e2e BPP-2 真 frame 后翻 同模式
- **capabilities const 跟 AP-0 #177 既有 wire 兼容**: capability 14 项 byte-identical 跟蓝图 §1; admin 不入此白名单 (admin god-mode 走 /admin-api/* 单独 mw, ADM-0 §1.3 红线 + ADM-0.2 既有 RequireAdmin mw 不动) — 跟 AL-3 #303 ⑦ + AL-4 #379 v2 + AL-2b #471 §2.4 + ADM-2 #484 + BPP-2 #485 + AL-1 #492 同模式
- **scope resolver 跟 CHN-1 channelScope 同模式 byte-identical**: ArtifactScope(r) resolver `artifact:{id}` + ChannelScopeStr/ArtifactScopeStr 单源 builder, 跟 CHN-1 channelScope 同模式 (单 entry + 反向 raw 字面 hardcode 0 hit)
- **HasCapability 单 SSOT 跟其他 milestone seam 同精神**: 跟 BPP-2 ActionHandler interface seam + AL-2a AgentConfigPusher + RT-1 hub.cursors 单调发号 + AL-1 AppendAgentStateTransition 同模式 (单 entry + interface seam + 依赖反转), 防绕过路径
- **CV-1.2 agent commit 路径 grant 补全**: AP-1 commits 端点真 wire 后, CV-1.2 既有 agent commit 测试需 grant 补全, 全套 server test 18s PASS (含 CV-1.2 agent commit 路径) 现网回归不破
- **forward-only 立场承袭**: capabilities const 14 项 + scope 三层 (`*` / `channel:<id>` / `artifact:<id>`) 字面锁, schema slot v=24 expires_at 仅作 future 业务化储备 (跟 ADM-2.1 admin_actions + ADM-2.2 impersonation_grants + AL-1 agent_state_log forward-only 同精神)

---

## 4. 留账 (AP-1 闭闸不阻, Phase 5 / v2+ follow-up — 跟蓝图 §6 字面承袭)

- ⏸️ **agent 创建/管理 UI** — 蓝图 §6 第 11 轮 client SPA, AP-1 server scope 不含; bundle UI 渲染 client follow-up
- ⏸️ **permission_denied BPP frame 推送 + owner DM 一键 grant UI** — BPP-3 (Phase 5) zhanma-c BPP-3.1 in-flight 接管; 此 PR 落 server 端 403 body 字段 (`required_capability` + `current_scope`) 供 future BPP-3 consumer 用 — 跟 AL-1b deferred e2e BPP-2 真 frame 后翻 + AL-1 REG-AL1-006 dispatcher wire 同模式
- ⏸️ **AP-3 跨 org owner-only grant 强制** — 后续 milestone (admin handleGrantPermission 已 admin only, AP-3 落地真扩 org 维度); 跟 蓝图 §1.4 "跨 org 只能减权" + AP-1 立场 ② agent 不享 wildcard 短路同精神
- ⏸️ **expires_at runtime/server check** — schema slot v=24 留, v2+ 业务化时 server 端消费 (蓝图 §1.2 + spec §5 字面)
- ⏸️ **其它 artifact-write 端点 wiring** (rollback / iterations) — follow-up patch (本 PR commit 端点 POC, 跟 G4.audit 飞马软 gate 同期收口)
- ⏸️ **bundle UI / workspace / org scope** — v1 不做 (spec §5 + 蓝图 §6 字面)
- ⏸️ **admin god-mode capability check** — 走 /admin-api/* 单独 mw, ADM-0 §1.3 已落不动
- ⏸️ **spec §2 #2-#5 反向 grep follow-up** — ad-hoc role==admin / bundle 字面入 server / scope 漂出 v1 三层 / admin god-mode 走 ABAC 4 反向 grep CI lint, 留野马 stance checklist 落 follow-up patch

---

## 5. 解封路径 + Registry 数学验 (Phase 4 entry 8/8 收口)

**Phase 4 entry 8/8 全收**:
- ✅ **G4.1 ADM-1**: 野马 ✅ #459
- ✅ **G4.2 ADM-2**: 烈马 ✅ #484 6cf5240
- ✅ **G4.3 BPP-2**: 烈马 ✅ G4 batch
- ✅ **G4.4 CM-5**: 烈马 ✅ G4 batch
- ✅ **G4.5 AL-2a + AL-2b + AL-4 联签**: 烈马 ✅ G4 batch
- ✅ **AL-1 状态四态 wrapper**: 烈马 ✅ #492 (post-merged)
- ✅ **BPP-3 plugin 上行 dispatcher**: ✅ #489 (post-merged)
- ✅ **AP-1 ABAC SSOT + 严格 403**: 烈马 acceptance ✅ 本 signoff (5/5 验收 + REG-AP1-001..007 7🟢 + REG-AP1-101..104 4🟢 + REG-CHN1-007 ⏸️→🟢 flip + 全套 server test 18s PASS)

**Registry 数学验 (post-rework 230d702)**:
- 总计 253 → **250** (-3, AP-1 15→11 + CHN-1 9/1→10/0 含 -007 flip)
- active 228 → **225** (-3 净)
- pending **25** → **25** (CHN-1 -1 + AP-1 0)
- 跟 #475 spec brief / G4.audit 飞马 row + AL-1 5🟢 + BPP-2 17🟢 + CM-5 5🟢 + AL-2a 7🟢 + AL-1b 6🟢 baseline 累加链

后续:
- ⏸️ **G4.audit** Phase 4 代码债 audit (软 gate 飞马职责) — 含 AP-1 follow-up 8 项 + AL-1 4 项 + AL-4.2/4.3 5⚪ + AL-2b ack ingress BPP-3 接管
- ⏸️ **Phase 4 closure announcement** (Phase 4 entry 8/8 全签 ✅ + G4.audit 飞马软 gate 链入)

---

## 6. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 烈马 | v0 — AP-1 ABAC 单 SSOT + capability 白名单 + 严格 403 (Phase 4 entry 8/8 收口) ✅ SIGNED post-#493 rework 230d702 (zhanma-c). 5/5 验收通过 covers acceptance ap-1.md 16 项 ✅: 立场 ① REG-CHN1-007 ⏸️→🟢 flip channels 404→403 + 三处 e2e 同步 / 立场 ② HasCapability ABAC 单 SSOT + 7 unit case (含 agent 不享 wildcard 短路 + cross-scope 严格 + human owner wildcard + nil defense + ArtifactScope resolver + ScopeStr builder) / 立场 ③ 14 项 const 白名单 byte-identical + admin 不入 / 反约束 HasCapability hardcode 0 hit CI lint 单测守 future drift / e2e 真路由 wired POC commits 端点 4 PASS (无 grant 403 + BPP 路由字段 / 显式 grant 200 / cross-artifact 403 / human wildcard 200). 跟 #403 G3.3 / #449 G3.1+G3.2+G3.4 / #459 G4.1 / G4.2 / G4.3 / G4.4 / G4.5 / AL-1 烈马代签机制同模式 — 真单测+e2e 实施证据 + 立场反查 + acceptance template 闭锁 + 烈马代签 (AP-1 工程内部 entry 收口不进野马 G4 流). 反向断言三处全过 (HasCapability hardcode 0 hit / parallel impl 已删 含 abac_artifact.go + abac_artifact_test.go / expires_at runtime 不消费 schema slot 留 / agent 不享 wildcard 短路双向锁 / 403 body BPP 路由字段). 跨 milestone 链全锚 (REG-CHN1-007 flip 兑现 CHN-1 acceptance 闭环 + capabilities const 跟 AP-0 既有 wire 兼容 + scope resolver 跟 CHN-1 channelScope 同模式 + HasCapability 单 SSOT 跟 ActionHandler/Pusher/AppendAgentStateTransition 同精神 + CV-1.2 agent commit grant 补全 + ADM-0 §1.3 红线承袭 + forward-only 跨 milestone 同精神). 留账 8 项 ⏸️ deferred (agent 创建/管理 UI 蓝图 §6 v2+ + permission_denied BPP frame BPP-3 in-flight zhanma-c + AP-3 cross-org owner-only + expires_at runtime 业务化 + 其它 artifact-write 端点 wiring rollback/iterations + bundle UI/workspace/org scope + admin god-mode 走 ABAC ADM-0 §1.3 + spec §2 #2-#5 反向 grep CI lint follow-up). registry 数学: 253 → 250 (-3 含 AP-1 rework 15→11 + CHN-1 -007 flip), active 228 → 225 (-3 净), pending 25 → 25. **AP-1 闭即 Phase 4 entry 8/8 全收, Phase 4 闭幕** (等 G4.audit 飞马软 gate + Phase 4 closure announcement 飞马职责). |
