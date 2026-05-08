# Acceptance Template — AP-1: ABAC 单 SSOT + capability 白名单 + 严格 403 (Phase 4 entry 8/8)

> Spec: `docs/implementation/modules/ap-1-spec.md` (飞马 v0, 96 行)
> 蓝图: `docs/blueprint/current/auth-permissions.md` §1 (ABAC + UI bundle 混合) + §1.2 v1 三 scope (`*` / `channel:<id>` / `artifact:<id>`) + §1.4 跨 org 只能减权 + §2 不变量 (Permission denied 走 BPP)
> 前置: AP-0 #177 ✅ + AP-0-bis #206 ✅ + ADM-0.2/0.3 cookie 拆 ✅ + Phase 4 ADM-1/2 ✅
> Owner: 战马C (主战) + 飞马 (spec 协作)

## 验收清单

### 立场 ① — 严格 403 flip (REG-CHN1-007 ⏸️→🟢)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 `GET /api/v1/channels/:id` 非 member → 403 (不再 404 隐藏存在性, 跟 GitHub repo 私有路径同模式) | server inline check | 战马C / 烈马 | `internal/api/channels.go::handleGetChannel` 404→403 flip — 真不存在 → 404, 存在但无权 → 403 (区分两态) |
| 1.2 三处 e2e 断言 flip 跟 server 同步 | unit/e2e | 战马C / 烈马 | `internal/api/channel_isolation_test.go::TestP0PrivateChannelIsolation` + `coverage_boost_test.go::TestChannelMemberOperations/PrivateChannelAccessControl` + `internal/ws/permission_ws_test.go::TestP1WebSocketPermissionChanges` 三处断 `status === 403` PASS |

### 立场 ② — ABAC capability check 单 SSOT

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 `auth.HasCapability(ctx, perm, scope) bool` 单 helper SSOT 落 `internal/auth/abac.go` | unit | 战马C / 烈马 | `abac.go::HasCapability` 函数体 + 包级 docstring 立场反查 |
| 2.2 agent 持显式 (perm, scope) 行 → true (正向通路) | unit | 战马C / 烈马 | `abac_test.go::TestHasCapability_AgentExplicitScope_Pass` |
| 2.3 **反约束: agent 持 (*,*) wildcard 行 → false** (蓝图 §1.4 字面承袭, owner 误 grant 仍拦) | unit | 战马C / 烈马 | `TestHasCapability_AgentNoWildcardShortcut` |
| 2.4 cross-scope: agent 持 art-1 行访 art-2 → false | unit | 战马C / 烈马 | `TestHasCapability_AgentCrossScope_False` |
| 2.5 human owner 享 (*,*) wildcard 短路 (立场 ④ 区分 agent/human, 双向锁) | unit | 战马C / 烈马 | `TestHasCapability_HumanWildcard_Pass` |
| 2.6 nil user → false (defense) | unit | 战马C / 烈马 | `TestHasCapability_NilUser_False` |
| 2.7 `auth.ArtifactScope(r)` resolver `artifact:{id}` (跟 channelScope 同模式) + `ChannelScopeStr` / `ArtifactScopeStr` 单源 builder | unit | 战马C / 烈马 | `TestArtifactScope_ResolvesPathValue` + `TestScopeStr_Builders` |

