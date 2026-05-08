# ADMIN-PASSWORD-PLAIN-ENV stance checklist

> 战马E v0 (野马待 review). 立场跟 spec brief §0+§4 byte-identical.

## 立场 (4 项, 跟 spec §0)

1. ✅ **HASH legacy path byte-identical 不破** — 仅设 BORGEE_ADMIN_PASSWORD_HASH 时行为完全不变, login verify path 走 bcrypt.CompareHashAndPassword 不动.
2. ✅ **PLAIN path 走 MinBcryptCost (10) 不让步** — bcrypt.GenerateFromPassword 内存哈希后写表; env 中明文 plain 永不写盘.
3. ✅ **二选一 fail-loud** — HASH + PLAIN 同设 → panic; 都不设 → panic. 反 silent priority.
4. ✅ **0 endpoint URL / 0 schema / 0 cookie / 0 admin login/logout/me 行为改** — server diff 仅 internal/admin/auth.go (~30 行) + cmd/collab/main.go (≤2 行 Bootstrap call); 0 client 改.

## 反约束 (锚 spec §2 反向 grep)

- env 字面 byte-identical (`BORGEE_ADMIN_PASSWORD_HASH` + `BORGEE_ADMIN_PASSWORD` 两个 const 各 1 hit, 反 typo 漂)
- legacy backward-compat: 既有 4 BootstrapWith 调用全加第 4 参数 `""` (反 break test fixture)
- 0 routes.go / 0 migration v 号 / 0 client diff
- bcrypt cost ≥ MinBcryptCost (10) 守 (review checklist 红线, 反 v2 调低 cost)

## 不在范围 (spec §3 留账)

password rotation / 强度策略 / multi-admin / env-file 加密 全留 v2+. dev/testing operator 自负 plain env 安全.

## 跨 milestone 锁 (spec §4)

- ADM-0.1 #199 server bootstrap env shape 不动
- admin-spa-shape-fix #633 admin login flow byte-identical 不破
- cookie-name-cleanup #634 SSOT 立场承袭 (env 字面 const SSOT 单源)
- ADM-0 §1.3 红线 (本 PR 仅改 admin bootstrap, 不动 user-rail)
- review checklist 1.A fail-loud + 1.B idempotent

## 更新日志

| 2026-05-01 | 战马E | v0 stance — 4 立场 byte-identical 跟 spec brief. 用户拍 B 方案启动. |
