# tasks/

## Authoritative v1.1 Phase Index

| Phase | Status | Exit condition | Current milestone |
|---|---|---|---|
| Phase 1: v1.1 Trust And Usability Closure | IMPLEMENTING | Close selected v1.1 trust/usability gaps: Helper/OpenClaw bounded actuator onboarding, channel attention/authority clarity, production client truthfulness, and account/sidebar IA, without expanding privacy/compliance scope or merging Helper/Remote Agent rails | `phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator` |

This one-phase index supersedes the earlier v1.1 `3 phases / 8 milestones` shape. A new Phase must now require a real prerequisite boundary, integration boundary, or downstream coordination reason. The old Phase 2 and Phase 3 slices were execution slots, not dependency or integration boundaries; the old Phase 1 milestones were one prerequisite chain toward the same user-facing Helper/OpenClaw loop. Coarser Phase/Milestone structure is authoritative for new execution.

The legacy `681-remote-agent-openclaw/` folder remains intake history and is not the execution path for the Helper actuator work. The old `phase-1-helper-openclaw-onboarding/`, `phase-2-collaboration-channel-control/`, and `phase-3-client-truth-navigation/` folders are retained as accepted task history and task-detail homes. Their canonical execution grouping is now recorded under `phase-1-v11-trust-usability-closure/`.

Current execution resume state:

- Phase/Milestone structure is replanned into one active Phase with three coarse milestones.
- Accepted history is preserved and remapped: PR #934 (`547f869`), PR #936 (`1ca5f95`), PR #937 (`2872905`), PR #938 (`64d56f1`), and PR #939 (`96dc0dc`) remain accepted work.
- Milestone 1 is active. Next ready work is the Helper/OpenClaw bounded actuator continuation after PR #939: `task-3-helper-pull-lease-result` and `task-4-local-policy-manifest-and-sandbox-profile` may start in parallel if file ownership/conflict risk is manageable.
- Current gate: task-start/four-piece review for the next ready Helper/OpenClaw tasks. Product implementation has not started for those remaining tasks.

## Active Task Resume

| Scope | Execution | Next task(s) | Owner | Worktree/branch | PR | Blocker | Progress |
|---|---|---|---|---|---|---|---|
| `phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator` | TASK_READY | `phase-1-helper-openclaw-onboarding/milestone-2-typed-job-policy-loop/task-3-helper-pull-lease-result`; `phase-1-helper-openclaw-onboarding/milestone-2-typed-job-policy-loop/task-4-local-policy-manifest-and-sandbox-profile` | Blueprintflow tasking workers under Teamlead | create one worktree/branch per task | not opened | none | See canonical milestone mapping below and retained task folders |

## Canonical v1.1 Milestone Mapping

| Canonical milestone | Status | Remapped prior folders | Resume notes |
|---|---|---|---|
| `phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator` | IMPLEMENTING | `phase-1-helper-openclaw-onboarding/milestone-1-helper-enrollment-status`; `phase-1-helper-openclaw-onboarding/milestone-2-typed-job-policy-loop`; `phase-1-helper-openclaw-onboarding/milestone-3-configure-openclaw-closure` | Accepted work through PR #934, #936, #937, #938, and #939 is preserved. Continue with pull/lease/result and local policy/manifest/sandbox tasks before terminal settlement and Configure OpenClaw closure. |
| `phase-1-v11-trust-usability-closure/milestone-2-channel-attention-and-authority` | PLANNED | `phase-2-collaboration-channel-control/milestone-1-mention-delivery-controls`; `phase-2-collaboration-channel-control/milestone-2-channel-management-authority`; `phase-2-collaboration-channel-control/milestone-3-channel-visual-truth` | Old Phase 2 was an execution slot. It is now one milestone covering requireMention policy, `@Everyone`, client mention controls, channel management, allowed-action rules, authority checks, private indicator treatment, and sidebar state collision regression. |
| `phase-1-v11-trust-usability-closure/milestone-3-client-truth-and-navigation` | PLANNED | `phase-3-client-truth-navigation/milestone-1-production-surface-truthfulness`; `phase-3-client-truth-navigation/milestone-2-sidebar-account-entry` | Old Phase 3 was an execution slot. It is now one milestone covering production client truthfulness, forbidden-state UX, Settings PermissionsView reachability, reverse proof, sidebar/footer IA, avatar/logout, and Helper/Remote Nodes placement. |

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
- gh#724 — 已纳入 v1.1 canonical `phase-1-v11-trust-usability-closure/milestone-3-client-truth-and-navigation`，不再标为 v2 brainstorm 输入。
- [681-remote-agent-openclaw](681-remote-agent-openclaw/) — legacy intake history only. Its Helper/OpenClaw scope is superseded by `phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator`; do not resume this folder as the execution path.

## 已闭环

进 `_archive/`. 以下已合 milestone:

- [_archive/698-agent-config-form-overlap/](../_archive/698-agent-config-form-overlap/) — Manage 展开后 form label/input 重叠 (PR #706 已合, gh#698 closed)
- [_archive/716-e2e-audit/](../_archive/716-e2e-audit/) — 全量审计 e2e case 删假 grep 充数 (PR #794 merged, gh#716 closed)
- [_archive/717-release-gate-cleanup/](../_archive/717-release-gate-cleanup/) — 删 release-gate / al-release-gate workflow + 真行为 test 替临时字符串 grep (PR #722 已合, gh#717 closed)
