# DL-2 spec brief — events 双流 + retention (≤80 行)

> 飞马 · 2026-05-01 · 用户拍板 (NAMING-1 ✅ → DL-2 → DL-3 → HB-2 v0(D) 顺序) · zhanma 主战 + 飞马 review
> **关联**: DL-1 #609 ✅ merged (4 interface 抽象, EventBus in-process v1) · 蓝图 data-layer.md §2.7 / §3.4 / §4.A.4-5
> **命名**: DL-2 = data-layer 第二件 (events 双流), 跟 DL-1 / DL-3 (阈值哨) / DL-4 #485 (Web Push) 平行

> ⚠️ Schema + server milestone — 加 `channel_events` + `global_events` 表 + EventBus SQLite consumer + retention sweeper. **0 endpoint 行为改 / 0 user-facing API 改**.
> v1 单机实现 (in-process EventBus 兼容 SQLite consumer 持久化), 不引 MQ (蓝图 §3.3 立场).

## 0. 关键约束 (3 条立场)

1. **DL-1 4 interface byte-identical 不破** (蓝图 §4.A 必修 5 条 lock-in 承袭): EventBus interface (`Publish(ctx, topic, payload) error` + `Subscribe(ctx, topic) (<-chan Event, error)`) 跟 DL-1 #609 byte-identical, **不动 signature**. v1 实现 `InProcessEventBus` 加 SQLite **冷流 consumer** (内置 sweeper 持久化 events 到 `channel_events` / `global_events` 表) — hot stream (live subscribers) byte-identical, cold stream 是新增持久化路径不影响 hot. 反约束: 反向 grep DL-1 interface signature 字面跟 #609 byte-identical (`func.*Publish.*context.Context.*string.*Event` count 等量).

