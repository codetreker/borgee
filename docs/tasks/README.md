# tasks/

每个 milestone 或 issue 一个文件夹. spec / design / acceptance / regression / progress 都放在同一文件夹里.

## 命名规则

- 蓝图 milestone: 用蓝图代号, 如 `al-2a-content-lock` / `chn-4-cross-org`
- feature / bugfix: 用 issue 号 + 简短描述, 如 `698-agent-config-form-overlap` / `716-e2e-real-ui-audit`
- 不允许只用 milestone 编号或 issue 号开头无描述 (反 m698 / gh698 模式)

## 文件夹里放什么

按需要选, 不强制四件套:

| 文件 | 什么时候要 |
|---|---|
| `spec.md` | 任何 task 都要 — 写要做什么 / 不做什么 / 边界 |
| `design.md` | 实施前的技术方案 (改哪里 / 怎么改) |
| `acceptance.md` | 验收清单 (用户能看到的行为) |
| `regression.md` | 跨 milestone 长期回归项 (如果有) |
| `progress.md` | 实施进度 / 已合 PR / 未做完的尾巴 |

## 当前在做的

- [716-e2e-real-ui-audit](716-e2e-real-ui-audit/) — 全量审计 e2e case 删假 grep 充数 (P0, in-flight)
- gh#718 — 全 repo 黑话整治 (P2, in-flight). sed 时代 12 PR (#721/#723/#725/#726/#727/#728/#729/#730/#731/#732/#733/#734) 已合, 用户拍方法学违反 issue 原文重做; v13+ 按语境读真做, 已合 9 PR (#738/#739/#740/#742/#743/#744/#745/#746/#747) / 50 文件 / 余 ~490 文件 / 估 ~85 PR. v17-v21 累计 0/217 = 0% 漏率 5 连标杆 (v22 起加 Pre-flight Layer 7 防注意力疲劳). 无独立 tasks/ 目录.
- gh#724 — 已 triage (Task / backlog / p2-normal). zhanma-c follow-up: ArtifactComments 系列 mount + ACL forbidden state UX, 留 v2 brainstorm 输入

**v2 brainstorm 输入 (不是 current-iteration)**:

- [681-remote-agent-openclaw](681-remote-agent-openclaw/) — Remote-Agent 网页配 OpenClaw (P1 backlog). PR #720 (`borgee-helper-v1-release/` wave 容器) WIP, 4 角色 review feedback 留作 v1 收尾后 v2 brainstorm 输入, **不进 current-iteration**.

## 已闭环

进 `_archive/`. 以下已合 milestone:

- [_archive/698-agent-config-form-overlap/](../_archive/698-agent-config-form-overlap/) — Manage 展开后 form label/input 重叠 (PR #706 已合, gh#698 closed)
- [_archive/717-release-gate-cleanup/](../_archive/717-release-gate-cleanup/) — 删 release-gate / al-release-gate workflow + 真行为 test 替临时字符串 grep (PR #722 已合, gh#717 closed)
