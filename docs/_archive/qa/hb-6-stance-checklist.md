# HB-6 stance checklist (战马D v0)

> 战马D · 2026-04-30 · HB-6 立场守门 (3 + 3 边界).

## §0 立场 3 项

- [x] **① 0 schema 改** — 复用 agent_runtimes.last_heartbeat_at + agent_id +
  status (AL-4.1 #398 v=16 既有). 反向 grep `migrations/hb_6_\d+` 在
  internal/migrations/ 0 hit (本 milestone 无 schema 段).
- [x] **② lag read-side derived** — server 内存计算 30s 滚窗 P50/P95/P99,
  不写表 / 不另起 retention / 不另起 admin_actions enum. 反向 grep
  `lag_audit\|hb_lag_log\|heartbeat_lag_table` 在 internal/ 0 hit. AST 锁链
  延伸第 16 处 forbidden 3 token (`pendingLagSample / lagSampleQueue /
  deadLetterLag`) 0 hit.
- [x] **③ admin-rail only** GET /admin-api/v1/heartbeat-lag (跟 ADM-2.2
  + AL-7.2 + AL-8 admin-rail 同模式; admin readonly 不挂 PATCH/POST/DELETE
  ADM-0 §1.3 红线 — admin 看不能改). AL-1a reason 复用 reasons.
  NetworkUnreachable byte-identical 跟 BPP-4 #499 watchdog 同源 (AL-1a
  锁链第 19 处 — DM-7 #18 + HB-5 #17 + AL-8 #16 + AL-7 #15 承袭).

## §0.边界 3 项

- [x] **④ watchdog 节奏不动** — BPP-4 #499 watchdog 30s 周期 + HB-3 v2
  #14 decay 三档不动. HB-6 是同步 GET handler 即时聚合, NOT scheduled
  job. 反向 grep `HeartbeatLagSweeper\|hb6.*Ticker\|lag_ticker` 在 internal/
  0 hit.
- [x] **⑤ WindowSeconds 双向锁** WindowSeconds=30 const byte-identical
  跟 BPP-4 WatchdogPeriod 同源 (改一处 = 改两处反向锁守门); LagThreshold
  Ms=15000 const (=watchdog 周期一半).
- [x] **⑥ 不挂 client UI v1** — admin dashboard 留 v3, 跟 AL-7/AL-8
  admin-rail v0 only 同精神. 反向 grep `useHeartbeatLag\|HeartbeatLagPanel`
  在 client/src/ 0 hit.

## §1 测试覆盖

- [x] REG-HB6-001 0 schema (`TestHB61_NoSchemaChange` 反向 grep 守门).
- [x] REG-HB6-002 lag aggregator 准 (`TestHB61_AggregateLag_PercentileCorrect`
  HappyPath 5 sample → P50=mid / P95=top / count==5).
- [x] REG-HB6-003 30s window cutoff (`TestHB61_WindowCutoffExcludesStale`
  超 30s 的 sample 排除; status≠running 排除).
- [x] REG-HB6-004 admin readonly + non-admin 401 + 不挂 PATCH/POST/DELETE
  (`TestHB62_AdminHappyPath` + `_NonAdmin401` + `TestHB63_NoAdminWritePath`).
- [x] REG-HB6-005 at_risk reason byte-identical (`TestHB62_AtRiskReason
  ByteIdentical` reason='network_unreachable' 字面跟 reasons.NetworkUnreachable
  同源 — AL-1a 锁链第 19 处).
- [x] REG-HB6-006 AST 锁链延伸第 16 处 (`TestHB63_NoLagSampleQueue` 反向
  grep 3 forbidden token + 反向 grep user-rail /api/v1/heartbeat-lag 0 hit).

## §2 反约束 grep 锚

- 0 schema: `migrations/hb_6_\d+|ALTER agent_runtimes.*lag` 0 hit.
- 0 sweeper: `HeartbeatLagSweeper|hb6.*Ticker|lag_ticker` 0 hit.
- 0 client UI: `useHeartbeatLag|HeartbeatLagPanel` 在 client/src/ 0 hit.
- AL-1a reason 锁链第 19 处: lag at-risk reason 字面='network_unreachable'.
- AST 锁链延伸第 16 处: 3 forbidden token 0 hit.
- admin-rail only: PATCH/POST/PUT/DELETE 在 admin-api 0 hit + user-rail
  /api/v1/heartbeat-lag 0 hit.