### 立场 ③ — capability 字面白名单 ≤30 (`capabilities.go`)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 v1 14 项 const 白名单 byte-identical 跟 spec §1 ③ + 蓝图 §1: `read_channel`/`write_channel`/`delete_channel`/`read_artifact`/`write_artifact`/`commit_artifact`/`iterate_artifact`/`rollback_artifact`/`mention_user`/`read_dm`/`send_dm`/`manage_members`/`invite_user`/`change_role` | unit | 战马C / 烈马 | `auth/capabilities.go` const + `Capabilities` map; `TestCapabilities_WhitelistByteIdentical` (count + 字面 const 锁) |
| 3.2 admin 不入此白名单 (admin god-mode 走 /admin-api/* 单独 mw, ADM-0 §1.3 + spec §1 ③) | docstring + 反向断言 | 战马C / 烈马 | `capabilities.go` 包级 docstring 立场反查 § + ADM-0.2 既有 admin RequireAdmin mw 不动 |

### 反约束 grep (spec §2)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 `git grep -nE 'HasCapability\("[a-z_]+"' packages/server-go/internal/api/` count==0 (走 const, 反 hardcode 字面) | CI lint 等价单测 | 战马C / 烈马 | `abac_test.go::TestReverseGrep_NoHardcodedPermissionLiteral` (filepath.Walk + regex 自动扫 internal/api/*.go) |
| 4.2 (留 follow-up) ad-hoc role==admin / bundle 字面入 server / scope 漂出 v1 三层 / admin god-mode 走 ABAC — 4 反向 grep | follow-up CI lint | 烈马 / 飞马 | spec §2 #2-#5 反向 grep 留 follow-up patch (野马 stance checklist 落) |

### e2e 真路由 wired (POC: commits 端点)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 5.1 POST /api/v1/artifacts/{id}/commits 走 `auth.HasCapability(ctx, auth.CommitArtifact, auth.ArtifactScopeStr(id))` 单 SSOT — agent 无 grant → 403 + body 含 `required_capability` + `current_scope` BPP 路由字段 | http e2e | 战马C / 烈马 | `internal/api/ap_1_2_artifacts_e2e_test.go::TestAP12_AgentNoGrant_403WithBPPRoutingHints` |
| 5.2 agent 持显式 (commit_artifact, artifact:<id>) → 200 | http e2e | 战马C / 烈马 | `TestAP12_AgentWithExplicitGrant_200` |
| 5.3 cross-artifact: art-other grant 不放过 art-target → 403 | http e2e | 战马C / 烈马 | `TestAP12_AgentCrossArtifactGrant_403` |
| 5.4 human owner 享 wildcard 短路 → 200 (立场 ④ 区分) | http e2e | 战马C / 烈马 | `TestAP12_HumanWildcardStillWorks_200` |

## 不在本轮范围 (spec §5 + 蓝图 §6 字面承袭)

- **expires_at runtime/server check** → schema slot v=24 migration 留 (`migrations/ap_1_1_user_permissions_expires.go`), v2+ 业务化时 server 端消费. 蓝图 §1.2 + spec §5 字面 "schema 保留, UI/runtime 不做".
- **bundle UI 渲染** → client follow-up, 不在 AP-1 server scope (蓝图 §6 第 11 轮 client SPA)
- **workspace / org scope** → v1 不做 (spec §5)
- **admin god-mode capability check** → 走 /admin-api/* 单独 mw, ADM-0 §1.3 已落
- **AP-3 跨 org owner-only grant 强制** → 后续 milestone (admin handleGrantPermission 已 admin only)
- **permission_denied BPP frame 推送 + owner DM 一键 grant UI** → BPP-3 (Phase 5; 本 PR 落 server 端 403 body 字段供 future consumer 用)
- **其它 artifact-write 端点 wiring** (rollback / iterations) → follow-up patch (本 PR commit 端点 POC)

## 退出条件

- 立场 ① 1.1+1.2 (REG-CHN1-007 flip ⏸️→🟢) ✅
- 立场 ② 2.1-2.7 (HasCapability SSOT + 6 unit case) ✅
- 立场 ③ 3.1-3.2 (14 项 const 白名单 byte-identical) ✅
- 反约束 4.1 (CI lint 单测守 future drift) ✅
- e2e 5.1-5.4 (commits 端点 4 真路径) ✅
- 现网回归不破: 全套 server test 18s PASS (含 CV-1.2 agent commit 路径 grant 补全)
- REG-CHN1-007 + REG-AP1-001..007 + REG-AP1-101..104 = **11 行 🟢** + 1 flip

## 更新日志

- 2026-04-29 — 战马C v0 重做 (按 spec §1 三立场对齐): 上轮 c00f38e drift 4 项, 重做删 parallel impl + 建 abac.go SSOT + capabilities.go 白名单 + channels.go 404→403 flip + 删 expires_at runtime check. 全套 server test 18s PASS.
