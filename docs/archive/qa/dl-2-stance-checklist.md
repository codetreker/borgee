# DL-2 stance checklist — events 双流 (hot live + cold archive) + retention (server-only)

> 7 立场 byte-identical 跟 dl-2-spec.md §0+§2 (飞马 v0 待 commit). **真有 prod code (events_archive 表 + retention sweeper + EventBus 双流接) 但 0 endpoint 行为改 + 0 client UI**. 跟 DL-1 #609 4 interface + REFACTOR-1 #611 + RT-3 #588 + AL-7 retention 类别同模式承袭. content-lock 不需 (server-only plumbing 0 user-visible 字面改).

## 1. events 双流 byte-identical 跟蓝图 §4 (hot live + cold archive)
- [ ] **hot live stream** — DL-1 EventBus.Publish/Subscribe byte-identical 不破 (in-process 既有路径承袭)
- [ ] **cold archive stream** — `events_archive` 表单源, sweeper 周期搬迁 (反双轨 drift)
- [ ] 双流字面 byte-identical 跟蓝图 §4 (`hot_live` / `cold_archive` 命名锚, 反 `live_stream / archive_stream` / `event_log_alt` 同义词漂)
- [ ] 反向 grep 双流字面跨 spec/code/单测三处对锁 byte-identical

## 2. retention 阈值按类型分 (channel / agent_task / artifact 各自 enum)
- [ ] **3 类型 enum 分立** — `channel` / `agent_task` / `artifact` 各自阈值 const, 反单一 `RETENTION_DAYS` 常数塌陷 (反扁平化)
- [ ] **黑名单 grep 真测**: `grep -rn "RETENTION_DAYS = " | wc -l` ≤ 1 (单源 enum, 反多处散布常数)
- [ ] 跟 AL-1a 6-dict + HB-1 7-dict + HB-2 8-dict + AP-4-enum 14-cap 字典分立第 5 处承袭
- [ ] enum 命名锁 (例: `RetentionChannel` / `RetentionAgentTask` / `RetentionArtifact`) byte-identical, 反 `Channel_Retention` / `RETENTION_CHANNEL_DAYS` 同义词漂
- [ ] 反向 grep 反 enum 第 4 类型漂入 (反 5/3 偷工减料, 跟 DL-1 4 interface count==4 同精神)