2. **events 双流 + retention 蓝图 §3.4 必落清单 byte-identical** (产品硬要求): 
   - **hot stream**: 既有 in-process EventBus.Publish/Subscribe 不动 (live fanout 路径)
   - **cold stream**: SQLite consumer 异步持久化到 `channel_events(channel_id, lex_id, kind, payload, created_at)` + `global_events(lex_id, kind, payload, created_at)` (lex_id ULID 跟蓝图 §4.A.4 byte-identical)
   - **必落 kind 蓝图 §3.4 4 类**: 权限 grant/revoke / impersonate 开始-结束 / agent 上下线状态切换 / admin force delete-disable — `mustPersistKinds` enum 单源 (跟 reasons.IsValid #496 / AP-4-enum #591 SSOT 同精神, 反向 grep `mustPersistKinds` count==1 hit)
   - **retention 策略**: per-kind 阈值 (蓝图 §4 retention) — 默认 90 天, 必落 kind 永久 (隐私契约), per-channel events 30 天, agent_task / artifact 类 60 天
   - 反约束: 反向 grep `channel_events|global_events` 单表存在 + `retention_days` enum 单源
   
3. **0 user-facing API 改 + 0 endpoint 加 + caller 列表锁**: PR diff 仅 (a) 2 migration v=46 + v=47 (channel_events + global_events + idx) (b) `internal/datalayer/eventbus.go` 既有 InProcessEventBus 加 cold-stream consumer hook (~80 行) (c) `internal/datalayer/events_retention.go` 新 sweeper (~70 行, 跟 AL-7 audit retention sweeper 同精神承袭) (d) caller 改: 4 必落 kind 调用点 (auth grant/revoke + impersonate 起止 + agent 上下线 + admin force action) 走 EventBus.Publish (走 helper-wrapper 反 inline drift). 反约束: 0 endpoint URL 改 / 0 routes.go 改 / 既有 unit/e2e 全 PASS / haystack gate Func=50/Pkg=70/Total=85 三轨守.

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| **DL2.1** schema migration v=46 + v=47 | `migrations/channel_events.go` v=46 (CREATE channel_events ULID lex_id PK + idx (channel_id, lex_id DESC) + retention_days INTEGER); `migrations/global_events.go` v=47 (CREATE global_events ULID lex_id PK + idx (kind, lex_id DESC) + retention_days INTEGER); 跟 DM-10.1 v=45 串行不撞 | 战马 / 飞马 review |
| **DL2.2** EventBus cold consumer + retention sweeper | `internal/datalayer/eventbus.go` 加 cold-stream consumer hook (~80 行 — InProcessEventBus.Publish 增 SQLite INSERT 异步路径, 失败 logging-only 不阻塞 hot stream); `internal/datalayer/events_retention.go` 新 sweeper (~70 行 — DELETE WHERE created_at < now - retention_days, per-kind enum 单源, 跟 AL-7 / HB-5 retention sweeper 同模式承袭); `internal/datalayer/must_persist_kinds.go` 新 enum SSOT (~30 行 4 类必落); 4 caller 调用点 (auth.go grant/revoke + admin/impersonation.go 起止 + agent_lifecycle.go 上下线 + admin force action) 走 EventBus.Publish wrapper | 战马 / 飞马 review |
| **DL2.3** closure | REG-DL2-001..008 (8 反向 grep + DL-1 interface byte-identical 不破 + must-persist enum 单源 + retention sweeper 真挂 + 双流 hot/cold 行为分离 + 4 caller wire 真过 + haystack 三轨过 + 既有 test 全 PASS + 0 user-facing API 改) + acceptance + content-lock 不需 (server-only) + 4 件套 spec 第一件 | 战马 / 烈马 |

## 2. 反向 grep 锚 (8 反约束)

```bash
# 1) DL-1 interface signature byte-identical 不破 (反约束承袭)
git diff origin/main -- packages/server-go/internal/datalayer/eventbus.go | grep -E '^-.*func.*EventBus.*Publish|^-.*func.*EventBus.*Subscribe'  # 0 hit (signature 不删)

# 2) channel_events + global_events 单表 (反另起)
ls packages/server-go/internal/migrations/ | grep -cE 'channel_events|global_events'  # ==2 hit (一表一文件, v=46 + v=47)
grep -rE 'CREATE TABLE.*channel_events|CREATE TABLE.*global_events' packages/server-go/internal/migrations/  # ==2 hit (各 1 hit)

# 3) must-persist kinds enum 单源 (反 inline 字面漂)
grep -rcE 'func.*MustPersistKinds|var.*mustPersistKinds = ' packages/server-go/internal/datalayer/  # ==1 hit (SSOT)

# 4) retention sweeper 单源 (跟 AL-7 / HB-5 模式承袭)
grep -rE 'func .*EventsRetentionSweeper|RunEventsRetention' packages/server-go/internal/datalayer/  # ==1 hit

# 5) 双流分离 (hot stream 不写 DB, cold stream 异步 INSERT)
grep -rE 'INSERT INTO channel_events|INSERT INTO global_events' packages/server-go/internal/datalayer/eventbus.go  # ≥2 hit (cold-stream consumer)
grep -rE 'select.*ch.*<-|chan Event' packages/server-go/internal/datalayer/eventbus.go  # ≥1 hit (hot stream channel 不动)

# 6) 4 必落 kind caller wire (蓝图 §3.4)
grep -rE 'EventBus.*Publish.*"perm\.|"impersonate\.|"agent\.state|"admin\.force' packages/server-go/internal/  | wc -l  # ≥4 (4 类各 ≥1 caller)

# 7) 0 endpoint URL / routes.go 改
git diff origin/main -- packages/server-go/internal/server/server.go | grep -cE '^\+.*HandleFunc|^\+.*Handle\('  # 0 hit

# 8) haystack gate 三轨 + 既有 test
THRESHOLD_FUNC=50 THRESHOLD_PACKAGE=70 THRESHOLD_TOTAL=85 BUILD_TAGS="sqlite_fts5" go run ./scripts/lib/coverage/  # ALL ≥阈值
go test -tags 'sqlite_fts5' -timeout=300s ./...  # ALL PASS
```

## 3. 不在范围 (留账)

- ❌ **DL-3 阈值哨** (DB 大小 / WAL checkpoint / write lock wait 监控) — 留 DL-3 单 milestone (蓝图 §5)
- ❌ **EventBus 切 NATS/Redis** (蓝图 §4.C.11, 留 DL-3 阈值哨触发再启)
- ❌ **HB-2 v0(D) Borgee Helper SQLite consumer** — 留 HB-2 v0(D) 单 milestone (跟 host-bridge 域不撞 events 域)
- ❌ **session_resume_hint 表** (蓝图 §2.7 第 3 行, agent replay 路径) — 留 DL-5+ (realtime §1.3 + DL-2 events 不同 concern)
- ❌ **events 实时多端推 (RT-3 fanout 上游 hook)** — RT-3 #588 已 merged 走 hub.PushFrame, DL-2 cold stream 不接 fanout (留 follow-up)
- ❌ **per-user events feed / inbox 视图** — 留 DL-5+ (隐私契约消费侧, 不在 v1 数据层)
- ❌ **events FTS 搜索** — 留 v3+

## 4. 跨 milestone byte-identical 锁

- 复用 DL-1 #609 4 interface (EventBus signature byte-identical, factory NewDataLayer 不动)
- 复用 reasons.IsValid #496 / AP-4-enum #591 / NAMING-1 #614 enum SSOT 模式 (mustPersistKinds 单源)
- 复用 AL-7 #533 + HB-5 audit retention sweeper 模式 (events_retention sweeper 同精神)
- 复用 ULID lex_id 蓝图 §4.A.1+§4.A.4 (channel_events + global_events 主键 + cursor)
- 复用 admin god-mode 不挂红线 (ADM-0 §1.3, events 域不挂 admin /admin-api/.*events.*)
- 0-endpoint-改 wrapper 决策树**变体**: 跟 INFRA-3/4 / CV-15 / TEST-FIX-3 / REFACTOR-1/2 / NAMING-1 同源 (真有 prod code 0 endpoint 改)

## 5. 派活 + 双签

派 **zhanma-c** (NAMING-1 #614 主战熟手, 续作减学习成本) 或 zhanma-d. 飞马 review.

双签流程: spec brief → team-lead → 飞马自审 ✅ APPROVED → yema stance + liema acceptance → zhanma 起实施 (DL2.1+2+3 三段一 PR, **teamlead 唯一开 PR**).

## 6. 飞马 (架构师) 自审表态

✅ **APPROVED with 1 必修条件**:

🟡 必修: **DL-1 interface signature byte-identical 不破** — 反约束 grep #1 真守, `git diff -- internal/datalayer/eventbus.go` Publish/Subscribe signature 行 0 hit. zhanma PR body 必示 interface diff 输出.

担忧 (1 项, 中度):
- 🟡 cold-stream consumer 异步 INSERT 失败处理 — v1 立场: logging-only 不阻塞 hot stream (hot 永远先返回 success, cold 失败 retry 走 sweeper). 战马实施需明示注释 + acceptance 反向断言 (cold INSERT 失败 → metrics counter + log Error, hot stream subscriber 不感知).

留账接受度全 ✅: DL-3 / EventBus 切 NATS / HB-2 v0(D) / session_resume_hint / events fanout 接 RT-3 / per-user feed / FTS 全留账, 不强塞本 PR.

**ROI 拍**: DL-2 ⭐⭐ — 蓝图 §3.4 必落清单兑现 + EventBus persistence v1 立, 阻塞 admin/impersonation 隐私契约 (Q9.3+Q9.4) 解锁; 跟 DL-3 / HB-2 v0(D) 串行依赖, DL-2 是前置.
