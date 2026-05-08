# Acceptance Signoff — PERF-AST-LINT (烈马自签)

> **状态**: ✅ SIGNED 2026-04-29 — PERF-AST-LINT 一 PR 整闭
> **关联**: 飞马 spec v0 (`perf-ast-lint-spec.md` 9737200) + zhanma-d 实施
> **方法**: perf/test-infra 不进野马 G4 流, 烈马代签 (跟 PERF-TEST / PERF-JWT-CLOCK / CI-SPLIT-RACE-COV / PERF-SCHEMA-SHARED / REFACTOR-REASONS / CM-5 / ADM-1 / AL-1 deferred 同模式)

## 验收对照

| # | 锚点 | 实施证据 | 状态 |
|---|---|---|---|
| ① | astscan API + TestingT interface seam (兼容 *testing.T + fakeT) | 8 self-check unit PASS race-clean | ✅ pass |
| ② | 立场 ① 默认仅扫 *ast.Ident — 不扫 comment / string | `_HitsIdentifier` + `_IgnoresComments` + `_IgnoresStrings` PASS | ✅ pass |
| ③ | 立场 ② IncludeStrings / IncludeComments opt-in 路径 | 同 ② opt-in case PASS | ✅ pass |
| ④ | 立场 ③ production-side 0 import — `go tool nm` count==0 | `/tmp/borgee-server` 真跑 grep 0 hit | ✅ pass |
| ⑤ | _test.go 始终跳过 + SkipFiles glob 额外排除 | `_SkipsTestFiles` + `_SkipFilesGlob` PASS | ✅ pass |
| ⑥ | BPP-4 dead_letter_test 重构 — inline AST 41 行 → helper 8 行, forbidden 4 字面 byte-identical | `dead_letter_test.go::TestBPP4_NoRetryQueueInBPPPackage` PASS | ✅ pass |
| ⑦ | BPP-4 测试集合不破 (refactor 不损语义) | `go test ./internal/bpp/` 全 PASS | ✅ pass |

## 立场关键

- **API ship + BPP-4 重构 first batch** — BPP-5 reconnect_handler_test (#503 未 merge) 留 follow-up. CM-5.1 AST walk 留 v2 (需要 forbidden import support).
- **production-side 0 import 验 真跑**: `go build -o /tmp/borgee-server ./cmd/collab; go tool nm /tmp/borgee-server | grep -c astscan == 0`. lint 包真不入 binary.
- **TestingT interface seam**: 不耦合 `*testing.T` 强类型, 既允许 *testing.T (production 用) 又允许 fakeT (self-check), 8 unit 自检覆盖.

## 跨 milestone 锁链承袭

- BPP-4 #499 forbidden 4 id (`pendingAcks` / `retryQueue` / `deadLetterQueue` / `ackTimeout`) byte-identical 跟原 inline scan 同源
- 跟 PERF-TEST / PERF-JWT-CLOCK / CI-SPLIT-RACE-COV / PERF-SCHEMA-SHARED 同模式 perf 工具箱
- 跟 BPP-1 #304 envelope CI lint 同精神 (lint 包只 test build, 不入 production)

## Follow-up ⏸️ deferred

- **REG-AL-006** BPP-5 #503 `TestBPP5_NoReconnectQueueInBPPPackage` 重构 — 待 #503 land
- **REG-AL-007** CM-5.1 #473 `cm5stance/cm_5_1_anti_constraints_test.go` AST walk 重构 — 涉 import path scan, 需 v2 helper 加 `ForbiddenImport` support
- **REG-AL-008** v3+ — `golang.org/x/tools/go/analysis` vet analyzer 升级 (当前 testing.T 调用足够)

## 烈马签字

烈马 (代 zhanma-d) 2026-04-29 ✅ SIGNED post-PERF-AST-LINT PR
- 11/11 验收通过 (acceptance template)
- 8 self-check unit + BPP-4 重构全 PASS race-clean
- production binary 0 astscan symbols (真 `go tool nm` 验)
- 跨 milestone 锁链不破 (BPP-4 #499 forbidden 4 字面 byte-identical 承袭)
- 跟 PERF-TEST / PERF-JWT-CLOCK / CI-SPLIT-RACE-COV / PERF-SCHEMA-SHARED / REFACTOR-REASONS / CM-5 / ADM-1 / AL-1 烈马代签机制同模式

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-29 | 烈马 | v0 — PERF-AST-LINT ✅ SIGNED 一 PR 整闭. 11/11 验收通过 (astscan API + 3 立场 + 5 反约束 + BPP-4 dead_letter 重构 forbidden 4 字面承袭). REG-AL-001..005 5🟢. 留账 3 项 ⏸️ deferred (REG-AL-006 BPP-5 #503 重构 + REG-AL-007 CM-5.1 #473 v2 import scan + REG-AL-008 vet analyzer 升级). |
