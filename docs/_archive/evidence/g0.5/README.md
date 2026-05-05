# G0.5 Evidence — Phase 0 闸 5 (PR 模板自检 + current 同步硬规则) 实测

> 烈马 (QA) 验收, Task #26 Phase 0 退出闸验证。
> 闸 5 (README §3 闸 5): PR 模板必填块 + current 同步硬规则, lint 卡 merge。

## G0.5.A — `pr-template` lint 实测 (PR #170 自检)

PR #170 第一次推送时 PR body 没用模板里的 `Blueprint: <module> §X.Y` 和
`Touches: <subsystems>` 行, lint 直接卡 merge:

| run | 时间 | 结果 | 错误 |
|-----|------|------|------|
| [25008169145](https://github.com/codetreker/borgee/actions/runs/25008169145) | 16:50Z | ❌ FAIL | `PR body missing 'Blueprint: <module> §X.Y' line` + `PR body missing 'Touches: <subsystems>' line` |
| [25008782536](https://github.com/codetreker/borgee/actions/runs/25008782536) | — | ❌ FAIL | 第一次修后还差一项 |
| [25008849364](https://github.com/codetreker/borgee/actions/runs/25008849364) | 17:09Z | ✅ PASS | 战马补齐 `Blueprint: README §4, §6` + `Touches: ci, docs` |

annotation 原文 (gh run view 25008169145):

```
X PR body missing 'Touches: <subsystems>' line
X PR body missing 'Blueprint: <module> §X.Y' line
X Process completed with exit code 1.
```

→ 模板自检 fail 阻断 merge ✅, 改对再放行 ✅.

## G0.5.B — `current-sync` lint 实测 (跨 PR)

`.github/lint-current-sync.yml` 定义模块映射:
- `packages/server-go/internal/` → `docs/current/server/`

INFRA-1b 三个 PR (#171/#172/#173) 都动了 `internal/testutil/...`, 提交时
都带了 `docs/current/server/testing.md`, 三次 lint 全 pass:

| PR | run | current-sync 结果 |
|----|-----|------------------|
| #170 | [25008849364](https://github.com/codetreker/borgee/actions/runs/25008849364/job/73238942185) | ✅ pass |
| #171 | [25009176413](https://github.com/codetreker/borgee/actions/runs/25009176413/job/73240100247) | ✅ pass |
| #172 | [25009328693](https://github.com/codetreker/borgee/actions/runs/25009328693/job/73240622628) | ✅ pass |
| #173 | [25009485133](https://github.com/codetreker/borgee/actions/runs/25009485133/job/73241167354) | ✅ pass |

`exclude_globs` 列表 (`**/*_test.go`, `**/__snapshots__/**`, `**/testdata/**` 等)
让纯测试 PR 不会被错误卡住 — 飞马 review on PR #170, 已落 yaml.

## G0.5 结论

闸 5 工作流闭环, 双向已验:
1. fail 路径: 模板缺块 → CI 红 → 不能 merge ✅
2. pass 路径: 修对模板 + 同步 docs/current → CI 绿 → 可 merge ✅
3. exclude_globs 防误伤纯测试 PR ✅

evidence: GH Actions 日志 (links above) + 本地 `gh run view` annotation。
