# Data Layer (DL-1) — 4 接口抽象当前状态 (≤80 行)

> 落地: PR feat/dl-1 · DL-1.1 4 interface + factory + 12 unit + DL-1.2 server.go wire + 5 sample handler 注入 + CI dl1-no-direct-store
> 蓝图出处: [`data-layer.md`](../../blueprint/current/data-layer.md) §4 B "可换 4 条 (接口抽象, 迁移低成本)"
> 设计沿用: [`dl-1-spec.md`](../../implementation/modules/dl-1-spec.md) §0 ① 4 interface byte-identical + ② factory + DI seam + ③ 0 schema/0 endpoint

## 1. interface 4 条单一来源 (`internal/datalayer/`)

| Interface | 方法 | v1 实现 | v3+ 切换路径 |
|---|---|---|---|
| `Storage` | `GetURL / PutBlob / Delete` | `localDBStorage` (DB blob 占位, `db://artifact/{key}` URL) | `S3Storage / R2Storage` (DL-3 阈值哨触发) |
| `PresenceStore` | `IsOnline / Sessions` | `inMemoryPresence` wrap AL-3 #324 `presence.SessionsTracker` byte-identical | `DistributedPresence` (Redis / NATS, 留 DL-3) |
| `EventBus` | `Publish / Subscribe` | `inProcessEventBus` (in-process map + buffered chan, best-effort drop) | `NATSEventBus / RedisEventBus` (留 DL-3 阈值哨) |
| `UserRepository` / `ChannelRepository` / `MessageRepository` | `GetByID / GetByEmail / GetByAPIKey / GetByDisplayName / Create` (各 typed) | `sqlite{User,Channel,Message}Repo` wrap `store.Store` gorm 直查 byte-identical | `PostgresRepository` 走标准 SQL (蓝图 §4 C #10 字面禁 ORM) |

**Note**: `ArtifactRepository` 留 v1.5 后续 — `store.Artifact` model 没抽出 (artifact body 现走 `internal/api/artifacts.go` 直 gorm). 蓝图 §4 B 列 4 typed Repo 字面, v1 现状只 3 typed 真有 store.Store CRUD.

## 2. factory + DI seam (`factory.go`)

```go
// 单一来源 bundle — 6 字段, server.go boot 单一来源 wire
func NewDataLayer(s *store.Store, pt presence.PresenceTracker) *DataLayer {
    return &DataLayer{
        Storage: NewLocalDBStorage(s), Presence: NewInMemoryPresence(pt),
        EventBus: NewInProcessEventBus(), UserRepo: NewSQLiteUserRepository(s),
        ChannelRepo: NewSQLiteChannelRepository(s), MessageRepo: NewSQLiteMessageRepository(s),
    }
}
```

server.go `New()` 单一来源调用 `datalayer.NewDataLayer(s, presenceTracker)`; handler 拿 `*DataLayer` 字段 (DI), 不直 import `internal/store`. v3+ 切实现仅改 factory, handler 0 改.

## 3. 5 sample handler 迁移现状 (DL-1.2)

| Handler | DataLayer 字段 | 真迁移 path |
|---|---|---|
| `UserHandler` (users.go) | ✅ nil-safe | (无 basic CRUD 调用 — 留 v1.5 PermissionRepo 抽出后) |
| `RemoteHandler` (remote.go) | ✅ nil-safe | (无 basic CRUD — 留 v1.5 RemoteNodeRepo 抽出后) |
| `CommandHandler` (commands.go) | ✅ nil-safe | (无 store 调用 — 仅占位 wire) |
| `AgentHandler` (agents.go) | ✅ nil-safe | `Store.CreateUser(agent)` → `DataLayer.UserRepo.Create(ctx, agent)` (DataLayer 非 nil 时) |
| `AL5Handler` (al_5_recover.go) | ✅ nil-safe | `Store.GetUserByID(agentID)` → `DataLayer.UserRepo.GetByID(ctx, agentID)` (DataLayer 非 nil 时) |

**渐进迁移**: 行为 test `TestDL12_DirectStoreImportBaseline` (`packages/server-go/internal/api/dl12_direct_store_baseline_test.go`) 在代码里搜 `"borgee-server/internal/store"` import 字面, 锁 `internal/api/` production .go 直 import `internal/store` 文件数 ≤ baseline 50 (production only, 跳 _test.go fixture; DL-1.2 wire-up 时定 108, 后续渐进调整); 后续 milestone PR 顺手补迁移, 不要求一次清零 (反 over-engineer).

## 4. CI 守门 (`TestDL12_DirectStoreImportBaseline` 行为 test)

```go
// packages/server-go/internal/api/dl12_direct_store_baseline_test.go
const baseline = 50
// walk internal/api/*.go (跳 _test.go), 在代码里搜 `"borgee-server/internal/store"` import 字面;
// count > baseline → fail (反 commit 时 handler 直 store 突击).
```

历史 release-gate.yml::dl1-no-direct-store yaml grep step 已随 #717 整治删除;
真行为 test 替临时字符串 grep, 走 `go test ./...` 默认 coverage. 计数应单调
下降 (新增 handler 必走 DataLayer.Repo seam).

## 5. 反向约束 / 不在范围

- ❌ DL-2 events 双流 + retention (留 DL-2 单 milestone)
- ❌ DL-3 阈值哨 (WAL checkpoint / write lock wait / DB 大小, 蓝图 §5)
- ❌ SQLite → PG/CockroachDB 真切 (蓝图 §4 C 必重写 3 条, v1 不投入)
- ❌ EventBus → NATS/Redis 真切 (DL-3 阈值哨触发再启)
- ❌ 全 handler 一次切 Repository (渐进迁移, 单调下降)
- ❌ generic ORM abstraction (蓝图 §4 C #10 字面禁)
- ❌ admin god-mode 挂 datalayer (ADM-0 §1.3 红线, grep 检查 0 hit)
