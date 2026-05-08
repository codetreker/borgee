# DL-3 stance checklist — events_archive 监控 + 阈值告警 + DL-2 sweeper 统计 (server-only)

> 7 立场 byte-identical 跟 dl-3-spec.md §0+§2 (飞马 v0 待 commit). **真有 prod code (阈值常数 SSOT + Prometheus metrics + sweeper 统计 hook + cold archive offload 走 DL-1 Storage interface) 但 0 schema 改 / 0 endpoint 改 / 0 client UI**. 跟 DL-1 #609 4 interface + DL-2 events 双流 + REFACTOR-1/2 字面锁同模式承袭. content-lock 不需 (server-only metrics, 0 user-visible 字面改).

## 1. 0 schema 改 (复用 DL-2 表)
- [ ] 反向 grep `migrations/dl_3_` 在 packages/server-go/ 0 hit
- [ ] `currentSchemaVersion` 不动 (反向断 0 行改)
- [ ] 复用 DL-2 events_archive 单表 (反"加 dl_3_metrics 表"漂入)
- [ ] 反 ALTER 既有 schema (反 events_archive 加字段漂)

## 2. DL-2 EventBus / DL-1 4 interface byte-identical
- [ ] **DL-1 EventBus.Publish/Subscribe** 签名 byte-identical 不破 (4 interface count==4 锚守)
- [ ] **DL-2 events 双流** byte-identical 协同 — DL-3 监控 hook 仅 read 不写 (反污染 hot live + cold archive 写路径)
- [ ] **0 新 interface** — 反 `MetricsBus / AlertBus / EventBusV3` 同义词漂 (反 SSOT)
- [ ] factory `NewDataLayer(cfg)` 单源不破 + handler baseline N=108 (DL-1 #609 CI 守门链第 6 处承袭)
- [ ] 反向 grep `EventBusV3|MetricsBus|AlertBus|grants_v3` 0 hit

## 3. 阈值常数单源 SSOT (反 magic number)
- [ ] **3 阈值常数 const SSOT** (跟 DL-2 retention 3 enum 分类同精神承袭): events_archive 行数告警 / sweeper 滞后秒数告警 / cold archive offload 触发阈值
- [ ] **黑名单 grep 真测**: 反向 grep magic number 散布 (反 `if count > 10000` / `if lag > 60` 硬编码), 反多处 `const Threshold = N` 散布
- [ ] **enum 命名锁** (例: `ArchiveRowsAlertThreshold` / `SweeperLagAlertSeconds` / `ColdArchiveOffloadRows`) byte-identical, 反 `THRESHOLD_*` / `*_LIMIT` 同义词漂
- [ ] 跟 AL-1a 6-dict + HB-1 7-dict + HB-2 8-dict + AP-4-enum 14-cap + DL-2 retention 3-enum 字典分立第 6 处承袭

## 4. metrics 走 Prometheus / log 单源 (反多渠道散布)
- [ ] **Prometheus metrics 单一渠道** — 反"同时走 OpenTelemetry / StatsD / Datadog 多渠道散布"漂
- [ ] **log 走 既有 logger 单源** — 反另起 alert log 旁路 (反 SSOT)
- [ ] 反向 grep `otel|opentelemetry|statsd|datadog|newrelic` 在 internal/datalayer/ 0 hit (单渠道真守)
- [ ] metrics 命名 `borgee_dl3_*` 前缀 byte-identical (跟 Prometheus naming convention 同源, 反 `dl3.*` / `borgee.dl3.*` dot-notation 漂)

## 5. cold archive offload 走 DL-1 Storage interface (反另起 path)
- [ ] **Storage.PutBlob 真兑现 cold archive offload** — 跟 DL-1 #609 Storage 3 method (GetURL/PutBlob/Delete) byte-identical 复用
- [ ] **不另起 offload path** — 反 `S3Uploader / OffloadClient / ColdArchiveSync` 同义词漂入 (反 SSOT)
- [ ] 反向 grep `s3Upload|coldArchiveSync|offloadClient` 0 hit
- [ ] 跟 CV-1 artifacts.go Storage 既有 byte-identical 承袭 (DL-1 #609 锚)
- [ ] cold archive offload 仅 server-side (反 client 直 S3 漂)

## 6. 0 endpoint 改 (server plumbing)
- [ ] 0 endpoint shape 改 — `git diff origin/main -- internal/api/server.go | grep -E '\\+.*Method|\\+.*Register'` 0 hit
- [ ] 0 response body / 0 error code 字面改 — 既有错码 (`dm.*`/`pin.*`/`chn.*`) byte-identical
- [ ] 0 client UI 改 (server-only metrics + log)
- [ ] 既有 unit + e2e 全 PASS byte-identical (反 race-flake, 跟 #612/#613 cov 85% 协议承袭)

## 7. admin god-mode 不挂 (ADM-0 §1.3 红线)
- [ ] 反向 grep `admin.*metrics|admin.*alert|admin.*sweeper` 在 packages/server-go/ 0 hit
- [ ] 反向 grep `/admin-api.*metrics|/admin-api.*alert` 0 hit
- [ ] metrics + alert + sweeper 走 user-rail / Prometheus scrape, 反 admin override (anchor #360 owner-only ACL 锁链 22+ PRs 立场延伸)

## 反约束 — 真不在范围
- ❌ 加 dl_3_metrics 表 / 加 schema 字段 / 加 migration v 号
- ❌ 加新 endpoint / 改既有 endpoint shape / 0 client UI 改
- ❌ OpenTelemetry / StatsD / Datadog 多渠道 (留 v3+, 反 SSOT)
- ❌ 另起 cold archive offload path (反 DL-1 Storage interface 复用)
- ❌ admin god-mode 加挂 metrics / alert / sweeper (永久不挂, ADM-0 §1.3 红线)
- ❌ 加新 CI step (跟 DL-1/2 + REFACTOR-1/2 + INFRA-3 + TEST-FIX-* 同精神)

## 跨 milestone byte-identical 锁链 (5 链)
- **DL-1 #609** — 4 interface byte-identical (Storage/Presence/EventBus/Repository) + factory 单源 + handler baseline N=108
- **DL-2 events 双流** — DL-3 监控仅 read hot live + cold archive (反污染写路径)
- **AL-7 #533 + AP-2.1 #525 retention sweeper 模式** — DL-3 sweeper 统计 hook ctx-aware (跟 #608 + #612/#613 deterministic 协议承袭)
- **CV-1 artifacts.go Storage** — DL-3 cold archive offload 走 DL-1 Storage 复用 (Storage 3 method byte-identical)
- **anchor #360 owner-only ACL 锁链 22+ PRs** + REG-INV-002 fail-closed + ADM-0 §1.3 红线

## PM 拆死决策 (3 段)
- **阈值常数 SSOT vs magic number 拆死** — 3 const enum (本 PR 选, 跟 DL-2 3-enum 同精神承袭), 反 if count > 10000 散布 + 反 THRESHOLD_* 同义词漂
- **Prometheus 单渠道 vs 多观测平台拆死** — Prometheus 选 (本 PR), 反 OpenTelemetry / StatsD / Datadog 多渠道散布 (反 SSOT)
- **DL-1 Storage interface 复用 vs 另起 offload 拆死** — Storage.PutBlob 复用 (本 PR), 反 S3Uploader / OffloadClient / ColdArchiveSync 同义词漂

## 用户主权红线 (5 项)
- ✅ 0 行为改 (e2e + unit 全 PASS byte-identical, 反 race-flake)
- ✅ 既有 ACL gate 字面 + 行为 byte-identical (anchor #360 + REG-INV-002 守)
- ✅ 0 user-facing change (server-only metrics + log)
- ✅ 0 schema / 0 endpoint shape / 0 既有 ACL 改
- ✅ admin god-mode 不挂 metrics / alert (ADM-0 §1.3 红线)

## PR 出来 5 核对疑点
1. 黑名单 grep `migrations/dl_3_|EventBusV3|MetricsBus|otel|opentelemetry|statsd|s3Upload|coldArchiveSync` count==0
2. 0 endpoint 改 + 0 既有 schema 改 (`git diff` 反向断言)
3. 阈值常数 3 enum SSOT (反 magic number 散布, 命名锁 byte-identical)
4. Prometheus metrics `borgee_dl3_*` 前缀单一渠道 + log 单源 (反多渠道)
5. cov ≥85% (#613 gate) + 0 race-flake + admin grep 0 hit
