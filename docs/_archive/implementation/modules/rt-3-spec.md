# RT-3 ⭐ spec brief — multi-device fanout + 活物感 + thinking subject 反约束 (≤80 行)

> 飞马 · 2026-05-01 · 用户拍板 (NAMING-1 ✅ → DL-2 + RT-3 并行) · zhanma 主战 + 飞马 review
> **关联**: DL-1 #609 ✅ EventBus interface · DL-2 (并行 storage 端) · RT-1 #290/292 ✅ cursor 协议 + ≤3s SLA · DM-3 / CV-* multi-device 复用此 fanout · 蓝图 realtime.md §0 + §1.1 + §1.4
> **命名**: RT-3 = realtime 第三件 ⭐ (升 ⭐ 取代 RT-2, 蓝图字面)

> ⚠️ Server + client milestone — fanout 端 (跟 DL-2 storage 端不撞). **0 schema 改 / 0 endpoint URL 改 / 0 user-facing API 行为改**.
> v1 单机 fanout (复用 RT-1 cursor + DL-1 EventBus interface). 蓝图 §0 "v1 realtime 只做'足够让用户感到 AI 在工作'的最小集".

## 0. 关键约束 (3 条立场)

1. **DL-1 #609 EventBus interface byte-identical 不破 + RT-1 #290 cursor 协议 byte-identical 承袭** (蓝图 §4.A 必修 5 条 lock-in): RT-3 走 DL-1 EventBus.Publish/Subscribe 路径**不动 signature**, fanout hub 是 EventBus subscriber (跟 DL-2 cold-stream consumer 平行 — RT-3 是 hot path live fanout); cursor 单调发号走 RT-1 #290 既有 ULID `kind+ulid` 协议 byte-identical (蓝图 §4.A.4). 反约束: 反向 grep `EventBus.*Publish|EventBus.*Subscribe` signature 跟 #609 等量; 反向 grep `cursor.*ulid` 协议字面跟 RT-1 #290 byte-identical 不漂.

