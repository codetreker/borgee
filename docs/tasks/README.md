# tasks/

## Phase Index

| Phase | Status | Exit condition | Current milestone |
|---|---|---|---|
| Phase 1: Helper / OpenClaw Onboarding | IMPLEMENTING | A user can enroll Helper once, configure OpenClaw from Web, see truthful status/logs, and revoke the delegation without merging Helper and Remote Agent rails | `milestone-1-helper-enrollment-status` |
| Phase 2: Collaboration Channel Control | TASK_SET_READY | A user can control agent attention and channel membership/authority without hidden fanout, confusing leave actions, or overloaded private-channel indicators | `milestone-1-mention-delivery-controls` |
| Phase 3: Client Truth And Navigation | TASK_SET_READY | Production-visible surfaces are reachable and forbidden states are truthful, while account/sidebar IA exposes the right primary entries without expanding privacy/compliance product scope | `milestone-1-production-surface-truthfulness` |

This Phase Index records the v1.1 execution path opened from the selected next-blueprint anchors. The legacy `681-remote-agent-openclaw/` folder remains intake history and is not the execution path for the Helper actuator work.

Current execution resume state:

- Milestone breakdown is accepted across all 8 v1.1 milestones.
- Accepted task: `phase-1-helper-openclaw-onboarding/milestone-1-helper-enrollment-status/task-1-helper-enrollment-model-and-status` via PR #934, merged at `547f869`.
- Active task: `phase-1-helper-openclaw-onboarding/milestone-1-helper-enrollment-status/task-2-helper-credential-rotation-and-revoke`.
- Current gate: tasking/four-piece and Dev design review before implementation; PR #935 was closed, so this task PR carries the task-1 acceptance-state remediation.

## Active Task Resume

| Scope | Execution | Active task | Owner | Worktree/branch | PR | Blocker | Progress |
|---|---|---|---|---|---|---|---|
| `phase-1-helper-openclaw-onboarding/milestone-1-helper-enrollment-status` | TASKING | `task-2-helper-credential-rotation-and-revoke` | Dev/Writer helper under Teamlead | `.worktrees/task-2-helper-credential-rotation-and-revoke` / `feat/task-2-helper-credential-rotation-and-revoke` | pending | none | `docs/tasks/phase-1-helper-openclaw-onboarding/milestone-1-helper-enrollment-status/task-2-helper-credential-rotation-and-revoke/progress.md` |

每个 milestone 或 issue 一个文件夹. spec / design / acceptance / regression / progress 都放在同一文件夹里.

## 命名规则

- 蓝图 milestone: 用蓝图代号, 如 `al-2a-content-lock` / `chn-4-cross-org`
- feature / bugfix: 用 issue 号 + 简短描述, 如 `698-agent-config-form-overlap` / `716-e2e-audit`
- 不允许只用 milestone 编号或 issue 号开头无描述 (反 m698 / gh698 模式)

## 文件夹里放什么

Milestone breakdown 只创建 `task.md` skeleton。下面这些文件在具体 task 进入 `bf-task-execute` 后按需要创建，不在 breakdown PR 里预生成:

| 文件 | 什么时候要 |
|---|---|
| `spec.md` | task 开始后创建 — 写要做什么 / 不做什么 / 边界 |
| `design.md` | 实施前的技术方案 (改哪里 / 怎么改) |
| `acceptance.md` | 验收清单 (用户能看到的行为) |
| `regression.md` | 跨 milestone 长期回归项 (如果有) |
| `progress.md` | 实施进度 / 已合 PR / 未做完的尾巴 |

## 其他记录

- gh#718 — 全 repo 黑话整治仍按既有后台队列跟进，不属于本 v1.1 Phase/Milestone plan。
- gh#724 — 已纳入 v1.1 `phase-3-client-truth-navigation/milestone-1-production-surface-truthfulness`，不再标为 v2 brainstorm 输入。
- [681-remote-agent-openclaw](681-remote-agent-openclaw/) — legacy intake history only. Its Helper/OpenClaw scope is superseded by `phase-1-helper-openclaw-onboarding/`; do not resume this folder as the execution path.

## 已闭环

进 `_archive/`. 以下已合 milestone:

- [_archive/698-agent-config-form-overlap/](../_archive/698-agent-config-form-overlap/) — Manage 展开后 form label/input 重叠 (PR #706 已合, gh#698 closed)
- [_archive/716-e2e-audit/](../_archive/716-e2e-audit/) — 全量审计 e2e case 删假 grep 充数 (PR #794 merged, gh#716 closed)
- [_archive/717-release-gate-cleanup/](../_archive/717-release-gate-cleanup/) — 删 release-gate / al-release-gate workflow + 真行为 test 替临时字符串 grep (PR #722 已合, gh#717 closed)
