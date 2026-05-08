# HB-6 spec brief — heartbeat lag detection (战马D v0)

> Phase 6 heartbeat lag percentile 监控 — agent_runtimes.last_heartbeat_at
> 既有列 (AL-4.1 #398 v=16) 计算 P50/P95/P99 lag, 跟 BPP-4 #499 watchdog
> (30s 周期检) + HB-3 v2 #14 decay 三档同源, 给 admin 一个 readonly
> 监控眼: 现网 plugin 是否 lag 接近 watchdog 阈值. 0 schema / 0 新表 /
> 0 新 admin_actions enum (复用既有列 + AL-1a reasons.NetworkUnreachable
> 字面单源).

## §0 立场 (3 + 3 边界)

- **①** 0 schema 改 (复用 agent_runtimes.last_heartbeat_at + agent_id +
  status 既有列 — AL-4.1 #398 v=16 既有). 反向 grep `migrations/hb_6_\d+
  | ALTER agent_runtimes.*lag_p` 在 internal/migrations/ 0 hit (本 milestone
  无 schema 段).
- **②** lag 计算 server 内存 read-side 走 SELECT (now - last_heartbeat_at)
  FROM agent_runtimes WHERE status='running' AND last_heartbeat_at IS NOT
  NULL — 计算 30s 滚窗 P50/P95/P99 + count, **不写表 / 不另起 retention
  / 不另起 admin_actions enum** (反向 grep `lag_audit\|hb_lag_log\|
  heartbeat_lag_table` 0 hit; AST 锁链延伸第 16 处 forbidden 3 token
  `pendingLagSample / lagSampleQueue / deadLetterLag`).
- **③** admin-rail only GET /admin-api/v1/heartbeat-lag (跟 ADM-2.2 +
  AL-7.2 + AL-8 admin-rail 同模式; admin readonly 不挂 PATCH/POST/DELETE
  ADM-0 §1.3 红线 — admin 看不能改 / lag 是 derived metric 写也无意义).
  AL-1a reason 复用 reasons.NetworkUnreachable byte-identical 跟 BPP-4
  #499 watchdog timeout reason 同源 (AL-1a 锁链第 19 处 — DM-7 #18 +
  HB-5 #17 + AL-8 #16 + AL-7 #15 承袭).

边界:
- **④** BPP-4 #499 watchdog 30s 周期 + HB-3 v2 #14 decay 三档不动 — HB-6
  仅 read-side 视图, 不改 watchdog 节奏 / 不改 decay 阈值 / 不另起 sweeper
  goroutine (反向 grep `HeartbeatLagSweeper\|lag_ticker` 在 internal/
  count==0; HB-6 是同步 GET handler 即时聚合 NOT scheduled job).
- **⑤** WindowSeconds=30 const byte-identical 跟 BPP-4 watchdog WatchdogPeriod
  同源 (改一处 = 改两处 — internal/api/hb_6_lag.go::WindowSeconds + internal/
  bpp watchdog WatchdogPeriod 反向锁); LagThresholdMs=15000 const
  (=watchdog 周期一半, percentile 高于阈值视为 'at risk', 体现 reason
  NetworkUnreachable 的潜在风险).
- **⑥** 不挂 client UI v1 — admin dashboard 留 v3, 跟 AL-7/AL-8 admin-rail
  v0 only 同精神 (admin 用 curl / SPA v3 拉数据). 反向 grep `useHeartbeat
  Lag\|HeartbeatLagPanel` 在 client/src/ 0 hit.

## §1 拆段

**HB-6.1 — server lag aggregator** (`internal/api/hb_6_lag.go`):
- `HB6LagHandler{Store, Logger}` struct + `RegisterAdminRoutes(mux,
  adminMw)`.
- `aggregateLag(nowMs int64)` — SELECT (nowMs - last_heartbeat_at) AS
  lag_ms FROM agent_runtimes WHERE status='running' AND last_heartbeat_at
  IS NOT NULL AND last_heartbeat_at >= ? (nowMs - WindowSeconds*1000);
  in-memory sort + linear interpolation P50/P95/P99 (跟 std stat lib
  modeled, reason 复用 reasons.NetworkUnreachable byte-identical).
- response: `{count: N, p50_ms, p95_ms, p99_ms, threshold_ms,
  at_risk: bool, sampled_at: nowMs, reason_if_at_risk: 'network_unreachable'
  | null, window_seconds: 30}`.

**HB-6.2 — admin readonly endpoint** GET /admin-api/v1/heartbeat-lag —
admin-rail (跟 ADM-2.2 + AL-7.2 + AL-8 同模式).

**HB-6.3 — closure**: REG-HB6-001..006 6 🟢.

## §2 反约束 grep 锚

- 0 schema: 反向 grep `migrations/hb_6_\d+\|ALTER agent_runtimes.*lag` 0 hit.
- 0 sweeper goroutine: 反向 grep `HeartbeatLagSweeper\|hb6.*Ticker` 0 hit.
- 0 client UI v1: 反向 grep `useHeartbeatLag\|HeartbeatLagPanel` 在
  client/src/ 0 hit.
- AL-1a reason 锁链第 19 处: lag at-risk reason 字面=`'network_unreachable'`
  byte-identical 跟 reasons.NetworkUnreachable.
- AST 锁链延伸第 16 处 forbidden 3 token (`pendingLagSample / lagSampleQueue
  / deadLetterLag`) 0 hit.
- admin-rail only: 反向 grep `mux.Handle("(POST|PATCH|PUT|DELETE).*admin-
  api/v[0-9]+/.*heartbeat-lag` 0 hit + 反向 grep `/api/v1/heartbeat-lag`
  user-rail 0 hit.

## §3 不在范围

- lag 历史持久化表 (留 v3, AL-7 retention 同期).
- admin SPA dashboard UI (留 v3 跟 AL-7/AL-8 同精神).
- Prometheus metrics exporter (留 v3).
- 主动告警 (Slack/Email) — HB-6 是 read-side 视图, 告警留 v4.
- 跨 org lag 隔离 (留 AP-3 同期).
- per-agent lag 历史时序 (留 v3 — v0 仅 30s 滚窗 percentile).
- runtime hot-mutate WindowSeconds (留 v3 — const 字面单源).