2. **多端全推 + 活物感 4 态 (蓝图 §1.4) + thinking subject 必带 (蓝图 §1.1)**:
   - **多端全推**: 一用户多设备走 hub.PushFrame fanout, 每设备独立 cursor (跟 DM-3 #449 既有模式承袭, RT-3 是 fanout 上游 SSOT)
   - **活物感 4 态 enum SSOT** `presenceState`: `online` / `away` / `offline` / `thinking` — 走 `internal/datalayer/presence.go` (DL-1 PresenceStore interface) consumer, 4 态 enum 单源 (跟 reasons.IsValid #496 / NAMING-1 enum 模式承袭)
   - **thinking subject 反约束** (蓝图 §1.1 ⭐ 关键纪律 — "沉默胜于假 loading"): thinking 帧**必带 subject 字段** (e.g. `"writing section 3"` / `"calling tool: bash"`); 反向断言 `subject == "" || subject == "AI is thinking…"` → server reject 400 `thinking.subject_required` (字面跟 chn-3 content-lock 5 源 byte-identical 守门)
   - 反约束: 反向 grep `presenceState` 4 态 const SSOT count==4 hit + thinking subject 反约束 reason code 字面 `thinking.subject_required` ≥1 hit

3. **0 schema / 0 endpoint URL / 0 user-facing API 行为改 + 0 routes.go 改** (RT-3 立场, 跟 INFRA-3/4 / REFACTOR-1/2 / NAMING-1 / DL-2 wrapper 系列承袭): PR diff 仅 (a) `internal/ws/hub.go` PushFrame 加 thinking subject 守门 + 多端 fanout 路径 (~80 行) (b) `internal/datalayer/presence.go` PresenceStore 4 态 enum SSOT (~30 行) (c) `client/src/hooks/usePresence.ts` 4 态 UI hook + activity dot 组件 (~80 行) (d) caller 改: thinking 帧 caller 4 处 (BPP-2 plugin upstream / agent runtime / chat / canvas) 走 helper-wrapper 反 inline drift. 反约束: 0 endpoint URL 改 + 0 routes.go 改 + 0 schema column 改 + 0 migration v 号 (RT-3 不动数据契约).

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| **RT3.1 server fanout** | `internal/ws/hub.go` PushFrame 加 thinking subject 守门 + multi-device fanout 路径 ~80 行 (复用 RT-1 cursor); `internal/datalayer/presence.go` 加 PresenceState 4 态 enum SSOT ~30 行 (走 DL-1 PresenceStore interface byte-identical); 4 thinking caller wire (BPP-2 / agent runtime / chat / canvas) 走 helper. 反约束: thinking subject 反约束 unit test ≥4 (per-caller) | 战马 / 飞马 review |
| **RT3.2 client UI** | `client/src/hooks/usePresence.ts` 4 态 hook ~80 行 + `client/src/components/ActivityDot.tsx` 4 态 UI ~50 行 + `client/src/__tests__/Presence.test.tsx` vitest ~6 case (4 态 + last-seen + multi-device) | 战马 / 飞马 review |
| **RT3.3 closure** | REG-RT3-001..008 (8 反向 grep + DL-1 interface byte-identical 不破 + RT-1 cursor 协议 byte-identical + presenceState enum SSOT + thinking subject 守门 + 多端 fanout 真过 + 0 schema/endpoint 改 + haystack 三轨过 + 既有 test 全 PASS) + acceptance + content-lock §1+§2 (DOM data-attr `data-rt3-activity-dot` + thinking subject 字面锁) + 4 件套 spec 第一件 | 战马 / 烈马 |

## 2. 反向 grep 锚 (8 反约束)

```bash
# 1) DL-1 interface byte-identical 不破 (跟 DL-2 同要求)
git diff origin/main -- packages/server-go/internal/datalayer/eventbus.go packages/server-go/internal/datalayer/presence.go | grep -E '^-.*func.*(Publish|Subscribe|IsOnline|Sessions)\('  # 0 hit

# 2) RT-1 cursor 协议 byte-identical (反 cursor 漂)
grep -rcE 'cursor.*ulid|kind\+ulid' packages/server-go/internal/ws/cursor.go  # ≥1 hit (RT-1 #290 既有不动)
git diff origin/main -- packages/server-go/internal/ws/cursor.go | grep -cE '^\+|^-'  # ≤2 hit (RT-1 cursor 不动, 仅 import 微调)

# 3) presenceState 4 态 enum SSOT (单源)
grep -rcE 'PresenceStateOnline|PresenceStateAway|PresenceStateOffline|PresenceStateThinking' packages/server-go/internal/datalayer/presence.go  # ==4 hit (4 态各 1 const)
grep -rE 'type PresenceState ' packages/server-go/internal/datalayer/presence.go  # ==1 hit (SSOT)

# 4) thinking subject 反约束 (蓝图 §1.1 ⭐)
grep -rE 'thinking\.subject_required' packages/server-go/internal/  # ≥2 hit (server reject + test 反向断言)
grep -rE 'AI is thinking…|"AI is thinking"' packages/server-go/internal/  # 0 hit (反"假 loading"漂)

# 5) 多端 fanout 单源 (hub.PushFrame, 反 dm-only frame 漂)
grep -rcE 'hub\.PushFrame|hub\.broadcast' packages/server-go/internal/ws/hub.go  # ≥2 hit (multi-device 走单源)
grep -rE 'fanout.*dm_only|dmOnlyFrame' packages/server-go/internal/  # 0 hit (反 dm 旁路)

# 6) 0 endpoint URL / 0 routes.go 改
git diff origin/main -- packages/server-go/internal/server/server.go | grep -cE '^\+.*HandleFunc|^\+.*Handle\('  # 0 hit
git diff origin/main -- packages/server-go/internal/migrations/ | grep -cE '^\+\s*Version:'  # 0 hit (0 migration)

# 7) DOM data-attr content-lock (RT-3.2 client)
grep -rE 'data-rt3-activity-dot' packages/client/src/components/ packages/client/src/__tests__/  # ≥2 hit (component + test)

# 8) haystack gate 三轨 + 既有 test
THRESHOLD_FUNC=50 THRESHOLD_PACKAGE=70 THRESHOLD_TOTAL=85 BUILD_TAGS="sqlite_fts5" go run ./scripts/lib/coverage/  # ALL ≥阈值
go test -tags 'sqlite_fts5' -timeout=300s ./...  # ALL PASS
pnpm vitest run --testTimeout=10000  # ALL PASS
```

## 3. 不在范围 (留账)

- ❌ **events 接 RT-3 fanout 上游 hook** — RT-3 是 hot path live fanout, DL-2 cold stream 不接 fanout (留 follow-up wire-up PR, scope 是 DL-2 events 流跟 RT-3 hub 之间桥接)
- ❌ **typing-indicator** (蓝图 §1.1 ⭐ 反约束 — typing-indicator 是 "假 loading" 漂, 永久留)
- ❌ **last-seen UI 跨设备 sync** — 留 RT-3.2 follow-up (单设备 last-seen 已就 RT-1 #290 cursor)
- ❌ **agent presence (跟 human 4 态分开)** — 蓝图 §1.4 仅人类 4 态, agent 走 BPP heartbeat (HB-1..6)
- ❌ **session_resume_hint** — 蓝图 §1.3 留 DL-5+
- ❌ **per-channel presence 视图** (e.g. "3 人在 channel-X") — 留 v3+
- ❌ **HB-2 v0(D) Borgee Helper SQLite events consumer** — 留 HB-2 v0(D) 单 milestone

## 4. 跨 milestone byte-identical 锁

- 复用 DL-1 #609 EventBus + PresenceStore interface (signature byte-identical, factory NewDataLayer 不动)
- 复用 RT-1 #290 cursor 协议 ULID `kind+ulid` byte-identical (蓝图 §4.A.4)
- 复用 reasons.IsValid #496 / AP-4-enum #591 / NAMING-1 #614 enum SSOT 模式 (presenceState 4 态单源)
- 复用 chn-3 content-lock §1 字面锁 (thinking subject 反约束 reason code byte-identical)
- 复用 DM-3 #449 multi-device cursor 模式 (RT-3 是 fanout 上游 SSOT, DM-3 是 caller)
- 复用 admin god-mode 不挂红线 (ADM-0 §1.3, RT-3 域不挂 admin /admin-api/.*presence|/admin-api/.*thinking)
- 0-schema-改 wrapper 决策树**变体**: 跟 INFRA-3/4 / CV-15 / TEST-FIX-3 / REFACTOR-1/2 / NAMING-1 / DL-2 同源

## 5. 派活 + 双签

派 **zhanma-d** (CS-* / NAMING-1 client tsx 熟手) 或 zhanma-c (DL-2 并行不撞). 飞马 review.

双签流程: spec brief → team-lead → 飞马自审 ✅ APPROVED → yema stance + content-lock + liema acceptance → zhanma 起实施 (RT3.1+2+3 三段一 PR, **teamlead 唯一开 PR**).

## 6. 飞马 (架构师) 自审表态

✅ **APPROVED with 2 必修条件**:

🟡 必修-1: **DL-1 + RT-1 byte-identical 双锁** — 反约束 grep #1+#2 真守, `git diff -- internal/datalayer/eventbus.go internal/datalayer/presence.go internal/ws/cursor.go` signature 行 0 hit + RT-1 cursor 协议字面不动. zhanma PR body 必示 diff 输出.

🟡 必修-2: **thinking subject 反约束**真守 (蓝图 §1.1 ⭐) — server reject 400 `thinking.subject_required` 真测; 反向 grep "AI is thinking…" / "假 loading" 字面 0 hit. acceptance 必含反向断言 case (subject="" → 400 / subject="AI is thinking…" → 400).

担忧 (1 项, 中度):
- 🟡 4 thinking caller wire (BPP-2 / agent runtime / chat / canvas) 跨域改, 战马实施需逐 caller 验证不漏 + acceptance 反向 grep `subject:` 字面 ≥4 hit (per-caller).

留账接受度全 ✅: typing-indicator / events 接 fanout / agent presence / session_resume_hint / per-channel presence / HB-2 v0(D) 全留账, 不强塞本 PR.

**ROI 拍**: RT-3 ⭐⭐⭐ — 蓝图 §1.1 ⭐ 关键纪律兑现 (thinking subject 真守) + §1.4 活物感 4 态 + 多端 fanout SSOT 立, 后续 DM-* / CV-* / BPP-2 复用基座. 跟 DL-2 (storage 端) 并行不撞.
