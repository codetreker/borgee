# Server: Testing 工具 (testutil)

> 同步 `packages/server-go/internal/testutil/` 当前状态。
> Blueprint 对应: [`../../blueprint/`](../../blueprint/) — testing 不是产品设计, 这里只描述 **当前实现**。

## 总览

| 子包 | 路径 | 用途 |
|------|------|------|
| `clock` | `internal/testutil/clock/` | Clock 接口 + Real / Fake 实现, 替代 `time.Now()` 直调 |
| `db` | `internal/testutil/db/` | 内存 sqlite + fixture seeder, 每用例独立隔离 |
| `regression` | `internal/testutil/regression/` | 闸 5 回归入册, 已合 milestone 的 4.1 acceptance 自动入册 |
| `regression_suite` | `internal/testutil/regression_suite/` | `make regression` 的 dispatcher |
| (legacy) | `internal/testutil/server.go` | 历史遗留, 启服务 + ws 工具, 保留不动 |

## clock 子包 (INFRA-1b.1)

### 接口

```go
type Clock interface {
    Now() time.Time
    Since(t time.Time) time.Duration
    After(d time.Duration) <-chan time.Time
    Sleep(d time.Duration)
}
```

### 实现

- **`clock.NewReal()`** — 生产用, 直接代理 stdlib `time` 包。
- **`clock.NewFake(start time.Time)`** — 测试用, 时间只在显式 `Set` / `Advance` 时变。
  - `start = time.Time{}` → 默认 epoch `2025-01-01 UTC` (输出可重复)
  - `Advance(d)` → d ≤ 0 时静默忽略 (避免测试代码意外回退)
  - `Set(t)` → 跳到 t, deadline 已过的 waiter 立即 fire
  - `After(d) / Sleep(d)` → 等 fake 时间越过 deadline 才 fire; d ≤ 0 立即 fire
  - 并发安全 (mu 保护 now + waiters)

### 用法

生产代码: 注入 `Clock` 而不是直调 `time.Now()`。

```go
type RateLimiter struct {
    clock clock.Clock
    // ...
}

func (r *RateLimiter) Allow() bool {
    now := r.clock.Now()
    // ...
}
```

测试代码:

```go
fake := clock.NewFake(time.Time{})
limiter := NewRateLimiter(fake, ...)
// 不 sleep, 直接快进
fake.Advance(time.Hour)
require.True(t, limiter.Allow())
```

### 当前未接入

`time.Now()` 直调在以下文件还存在 (后续 milestone 替换):
- `internal/api/admin_auth.go` (JWT iat/exp)
- `internal/server/middleware.go` (rate limiter, request log)
- `internal/ws/client.go` (心跳)
- `internal/store/query_gap_test.go` (测试代码可保留)

替换不是 INFRA-1b.1 范围 — 引入 Clock 抽象 + 100% 覆盖单测先, 替换在使用方各自 PR 里做。

## 测试覆盖率

| 包 | 行覆盖率 |
|-----|---------|
| `internal/testutil/clock` | 100.0% |
| `internal/testutil/db` | 91.7% |
| `internal/testutil/regression` | 100.0% |

跑法:
```
cd packages/server-go && go test ./internal/testutil/clock/... -cover
```

## Phase 0 验收 (G0.2 — INFRA-1b)

- [x] **1b.1**: Clock 抽象 + Fake/Real 双实现, ≥ 80% 覆盖率, 1 demo 用例
- [x] **1b.2**: 内存 sqlite + fixture seeder, ≥ 80% 覆盖率
- [x] **1b.3**: 回归测试入册机制 + Makefile target

## regression 子包 (INFRA-1b.3)

### 目的

闸 5 (README.md §测试 + regression 防护硬规则) 要求: **已合并 milestone 的 4.1 acceptance 自动入册**, 后续 milestone 改 schema 不能打穿之前的不变量。

### 用法

每个 milestone 的 acceptance 测试在自己的测试文件 `init()` 里登记:

```go
package cm1_test

import (
    "testing"
    "borgee-server/internal/testutil/regression"
)

func init() {
    regression.Register("CM-1", "users.org_id NOT NULL backfill",
        func(t *testing.T) {
            // ... 不变量断言
        })
}
```

然后在 `internal/testutil/regression_suite/suite_test.go` 加一行 blank import:

```go
import (
    _ "borgee-server/internal/cm1_test_or_wherever"
)
```

之后:
- `go test ./...` 不变, 该测试还是作为常规单测跑
- `make regression` 走 dispatcher 把所有 Register 过的入口跑一遍, 名字 `<milestone>/<name>`

### 不变量

- 重复 (milestone, name) → `panic` (强制审计可追溯)
- 空 milestone / 空 name / nil fn → `panic`
- registry 为空时 RunAll 调 `t.Skip` 而不是 fail (Phase 0 落地时尚无 entry)
- `Entries()` 返回**拷贝** + 按 (milestone, name) 排序 (输出稳定)

### Makefile 入口

```makefile
make regression
# go test ./internal/testutil/regression_suite -run TestRegressionSuite -v
```

后续每个 milestone 关闭时, 烈马在 PR review 阶段确认: "本 milestone 的 4.1 acceptance 是否已 Register + 是否已加 blank import 到 regression_suite"。

## db 子包 (INFRA-1b.2)

### 入口

```go
// 单测开局: 干净的内存 sqlite, t.Cleanup 自动关
d := db.Open(t)

// 跑 fixture (raw .sql 文件, -- 注释支持, ; 切句)
db.Seed(t, d, "testdata/cm-1/seed.sql")

// 一步到位 (Open + Seed)
d := db.OpenSeeded(t, "testdata/cm-1/seed.sql")
```

### 隔离策略

每次 `Open(t)` 拿到的是**独立**的内存数据库:
- DSN 用 `file:testdb_<8字节随机>?mode=memory&cache=shared` 命名,
  shared-cache 让同一 DSN 的多个连接见同一份数据, 不同 DSN 互不可见
- `MaxOpenConns = 1` 防止 sqlite `:memory:` 因多连接丢表
- pragmas 与 prod 一致 (`foreign_keys=ON`, `busy_timeout=5000`); 略掉 `journal_mode=WAL` (内存库无意义)

### 与 migrations 的关系

`db` 包**不**直接依赖 `internal/migrations` (避免编译耦合)。需要 schema 时:

```go
d := db.Open(t)
require.NoError(t, migrations.Default(d).Run(0))
db.Seed(t, d, "testdata/cm-1/seed.sql")
```

未来若发现 99% 用例都跑 migrations, 可能再加 `db.OpenMigrated(t)` shortcut, 现在先不加。

### 不变量

- `Open(t)` 返回的 db 在 t 结束时自动关闭 (t.Cleanup 注册)
- `Seed` 的 SQL 解析: `-- ` 行注释忽略, `;` 分句, 空句忽略;
  **不支持** 字符串字面量内含 `;` (fixture 里别这么写, 必要时拆 Exec)
- `Seed` 任意一句失败 → `t.Fatalf` 立即 fail, 把 stmt 文本一起打印
