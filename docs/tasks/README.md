# tasks/

## Authoritative v1.1 Phase Index

| Phase | Status | Exit condition | Current milestone |
|---|---|---|---|
| Phase 1: v1.1 Trust And Usability Closure | IMPLEMENTING | Close selected v1.1 trust/usability gaps: Helper/OpenClaw bounded actuator onboarding, channel attention/authority clarity, production client truthfulness, and account/sidebar IA, without expanding privacy/compliance scope or merging Helper/Remote Agent rails | `phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator` |

This one-phase index supersedes the earlier v1.1 `3 phases / 8 milestones` shape. A new Phase must now require a real prerequisite boundary, integration boundary, or downstream coordination reason. The prior channel-control and client-truth slices were execution slots, not dependency or integration boundaries; the prior Helper/OpenClaw milestones were one prerequisite chain toward the same user-facing Helper/OpenClaw loop. Coarser Phase/Milestone structure is authoritative for new execution.

The legacy `681-remote-agent-openclaw/` folder remains intake history and is not the execution path for the Helper actuator work. Accepted Helper/OpenClaw task docs and the remaining unexecuted Helper/OpenClaw skeletons now live under the canonical Milestone 1 directory in `phase-1-v11-trust-usability-closure/`. The unexecuted channel-control and client-truth task skeletons also live only under the canonical milestones in `phase-1-v11-trust-usability-closure/`; their former phase folders were removed to avoid presenting multiple active phases.

Current execution resume state:

- Phase/Milestone structure is replanned into one active Phase with three coarse milestones.
- Accepted history is preserved and remapped: PR #934 (`547f869`), PR #936 (`1ca5f95`), PR #937 (`2872905`), PR #938 (`64d56f1`), PR #939 (`96dc0dc`), PR #942 (`642fb57`), and PR #943 (`c2c61e`) remain accepted work.
- Milestone 1 is active. Task 8 `phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator/task-8-bounded-status-logs-and-revoke-settlement` is implementing from a dedicated worktree after accepted Task6 and Task7 dependencies.
- Milestone 3 has an explicit parallel start for `task-1-artifactcomments-production-mount`; it is file-isolated from Helper task 6, Settings Permissions, and sidebar/footer work.
- Current gate: task 8 implementation, verification, PR, and merge.

## Active Task Resume

| Scope | Execution | Next task(s) | Owner | Worktree/branch | PR | Blocker | Progress |
|---|---|---|---|---|---|---|---|
| `phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator` | ACCEPTING | `task-8-bounded-status-logs-and-revoke-settlement` | M1 Task8 owner worker | `.worktrees/task-8-bounded-status-logs-and-revoke-settlement` / `feat/task-8-bounded-status-logs-and-revoke-settlement` | #954 | none | `phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator/task-8-bounded-status-logs-and-revoke-settlement/progress.md` |
| `phase-1-v11-trust-usability-closure/milestone-3-client-truth-and-navigation` | ACCEPTING | `task-3-security-permission-surface-reachability` | owner worker | `.worktrees/m3-task3-settings-permissionsview-reachability` / `feat/m3-task3-settings-permissionsview-reachability` | #944 | none | `phase-1-v11-trust-usability-closure/milestone-3-client-truth-and-navigation/task-3-security-permission-surface-reachability/progress.md` |
| `phase-1-v11-trust-usability-closure/milestone-2-channel-attention-and-authority` | ACCEPTING | `task-7-private-indicator-state-inventory` | Blueprintflow owner worker under Teamlead | `.worktrees/task-7-private-indicator-state-inventory` / `feat/task-7-private-indicator-state-inventory` | #945 | none | `phase-1-v11-trust-usability-closure/milestone-2-channel-attention-and-authority/task-7-private-indicator-state-inventory/progress.md` |
| `phase-1-v11-trust-usability-closure/milestone-2-channel-attention-and-authority` | ACCEPTING | `task-4-channel-management-surface` | M2 Task4 owner worker | `.worktrees/m2-task4-channel-management-surface` / `feat/m2-task4-channel-management-surface` | #948 | none | `phase-1-v11-trust-usability-closure/milestone-2-channel-attention-and-authority/task-4-channel-management-surface/progress.md` |
| `phase-1-v11-trust-usability-closure/milestone-2-channel-attention-and-authority` | ACCEPTING | `task-2-everyone-fanout-acl-rate-loop` | M2 Task2 owner worker | `.worktrees/m2-task2-everyone-fanout-acl-rate-loop` / `feat/m2-task2-everyone-fanout-acl-rate-loop` | #951 | CI re-run after current-main rebase | `phase-1-v11-trust-usability-closure/milestone-2-channel-attention-and-authority/task-2-everyone-fanout-acl-rate-loop/progress.md` |
| `phase-1-v11-trust-usability-closure/milestone-2-channel-attention-and-authority` | ACCEPTING | `task-8-private-indicator-visual-treatment` | Blueprintflow owner worker under Teamlead | `.worktrees/task-8-private-indicator-visual-treatment` / `feat/task-8-private-indicator-visual-treatment` | #952 | none | `phase-1-v11-trust-usability-closure/milestone-2-channel-attention-and-authority/task-8-private-indicator-visual-treatment/progress.md` |
| `phase-1-v11-trust-usability-closure/milestone-3-client-truth-and-navigation` | ACCEPTING | `task-1-artifactcomments-production-mount` | Blueprintflow M3 task owner worker | `.worktrees/task-1-artifactcomments-production-mount` / `feat/task-1-artifactcomments-production-mount` | #946 | none | ArtifactComments production mount in PR; no Settings/sidebar overlap |
| `phase-1-v11-trust-usability-closure/milestone-2-channel-attention-and-authority` | ACCEPTING | `task-5-channel-allowed-action-rules` | M2 Task5 owner worker | `.worktrees/m2-task5-channel-allowed-action-rules` / `feat/m2-task5-channel-allowed-action-rules` | #953 | none | `phase-1-v11-trust-usability-closure/milestone-2-channel-attention-and-authority/task-5-channel-allowed-action-rules/progress.md` |

## Canonical v1.1 Milestone Mapping

| Canonical milestone | Status | Task-detail source | Resume notes |
|---|---|---|---|
| `phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator` | IMPLEMENTING | Canonical task homes `task-1-helper-enrollment-model-and-status` through `task-12-configure-openclaw-terminal-ui` | Accepted work through PR #934, #936, #937, #938, #939, #942, and #943 is preserved in `accepted-history.md` and the accepted task folders. Continue with task 8 terminal settlement before Configure OpenClaw closure tasks. |
| `phase-1-v11-trust-usability-closure/milestone-2-channel-attention-and-authority` | PLANNED | Canonical task homes `task-1-requiremention-policy-model` through `task-9-sidebar-state-collision-regression` | The former channel-control execution slot is now one milestone covering requireMention policy, `@Everyone`, client mention controls, channel management, allowed-action rules, authority checks, private indicator treatment, and sidebar state collision regression. |
| `phase-1-v11-trust-usability-closure/milestone-3-client-truth-and-navigation` | IMPLEMENTING | Canonical task homes `task-1-artifactcomments-production-mount` through `task-7-helper-remote-nodes-entry-placement` | Parallel start is limited to `task-1-artifactcomments-production-mount`; later forbidden-state and reverse-proof tasks still depend on this task, while Settings Permissions and sidebar/footer remain separate task lanes. |

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
