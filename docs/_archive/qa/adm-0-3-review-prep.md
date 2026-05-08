# ADM-0.3 Review Prep — 5 分钟过审 checklist

> 飞马 · 2026-04-28 · 战马A ADM-0.3 PR 预备
> 引用: `docs/implementation/modules/adm-0-review-checklist.md` §ADM-0.3 + #197 review 5 条盯点

## 1. 5 条盯点逐项

| # | 盯点 | 看文件 | 通过条件 |
|---|------|--------|---------|
| P1 | ADM-0.2 临时 wildcard `(*, *)` 清理 | `internal/testutil/server.go` | grep `permission='\*'\|Scope.*\*` 命中 0 (除 admins) |
| P2 | backfill 顺序锁 (INSERT admins → DELETE sessions → DELETE users) | `internal/migrations/adm_0_3_users_role_collapse.go` Up() | SQL 三步顺序; 任一失败整 tx 回滚 |
| P3 | idempotent 反向断言 | `..._test.go` | sub-test 跑 Up() 两次, admins 行不变 + users role='admin' 仍 0 |
| P4 | testutil `role:"admin"` 全删 | `internal/testutil/` + `_test.go` | `grep -rEn '"admin"' \| grep -i role` 命中 0 |
| P5 | Go const `RoleAdmin` 删 | `internal/store/models.go` / `auth/roles.go` | `grep -rEn 'UserRoleAdmin\|RoleAdmin' internal/` 命中 0 |

## 2. checklist §ADM-0.3 数据契约

- v=10 在 registry.go 末尾 (v=6 已跳, 见注释) · DDL 走 SQLite 表重建 (CREATE users_new + INSERT SELECT + DROP + RENAME) 加 CHECK · post-migration `SELECT count(*) FROM users WHERE role='admin' == 0` (单测 4.1.d) · admins backfill 复用 login + password_hash · 旧 admin user-api session 全 DELETE

## 3. 行为不变量 + LOC

- 4.1.a admin cookie → user-api 401 (从 ADM-0.2 保留) · 4.1.d users role='admin' = 0 · 双轨彻底关 (`admin_auth.go` 已 #201 删, 不可复活) · ≤ 800 LOC · forward-only (无 Down) · 配套 docs/current/server/{migrations.md §7 v=10 + data-model.md users.role `(member/agent)` + admins / admin_sessions 行} 一并验 (audit #212 D10d / D11)

## 4. 拒收红线

❌ 加 Down() · ❌ admins 加 org_id/role/is_admin/email · ❌ backfill 顺序颠倒 (先删 users 再插 → 数据丢) · ❌ 残留 `RoleAdmin` const · ❌ docs/current/server/ 没动 (规则 6)
