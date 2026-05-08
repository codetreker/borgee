# DL-1 stance checklist — Data Layer 4 接口抽象 (Wrapper milestone)

> 7 立场 byte-identical 跟 dl-1-spec.md §0 + §2. **Wrapper milestone — 真有 server prod code (4 interface + 4 wrapper + factory + 5 sample handler 改) 但 0 schema + 0 endpoint**. 跟 INFRA-3 #594 / RT-3 #588 / AP-4-enum #591 / TEST-FIX-1 #596 真有 prod code wrapper 类别同模式承袭.

## 1. 4 interface byte-identical 跟蓝图 §4 B 表

- [ ] `Storage` interface — 3 method `GetURL` / `PutBlob` / `Delete` byte-identical (v1 实现 `LocalFSStorage` wrap 既有 artifacts.go)
- [ ] `PresenceStore` interface — 2 method `IsOnline` / `Sessions` byte-identical (v1 实现 `InMemoryPresence` wrap AL-3 #324 PresenceTracker)
- [ ] `EventBus` interface — 2 method `Publish` / `Subscribe` byte-identical (v1 实现 `InProcessEventBus` wrap ws hub)
- [ ] `Repository` interface — 4 typed (`UserRepo` / `ChannelRepo` / `MessageRepo` / `ArtifactRepo`) + 5 generic CRUD method (`Get` / `List` / `Create` / `Update` / `Delete`) byte-identical (v1 wrap 既有 queries.go SQLite gorm)
- [ ] 反向 grep `^type (Storage|PresenceStore|EventBus|Repository)` count==4 in `packages/server-go/internal/datalayer/` (反 5 漂 / 反 3 偷工减料)

## 2. factory pattern + DI seam 单源

- [ ] `NewDataLayer(cfg)` 单源, server.go boot wire 走 factory (反 `BuildDataLayer` / `MakeDataLayer` / `CreateDataLayer` 同义词漂; Go idiom `New*` byte-identical)
- [ ] handler 不直 import `internal/store/` (走 Repository interface 单源)
- [ ] **CI 守门链第 6 处** — `dl1-no-direct-store` step 真挂 release-gate.yml (反向 grep `internal/store\.|gorm\.` in `internal/api/` baseline N 不增, 跟 BPP-4 + HB-3 + HB-4 + AP-4-enum + INFRA-3 + TEST-FIX-1 同模式)
- [ ] factory 函数单源 (反向 grep `func NewDataLayer` count==1)

## 3. 0 schema 改 + 0 endpoint 加 + 既有 byte-identical 不破

- [ ] 反向 grep `migrations/dl_1_` 0 hit (Wrapper 真有 prod code 但 0 schema)
- [ ] 反向 grep 新 endpoint `/api/v1/datalayer` 0 hit (0 endpoint 加)
- [ ] 既有 unit tests 全 PASS (AL-3 / CV-1 / messages / channels 等既有不动)
- [ ] DL-1.2 sample 5 handler 改 (channels / messages / artifacts / users / agents) 立 baseline N (反一次全切, 后续 milestone PR 顺手补)

## 反约束 — 同义词反向 grep (interface vs 5 概念拆死)

**英 10 类同义词 reject** (`interface` byte-identical 锁):
- `service` / `manager` / `adapter` / `driver` / `facade` / `provider` / `handler` / `wrapper` / `contract` / `abstraction` reject

**中 10 类同义词 reject** (`interface` 字面在代码 + 蓝图字面 byte-identical):
- `服务` / `管理器` / `适配器` / `驱动` / `门面` / `提供者` / `处理器` / `包装器` / `契约` / `抽象层` reject

**5 概念各自单源不混** (PM 拆死立场):
- ✅ `interface` = Go interface seam (data layer 抽象 SSOT, server-side internal/datalayer/)
- ❌ `contract` = IPC contract = 通信协议 byte-identical (跟 HB-1/HB-2 daemon IPC 各自单源, 不混入 data layer)
- ❌ `abstraction` = generic ORM abstraction (蓝图 §4 C #10 字面禁 "v1 写标准 SQL, 不写 ORM 抽象")
- ❌ `API` = REST API 业务路由 (跟 internal/api/ handlers 各自单源, 不是 REST API)
- ❌ `SDK` = client 接入 SDK (跟 BPP-7 plugin SDK 占号 v3+ 各自单源)

**factory 拆死**: `New*` byte-identical, 反 `Builder` / `Maker` / `Creator` 漂 (Go idiom).

## 反约束 — DL-1 真不在范围

- ❌ 真切 PG / CockroachDB (留 v3+; v1 仅 wrap 既有 SQLite gorm)
- ❌ generic ORM abstraction (蓝图 §4 C #10 字面禁)
- ❌ admin god-mode datalayer (反向 grep `admin.*datalayer|/admin-api.*datalayer` 0 hit, ADM-0 §1.3 红线 + PR #571 §2 ⑥ 精神延伸)
- ❌ 一次全切 handler (反 PR scope 红线; 立 baseline N + 后续 milestone PR 顺手补)
- ❌ DL-2/DL-3/DL-4 (留各自 milestone, DL-4 #485 已起独立路径)

## 跨 milestone byte-identical 锁链 (5 链)

- **BPP-3 #489 PluginFrameDispatcher interface seam 模式** — DL-1 factory + DI 单源跟 BPP-3 PluginFrameDispatcher Register / Route 模式同精神 (cross-package boundary glue)
- **reasons.IsValid #496 SSOT 包模式** — DL-1 4 interface 单源跟 AL-1a 6-dict reason ALL slice + IsValid helper 同精神 (改 = 改一处)
- **AL-3 #324 PresenceTracker 既有 byte-identical 不破** — DL-1.1 PresenceStore wrap 既有 IsOnline / Sessions byte-identical (跟 G2.5 contract 锁同源)
- **CV-1 artifacts.go Storage 既有 byte-identical 不破** — DL-1.1 Storage wrap 既有 GetURL / PutBlob / Delete byte-identical
- **CI 守门链第 6 处** — `dl1-no-direct-store` step 跟 BPP-4 + HB-3 + HB-4 + AP-4-enum + INFRA-3 + TEST-FIX-1 同模式 (release-gate.yml CI 守门链跨 7 milestone byte-identical 第 6 处)

## PM 立场拆死决策

**DL-1 wrapper 真有 prod code vs 0-server-prod 系列拆死**:
- ✅ DL-1 = Wrapper milestone 真有 server prod code (4 interface + 4 wrapper + factory + 5 sample handler 改) 但 0 schema + 0 endpoint
- ❌ 跟 CS-1 / CS-3 0-server-prod client-only Wrapper 拆死 (CS 是 client-only, DL-1 是 server-side wrapper)
- ✅ 跟 INFRA-3 #594 + RT-3 #588 + AP-4-enum #591 + TEST-FIX-1 #596 **真有 prod code wrapper 类别**同模式承袭

**4 interface 单源 vs 5/3 漂拆死**:
- ✅ Storage / PresenceStore / EventBus / Repository 4 interface byte-identical (反第 5 漂入 / 反 3 偷工减料)
- ❌ 反 generic ORM abstraction (蓝图 §4 C #10 字面禁)
- ❌ 反 Service / Manager / Adapter 同义词漂 (`interface` byte-identical 锁)

**渐进迁移 vs 一次切完拆死**:
- ✅ DL-1.2 sample 5 handler 改立 baseline N
- ❌ 反一次全切 (反 PR scope 红线)
- 跟 INFRA-3 #594 拆分协议 + 0-server-prod 系列同精神承袭

## 用户主权红线锚 (7 项)

- ✅ handler 不直 import store (走 Repository interface) — 保护用户数据访问层, future 切 PG / CockroachDB 时 handler 0 改 (蓝图 §4 B 接口抽象立场守)
- ✅ factory + DI seam 单源 — `NewDataLayer(cfg)` 单源 server.go boot wire (反 multiple instantiate 漂, 跟 BPP-3 PluginFrameDispatcher + reasons.IsValid SSOT 同精神)
- ✅ 既有实施 byte-identical 不破 — Wrapper milestone 立场 (反 over-engineer)
- ✅ CI 守门链第 6 处 `dl1-no-direct-store` step — 真测 handler grep baseline N 不增 (反 future PR 漂入直查)
- ✅ agent ↔ human 同源 — data layer 不分 sender 类型 (PR #568 §4 端点延伸)
- ✅ admin god-mode 不挂 datalayer — 反向 grep `admin.*datalayer|/admin-api.*datalayer` 0 hit (ADM-0 §1.3 红线 + PR #571 §2 ⑥ 精神延伸)
- ✅ 0-server-prod 系列变体真兑现 — 真有 prod code + 0 schema + 0 endpoint (跟 INFRA-3 + RT-3 + AP-4-enum + CS-2/CS-3 wrapper 同模式)

## PR 出来 4 核对疑点 (PM 真测)

1. **0 schema 改** — 反向 grep `migrations/dl_1_` 0 hit
2. **handler grep baseline N 真测** — CI step `dl1-no-direct-store` 真挂 release-gate.yml + baseline N 锁 (反向 grep `internal/store\.|gorm\.` in `internal/api/` ≤ N)
3. **4 interface count==4 真守门** — `grep -cE '^type (Storage|PresenceStore|EventBus|Repository)' packages/server-go/internal/datalayer/ ==4` (反 5/3 漂)
4. **既有 unit tests 全 PASS** — AL-3 / CV-1 / messages / channels 等既有不动 (Wrapper 不破既有 byte-identical, 跟 INFRA-3 #594 拆分协议同精神)
