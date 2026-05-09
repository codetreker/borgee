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

- [716-e2e-real-ui-audit](716-e2e-real-ui-audit/) — 全量审计 e2e case 删假 grep 充数 (P0)
- [698-agent-config-form-overlap](698-agent-config-form-overlap/) — Manage 展开后 form label/input 重叠 (P2)
- [borgee-helper-v1-release/](borgee-helper-v1-release/) — Borgee Helper v1 release wave (gh#681, 8 milestone HB-7~HB-14, P1)

## 已闭环

进 `_archive/`. 以后新 milestone 闭环走 `tasks/archived/<m>/` (现在还空).
