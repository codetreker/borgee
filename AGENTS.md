# Agent Instructions

## Blueprintflow Planning Rules

- Documentation-stage review focuses on whether content expresses the direction and boundary accurately. Do not block Phase, Milestone, or Task planning on wording nits or implementation-level detail.
- Phase -> Milestone -> Task -> Dev design is coarse-to-fine. Phase/Milestone/Task gates check direction, boundary, and recoverability; execution detail belongs in task execution and Dev design.
- A Phase is a small stage inside a major iteration. Keep a Phase to 3 or fewer user-facing milestones by default. If a wave adds more, record why the Phase still holds together and why another Phase would be worse.
- A Milestone is a milestone inside a Phase. Keep milestones per Phase to 3 or fewer by default.
- Tasks are the work needed to complete a milestone. Task count is not capped, but a healthy milestone usually has at least 3 tasks. If a milestone has too few tasks, re-check whether the milestone or Phase split is too fine-grained.
- Milestone breakdown does not create a `task-0-breakdown-*` first task inside the milestone. The first numbered task skeleton is real product/planning work, starts at `task-1-*`, and milestone-start bookkeeping happens when the whole milestone starts, not as a milestone task.
- Within a milestone, run multiple tasks in parallel when there is no real dependency and file ownership/conflict risk is manageable. Record true dependencies in the milestone task index; do not serialize independent work by habit.
- Publish `phase-plan` work in one PR. Publish milestone breakdowns in one PR across all planned milestones when feasible; if dependency order requires staging, record why one PR would be worse.
- One task = one worktree = one branch = one PR. All task-related four-piece, Dev design, implementation, tests, docs/current sync, progress, and acceptance state land in that task PR; do not open a closure/status follow-up PR for state that belongs to the task.
- Do not create excessive PRs for pure documentation or process work. The goal of Blueprintflow planning is to get to feature development and ship the feature.

## Blueprintflow Operating Rules

- Main/parent Teamlead context is orchestration only: advance workflow, decide gates, dispatch workers, synthesize results, and preserve context.
- Main/parent Teamlead context must not run `git` or `gh`. Delegate all git/GitHub work asynchronously to workers, including status, diff, commits, pushes, PR creation/checks/merge, branch/worktree cleanup, and CI gate polling.
- Main context should avoid leaf implementation and detailed verification; delegate those to workers and synthesize outcomes.

## Worktree Rule — 任何修改必须走 worktree, 无例外

任何代码 / 配置 / 文档修改, **必须**先在 `.worktrees/<task-or-fix-slug>` 起独立 worktree 再动手, 不准在主工作树 (`/workspace/borgee` 当前 checkout) 直接改. 包括看起来"很小"的 bug fix, 单文件改动, 临时实验 — 一律 worktree.

理由: 主工作树要长期保持 clean 才能随时切分支 / 拉 main; 主树直接改会污染 in-flight 状态, agent 之间也会撞车. 一 task = 一 worktree = 一 branch = 一 PR (跟上面 Blueprintflow Planning Rules 第 13 条同源).

操作:
```bash
# 起 worktree (跟 task slug 同名分支)
cd /workspace/borgee && git worktree add .worktrees/<slug> -b fix/<slug>
cd .worktrees/<slug>
# 在这里改, commit, push, 开 PR
```

PR 合并后清理三件套:
```bash
git -C /workspace/borgee worktree remove .worktrees/<slug>
git -C /workspace/borgee branch -D fix/<slug>
```

如发现已经在主树动了手, 立刻 `git stash` → 起 worktree → `git stash pop`, 再继续.

## 本地 e2e — 见 skill

任何要做本地端到端测试 (helper / openclaw / install / configure / channel-bridge / 任何同时跨 web UI + Linux VM 的流程) 之前, 必须**先读** `.claude/skills/borgee-local-e2e/SKILL.md`.

核心铁律 (skill 全文展开):
- server-go 跑在**宿主**, 不在容器
- dev-vm 是**干净 Ubuntu+systemd 容器**, 不预装 borgee/Node/openclaw 任何东西
- server-go 跟 dev-vm **绝对禁止**塞同一 docker network (历史 `scripts/dev-stack/` 同网骨架是错例, 已删)
- e2e 你 = 用户, 所有命令 / token / origin **从 web UI 拿**, 不准 hardcode
- dev-vm 是**一次性 fixture**, 跑完 `docker compose down -v` 删干净, 下次重起干净环境

