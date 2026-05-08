# HB-6 acceptance — heartbeat lag detection

战马D · 2026-04-30 · spec `hb-6-spec.md` + stance `hb-6-stance-checklist.md`.

## §1 schema

- §1.1 ✅ 0 schema 改 — 复用 agent_runtimes (AL-4.1 #398 v=16) 既有列
  last_heartbeat_at + agent_id + status. 反向 grep
  `migrations/hb_6_\d+|ALTER agent_runtimes.*lag` 0 hit.

## §2 server lag aggregator

- §2.1 ✅ aggregateLag 30s 滚窗 (`internal/api/hb_6_lag.go::aggregateLag`)
  SELECT lag_ms FROM agent_runtimes WHERE status='running' AND last_
  heartbeat_at IS NOT NULL AND last_heartbeat_at >= nowMs - 30000.
- §2.2 ✅ P50/P95/P99 linear interpolation (5 sample → P50=mid / P95=top
  / P99=top).
- §2.3 ✅ count + at_risk + reason_if_at_risk (P95>15000ms → at_risk=true
  + reason='network_unreachable' byte-identical 跟 reasons.NetworkUnreachable).
- §2.4 ✅ window cutoff (>30s stale sample 不计入).

## §3 admin-rail endpoint

- §3.1 ✅ GET /admin-api/v1/heartbeat-lag — admin happy path 200.
- §3.2 ✅ non-admin 401 (admin cookie 缺失).
- §3.3 ✅ user-rail /api/v1/heartbeat-lag 不挂 (404).
- §3.4 ✅ POST/PATCH/PUT/DELETE 在 admin-api/v1/heartbeat-lag 不挂 (反向
  grep 0 hit).

## §4 反约束

- §4.1 ✅ 0 schema (反向 grep migrations/hb_6_).
- §4.2 ✅ 0 sweeper goroutine (反向 grep HeartbeatLagSweeper).
- §4.3 ✅ 0 client UI v1.
- §4.4 ✅ AL-1a reason 锁链第 19 处.
- §4.5 ✅ AST 锁链延伸第 16 处.
- §4.6 ✅ admin-rail only.

## §5 测试矩阵

- TestHB61_NoSchemaChange ✅
- TestHB61_AggregateLag_PercentileCorrect ✅
- TestHB61_WindowCutoffExcludesStale ✅
- TestHB62_AdminHappyPath ✅
- TestHB62_NonAdmin401 ✅
- TestHB62_AtRiskReasonByteIdentical ✅
- TestHB62_NoUserRailPath ✅
- TestHB63_NoAdminWritePath ✅
- TestHB63_NoLagSampleQueue ✅
