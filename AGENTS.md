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
