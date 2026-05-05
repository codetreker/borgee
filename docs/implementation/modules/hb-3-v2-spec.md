# HB-3 v2 spec brief — heartbeat decay 锁链第 3 处 (≤80 行)

> 战马D · Phase 6 · ≤80 行 · 蓝图 [`plugin-protocol.md`](../../blueprint/current/plugin-protocol.md) §1.6 (失联与故障状态) + BPP-4 #499 watchdog 30s threshold + BPP-8 #532 lifecycle audit. 模块锚: HB-3 v2 (新 wrapper, 跟 #507 HB-3 host_grants 拆死锚). 依赖 BPP-4 #499 watchdog + BPP-8 #532 lifecycle audit + AL-1 #492 5-state + REFACTOR-REASONS #496 6-dict.
>
> ⚠️ **命名拆死**: HB-3 #507 host_grants schema 已落 (蓝图 host-bridge §1.3); 本 milestone HB-3 v2 = **heartbeat decay** 主题 (跟 host-bridge 域不交). spec 文件名 `hb-3-v2-spec.md` 字面拆死, 反向 grep `HB3HostGrants\|host_grants` 在 hb_3_v2*.go 0 hit 反断.

## 0. 关键约束 (3 条立场, 蓝图 §1.6 + BPP-4 字面承袭)

1. **heartbeat decay 走 stale ratio 三档** — 当前 BPP-4 #499 watchdog 是 binary (≤30s online / >30s error/network_unreachable); HB-3 v2 加 **decay 三档** (`fresh` ≤30s / `stale` 30..60s / `dead` >60s) — 反映 plugin 健康度连续衰减 (跟蓝图 §1.6 "失联状态非 binary" 字面承袭). **反约束**: 反向 grep `heartbeat_decay_table\|hb3_decay_log\|stale_ratio_history` 在 internal/ 0 hit (不裂表 — decay 状态从 `last_heartbeat_at` 反向 derive); 不另起 sequence (跟 BPP-4 watchdog 30s threshold byte-identical 复用 — 不改既有 const).

