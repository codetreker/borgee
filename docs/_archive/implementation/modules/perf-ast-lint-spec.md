# AST Scan Reusable Lint Package — spec brief (飞马 v0)

> 飞马 · 2026-04-29 · ≤80 行 · Phase 4+ 测试基础设施 (G4.audit row → reusable lint)
> 关联: BPP-4 #499 `TestBPP4_NoRetryQueueInBPPPackage` (AST scan 首落) / BPP-5 #503 `TestBPP5_NoReconnectQueueInBPPPackage` (AST scan 第二落, 累加 forbidden) / CM-5.1 #473 (AST walk 早期同根) / G4.audit kindBadge row #489 (反约束 grep → AST 升级位)
> Owner: zhanma-d 主战 + 飞马 spec 协作

---

## 1. 范围 (3 立场)

### 立场 ① — AST scan 单源 (不再每 milestone 写 inline ast.Walk)
- 当前: BPP-4 + BPP-5 各自写 `go/ast` + `go/parser` + `ast.Inspect` walk, 重复 80+ 行 boilerplate
- 目标: 抽 `packages/server-go/internal/lint/astscan` 单包, 提供 `AssertNoForbiddenIdentifiers(t, pkgDir, forbidden, opts)` helper
- 累加不替换: BPP-4/BPP-5 旧 forbidden tokens 全部迁入 reusable package, 各 test 调 helper 列自己的 forbidden set

### 立场 ② — 比 grep 更狠 (扫 AST identifier 不扫 comment/string)
- grep 会被注释 / 字符串字面 / docstring 误匹配 (false positive)
- AST scan 只看 `*ast.Ident.Name` (production identifier), 跳 `*ast.Comment` + 字符串 literal 默认
- options 显式开关: `IncludeStrings bool` (用于反约束 string literals 用例) / `IncludeComments bool` (默认 false)

### 立场 ③ — production-side 0 hit (lint 包只 _test.go import)
- `internal/lint/astscan` 是测试基础设施, **production code 必 0 import**
- 反向断言: AST scan 包自检 — `astscan.go` 不出现在 production binary import graph
- 跟 `internal/store/testutil` 同精神 (test-only seam)

---

## 2. 反约束 (5 grep + AST self-check)

```bash
# 1) AST scan 包不入 production import graph
go list -deps ./packages/server-go/cmd/... | grep 'internal/lint/astscan'   # 0 hit

# 2) 旧 inline AST walk 不残留 (重构后)
git grep -nE 'go/ast.*Walk|ast.Inspect' packages/server-go/internal/bpp/*_test.go   # 0 hit (全走 helper)

# 3) helper 不被 production code 误调
git grep -nE 'astscan\.' packages/server-go/internal/ --include='*.go' --include-not='*_test.go'   # 0 hit

# 4) forbidden lists 不字面散落
git grep -rn '"pendingAcks"\|"retryQueue"\|"deadLetterQueue"\|"pendingReconnects"' packages/server-go/internal/   # 仅 test 命中, production 0

# 5) lint 包 production 二进制不连接
go build -o /tmp/borgee-server ./packages/server-go/cmd/server && go tool nm /tmp/borgee-server | grep astscan   # 0 hit
```

---

## 3. 文件清单 (≤6 文件)

| 文件 | 范围 |
|---|---|
| `packages/server-go/internal/lint/astscan/astscan.go` | helper + `ScanOpts` struct + forbidden 类型 |
| `packages/server-go/internal/lint/astscan/astscan_test.go` | 自检 (5 反约束 + happy/edge cases) |
| `packages/server-go/internal/bpp/dead_letter_test.go` | BPP-4 inline AST scan 重构成 helper 调用 (≤10 行 diff) |
| `packages/server-go/internal/bpp/reconnect_handler_test.go` | BPP-5 inline AST scan 重构成 helper 调用 |
| `packages/server-go/internal/api/cm5stance/cm_5_1_anti_constraints_test.go` | CM-5.1 AST walk 同步迁移 (可选, 第二阶段) |
| `docs/qa/acceptance-templates/ast-lint.md` (烈马) | API 锚 + REG-AL-001..005 + 5 反约束 |

---

## 4. helper API (建议)

```go
package astscan

type ForbiddenIdentifier struct {
    Name   string  // 标识符名 (不含 package qualifier)
    Reason string  // 错误信息 (e.g., "BPP-4 ack-best-effort: 不重发")
}

type ScanOpts struct {
    IncludeStrings  bool     // 默认 false
    IncludeComments bool     // 默认 false
    SkipFiles       []string // 默认 ["*_test.go"] — 不扫测试自身
}

// AssertNoForbiddenIdentifiers 扫 pkgDir 下所有 .go (skip _test.go), 任 forbidden 命中即 t.Errorf
func AssertNoForbiddenIdentifiers(t *testing.T, pkgDir string, forbidden []ForbiddenIdentifier, opts ScanOpts)
```

调用方 (BPP-4 重构后):
```go
func TestBPP4_NoRetryQueueInBPPPackage(t *testing.T) {
    astscan.AssertNoForbiddenIdentifiers(t, "../bpp", []astscan.ForbiddenIdentifier{
        {Name: "pendingAcks", Reason: "ack best-effort 不重发"},
        {Name: "retryQueue", Reason: "ack best-effort 不重发"},
        {Name: "deadLetterQueue", Reason: "audit log 不持久"},
        {Name: "ackTimeout", Reason: "30s 字面单源在 const"},
    }, astscan.ScanOpts{})
}
```

---

## 5. 验收挂钩

- REG-AL-001 helper API 5 反约束 grep 全 count==0
- REG-AL-002 BPP-4 重构后 dead_letter_test 单测通过, forbidden 集字面承袭不丢
- REG-AL-003 BPP-5 重构后 reconnect_handler_test 单测通过, forbidden 集字面承袭不丢
- REG-AL-004 production binary 不连 astscan (`go tool nm` count==0)
- REG-AL-005 helper 自检 ScanOpts.IncludeStrings / IncludeComments 默认值正确

---

## 6. 不在范围 (留账)

- CM-5.1 AST walk 重构 (二阶段, 第一阶段先 BPP-4/5)
- forbidden imports (扫 import 路径) — v2 加, v0 仅 identifier
- 用 `golang.org/x/tools/go/analysis` 改写成 vet analyzer (v3+, 当前 testing.T 调用足够)
- AST scan 报告 HTML/JSON 格式 (单 t.Errorf 文本足够)

---

## 7. 跨 milestone byte-identical 锁

- 跟 BPP-4 #499 `TestBPP4_NoRetryQueueInBPPPackage` forbidden 集字面承袭, 重构后单测断言文本 byte-identical
- 跟 BPP-5 #503 `TestBPP5_NoReconnectQueueInBPPPackage` forbidden 集字面承袭
- 跟 BPP-1 envelope-lint #237 同精神 (lint package 只 test build, 不入 production)

---
