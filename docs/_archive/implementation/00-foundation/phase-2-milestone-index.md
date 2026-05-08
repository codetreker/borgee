# Phase 2 Milestone Index — dev 速查页

> 飞马 · 2026-04-28 · Phase 2 全 milestone 一行速查 (跟 PROGRESS.md 区分: 给 dev / reviewer 看代码点 + acceptance + regression)
> 状态汇总: ✅ 6 / ⏳ 3 — 待 ADM-0.3 + RT-0 server + RT-0 client merge 后 Phase 2 解封进闸 4

## 索引表

| Milestone | PR | LOC | mig v | 主代码点 | acceptance template | regression | status |
|-----------|----|-----|-------|----------|---------------------|------------|--------|
| INFRA-2 (Playwright) | #195 | 743 | - | `packages/e2e/playwright.config.ts` | `acceptance-templates/infra-2.md` | `REG-INFRA-2-*` | ✅ |
| ADM-0.1 admins 表 + env bootstrap | #197 | 780 | v=4 | `internal/admin/auth.go` | `acceptance-templates/adm-0.md §0.1` | `REG-ADM0-001/004/008` | ✅ |
| ADM-0.2 cookie 拆 + 短路砍 + god-mode 白名单 | #201 | 621 | v=5 | `internal/admin/sessions.go` | `adm-0.md §0.2` | `REG-ADM0-002/003` | ✅ |
| CM-onboarding Welcome channel | #203 | 812 | v=7 | `internal/api/auth.go::register` | `acceptance-templates/cm-onboarding.md` | `REG-CMO-*` | ✅ |
| AP-0-bis message.read 默认 + backfill | #206 | 410 | v=8 | `internal/api/messages.go RequirePermission` | `acceptance-templates/ap-0-bis.md` | `REG-APB-*` | ✅ |
| CM-3 资源归属 org_id 直查 | #208 | 394 | v=9 | `internal/store/queries_cm3.go` | `cm-3-resource-ownership-checklist.md` | `REG-CM3-*` | ✅ |
| ADM-0.3 users.role enum 收 + backfill | TBD (task #63) | TBD | v=10 | `internal/migrations/adm_0_3_users_role_collapse.go` (predicted) | `adm-0.md §0.3` | `REG-ADM0-005/007/010` | ⏳ |
| RT-0 client (ws frames + listener) | #218 | 308 | - | `packages/client/src/types/ws-frames.ts` | `acceptance-templates/rt-0.md (client)` | `REG-RT0-*` | ⏳ review |
| RT-0 server (hub.Broadcast + e2e flip) | TBD (task #40) | TBD | - | TBD `internal/ws/hub.go` push site | `rt-0.md (server)` | TBD | ⏳ |

## 备注

- v=6 跳号 (历史预留 ADM-0.3 早期方案, 见 `registry.go` 注释)
- ADM-0.3 必须落地后 G2.0 才完整 (`users WHERE role='admin'` count=0 即 4.1.d)
- RT-0 拆 client/server 两 PR 是 #218 战马B 战术决定 (避免与战马A ADM-0.3 worktree 串)
- regression registry 主体在 烈马 #193 + Phase 2 regression registry (#49); 每条 `REG-*` 锚点见对应 acceptance template
- LOC 全部 ≤ 800 软上限, 仅 CM-onboarding (812) 因 onboarding-journey 联动 doc 同 PR 略 over (review 已批)

## 阅读指南

- **想知道某个 milestone 改了哪些代码** → 主代码点列
- **想跑 acceptance 验** → acceptance template 列 (含 12 项 R3 模板)
- **CI regression 抓哪条** → regression 列 (REG-* 锚 #193 表)
- **想看时间线 / 派活归谁** → `PROGRESS.md`
- **想看闸 gate 状态** → `phase-2-exit-gate.md` (#221)
