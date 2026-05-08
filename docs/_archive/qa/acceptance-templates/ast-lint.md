# Acceptance Template — PERF-AST-LINT: AST scan reusable lint package

> 类型: perf/test-infra (test-only API surface, 0 production change) — reusable AST identifier scan helper
> 飞马 spec v0 (`perf-ast-lint-spec.md` 9737200) + zhanma-d 实施
> Owner: 战马D 实施 / 烈马 自签 (perf 不进野马 G4 流, 跟 PERF-TEST / PERF-JWT-CLOCK / CI-SPLIT-RACE-COV / PERF-SCHEMA-SHARED / REFACTOR-REASONS deferred 同模式)

## 拆 PR 顺序

- **PERF-AST-LINT 一 PR** — `internal/lint/astscan` 包 + 8 self-check unit + BPP-4 dead_letter_test 重构 (inline AST walk → `astscan.AssertNoForbiddenIdentifiers` 调用) + spec brief (飞马 9737200) + acceptance + signoff. BPP-5 reconnect_handler_test 重构留 follow-up (#503 未 merge).

## 验收清单

### API surface (lint/astscan)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| `astscan.AssertNoForbiddenIdentifiers(t, pkgDir, forbidden, opts)` helper API + `TestingT` interface (兼容 *testing.T + fakeT) | unit | 战马D / 烈马 | ✅ — `astscan_test.go::TestAssertNoForbiddenIdentifiers_HitsIdentifier` (*testing.T satisfies TestingT 编译期验) + `_AcceptsRealTestingT` PASS |
| `ForbiddenIdentifier{Name, Reason}` struct + 失败 message 显 reason | unit | 战马D / 烈马 | ✅ — hit msg 含 identifier + reason byte-identical (`_HitsIdentifier`) |
| `ScanOpts{IncludeStrings, IncludeComments, SkipFiles}` zero-value 安全默认 | unit | 战马D / 烈马 | ✅ — `_ZeroValueIsSafe` (3 默认 false/nil) PASS |

### 行为不变量 (3 立场)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 立场 ① — 默认仅扫 *ast.Ident.Name (production identifier) | unit | 战马D / 烈马 | ✅ — `_HitsIdentifier` PASS, `_IgnoresComments` (default skip) PASS, `_IgnoresStrings` (default skip) PASS |
| 立场 ② — IncludeStrings / IncludeComments opt-in 后才扫 | unit | 战马D / 烈马 | ✅ — `_IgnoresComments` opt-in 路径 PASS, `_IgnoresStrings` opt-in 路径 PASS |
| 立场 ③ — `_test.go` 始终跳过 (tests legally mention forbidden via this helper) | unit | 战马D / 烈马 | ✅ — `_SkipsTestFiles` PASS (`fixture_test.go::pendingAcks` 不命中) |
| `SkipFiles` glob 排除额外 (e.g. `*.pb.go` 生成代码) | unit | 战马D / 烈马 | ✅ — `_SkipFilesGlob` PASS |
| 空 forbidden list 走 t.Fatalf (programming bug guard) | unit | 战马D / 烈马 | ✅ — `_EmptyListFatal` PASS |

### 反约束 (production-side 0 import)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| `go list -deps ./cmd/...` 不含 `internal/lint/astscan` | CI grep | 战马D / 烈马 | ✅ — count==0 真跑 |
| `go tool nm` production binary 不含 astscan symbols | CI grep | 战马D / 烈马 | ✅ — `go build -o /tmp/borgee-server ./cmd/collab; go tool nm /tmp/borgee-server \| grep -c astscan` == 0 |
| `astscan.` 仅 _test.go 命中 (不入 production *.go) | grep | 战马D / 烈马 | ✅ — production 路径 0 hit (仅 helper 自身 doc comment 命中, 非 import) |
| BPP-4 `dead_letter_test.go` inline `ast.Inspect` / `parser.ParseFile` 删, 调 `astscan.AssertNoForbiddenIdentifiers` | grep + diff | 战马D / 烈马 | ✅ — `go/ast` / `go/parser` / `os.ReadDir` import 全删, 函数体 8 行 `forbidden + Inspect + hits + Errorf` → 1 行 helper call |

### BPP-4 forbidden 集字面承袭

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4 forbidden id (`pendingAcks`, `retryQueue`, `deadLetterQueue`, `ackTimeout`) byte-identical 跟 BPP-4 #499 原 inline scan 同源 | grep + 字面 | 战马D / 烈马 | ✅ — `dead_letter_test.go::TestBPP4_NoRetryQueueInBPPPackage` 4 ForbiddenIdentifier 字面 byte-identical |

### 退出条件

- 上表 11 项: **11 ✅** (全绿)
- `go test ./internal/lint/astscan/` 8 unit PASS race-clean
- `go test ./internal/bpp/` BPP-4 dead_letter_test 全 PASS (refactor 不破)
- production binary 0 astscan 连接
- 烈马自签 (perf 不进野马 G4 流)
- REG-AL-001..005 5 🟢

### Follow-up 留账

- BPP-5 #503 (未 merged) `TestBPP5_NoReconnectQueueInBPPPackage` — 同模式 reuse helper, 待 #503 land 后 follow-up patch
- CM-5.1 #473 `cm5stance/cm_5_1_anti_constraints_test.go` AST walk — 二阶段重构 (规模略大, 涉及 import path scan 不仅 identifier; 需 v2 helper 加 forbidden import support)

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-29 | 战马D | v0 — PERF-AST-LINT 一 PR 整闭: `internal/lint/astscan` 新包 (`AssertNoForbiddenIdentifiers` + `ForbiddenIdentifier{Name, Reason}` + `ScanOpts{IncludeStrings, IncludeComments, SkipFiles}` + `TestingT` interface seam) + 8 self-check unit (HitsIdentifier / IgnoresComments / IgnoresStrings / SkipsTestFiles / SkipFilesGlob / EmptyListFatal / ZeroValueIsSafe / AcceptsRealTestingT) + BPP-4 `dead_letter_test.go` 重构 (inline AST scan 41 行 → helper 调用 8 行) + spec brief 127 行 (飞马 9737200) + acceptance template + 烈马自签 + REG-AL-001..005 5🟢. production-side 0 import 验 (`go list -deps` + `go tool nm` count==0). |
