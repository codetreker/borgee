# Acceptance Template — DL-1: Data Layer 4 接口抽象 (Wrapper milestone, 0 schema / 0 endpoint)

> 蓝图: `data-layer.md` §4 B "可换 4 条 (接口抽象, 迁移低成本)" + §1 v1 协议层 portable
> Implementation: `docs/implementation/modules/dl-1-spec.md` (飞马 v0, 90 行) + `docs/qa/dl-1-stance-checklist.md` (野马 PM)
> 配套: AL-3 #324 PresenceTracker (复用) / CV-1 artifacts.go Storage (复用) / BPP-3 #489 PluginFrameDispatcher (factory + DI 同模式)
> Owner: 战马D 实施 / 烈马 验收 (idle 期 zhanma-d 代翻据 CI evidence + 透明声明)

## 验收清单

### §1 4 interface SSOT (DL-1.1 — `internal/datalayer/{storage,presence,eventbus,repository}.go`)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 Storage 3 方法 byte-identical 跟蓝图 §4 B (`GetURL / PutBlob / Delete`) + `ErrStorageKeyNotFound` 单源; v1 `localDBStorage` 占位 (`db://artifact/{key}` URL) | unit + 反向 grep | 战马D | `internal/datalayer/storage.go` interface 33 行 + `v1_sqlite.go::localDBStorage` + `datalayer_test.go::TestStorage_GetURL_HappyAndEmpty` PASS (含 empty key → ErrStorageKeyNotFound 三方法全覆盖) |
| 1.2 PresenceStore 2 方法 (`IsOnline / Sessions`) byte-identical 跟 AL-3 #324; v1 `inMemoryPresence` wrap `presence.SessionsTracker` 不破 | unit | 战马D | `internal/datalayer/presence.go` + `v1_sqlite.go::inMemoryPresence` + `TestPresenceStore_IsOnline_OfflineUser` PASS (offline user → false + empty sessions) |
| 1.3 EventBus 2 方法 (`Publish / Subscribe`) + `Event{Topic, Payload}` 单源; v1 `inProcessEventBus` (in-process map + buffered chan, best-effort drop, RT-1.3 cursor 兜底) | unit (pubsub roundtrip) | 战马D | `internal/datalayer/eventbus.go` + `TestEventBus_PubSub_Roundtrip` PASS (subscribe → publish → 收到 + 无 sub topic publish 不报错) |
| 1.4 Repository 3 typed (UserRepo / ChannelRepo / MessageRepo) wrap `store.Store` gorm 直查 byte-identical; `ErrRepositoryNotFound` 单源 + `mapGormErr` 转 `gorm.ErrRecordNotFound` | unit (3 typed × 3 case = 9 case) | 战马D | `repository.go` + `v1_sqlite.go::sqlite{User,Channel,Message}Repo` + `mapGormErr` helper + 9 unit case PASS (User happy/email-not-found/empty + Channel create-get-happy/notfound/empty + Message create-get-happy/notfound/empty) |
| 1.5 反约束 — 不另起 interface 第 5 个 (Storage / PresenceStore / EventBus / 3 Repository = 6 type 锁) | grep | 战马D / 飞马 | `grep -cE '^type (Storage\|PresenceStore\|EventBus\|UserRepository\|ChannelRepository\|MessageRepository) interface' packages/server-go/internal/datalayer/` ==6 + ArtifactRepo 留 v1.5 follow-up 文档锚 (repository.go §0 注释) |
| 1.6 反约束 — 不另起 dl_1 schema (interface only) | grep + git diff | 战马D / 飞马 | `find packages/server-go/internal/migrations -name 'dl_1_*'` 0 hit + `git diff origin/main -- packages/server-go/internal/migrations/` 0 行 |

### §2 Factory + DI seam (DL-1.1 — `internal/datalayer/factory.go`)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 `NewDataLayer(s, pt) → *DataLayer` 单源 (40 行) + 6 字段 SSOT bundle (Storage / Presence / EventBus / UserRepo / ChannelRepo / MessageRepo) | unit (newTestDataLayer fixture) | 战马D | `factory.go` 40 行 + 12 unit 全用 `newTestDataLayer(t)` fixture 真走 factory 路径 PASS |
| 2.2 server.go boot 单源 wire (`datalayer.NewDataLayer(s, presenceTracker)` 仅 1 处调用); handler 走 DI 不直 instantiate | grep | 战马D / 飞马 | `grep -cE 'NewDataLayer\(' packages/server-go/internal/server/server.go` ==1 + `grep -cE 'datalayer\.NewDataLayer' packages/server-go/` ==1 |

