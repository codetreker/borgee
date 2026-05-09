# 717 — 验收 (Acceptance)

## 真行为 invariant 守门 (走 `go test ./...` 默认)

- [ ] `TestHB_DELETE_RevokeStampsRevokedAt` (revoke <100ms latency)
- [ ] `TestHB_NoUserPermissionsJoin` (host_grants vs user_permissions 字典分立)
- [ ] `TestHB_NoGrantQueueInAPIPackage` (HB-3 AST scan)
- [ ] `TestBPP_NoRetryQueueInBPPPackage` + `TestBPP_NoReconnectQueueInBPPPackage` (BPP package 边界)
- [ ] `TestValidateTransition*` + `TestAppendAgentStateTransition*` (5 状态机 valid edges)
- [ ] `TestAP_HandlerHelperOnly` + `TestAP_ReverseGrep_HardcodeCapability` + `TestAP_ReverseGrep_DirectMapAccess` (AP-4-enum §2 / §3)
- [ ] `TestDL12_DirectStoreImportBaseline` (新加; production-only baseline 50 hard ratchet; 替 release-gate.yml::dl1-no-direct-store)

## inline grep 反约束转 Go test (feima review #722 必答)

- [ ] `TestLint_BPPHeartbeat30sSingleSource` — heartbeat 30s 单源 + 禁 drift 涨到 >30s
- [ ] `TestLint_ReasonChainNo7th` — 6 reason 字典禁出现第 7 个
- [ ] `TestLint_ReasonsSSOTExists` + `TestLint_ReasonsCrossMilestoneCoverage` — reasons SSOT
- [ ] `TestLint_AgentStateLogNoConnecting` — connecting 不入持久态
- [ ] `TestLint_PresenceSessionsNoBusyWrite` — presence_sessions 不写 busy 列
- [ ] `TestLint_ALHBStackDictIsolation` — AL/HB stack 字典分立, 表不互相 JOIN
- [ ] `TestLint_AuditSchema5FieldsByteIdentical` — audit 5 字段 byte-identical

## yml 删

- [ ] `.github/workflows/release-gate.yml` 整文件已删
- [ ] `.github/workflows/al-release-gate.yml` 整文件已删
- [ ] branch protection ruleset 不需改 (本来不挂 release-gate / al-release-gate)

## 代码搜索清扫

- [ ] `grep -rn 'release-gate\|al-release-gate' --include='*.md' --include='*.go' --include='*.ts' --include='*.tsx' --include='*.yml' | grep -v 'docs/_archive/' | grep -v '\.claude/worktrees/'` 仅剩 #717 self-doc 引用 (3 处历史注释), 无活引用
- [ ] `grep -lE '"borgee-server/internal/store"' packages/server-go/internal/api/*.go | grep -v _test.go | wc -l` 仍 = 50 (production-only baseline)

## CI 三签 (PR 合并前)

- [ ] feima 飞马 (架构) — 真行为 test 替临时字符串 grep, 设计签
- [ ] yema 野马 (PM) — Phase 闭后 over-defense 清账, 长期保系统 vs 当时一种写法, 产品签
- [ ] liema 烈马 (QA) — go-test-cov / go-test-race / e2e / check / vitest 全绿 + 反向代码搜索自核, 验收签
