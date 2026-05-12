# CI Lint — PR 模板 + Current 同步

> Phase 0 / Task 28 引入。Blueprint: README 规则 4 (PR 描述强制) + 规则 6 (current 同步硬规则)。

## 1. 两条 lint job

`.github/workflows/lint.yml` 在每个 PR 上跑两个 job:

### `pr-template` (G0.3)

检查 PR body 包含五个必备区块, 任一缺失立即 fail:

| 检查 | 通过条件 |
|------|---------|
| Blueprint 锚点 | 至少 1 行匹配 `^Blueprint:.+§` (gate 2 grep 自动化) |
| Touches 头 | 至少 1 行匹配 `^Touches:.+` |
| Current 同步 章节 | body 含字符串 `Current 同步` |
| Acceptance 章节 | body 含 `^## Acceptance` |
| Stage 行 | 含 `^Stage: (v0|v1)` |

### `current-sync` (G0.5, 软 gate)

读 PR diff. 模块映射在 `.github/lint-current-sync.yml`:

```
packages/server-go/internal/  → docs/current/server/
packages/server-go/cmd/       → docs/current/server/
packages/client/src/          → docs/current/client/
packages/plugins/             → docs/current/plugin/
packages/helper/              → docs/current/helper/
packages/remote-agent/        → docs/current/remote-agent/
```

PR 改了某 `code_prefix` 但没改对应 `docs_prefix` → fail。

**`exclude_globs`**: 仅改 `_test.go` / `*.test.ts(x)` / `*.spec.ts(x)` / `__snapshots__/` / `testdata/` 不算"行为变更", 不触发 current-sync (飞马 review on PR #170)。配置在 `.github/lint-current-sync.yml` 顶部 `exclude_globs:` + workflow 里的过滤 grep, 两处保持同步。

**Opt-out**: PR body `## Current 同步` 区块下写 `- N/A — <理由>` 时降级为 warning, 不阻断。reviewer 在 review 时人工判断理由是否成立。

## 2. PR 模板

`.github/pull_request_template.md` 提供占位骨架:

- `## What` — 1-3 句, why
- `## Blueprint: <module> §X.Y` — gate 2 锚点
- `## Touches` — 子系统列表; ≥2 → 强制拆分 (接口契约 PR ≤300 行 + 实现 PR)
- `## Current 同步` — `docs/current/...` 列表 或 `N/A — 理由`
- `## Acceptance` — 四选一; ⭐ 标志性双挂 (4.1+4.2)
- `Stage: v0` 行

## 3. 与 Phase 0 gate 的对应

- G0.3 PR 模板生效 → `pr-template` job 通过 ≥1 PR
- G0.5 current sync CI lint 工作 → `current-sync` job 在故意不同步的 demo PR 上 fail, 修复后 pass (按 R2 第 1 条建议, 软 gate, 不阻塞 Phase 0 退出)

## 4. 不在范围

- 自动生成 PR body (规则 4 要求 dev 写, 不让 lint 替代思考)
- 多语言 / lint-staged (后续 Phase 0 收尾时可加)
