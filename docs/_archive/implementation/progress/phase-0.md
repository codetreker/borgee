# Phase 0 — 基建闭环 (detail)

> 引自 [PROGRESS.md](../PROGRESS.md) 概览表 Phase 0 行 — milestone 翻牌点单源在此.

## Phase 0 — 基建闭环

**Milestones**

- [x] **INFRA-1a** schema_migrations 框架 — 战马 / 飞马 / 烈马
  - [x] PR-INFRA-1a.1 框架代码 + 跑一次假迁移 (PR #169, coverage 90.3%)
- [x] **INFRA-1b** 测试 harness — 战马 (主) / 飞马 / 烈马
  - [x] PR-INFRA-1b.1 fake/real Clock (PR #171, coverage 100%)
  - [x] PR-INFRA-1b.2 内存 sqlite + fixture seeder (PR #172, coverage 91.7%)
  - [x] PR-INFRA-1b.3 回归入册 + `make regression` (PR #173, coverage 100%)
- [x] **CI lint** PR 改 internal 必同步 docs/current — 战马 (实现) / 烈马 (验证) (PR #170)
- [x] **PR 模板生效** Blueprint / Touches / Current 同步 三区块强制 — 飞马 (PR #170)

**Gates**

- [x] G0.1 schema_migrations 能跑 — 证据: PR #169 `internal/migrations/migrations_test.go` 8 用例 PASS, coverage 90.3%
- [x] G0.2 acceptance 验证脚本 (1 fail + 1 pass) — 证据: PR #170 `pr-template` lint 自检, run [25008169145](https://github.com/codetreker/borgee/actions/runs/25008169145) FAIL → run [25008849364](https://github.com/codetreker/borgee/actions/runs/25008849364) PASS
- [x] G0.3 PR 模板生效 (≥ 1 PR 三区块齐) — 证据: PR #169-#173 全部含 `Blueprint:`/`Touches:`/`Current 同步`/`Stage:` 五块, lint 全绿
- [x] G0.4 测试 harness 可用 (1 个 fake clock 用例跑通) — 证据: PR #171 `TestAfterFiresWhenDeadlineCrossed` Advance 触发已注册 After waiter PASS; 烈马本地联合 smoke (fake clock + OpenSeeded + Advance) 一次通过
- [x] G0.5 current sync CI lint 工作 — 证据: [`docs/_archive/evidence/g0.5/README.md`](../evidence/g0.5/README.md) (双向闭环: fail 路径 PR #170 第一推送拒绝 + pass 路径 #170-#173 全绿; exclude_globs 防纯测试 PR 误伤)
- [x] **G0.audit** v0 代码债 audit 表本 Phase 行已登记 — 飞马 (README §audit: schema_migrations 框架 DONE + main flaky test TODO 已入表)


## 更新日志归档 (历史 changelog 迁入)

| 日期 | 更新人 | 变化 |
|------|--------|------|
| 2026-04-28 | 烈马 | Phase 0 闭环: PR #169-#173 全 merged + Gates G0.1-G0.5 + G0.audit 全 ✅, G0.5 evidence 落 `docs/_archive/evidence/g0.5/README.md` |

---