2. **stale 跨档复用 BPP-8 lifecycle audit** — fresh→stale / stale→dead 跨档 transition 走 `LifecycleAuditor.RecordHeartbeatTimeout` (BPP-8 #532 既有 method, **不另开 RecordHeartbeatStale 旁路**); audit reason 复用 `reasons.NetworkUnreachable` byte-identical (跟 BPP-4 watchdog SetError + BPP-8 RecordHeartbeatTimeout 同源, AL-1a reason 锁链 HB-3 v2 = **第 14 处** BPP-2.2/AL-2b/BPP-4/BPP-5/BPP-6/BPP-7/BPP-8/HB-3 v2). **反约束**: 反向 grep `runtime_recovered\|hb3_specific_reason\|stale_reason\|7th.*reason` 在 internal/ 0 hit; reasons SSOT 6-dict 不动.

3. **decay derive 仅 owner-only 视图** — `GET /api/v1/agents/{agent_id}/heartbeat-decay` owner-only (跟 AL-2a/BPP-3.2/AL-1/AL-5/DM-4/CV-4 v2/BPP-7/BPP-8 owner-only 8 处同模式); admin god-mode 不挂 (ADM-0 §1.3 红线). **反约束**: 反向 grep `admin.*heartbeat.*decay\|admin.*HB3` 在 admin*.go 0 hit; AST 锁链延伸第 6 处 — forbidden tokens (`pendingDecayQueue\|decayRetryQueue\|deadLetterDecay`) 0 hit 跟 BPP-4/5/6/7/8 best-effort 同精神.

## 1. 拆段 (一 milestone 一 PR, 整段一次合 — 跟 BPP-2/3/4/5/6/7/8 协议同源)

| 段 | 文件 | 范围 |
|---|---|---|
| HB-3 v2.1 stale ratio derive helper | `internal/bpp/heartbeat_decay.go` 新 (`DecayState` enum 3 态 fresh/stale/dead + `DeriveDecayState(now, lastHeartbeatAt)` 纯 fn const StaleThreshold=30s/DeadThreshold=60s; 0 schema 改) + 5 unit (3 态 boundary + nil-safe + reasons 复用反断) | 0 schema 改; 复用 BPP-4 既有 last_heartbeat_at 字段 |
| HB-3 v2.2 watchdog wire + GET endpoint | `internal/bpp/heartbeat_watchdog.go` 改 (cross-bucket transition 调 `auditor.RecordHeartbeatTimeout` — fresh→stale / stale→dead, **不另开 RecordHeartbeatStale**) + `internal/api/hb_3_v2_decay_list.go` 新 (GET /api/v1/agents/{agent_id}/heartbeat-decay owner-only ACL DESC limit 100; query admin_actions WHERE action='plugin_heartbeat_timeout' AND target_user_id=agent_id) + 6 unit (cross-bucket audit 真挂 / GET happy + 4 ACL + AST scan 反向) | 复用 BPP-8 audit 路径; 不另起 LifecycleAuditor method |
| HB-3 v2.3 closure + REG-HB3V2 + acceptance + PROGRESS [x] | REG-HB3V2-001..005 + acceptance/hb-3-v2.md + PROGRESS update + AST scan forbidden tokens 锁链延伸第 6 处 0 hit | best-effort 立场承袭 BPP-4/5/6/7/8 锁链延伸第 6 处 |

## 2. 留账边界

- **decay UI 时间轴** (留 v3) — owner-side decay history UI 留 client SPA follow-up, 跟 CV-4 v2 IterationTimeline 同模式
- **decay metric exporter** (留 v3) — Prometheus per-bucket count 留 metrics follow-up, 跟 BPP-7 §2 同源
- **decay window 阈值 owner override** (留 v3) — StaleThreshold/DeadThreshold const 当前固定; per-owner override 留 v3
- **cross-org decay aggregation** (留 v3 跟 AP-3 联动) — 跨 org 看组织级 plugin 健康度

## 3. 反查 grep 锚 (Phase 6 验收 + HB-3 v2 实施 PR 必跑)

```
git grep -nE 'DeriveDecayState|DecayState' packages/server-go/internal/bpp/   # ≥ 1 hit (helper 真挂)
git grep -nE 'GET.*heartbeat-decay' packages/server-go/internal/api/   # ≥ 1 hit (endpoint 真挂)
# 反约束 (5 条 0 hit)
git grep -nE 'heartbeat_decay_table|hb3_decay_log|stale_ratio_history' packages/server-go/internal/   # 0 hit (不裂表, §0.1)
git grep -nE 'runtime_recovered|hb3_specific_reason|stale_reason|7th.*reason' packages/server-go/internal/   # 0 hit (复用 6-dict, §0.2)
git grep -nE 'pendingDecayQueue|decayRetryQueue|deadLetterDecay' packages/server-go/internal/   # 0 hit (best-effort 锁链延伸第 6 处, §0.3)
git grep -nE 'admin.*heartbeat.*decay|admin.*HB3' packages/server-go/internal/api/admin*.go   # 0 hit (ADM-0 §1.3 红线)
git grep -nE 'RecordHeartbeatStale|LifecycleAuditor.*Stale' packages/server-go/internal/   # 0 hit (复用 BPP-8 RecordHeartbeatTimeout, §0.2)
git grep -nE 'CREATE TABLE.*heartbeat_decay|ALTER TABLE.*heartbeat_decay' packages/server-go/internal/migrations/   # 0 hit (0 schema 改)
```

## 4. 不在本轮范围 (反约束 deferred)

- ❌ decay UI 时间轴 (留 v3 client SPA)
- ❌ Prometheus metrics exporter (留 v3 跟 BPP-7 §2 同源)
- ❌ owner override decay 阈值 (留 v3)
- ❌ cross-org aggregation (留 v3 跟 AP-3 联动)
- ❌ admin god-mode 看 decay history (ADM-0 §1.3 红线)
- ❌ AL-1 6-dict 扩第 7 reason (字典分立, 跟 BPP-7/8 §4 同源)
- ❌ 另开 heartbeat_decay 表 (§0.1 立场, 反向 derive)
- ❌ RecordHeartbeatStale 旁路 method (§0.2 立场, 复用 BPP-8 既有)