## 3. EventBus interface 不破 (跟 DL-1 byte-identical)
- [ ] **DL-1 #609 EventBus.Publish/Subscribe** 签名 byte-identical 不破 (DL-1 4 interface count==4 锚守)
- [ ] **0 新增 interface** — DL-2 仅 impl 层加双流路径, 反"加 EventBusV2 / ArchivedEventBus" 等同义词漂 (反 SSOT)
- [ ] **factory `NewDataLayer(cfg)` 单源不破** — DL-1 boot wire 不动 (反向 grep `^func NewDataLayer` count==1)
- [ ] **handler 不直 import store** baseline N=108 (DL-1 #609 CI 守门链第 6 处 byte-identical, 反 future PR 漂入直查)

## 4. 0 endpoint 行为改 (后端 plumbing)
- [ ] **0 endpoint shape 改** — `git diff origin/main -- internal/api/server.go | grep -E '\\+.*Method|\\+.*Register'` 0 hit
- [ ] **0 response body / 0 error code 字面改** — 既有错码 (`dm.*`/`pin.*`/`chn.*`) byte-identical
- [ ] 既有 unit + e2e 全 PASS byte-identical (反 race-flake, 跟 #612/#613 cov 85% 协议承袭)

## 5. 0 schema 漂 (新表 events_archive 单源, 不动既有)
- [ ] **`events_archive` 表单源** — 反"events_archive_v2 / event_log_alt / archived_events" 等同义词漂入
- [ ] **黑名单 grep 真测**: `grep -rn "events_archive_v2\|event_log_alt"` 0 hit (单表)
- [ ] **不动既有 `messages` / `audit_log` schema** — 反 ALTER 漂 (反向 grep 0 行改既有 schema)
- [ ] **migration 单源**: `internal/migrations/dl_2_*_events_archive.go` 1 文件; v 号顺序锚 (post-#612 v 号字面 + audit_events ADM-3 #586 衔接)
- [ ] reverse grep 双源/三源 events 表 0 hit (events_archive 真单源)

## 6. retention sweeper deterministic (反 race-flake, 跟 #608 ctx wiring 同精神)
- [ ] **ctx-aware shutdown** — sweeper goroutine `Start(ctx)` 走 server.New(ctx) 入参 (跟 TEST-FIX-2 #608 + #612/#613 deterministic 协议承袭)
- [ ] **反 mask 5 模式 reject** — `time.Sleep(retry)` / retry loop / timeout 提升 / skip / 降阈值 0 hit (跟 TEST-FIX-1/2/3 + REFACTOR-1/2 同精神)
- [ ] **反 race-flake** — sweeper 单测走 t.Context() + 注入 nowFn (跟 #612 deterministic + AL-3 PresenceTracker 既有同精神)
- [ ] cov ≥85% 不降 (#613 gate 真过, user memory `no_lower_test_coverage` 铁律)

## 7. admin god-mode 不挂 (ADM-0 §1.3 红线)
- [ ] 反向 grep `admin.*events_archive|admin.*retention` 在 packages/server-go/ 0 hit
- [ ] 反向 grep `/admin-api.*events|/admin-api.*retention` 0 hit
- [ ] retention sweeper 走 user-rail 路径, 反 admin override 漂 (anchor #360 owner-only ACL 锁链 22+ PRs 立场延伸)

## 反约束 — 真不在范围
- ❌ DL-3 阈值哨 (留各自 milestone, 不在本 PR scope)
- ❌ SQLite → PG 真切 (留 v3+, 蓝图 §4 C #10 字面禁)
- ❌ NATS / Kafka / Redis Streams EventBus 真切 (留 v3+; 本 PR 仅 in-process EventBus 双流接)
- ❌ 改 production endpoint 行为 / response body / error code 字面 (反 0 行为改)
- ❌ 0 client UI / 0 acceptance template / 0 content-lock 改 (server-only plumbing)
- ❌ 加新 CI step (跟 DL-1 + REFACTOR-1/2 + INFRA-3 + TEST-FIX-* 同精神)
- ❌ admin god-mode 加挂 retention / events 任何路径 (永久不挂)

## 跨 milestone byte-identical 锁链 (5 链)
- **DL-1 #609** — 4 interface (Storage/PresenceStore/EventBus/Repository) byte-identical 不破 + factory 单源 + handler baseline N=108 承袭
- **AL-7 #533 + AP-2.1 #525** retention sweeper 模式 — sparse partial idx + ctx-aware sweeper 同模式承袭
- **TEST-FIX-2 #608 ctx-aware shutdown + #612/#613 deterministic** — sweeper 反 race-flake 协议承袭
- **5-field audit JSON-line schema 锁链** — `actor/action/target/when/scope` events_archive 真兑现承袭 (跨 HB-1/HB-2/BPP-4/HB-4/HB-3)
- **anchor #360 owner-only ACL 锁链 22+ PRs** — DL-2 retention 决策 owner-only 不破 + REG-INV-002 fail-closed + ADM-0 §1.3 红线

## PM 拆死决策 (3 段)
- **events 双流 vs 单流塌陷拆死** — hot live (DL-1 EventBus byte-identical) + cold archive (events_archive 表) 双流分立, 反"统一 events_log 单流" (反扁平化)
- **retention 3 enum 分类 vs 单常数拆死** — channel/agent_task/artifact 各自 const, 反 `RETENTION_DAYS` 单常数塌陷 (黑名单 grep ≤1 守)
- **EventBus interface 不破 vs V2/Archived 漂拆死** — DL-1 4 interface byte-identical, 反"加 EventBusV2 / ArchivedEventBus" 同义词漂 (反 SSOT)

## 用户主权红线 (5 项)
- ✅ 0 行为改 (e2e + unit 全 PASS byte-identical, 反 race-flake)
- ✅ 既有 ACL gate 字面 + 行为 byte-identical (anchor #360 owner-only + REG-INV-002 守)
- ✅ 0 user-facing change (server-only plumbing)
- ✅ 0 endpoint shape / 0 既有 schema 改 (反 events_archive_v2 同义词漂)
- ✅ admin god-mode 不挂 retention / events (ADM-0 §1.3 红线)

## PR 出来 5 核对疑点
1. 黑名单 grep `events_archive_v2|event_log_alt` count==0 + `RETENTION_DAYS = ` count ≤1 (单表 + 单源 enum)
2. 0 endpoint shape 改 + 0 既有 schema 改 (`git diff` 反向断言, migration 仅 dl_2_* 新文件)
3. EventBus interface byte-identical 不破 + factory `NewDataLayer` 单源 (DL-1 #609 锚承袭真验)
4. cov ≥85% (#613 gate) + 0 race-flake + sweeper deterministic (ctx + nowFn 注入)
5. admin god-mode 反向 grep 0 hit (`admin.*events_archive|admin.*retention|/admin-api.*events|/admin-api.*retention`)
