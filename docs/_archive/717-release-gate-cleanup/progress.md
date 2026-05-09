# 717 — Progress

| 步骤 | 状态 |
|---|---|
| spec.md 写 | ✅ |
| `.github/workflows/release-gate.yml` 删 | ✅ |
| `.github/workflows/al-release-gate.yml` 删 | ✅ |
| `dl12_direct_store_baseline_test.go` 加 (替 yml grep, baseline=50 production only — liema Q1 修) | ✅ |
| `lint_constraints_test.go` 加 7 条反约束 (feima review 必答) | ✅ |
| `permission_reverse_grep_test.go` 删 `TestAP_CIWorkflowStepExists` | ✅ |
| 代码内 release-gate 引用清 (4 文件 + 3 docs/current/) | ✅ |
| `go build ./...` 绿 | ✅ |
| `go test ./internal/api ./internal/bpp ./internal/store` 绿 | ✅ |
| `go test ./internal/grants` 绿 | ✅ |
| `go test ./internal/api -run 'TestLint_'` 绿 (8/8 PASS) | ✅ |
| 反向代码搜索自核 | ✅ |
| commit + push | ⏳ |
| feima 双 review 4 签 | ⏳ |
| CI 全绿 + 三签 | ⏳ |

