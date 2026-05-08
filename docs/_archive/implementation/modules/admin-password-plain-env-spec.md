# ADMIN-PASSWORD-PLAIN-ENV spec brief — B 方案明文 env (≤80 行)

> 战马E · 2026-05-01 · post-#633/#634 wave 合后启动 · v0
> **关联**: ADM-0.1 #199 admin bootstrap env (BORGEE_ADMIN_LOGIN+PASSWORD_HASH) · admin-spa-shape-fix #633 admin login byte-identical · cookie-name-cleanup #634 SSOT 立场承袭
> **命名**: ADMIN-PASSWORD-PLAIN-ENV = BORGEE_ADMIN_PASSWORD 明文 env 启动自 bcrypt 哈希 + HASH env fallback (二选一, 简化 dev/testing 部署)

> ⚠️ 用户拍 B 方案 — 部署 ergonomics 真改: dev/testing 不再先 htpasswd 算 hash 填 env, 直接明文 env (server 启动时 bcrypt.GenerateFromPassword 内存哈希后写 admins 表). prod 推荐继续走 HASH (env 泄露仅泄哈希).

## 0. 关键约束 (4 条立场)

1. **`BORGEE_ADMIN_PASSWORD_HASH` legacy path byte-identical 不破** (ADM-0.1 立场承袭): 仅设 HASH → 行为完全不变 (server 直存 hash 字面到 admins.password_hash, login verify 走 bcrypt.CompareHashAndPassword 不动). 反约束: legacy bootstrap test (TestBootstrap_1A_Panics + 1B_Idempotent + HashPriority_BackwardCompat) 全 PASS byte-identical.

2. **新 PLAIN path 走 MinBcryptCost (10) 不让步**: `BORGEE_ADMIN_PASSWORD` plain → bcrypt.GenerateFromPassword([]byte(plain), MinBcryptCost) → 写 admins.password_hash. cost ≥ 10 review checklist 红线守; env 中明文 plain 永不写盘 (内存哈希后 env 不再读).

3. **二选一 fail-loud (反 surprise / 反 silent priority)**: HASH + PLAIN 同时设 → bootstrap panic mutually exclusive (msg 提两个 env 名). 都不设 → bootstrap panic 至少设一个 (msg 提两个 env 名 + cost ≥ 10). 反约束: TestBootstrap_BothEnvSet_Panics + NeitherEnv_Panics 真测 panic msg 反向锚.

4. **0 endpoint URL / 0 schema / 0 cookie / 0 admin login/logout/me 行为改**: server diff scope = `internal/admin/auth.go` 加 const + Bootstrap/BootstrapWith ~30 行; **0 routes.go / 0 migration v 号 / 0 client UI 改 / 0 cookie 字面值改**. 反约束: `git diff origin/main -- packages/server-go/cmd/ packages/server-go/internal/migrations/ packages/client/src/` = 0 行 (cmd/collab Bootstrap 调签字符串扩 1 字段 = ≤2 行).

## 1. 拆段实施 (单 milestone 一 PR)

| 段 | 范围 |
|---|---|
| **APE.1 server const + bootstrap** | `internal/admin/auth.go` 加 `EnvAdminPassword = "BORGEE_ADMIN_PASSWORD"` const; Bootstrap()/BootstrapWith() 加 plain string 参数; 二选一 panic + GenerateFromPassword 内存哈希 (≤30 行). cmd/collab/main.go Bootstrap call 跟随 (≤2 行). |
| **APE.2 既有 BootstrapWith 调用跟随 + 4 新 unit + docs/current/admin** | `auth_bootstrap_test.go` + `auth_handlers_test.go` + `auth_test.go` + `middleware_test.go` 既有 BootstrapWith callsite 全加第 4 参数 `""` (legacy backward-compat). 加 4 new unit case (PlainEnv happy / BothEnvSet panics / HashPriority backward compat / NeitherEnv panics). docs/current/admin/README.md 加段说明二选一 + 安全 note. |
| **APE.3 closure** | REG-APE-001..006 + acceptance + 4 件套 spec 第一件; 既有 server-go ./internal/admin + ./internal/api + ./internal/auth + ./internal/server 全包不破; 0 client diff. |

## 2. 反向 grep 锚 (6 反约束)

```bash
# 1) const SSOT 立 + env 名字面 byte-identical
grep -nE 'EnvAdminPassword\b' packages/server-go/internal/admin/auth.go  # ≥1 hit
grep -cE '"BORGEE_ADMIN_PASSWORD"' packages/server-go/internal/admin/auth.go  # ==1 (SSOT 一行)

# 2) bcrypt.GenerateFromPassword 真挂 + MinBcryptCost 守
grep -nE 'bcrypt\.GenerateFromPassword.*MinBcryptCost' packages/server-go/internal/admin/auth.go  # ≥1

# 3) 二选一 panic + 都不设 panic
grep -cE 'mutually exclusive|二选一' packages/server-go/internal/admin/auth.go  # ≥1
grep -cE 'either.*or.*is required' packages/server-go/internal/admin/auth.go  # ≥1

# 4) legacy backward-compat 不破 — 4 既有 BootstrapWith 调用全加第 4 参数 ""
grep -rcE 'BootstrapWith\(.*, ""\)' packages/server-go/internal/admin/  # ≥4 hit

# 5) 0 schema / 0 endpoint / 0 client / 0 routes 改
git diff origin/main -- packages/server-go/internal/migrations/ packages/server-go/internal/server/ packages/client/src/ | wc -l  # 0
git diff origin/main -- packages/server-go/cmd/collab/main.go | grep -cE '^\+'  # ≤2 行 (Bootstrap call wrapping)

# 6) 既有 test 全 PASS + 4 新 unit
go test -tags sqlite_fts5 -timeout=60s ./internal/admin/ ./internal/api/ ./internal/auth/ ./internal/server/  # ALL PASS
```

## 3. 不在范围 (留账)

- ❌ admin login UI 改 (#633 已锁字面 byte-identical 不动)
- ❌ admin password rotation (留 v2+ — 启动时 hash, runtime 不支持改)
- ❌ admin password 强度策略 (留 v2+ — plain env 用户自定义任意字面)
- ❌ multi-admin bootstrap (留 v2+ — 当前 1 admin login)
- ❌ env-file 加密 / SOPS / vault (留 v2+ — operator 责任)

## 4. 跨 milestone byte-identical 锁

- ADM-0.1 #199 server bootstrap env shape (LOGIN + HASH 二字段) byte-identical 不动 (HASH 仍合法)
- admin-spa-shape-fix #633 admin login flow byte-identical 不破 (server loginRequest{Login,Password} 字段 + handleMe writeJSON {id, login} 字面不动)
- cookie-name-cleanup #634 user-rail SSOT 立场承袭 (本 PR 跟 admin-rail 同模式 — env 字面 SSOT 单源 const)
- ADM-0 §1.3 admin/user 路径分叉红线 (本 PR 仅改 admin bootstrap path, 不动 user-rail)
- review checklist 1.A bootstrap fail-loud + 1.B idempotent 立场承袭