### §3 5 sample handler 注入 (DL-1.2 — server.go wire + 渐进迁移)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 5 handler 加 `DataLayer *datalayer.DataLayer` 字段 (UserHandler / RemoteHandler / CommandHandler / AgentHandler / AL5Handler) nil-safe | grep | 战马D | `grep -cE 'DataLayer \*datalayer\.DataLayer' packages/server-go/internal/api/{users,remote,commands,agents,al_5_recover}.go` ==5 + 各 handler doc-comment "nil-safe; legacy boot 不破" |
| 3.2 真迁移 path — al_5_recover.go agent ACL 走 `UserRepo.GetByID(ctx, agentID)` 替 `Store.GetUserByID(agentID)` (DataLayer 非 nil 时); 既有 TestAL5_Recover_* 全 PASS 数据契约不变 | unit | 战马D | `al_5_recover.go::handleRecover` 双路径 (DataLayer != nil → UserRepo.GetByID, else Store.GetUserByID) + `internal/api/al_5_recover_test.go` PASS |
| 3.3 真迁移 path — agents.go agent 创建走 `UserRepo.Create(ctx, agent)` 替 `Store.CreateUser(agent)` (DataLayer 非 nil 时); 既有 TestAgent* 全 PASS | unit + e2e | 战马D | `agents.go::handleCreateAgent` 双路径 + `internal/api/` TestAgent* / agent_invitations_test.go 全 PASS |
| 3.4 server.go wire — 5 handler 真注入 `s.dl` (`&api.AL5Handler{Store: s.store, DataLayer: s.dl, ...}` 同模式 ×5) | grep | 战马D | `grep -cE 'DataLayer: s\.dl' packages/server-go/internal/server/server.go` ==5 |

### §4 CI 守门链第 6 处 + 跨 milestone 不破

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 `release-gate.yml::dl1-no-direct-store` step 真挂 (baseline=108 锁 internal/api/ 直 import internal/store 文件数 ≤ baseline) | CI grep + count | 战马D / 飞马 | `grep -nE 'name: dl1-no-direct-store' .github/workflows/release-gate.yml` ≥1 hit + `grep -rl 'borgee-server/internal/store' packages/server-go/internal/api/ --include='*.go' \| wc -l` ≤108 |
| 4.2 既有 unit tests 全 PASS (Wrapper 不破 — AL-3 / CV-1 / messages / channels / agents / al_5 等不动) | go test sweep | 战马D / 烈马 | `go test -tags sqlite_fts5 -timeout=180s ./internal/{datalayer,api,server}/` 全绿 (datalayer 12/12 + api PASS + server PASS) |
| 4.3 admin god-mode 不挂 datalayer (ADM-0 §1.3 红线) | 反向 grep | 战马D / 飞马 / 烈马 | `git grep -nE 'admin.*datalayer\|/admin-api.*datalayer' packages/server-go/internal/` 0 hit |

## 透明声明

本 acceptance template 由 zhanma-d (战马D) 在 liema (烈马 QA) idle 期间代翻 ⚪→✅, 据 CI evidence 真证据 (本地 `go build` + `go test` 全绿, `grep` count 真测) + content-lock 不需 (server-only milestone 无文案锁). 烈马回归后据本表 §1-§4 全项 audit; 任何 evidence 漏挂或不符合实测, 烈马回滚翻 ⚪ 重补.

## 4 件套状态

- ✅ spec brief — `dl-1-spec.md` (飞马 v0, 90 行)
- ✅ stance-checklist — `dl-1-stance-checklist.md` (野马 PM, 92 行 commit c8b23a25)
- ✅ acceptance template — 本文件 (zhanma-d 代翻, liema 回归 audit)
- ✅ content-lock 不需 — server-only milestone, 无 user-visible 文案
