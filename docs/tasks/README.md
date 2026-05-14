# tasks/

## Phase Index

| Phase | Status | Exit condition | Current milestone |
|---|---|---|---|
| Phase 1: Helper Actuator Trust Preflight | PLANNED | `HB-RA-1A` boundary guardrail lock has strict, user-perceivable, carry-over, and fake-green checks before helper actuator implementation begins | `milestone-1-boundary-guardrail-lock` |

This Phase Index records the new execution path opened by the locked `HB-RA-1A` next-blueprint anchor. The legacy `681-remote-agent-openclaw/` folder remains intake history and is not the execution path for the Helper actuator work.

每个 milestone 或 issue 一个文件夹. spec / design / acceptance / regression / progress 都放在同一文件夹里.

## 命名规则

- 蓝图 milestone: 用蓝图代号, 如 `al-2a-content-lock` / `chn-4-cross-org`
- feature / bugfix: 用 issue 号 + 简短描述, 如 `698-agent-config-form-overlap` / `716-e2e-audit`
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

- gh#718 — 全 repo 黑话整治 (P2, in-flight). sed 时代 12 PR (#721/#723/#725/#726/#727/#728/#729/#730/#731/#732/#733/#734) 已合, 用户拍方法学违反 issue 原文重做; v13+ 按语境读真做, 已合 53 PR / 267 文件 / 余 ~273 文件 / 估 ~45 PR. v17-v64 累计 0/1648 = 0% 漏率 49 PR (含 v21 + v34 + v38 + v55 + v63.2 五次 conditional-then-fixed, v55 L17 borderline 修, v58.1 push-rebase NACK→fix, v62.1 + v63.1/v63.2 + v64.1 三次 expanded-grep force-push 扩 cumulative 词典 12 词). **3 大里程碑达成** (docs/current/client/ 33 .md 文件 + docs/current/server/ ~32 文件 / 5 子目录 + packages/borgee-helper/ 24 Go 文件 production+test+e2e). Layer 7 防注意力疲劳 39 次成功. v34/v38 教训触发 Layer 7 词典扩 (single-source-of-truth + stance + SSOT + 拆死 + 二闸 + 单源 + 锁精准式). v45v3 → v46 → v47 → v48 → v49 → v50 → v51 → v52 → v53 → v54 → v55 → v56 → v57 → v57b → v58 → v59 → v60 → v61 → v62 → v63 → v64 真合规 PR #1-#21 (反字典化 / 真 per-context / 诚实同译 / iterator var 不动 / 包名不动 / content-lock 字面不动 / anchor check / 保守 sweeping / **cross-PR consistency 100% stable 复用** — 标杆八件套). **v55 blueprint 4 角色 review path 完成** (yema + feima + liema + heima 联签, L17 `五条产品立场` borderline conditional-then-fixed). **v58.1 push-rebase 协议加固** (memory `dev_push_must_rebase_main_first` 5th 守则: push 前一秒 fetch + 重 rebase atomic chain, parallel mode ~5-10 min/PR 节奏下 main HEAD 跳得快). **autonomous mode 稳态** (yema 自治 v60+ 不停, push 前一秒 fetch + cumulative grep 自检 + body REST API PATCH 三协议). cumulative per-context 词典稳态 (12 stable mapping + 7 特殊语境 + 7 新扩展 + 12 v63 NACK 新扩 cross-PR 验证 — `反约束 → 反向检查` v57b+v57 / `drift → mismatch+漂移` v56 双译 + v58-v61 单译 stable / `立场 → 设计+约束+约定` v56-v57 三语境 / `承袭 → 去掉` v53+v57b+v62.1 / `单源 → 单一来源` v56.1 引入 v58-v61 全沿用 / `拆死 → 分立` v56.1 引入 v58 沿用 / `Stance → 约定` v52+v58 / 6 源词混合 v51 / **v63 NACK 12 词扩** v62.1+v63.1/v63.2+v64.1: `byte-identical → 字节级一致` / `设计 ①-⑦ → 设计第 1-7 条` / `锚点 → 字面 (must-contain context)` / `反 X → 不允许 X` / `legacy caller → 为兼容老调用方保留` / `反断 → 反向断言` / `真 X 系列禁词`). v51 6 源词混合验证 cumulative 词典稳定. sed era 平反 (v49 修 7a/7b/7c 漏率 82% 名单中 3/4 文件). 无独立 tasks/ 目录.
- gh#724 — 已 triage (Task / backlog / p2-normal). zhanma-c follow-up: ArtifactComments 系列 mount + ACL forbidden state UX, 留 v2 brainstorm 输入

**v2 brainstorm 输入 (不是 current-iteration)**:

- [681-remote-agent-openclaw](681-remote-agent-openclaw/) — Remote-Agent 网页配 OpenClaw (P1 backlog). PR #720 (`borgee-helper-v1-release/` wave 容器) WIP, 4 角色 review feedback 留作 v1 收尾后 v2 brainstorm 输入, **不进 current-iteration**.

## 已闭环

进 `_archive/`. 以下已合 milestone:

- [_archive/698-agent-config-form-overlap/](../_archive/698-agent-config-form-overlap/) — Manage 展开后 form label/input 重叠 (PR #706 已合, gh#698 closed)
- [_archive/716-e2e-audit/](../_archive/716-e2e-audit/) — 全量审计 e2e case 删假 grep 充数 (PR #794 merged, gh#716 closed)
- [_archive/717-release-gate-cleanup/](../_archive/717-release-gate-cleanup/) — 删 release-gate / al-release-gate workflow + 真行为 test 替临时字符串 grep (PR #722 已合, gh#717 closed)
