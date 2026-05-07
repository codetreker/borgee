# Borgee 协作约定

## 团队角色映射

本工程用 [blueprintflow](https://github.com/codetreker/blueprintflow) 协作 skills. 我们自己起的代号 ↔ blueprintflow 通用名映射:

| 本工程代号 | 角色 | blueprintflow 通用名 |
|---|---|---|
| **feima 飞马** | 架构师 + 代码审稿人 | architect |
| **yema 野马** | 产品 PM | pm |
| **liema 烈马** | QA + 验收 | qa |
| **zhanma / zhanma-c / zhanma-d 战马 (3 个)** | 开发 dev | dev |
| **team-lead** | 协调 + 合并把关 | facilitator |

**标准配置**: 3 dev + 1 architect + 1 PM + 1 QA + 1 team-lead (总 7 人).

## blueprintflow skills 安装

通过 Claude Code plugin marketplace 安装 (blueprintflow PR #10 已转 marketplace 结构):

```
/plugin marketplace add codetreker/blueprintflow
/plugin install blueprintflow@blueprintflow
```

安装后 skills namespace 为 `blueprintflow:blueprintflow-<name>` (skill name 字段保留 `blueprintflow-` 前缀).

skills 列表 (按职责):
- `blueprintflow:blueprintflow-workflow` — 总流程
- `blueprintflow:blueprintflow-brainstorm` — 产品方向头脑风暴
- `blueprintflow:blueprintflow-blueprint-write` — 蓝图写作
- `blueprintflow:blueprintflow-phase-plan` — Phase (一段开发周期) 规划
- `blueprintflow:blueprintflow-phase-exit-gate` — Phase 退出闸
- `blueprintflow:blueprintflow-milestone-fourpiece` — milestone 4 件套 (开工前 4 份基础文档)
- `blueprintflow:blueprintflow-pr-review-flow` — PR 审稿 + 合并流程
- `blueprintflow:blueprintflow-git-workflow` — git 工作流 (worktree / branch / PR)
- `blueprintflow:blueprintflow-team-roles` — 团队角色定位
- `blueprintflow:blueprintflow-teamlead-fast-cron-checkin` — Teamlead 快节奏巡检 (15 min)
- `blueprintflow:blueprintflow-teamlead-slow-cron-checkin` — Teamlead 慢节奏审查 (2 h)

## 跑 test 必须加 timeout

历史教训: 战马 e 跑 test 卡 40 分钟没响应, 拖死整个 milestone 推进.

**硬性规定**: 任何 `go test` / `npm test` / `pnpm test` / `playwright test` / `vitest` 调用 **必须**加 timeout, 不留没上界的 hang 路径.

```bash
# Go
go test -timeout=120s ./...
go test -timeout=120s -race -coverprofile=coverage.out ./...

# Playwright (默认有 30s per-test, 但整 suite 加 --max-failures + 总超时)
pnpm exec playwright test --timeout=30000

# Vitest
pnpm vitest run --testTimeout=10000
```

**Bash 工具调用**也必须设 `timeout` 参数 (最长 600000ms = 10min):
- 单个 test 包: 2-3 min
- 全套 test: 5-10 min
- **绝不无 timeout 跑 test**, 卡住 = 整个 agent 浪费

如 test 真需要 >10min, 用 `run_in_background: true` 提交后做别的, 不阻塞主线.

## 文档不要写重复内容 (跟 PR / git log 已有的不再抄一遍)

历史教训: spec brief (需求摘要文档) 第 §5 段往后, 第 §6 段, 第 §7 段塞分配任务、自检、更新日志这类内容; phase-N.md (一段开发周期的总文件) 写大段叙述; closure doc (milestone 收尾文档) 各段也是叙述 — 这些信息已经在 PR body、git log、git blame 里有原始记录了, 抄一遍既没新信息, 还成了 docs 冲突的主要原因 (blueprintflow PR #22 已经为同一件事立过同样的规矩).

**必须遵守的规则**:

- **spec brief 只写 §0-§4 段**: 关键约束 / 拆段安排 / 已知没做完的留着记 / 反向 grep 检查 / 不在范围内. 不写 §5+ 的分配任务、自检、更新日志段 — 这些信息走 SendMessage / Task / PR 审稿 / git log.
- **phase-N.md 不写大段叙述更新日志**: 把 PR 编号 + 一行 milestone 完成打勾就够了; 详情去看 PR body + git log + git blame.
- **closure doc 不重抄 PR 描述**: 详情的原始记录就在 PR body / git log 一处; closure 只留事实清单 + 反向 grep 检查 + 三个角色都签字通过 (PM 产品 + Architect 架构 + QA 验收) 的结论.
- **regression-registry (回归项总账) 每行只填关键字段**: 对应到哪条规格 / 怎么验证 / 负责人 / PR / 状态. 不要塞大段叙述.

这条规矩跟用户拍的「一次做干净不留尾」原则走, 也跟 blueprintflow PR #22 把"重复写文档"列为不该这么做的写法的决定一致.
