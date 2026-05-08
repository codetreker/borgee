# DL-1 spec brief — Data Layer 4 接口抽象 (Storage / Presence / EventBus / Repository) (≤80 行)

> 飞马 · 2026-04-30 · Phase 4+ Data Layer 接口抽象 (蓝图 data-layer.md §4 B 接口抽象 4 条)
> **蓝图锚**: [`data-layer.md`](../../blueprint/current/data-layer.md) §4 B "可换 4 条 (接口抽象, 迁移低成本)" + §1 v1 协议层 portable + 接口层抽象立场
> **关联**: DL-4 #485 PWA Web Push (已落, 是 EventBus 真消费者) + AL-3 #324 PresenceTracker (in-memory) + CV-1 artifacts.go Storage (local fs) + 全 milestone Repository (SQLite gorm 直查) — 4 处既有实施需 wrap 抽象层
> **命名**: DL-1 = data-layer 第一段接口抽象, 跟 DL-2 (events 双流) / DL-3 (阈值哨) 拆死

> ⚠️ Wrapper milestone — 0 schema 改 + 0 endpoint 加 + interface only (非真切实现).
> v1 4 interface 全用既有实现 (SQLite gorm + in-memory map + local fs + in-process map), 仅加 interface seam 锁住未来换实现路径.

## 0. 关键约束 (3 条立场, 蓝图 §4 B + §1 字面承袭)

1. **4 interface byte-identical 跟蓝图 §4 B 表 (Storage / Presence / EventBus / Repository)**:
   - **Storage interface**: `GetURL(ctx, key) (string, error)` + `PutBlob(ctx, key, data) error` + `Delete(ctx, key) error` — v1 实现 `LocalFSStorage` 走既有 `internal/store/artifacts.go` 字面 (本地 fs) byte-identical 不破
   - **PresenceStore interface**: `IsOnline(ctx, userID) (bool, error)` + `Sessions(ctx, userID) ([]Session, error)` — v1 实现 `InMemoryPresence` 走 AL-3 #324 既有 `PresenceTracker` byte-identical 不破 (跟 G2.5 contract 锁同源)
   - **EventBus interface**: `Publish(ctx, topic, payload) error` + `Subscribe(ctx, topic) (<-chan Event, error)` — v1 实现 `InProcessEventBus` 走既有 ws hub 字面 (in-process map) byte-identical 不破
   - **Repository interface**: 通用 generic CRUD (`Get / List / Create / Update / Delete`) wrap 既有 `internal/store/queries.go` SQLite gorm 直查; v1 4 typed Repository (UserRepo / ChannelRepo / MessageRepo / ArtifactRepo) byte-identical 不破
   
   反约束: 不真切实现 (v1 仅 wrap 既有, 反 over-engineer); 不另起 interface 第 5 个; 反向 grep `interface` count==4 in `internal/datalayer/`.

2. **factory pattern + DI seam 单源 (跟 BPP-3 PluginFrameDispatcher / reasons.IsValid SSOT 同精神)**: `internal/datalayer/factory.go` 单 `NewDataLayer(cfg)` 返 4 interface 实例 (`{Storage, Presence, EventBus, UserRepo, ChannelRepo, MessageRepo, ArtifactRepo}`); server.go boot wire 走 factory 不直 instantiate 实现; 反约束: handler 不准 import `internal/store/` 直查 (走 Repository interface 单源), 反向 grep `internal/store\.|gorm\.` in `internal/api/` ≤ N (既有 baseline N 不增) — **CI 守门链第 6 处** `release-gate.yml::dl1-no-direct-store` 加 step (跟 INFRA-3/AP-4-enum 同模式).

3. **0 schema 改 + 0 endpoint 加 + 既有实施 byte-identical 不破** (Wrapper milestone 立场, 跟 0-server-no-schema 系列变体 — 真有 server prod code 但 0 schema): PR diff 仅 `internal/datalayer/` 新 (4 interface .go + factory + 4 实现 wrapper) + server.go wire 改 ≤ 20 行 + 既有 handler 改走 Repository interface (按 grep baseline 渐进迁移, 不强制全 1 PR 切完); 反约束: 不引入 generic auth analyzer (v3+) + 不切 SQLite (DL-3 阈值哨触发再切); 反向 grep `migrations/dl_1_` 0 hit + `gorm.AutoMigrate.*&Storage` 0 hit.

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| **DL-1.1** 4 interface + factory | `internal/datalayer/storage.go` (Storage interface ≤30 行 + LocalFSStorage wrap ≤50 行 复用 artifacts.go); `internal/datalayer/presence.go` (PresenceStore interface ≤30 行 + InMemoryPresence wrap 复用 AL-3 PresenceTracker ≤40 行); `internal/datalayer/eventbus.go` (EventBus interface ≤30 行 + InProcessEventBus wrap 复用 ws hub ≤50 行); `internal/datalayer/repository.go` (4 typed Repo interface ≤50 行 + SQLite wrap ≤80 行 复用 queries.go); `internal/datalayer/factory.go` (NewDataLayer ≤40 行); 12 unit 4 interface 各 3 happy/empty/err | 战马 (主) / 飞马 review |
| **DL-1.2** server.go wire + 既有 handler 渐进迁移 (sample 5 处) | server.go 改 ≤20 行 (用 NewDataLayer 替 directly instantiate); 5 sample handler (`channels.go` / `messages.go` / `artifacts.go` / `users.go` / `agents.go`) 改走 Repository interface (≤10 行 each, 50 行 total); 反向 grep CI step `dl1-no-direct-store` 加 release-gate.yml (baseline N 不增) | 战马 / 飞马 review |
| **DL-1.3** closure | REG-DL1-001..006 (6 反向 grep + 4 interface count + factory 单源 + CI step 真挂 + 既有实施不破 + handler grep baseline 不增) + acceptance + content-lock 不需 (server-only) + docs/current/server/data-layer.md ≤80 行同步 (4 interface 字面 + v1 实现 + 切换路径) + 4 件套 spec 第一件 | 战马 / 烈马 |

