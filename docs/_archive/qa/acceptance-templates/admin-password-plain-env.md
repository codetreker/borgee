# Acceptance Template — ADMIN-PASSWORD-PLAIN-ENV

> Spec brief `admin-password-plain-env-spec.md` (战马E v0). Owner: 战马E 实施 / 飞马 review / 烈马 验收.
>
> **范围**: BORGEE_ADMIN_PASSWORD 明文 env 启动自 bcrypt 哈希 + HASH env fallback 二选一. server diff ~30 行 (auth.go) + ≤2 行 (cmd/collab Bootstrap call); 0 client / 0 schema / 0 endpoint URL 改.

## 验收清单

### §1 行为不变量

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 HASH legacy path byte-identical 不破 — 仅设 BORGEE_ADMIN_PASSWORD_HASH 时行为完全不变 | unit | `auth_bootstrap_test.go::TestBootstrap_HashPriority_BackwardCompat` PASS — 直存 hash 字面 byte-identical |
| 1.2 既有 TestBootstrap_1A_PanicsOnMissingEnv (4 sub-case) + 1B_Idempotent 不破 | unit | 5 sub-case 全 PASS 无回归 |
| 1.3 PLAIN path: 仅 BORGEE_ADMIN_PASSWORD → bcrypt.GenerateFromPassword(MinBcryptCost) → 写 admins.password_hash + bcrypt.CompareHashAndPassword 真验回原 plain | unit | `TestBootstrap_PlainEnv` PASS — 4 步: bootstrap → 读 stored → bcrypt.Cost ≥ 10 → CompareHashAndPassword(plain) 真匹配 |

### §2 fail-loud (二选一)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 HASH + PLAIN 同时设 → panic mutually exclusive (msg 提两个 env 名) | unit | `TestBootstrap_BothEnvSet_Panics` PASS — panic msg 含 EnvAdminPasswordHash + EnvAdminPassword 两个常量 |
| 2.2 都不设 → panic 提示至少设一个 (msg 提两个 env 名 + cost ≥ 10) | unit | `TestBootstrap_NeitherEnv_Panics` PASS |

### §3 数据契约 (server diff ~30 行 + 0 endpoint URL/schema/routes 改)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 server diff 仅 internal/admin/auth.go (const + Bootstrap 改) + cmd/collab/main.go ≤2 行 Bootstrap call wrap | git diff | `git diff origin/main -- packages/server-go/internal/migrations/ packages/server-go/internal/server/ packages/client/src/` = 0 行 |
| 3.2 既有 BootstrapWith 调用全加第 4 参数 `""` (legacy backward-compat) — 4 件 _test.go 修跟随 | grep | `grep -rcE 'BootstrapWith\(.*, ""\)' packages/server-go/internal/admin/` ≥ 4 hit |

### §4 closure (REG + cov gate + 跨 milestone 锁)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 4.1 既有全包 unit 全绿不破 (./internal/admin + ./internal/api + ./internal/auth + ./internal/server) + post-#634 haystack gate | full test + CI | 4 packages PASS |
| 4.2 立场承袭 — ADM-0.1 #199 + admin-spa-shape-fix #633 + cookie-name-cleanup #634 + ADM-0 §1.3 + review checklist 1.A/1.B | inspect | spec §4 byte-identical |

## REG-APE-* (initial ⚪ → 🟢)

- REG-APE-001 🟢 HASH legacy path byte-identical 不破 (TestBootstrap_HashPriority_BackwardCompat + 既有 1A/1B 全 PASS)
- REG-APE-002 🟢 PLAIN path bcrypt.GenerateFromPassword(MinBcryptCost) → 写 admins.password_hash + verify 回原 plain (TestBootstrap_PlainEnv PASS)
- REG-APE-003 🟢 二选一 fail-loud — HASH+PLAIN 同设 panic + 都不设 panic (BothEnvSet/NeitherEnv panics PASS)
- REG-APE-004 🟢 既有 BootstrapWith 调用全加第 4 参数 `""` (4 件 _test.go 修跟随; legacy backward-compat)
- REG-APE-005 🟢 server diff scope = internal/admin/auth.go + cmd/collab/main.go (0 schema / 0 endpoint URL / 0 routes / 0 client 改)
- REG-APE-006 🟢 cost ≥ MinBcryptCost (10) 守 + env 中明文 plain 永不写盘 (内存哈希后 env 不再读) + 立场承袭 ADM-0.1/0.2/admin-spa-shape-fix #633/cookie-name-cleanup #634

## 退出条件

- §1 (3) + §2 (2) + §3 (2) + §4 (2) 全绿 — 一票否决
- 4 新 unit + 既有 5 sub-case PASS
- server diff scope 守 ≤2 文件
- 登记 REG-APE-001..006

## 更新日志

| 2026-05-01 | 战马E | v0 acceptance template — 4 立场 byte-identical 跟 spec brief; 4 unit 真测验收. |
| 2026-05-01 | 战马E | v1 实施 — auth.go const + Bootstrap/BootstrapWith ~30 行 + cmd/collab/main.go 2 行 + 4 _test.go callsite 跟随 + 4 新 unit + docs/current/admin/README.md 段; ./internal/admin + ./internal/api + ./internal/auth + ./internal/server 全 PASS. REG-APE-001..006 ⚪→🟢. |
