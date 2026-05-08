# WIRE-1 stance checklist — DL-2 + DL-3 + RT-3 三处 wire-up 收口 (server-only)

> 7 立场 byte-identical 跟 wire-1-spec.md §0+§2 (飞马 v0 待 commit). **真兑现 G4.audit 交叉核验 P0 项 1 (zhanma-c 抓的 wire-up 死代码)** — 实施 ✅ 已合 (DL-2 #617 / DL-3 #618 / RT-3 #588) 但 server.go boot 未注入 production 路径. **真有 prod code (server.go boot wire-up + interface 注入) 但 0 schema / 0 endpoint shape / 0 client UI**. 跟 BPP-3 #489 wire-up 同模式承袭. content-lock 不需 (server-only plumbing).

## 1. server.go boot 注入三处 (production 真启)
- [ ] **DL-2 cold stream wire-up** — `server.go::SetupRoutes` 注册 `coldArchiveSweeper.Start(s.ctx)` (ctx-aware 跟 #608 协议)
- [ ] **DL-3 offloader wire-up** — `server.go` 注册 `offloader.Start(s.ctx)` + Prometheus metrics endpoint mount
- [ ] **RT-3 AgentTaskNotifier wire-up** — `server.go` 注册 `agentTaskNotifier` 接 BPP-2.2 task lifecycle frame → DL-4 push fanout
- [ ] 反向 grep 旧死代码注入点 0 hit (反 wire-up 残留)

## 2. interface 注入 byte-identical 跟 DL-1 #609 4 interface
- [ ] EventBus / Storage / Repository / PresenceStore 既有 interface byte-identical 不破
- [ ] factory `NewDataLayer(cfg)` 单源 + handler baseline N=108 不动 (DL-1 #609 CI 守门链第 6 处)
- [ ] 反 `EventBusV2 / EventBusWired / WiredDataLayer` 同义词漂

## 3. ctx-aware shutdown (跟 TEST-FIX-2 #608 + #612/#613 deterministic 协议)
- [ ] sweeper / offloader / notifier 全走 `s.ctx` (反 `context.Background()` 漏 cancel)
- [ ] 反 mask 5 模式 reject (Sleep / retry / timeout 提升 / skip / 降阈值)
- [ ] cov ≥85% 不降 (#613 gate, user memory `no_lower_test_coverage` 铁律)

## 4. 0 schema / 0 endpoint shape 改
- [ ] 反向 grep `migrations/wire_1_` 0 hit + `currentSchemaVersion` 不动
- [ ] 0 endpoint 加 / 0 既有 endpoint shape 改 (server.go register 仅 wire-up boot 注入)

## 5. 真兑现率 50% → 100% (蓝图立场承袭)
- [ ] DL-2 cold stream 真兑现 — 蓝图 `data-layer.md` §3 retention + concept-model §0 用户主权 (用户数据真归档)
- [ ] DL-3 offloader 真兑现 — 蓝图 §4 events_archive 阈值哨告警真触发
- [ ] RT-3 AgentTaskNotifier 真兑现 — 蓝图 `realtime.md §3.4` 隐私契约 + `§1.4` 多端 fanout 真推

## 6. 既有测试全 PASS (反 race-flake)
- [ ] 既有 unit + e2e byte-identical 不破 (反 wire-up 顺手改测试)
- [ ] 0 race-flake — 跟 TEST-FIX-1/2/3 协议承袭
- [ ] server-go ./... 全 25+ packages 全绿 (+sqlite_fts5 tag)

## 7. admin god-mode 不挂 wire-up (ADM-0 §1.3 红线)
- [ ] 反向 grep `admin.*coldArchive|admin.*offloader|admin.*agentTaskNotifier` 0 hit
- [ ] 反向 grep `/admin-api.*` wire 路径 0 hit
- [ ] 走 user-rail (anchor #360 owner-only ACL 锁链 22+ PRs 立场延伸)

## 反约束 — 真不在范围
- ❌ 改 endpoint shape / response body / error code 字面 (反 0 行为改, refactor 类立场)
- ❌ 0 schema / 0 migration / 0 client UI 改
- ❌ 加新 CI step (跟 DL-1/2/3 + REFACTOR-1/2 + INFRA-3 + TEST-FIX-* 同精神)
- ❌ admin 加挂 wire-up 路径 (永久不挂, ADM-0 §1.3 红线)

## 跨 milestone byte-identical 锁链 (5 链)
- **DL-2 #617 + DL-3 #618 + RT-3 #588** — 实施已合, 本 PR 仅 server.go boot wire-up 真启
- **DL-1 #609 4 interface byte-identical** — wire-up 走既有 interface, 反 V2 同义词漂
- **TEST-FIX-2 #608 ctx-aware shutdown + #612/#613 deterministic** — sweeper/offloader/notifier ctx 真启
- **BPP-3 #489 wire-up 模式** — server.go boot 注入跟 PluginFrameDispatcher Register 同精神承袭
- **anchor #360 owner-only ACL 锁链 22+ PRs** + REG-INV-002 fail-closed + ADM-0 §1.3 红线

## PM 拆死决策 (3 段)
- **wire-up 收口 vs 改实施拆死** — 仅 server.go boot 注入 (本 PR), 反"顺手改 cold stream / offloader / notifier 实施" (反 0 行为改)
- **ctx-aware vs context.Background() 拆死** — s.ctx (跟 #608 协议), 反 background 漏 cancel
- **0-行为改 vs SLO 收紧拆死** — 反"为绕 wire 改 endpoint shape" (反 SLO 收紧, 跟 REFACTOR-1/2 同精神)

## 用户主权红线 (5 项)
- ✅ 真兑现率 50%→100% (用户数据真归档 + 阈值哨真触发 + 多端推真推)
- ✅ 既有 ACL gate + interface byte-identical 不破
- ✅ 0 行为改 / 0 schema / 0 endpoint shape 改
- ✅ ctx-aware shutdown 真守 (反 race-flake)
- ✅ admin god-mode 不挂 wire-up (ADM-0 §1.3 红线)

## PR 出来 5 核对疑点
1. 三处 wire 真启 — server.go boot 注册 cold stream / offloader / notifier 各 1 处
2. 反向 grep `EventBusV2|WiredDataLayer|context.Background\\(\\).*sweeper` count==0
3. 既有 unit + e2e 全 PASS byte-identical + cov ≥85%
4. 真兑现 100% — 蓝图 §3 retention + §4 阈值 + §1.4+§3.4 多端推真活
5. admin god-mode + admin/* path 反向 grep 0 hit
