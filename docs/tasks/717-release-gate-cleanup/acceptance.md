# 717 — 验收 (Acceptance)

## 真行为 invariant 守门 (走 `go test ./...` 默认)

- [ ] `TestHB_DELETE_RevokeStampsRevokedAt` (revoke <100ms latency)
- [ ] `TestHB_NoUserPermissionsJoin` (host_grants vs user_permissions 字典分立)
- [ ] `TestHB_NoGrantQueueInAPIPackage` (HB-3 AST scan)
- [ ] `TestBPP_NoRetryQueueInBPPPackage` + `TestBPP_NoReconnectQueueInBPPPackage` (BPP package 边界)
- [ ] `TestValidateTransition*` + `TestAppendAgentStateTransition*` (5 状态机 valid edges)
- [ ] `TestAP_HandlerHelperOnly` + `TestAP_ReverseGrep_HardcodeCapability` + `TestAP_ReverseGrep_DirectMapAccess` (AP-4-enum 立场 ② / ③)
- [ ] `TestDL12_DirectStoreImportBaseline` (新加; baseline 115; 替 release-gate.yml::dl1-no-direct-store)

## yml 删

- [ ] `.github/workflows/release-gate.yml` 整文件已删
- [ ] `.github/workflows/al-release-gate.yml` 整文件已删
- [ ] branch protection ruleset 不需改 (本来不挂 release-gate / al-release-gate)

## 反向 grep 清扫

- [ ] `grep -rn 'release-gate\|al-release-gate' --include='*.md' --include='*.go' --include='*.ts' --include='*.tsx' --include='*.yml' | grep -v 'docs/_archive/' | grep -v '\.claude/worktrees/'` 仅剩 #717 self-doc 引用 (3 处历史注释), 无活引用
- [ ] `grep -rl "borgee-server/internal/store" packages/server-go/internal/api/ --include="*.go" | wc -l` 仍 = 115 (baseline 锁链)

## CI 三签 (PR 合并前)

- [ ] feima 飞马 (架构) — 真行为 test 替临时字符串 grep 立场签
- [ ] yema 野马 (PM) — Phase 闭后 over-defense 清账, 长期保系统 vs 当时一种写法立场签
- [ ] liema 烈马 (QA) — go-test-cov / go-test-race / e2e / check / vitest 全绿 + reverse-grep 自核签