## 2. 反向 grep 锚 (5 反约束, count==0)

```bash
# 1) 4 interface 真锁 (反 5 个 / 3 个漂)
grep -cE '^type (Storage|PresenceStore|EventBus|Repository)' packages/server-go/internal/datalayer/  # ==4

# 2) 不另起 dl_1 schema (interface only)
ls packages/server-go/internal/migrations/ | grep -cE 'dl_1_'  # 0

# 3) handler 不直 import internal/store/ (走 Repository) — baseline N 不增
new=$(git grep -cE '"borgee-server/internal/store"' packages/server-go/internal/api/)
[ "$new" -le "$baseline" ]  # CI step 守 (DL-1.2 sample 5 handler 减少, 整体 baseline 不增)

# 4) factory 单源 (反 multiple instantiate)
grep -cE 'NewDataLayer' packages/server-go/internal/server/server.go  # ==1

# 5) admin god-mode 不挂 datalayer (ADM-0 §1.3 红线)
git grep -nE 'admin.*datalayer|/admin-api.*datalayer' packages/server-go/internal/  # 0 hit
```

## 3. 不在范围 (留账)

- ❌ DL-2 events 双流 + retention (events table + EventBus 真消费者 wrapper, 留 DL-2 单 milestone)
- ❌ DL-3 阈值哨 (WAL checkpoint / write lock wait / DB 大小 监控, 留 DL-3 单 milestone, 蓝图 §5)
- ❌ SQLite → PG/CockroachDB 真切 (蓝图 §4 C 必重写 3 条, v1 不投入)
- ❌ EventBus 切 NATS/Redis (蓝图 §4 C #11, 留 DL-3 阈值哨触发再启)
- ❌ 全 handler 一次切 Repository — 渐进迁移 (sample 5 handler 立 baseline, 后续 milestone PR 顺手补)
- ❌ generic ORM abstraction (蓝图 §4 C #10 字面禁 "v1 写标准 SQL, 不写 ORM 抽象")

## 4. 跨 milestone byte-identical 锁

- 复用 BPP-3 #489 PluginFrameDispatcher interface seam 模式 (factory + DI 单源)
- 复用 reasons.IsValid #496 SSOT 包模式 (改 = 改一处, interface 单源)
- 复用 release-gate.yml CI 守门链 (跟 BPP-4/HB-3/AP-4-enum/HB-4/INFRA-3 同模式) — `dl1-no-direct-store` step **第 6 处链**
- 复用 AL-3 #324 PresenceTracker / CV-1 artifacts.go Storage / ws hub EventBus byte-identical 不破 (v1 实现 wrap 既有)
- 跨九 milestone 决策树**变体**: 真有 server prod code (4 interface + 4 wrapper) 但 0 schema 改 — 跟 INFRA-3 / RT-3 / AP-4-enum / CS-2 / CS-3 同 "wrapper 真有 prod code 0 schema" 类别同源 (区别 0-server-prod 第 17 处 CS-3)

## 5. 验收挂钩

- REG-DL1-001..006 (5 反向 grep + CI step `dl1-no-direct-store` 真守 + handler grep baseline 不增)
- 既有 unit tests 全 PASS (Wrapper 不破 — AL-3 / CV-1 / messages / channels 等不动)
- 12 unit (4 interface 各 3 case) + 5 sample handler 改走 interface 不破

## 6. 派活建议

**派 zhanma-c** (INFRA-3 #594 / INFRA-4 (待派) PROGRESS 拆分熟手, 续作减学习成本; 跟 wrapper 模式同源真值优) **或** zhanma-d (CS-3 #598 后接, client 段已闭可转 server). 飞马 review.

**优先序 (你拍)**:
- **DL-1 (本)**: ⭐ 优先 — 蓝图 §4 B 4 接口抽象, 真值有用 (锁住未来换实现路径); CI 守门链第 6 处真兑现
- DL-2 events 双流 + retention: 中度优先, 真依赖 DL-1 EventBus interface
- DL-3 阈值哨: 中度, 真依赖 DL-1 + DL-2 (监控 EventBus 吞吐 + DB 大小)
- CS-4 IndexedDB 乐观缓存: 留账, 跟 RT-1 cursor 协议同期 (CS-1 spec §3 留账)
