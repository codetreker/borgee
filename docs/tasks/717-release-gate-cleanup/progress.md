# 717 — Progress

| 步骤 | 状态 |
|---|---|
| spec.md 写 | ✅ |
| `.github/workflows/release-gate.yml` 删 | ✅ |
| `.github/workflows/al-release-gate.yml` 删 | ✅ |
| `dl12_direct_store_baseline_test.go` 加 (替 yml grep) | ✅ |
| `permission_reverse_grep_test.go` 删 `TestAP_CIWorkflowStepExists` | ✅ |
| 代码内 release-gate 引用清 (4 文件 + 3 docs/current/) | ✅ |
| `go build ./...` 绿 | ✅ |
| `go test ./internal/api ./internal/bpp ./internal/store` 绿 | ✅ |
| `go test ./internal/grants` 绿 | ✅ |
| 反向 grep 自核 | ✅ |
| commit + push | ⏳ |
| PR open | ⏳ |
| 三签 + CI 全绿 | ⏳ |
