# TEST-FIX-1 — TestClosedStoreInternalErrorBranches race budget fix (≤40 行 spec brief)

> Owner: 战马C / 派活 team-lead 2026-04-30 (CV-15 #592 race fail 4 跑根因诊断 — #555 AP-5 author 死, 由 zhanma-c 修)
> Mode: test refactor milestone — 0 production code 改, 0 schema, 0 endpoint, 0 client; 仅 1 行 `t.Parallel()` 加 + 1 行 `tc := tc` capture loop var.

## 1. 一句话定义

`TestClosedStoreInternalErrorBranches` (11 sub-test) 当前**串行跑** ⇒ race CI 总耗时 28s+ (本地) → 120s+ timeout (CI runner 4x 慢). 加 `t.Parallel()` 内 sub-test ⇒ 并发跑 ⇒ 总耗时降到 ~3s 本地 / ~12s CI 估算, 远低 race 总 budget.

## 2. 范围

### 在范围
- `internal/api/error_branches_test.go::TestClosedStoreInternalErrorBranches` 第 819-820 行加 `tc := tc` (capture loop var) + `t.Parallel()` (sub-test 内, 跟 outer t.Parallel 同精神)
- 11 sub-test 全保留 (覆盖率不降)
- 各 sub-test 独立 in-memory store + httptest server (无共享状态, parallel-safe)

### 不在范围
- ❌ 共享 server fixture (各 sub-test `_ = s.Close()` 自家 store, 共享会撞)
- ❌ 改 race workflow timeout (按 team-lead 拍板不 bypass)
- ❌ 减 sub-test (反向, 11 全保留)
- ❌ 改其他 race 慢 test (本 PR 仅修这一处, scope 最小)

## 3. 立场 (3 项)

1. **不降覆盖** — 11 sub-test 全保留, 仅改调度 (串行→并发)
2. **不 bypass CI race timeout** — 真修测试结构, 不改 workflow
3. **0 production code 改** — 仅 test 文件 2 行 (capture + Parallel), 反向断言 `git diff packages/server-go/{cmd,internal,sdk}/**/*.go --not-test` 0 行 (本 PR 仅 `*_test.go` 改)

## 4. 反向 grep 锚 (3 反约束)

```bash
# ① 仅 test 文件改, 0 production code
git diff origin/main...HEAD --stat | awk '/_test\.go/{t++} !/_test\.go/&&/\.go\b/{p++} END{print t,p}'  # p==0

# ② sub-test t.Parallel 真挂
grep -A1 'for _, tc := range tests' packages/server-go/internal/api/error_branches_test.go | grep 't.Parallel'  # ≥1 hit

# ③ 11 sub-test 全保留 (反向减覆盖)
grep -c '^\s*{"[a-z-]\+",' packages/server-go/internal/api/error_branches_test.go | head -1  # ≥11
```

## 5. 验收挂钩

- REG-TESTFIX1-001 立场 ① ② ③ — 0 prod / sub-test parallel / 11 全保留
- REG-TESTFIX1-002 race CI 真兑现 — `gh pr checks` go-test-race PASS ≤180s (本 PR CI run 验证)
- REG-TESTFIX1-003 cov 不降 — go-test-cov 仍 ≥84% (本 PR CI verify)

## 6. 退出条件

- 本地 `go test -race -run TestClosedStoreInternalErrorBranches` ≤10s (实测 2.56s ✓)
- 本地全 api 包 race ≤90s (实测 53s ✓)
- CI go-test-race PASS (本 PR 自验)
- CV-15 #592 解锁 (本 PR merge 后 CV-15 rebase 自然过)
