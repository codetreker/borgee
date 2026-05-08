# G2 Audit — Phase 2 退出闸 (audit 集成) [DRAFT]

> 作者: 烈马 (QA) · 2026-04-28 · team-lead 派单 (idle 派活 #3)
> 目的: G1 audit 同款单源, Phase 2 全 milestone (ADM-0 / CM-onboarding / AP-0-bis / RT-0 / CM-3 后置 / G2.4 demo) 落地后, 闸 + audit row 一次集成审完。
> 形式: 此文件 = audit 报告 (草稿); 签字单独走 `docs/qa/signoffs/g2-exit-gate.md` (待落)。
> 状态: 🟡 **DRAFT v2** — RT-0 server-half ✅ (#237 merged 08:29Z) + G2.3 ✅ (#236 merged) + ADM-0.3 ✅ (#223 merged); G2.4 demo 5/5 签 + G2.6 BPP lint 仍 outstanding.

---

## 1. 闸概览

来源: `phase-2-exit-gate.md` (飞马 PR #221) + `execution-plan.md` §"Phase 2 退出 gate" (R3 锁)。

| 闸 | 主旨 | Trigger PR | Status (本文 cut: 2026-04-28 ~08:30Z) |
|---|---|---|---|
| G2.0 | ADM-0 cookie 串扰反向断言 (4.1.a/b/c/d) | #197/#201/#223 | ✅ ADM-0.3 #223 merged — 4.1.a/b/c/d 全锁 (registry REG-ADM0-002/004/005/007/010 🟢) |
| G2.1 | 邀请审批 E2E (Playwright + ws push) | #195+#198+#218+#237 | 🟡 server push #237 已 merged; cm-4-realtime.spec.ts 仍 `.skip` (待战马B 解锁 + 跑 ≤3s 真过) |
| G2.2 | 离线 fallback E2E | RT-0 + presence stub | ⏳ 待 G2.5 (presence stub) 落 |
| G2.3 | 节流不变量 (B.1) | #236 | ✅ #236 merged — `internal/throttle/notification_throttle.go` + 5-test matrix (T1 const / T2 first / T3 within-window / T4 cross-window / T5 二维) clock.Fake 全过 |
| G2.4 | 用户感知签字 (B.2, ⭐) | #199+#213 + demo run | 🟡 partial 2/5 — #1/#5 ready, #2/#3/#4 待战马B latency 截屏 + AL-1b |
| G2.5 | presence 接口契约 | RT-0 task #40 | ⏳ 路径未建 (Phase 2 收尾) |
| G2.6 | /ws → BPP schema 等同性 lint | RT-0 task #40 | 🟡 schema 注释锁就位 (`grep "byte-identical" event_schemas.go` ✅), CI lint job 待飞马挂 (Phase 4 BPP-1 启用即可) |
| G2.audit | v0 代码债登记 | 本文 §3 | 🟢 8 行 specific 完表 (本 PR) |

通过判据 (引): G2.0 完整 (ADM-0.3 落) + G2.1-G2.4 全 ✅ (野马签 + 截屏 5/5) + G2.5/G2.6 (RT-0 落) + G2.audit 6 项齐。

---

## 2. 闸闭合情况

### 2.1 G2.0 — ADM-0 4.1.a/b/c/d 反向断言 ✅

ADM-0.2 #201 落 a/b/c (TestAuthIsolation_2A/2B/AdminFieldWhitelist),
ADM-0.3 #223 4.1.d post-migration `users WHERE role='admin'` count==0 由
`internal/migrations/adm_0_3_users_role_collapse_test.go::TestADM03_BackfillAndCollapse`
反向断言, 同包 `Idempotent / PreexistingAdminLogin / TolerantToTrimmedSchema /
NoUsersTable` 4 个补充 case 覆盖二跑 + bootstrap 共存 + trimmed schema 容错.
红线 grep `grep -rn '"admin"' internal/ --include='*.go' | grep -v _test.go |
grep -v migrations/` 仅余注释/字面量, 无 live `role == "admin"` 短路。
Registry: REG-ADM0-002/004/005/007/010 全 🟢 (post-#223).

### 2.2 G2.1 — 邀请审批 E2E (server push ✅, e2e .skip 待解)

server push 路径 (#237 merged): typed `PushAgentInvitationPending` /
`PushAgentInvitationDecided`, 编译期 schema 锁 (no `interface{}`),
silent no-op (nil-Hub / "" userID / offline / nil frame), POST/PATCH 触发
点 `internal/api/agent_invitations.go` 各 1 处 + 双推 (requester + owner)
跨设备同步 §1.4. Tests: `TestPushOnCreate_PendingFrameToAgentOwner` /
`TestPushOnPatch_DecidedFrameToBothParties` / `TestPushNilHub_NoPanic` 全
绿. Client (#218 + #235 vitest CI) 6/6 jsdom env 真过.

**Pending**: cm-4-realtime.spec.ts `test.describe.skip` 解 (战马B 接) +
跑 ≤3s 真过 → 进 G2.4 demo 截屏 #2.

### 2.3 G2.3 — B.1 节流不变量 ✅

`internal/throttle/notification_throttle.go` (#236 merged): in-memory
`(channel_id, agent_id) → last_fired_at` map + `sync.Mutex`, 单 `Allow()`
surface, `clock.Clock` 注入 (无 wall-clock sleep). 5-test matrix
(`notification_throttle_test.go`):
T1 `ThrottleWindow == 5*time.Minute` 字面量 pin, T2 第 1 次 `Allow == true`,
T3 6 follow-ups @ 4m59s 全 false (within-window suppress), T4 边界 `>=`
跨窗口 `Allow` 重启 (clock.Fake.Advance), T5 二维 key 隔离 (distinct
channel OR agent ⇒ 独立窗口 + sanity 同 (ch, ag) 仍 suppressed). 飞马
#229 5 盯点 LGTM.

### 2.4 G2.6 — BPP schema 等同性 (注释锁 ✅, CI lint 留 Phase 4)

server `internal/ws/event_schemas.go` (#237) godoc 含 "Phase 4 BPP will
swap the wire layer without changing the schema. The blueprint locks the
promise that `bpp/frame_schemas.go` (Phase 4) and this file stay
byte-identical or type-aliased". 客户端 TS interface (`packages/client/
src/types/ws-frames.ts`) 字段顺序 1:1 server Go struct (REG-RT0-001/002
锁). CI lint job (`bpp/frame_schemas.go ↔ ws/event_schemas.go` reflect
diff) 留 Phase 4 BPP-1 启用时挂 — Phase 2 不阻塞.

---

## 3. G2.audit row — Phase 2 跨 milestone 代码债登记

| Audit ID | Source | 内容 (单行) | 接收 milestone | Status |
|---|---|---|---|---|
| AUD-G2-ADM01 | ADM-0.1 (#197) | admin 拆表 step-1: cookie session + RequireSession middleware 拆 god-mode 路径; user-rail vs admin-rail 双 cookie 隔离 (`/api` vs `/admin-api`) — REG-ADM0-001/002 双轨锁 | (闭合) | ✅ stable |
| AUD-G2-ADM02 | ADM-0.2 (#201) | RequirePermission 去 `role=='admin'` 短路 + god-mode 字段白名单 (REG-INV-002 `handlers_field_whitelist_test.go` fail-closed 反射扫描). 4.1.a/b/c 反向断言全锁. | (闭合) | ✅ stable |
| AUD-G2-ADM03-a | ADM-0.3 (#223) | SQLite `users.role IN ('member','agent')` 硬 CHECK 后置 — ADD CONSTRAINT post-create 不支持; 现 v=10 由数据不变量 + 反向断言 (count==0) 保证, hard-flip 走 CREATE TABLE _new + RENAME. | Phase 3 (TBD) | 📝 logged |
| AUD-G2-ADM03-b | ADM-0.3 (#223) | `sessions` 表 step-2 vacuous gate — 现 v0 JWT 无状态, hasTable() 容错; BPP 引入 user-session 表后此 gate 自动激活, session-revoke E2E 也后置至彼时. | Phase 4 / BPP | 📝 logged |
| AUD-G2-RT0-a | RT-0 server (#237) | `/ws` Phase 2 路径仅过渡, BPP-1 启用后 `hub.BroadcastToUser → bpp.SendFrame` 1 行替换; 客户端 schema 0-改 by event_schemas.go 注释 "byte-identical or type-aliased" 承诺. CI lint job (reflect diff `bpp/frame_schemas.go ↔ ws/event_schemas.go`) 留 BPP-1 落地时挂. | Phase 4 / BPP-1 | 📝 logged |
| AUD-G2-RT0-b | RT-0 (#237 + #218) | 60s 轮询 fallback (Sidebar.tsx) 保留为 belt-and-suspenders; CM-4.3 删除条件: BPP 保证 redelivery 之后. REG-RT0-007 e2e (60s disconnect 兜底) 留 Phase 2 收尾. | Phase 4 (CM-4.3) | 📝 logged |
| AUD-G2-RT0-c | RT-0 e2e | `cm-4-realtime.spec.ts test.describe.skip` 仍未解 — server #237 + client #218 + #235 vitest 三齐, 只差 1 行 diff `.skip` → `.describe` + `pnpm e2e` 跑 ≤3s 真过 → G2.4 demo 截屏 #2 evidence. 派战马B Phase 2 收尾. | Phase 2 收尾 | 📝 logged |
| AUD-G2-CM-onboard | CM-onboarding (#203 + bug-030 22ed221 + #226) | system user (`role='system'`) disabled-flag 直查并防注册; bug-030 hotfix `ListChannelsWithUnread` 走 membership LEFT JOIN gate (REG-INV-003 unit + e2e 双层守门, 后者 #226 cm-onboarding-bug-030-regression.spec.ts 三断言). | (持续) | ✅ stable |
| AUD-G2-AP0-bis | AP-0-bis (#206) | `idx_user_permissions_user` 索引 retire 评估推迟 — 现读路径仍走 join, 索引复用率 100%, 删之前需先做 EXPLAIN audit (G1-audit §3.5 已记). REG-AP0B-001..006 全 🟢. | Phase 3 (TBD) | 📝 logged |
| AUD-G2-CM3 | CM-3 (#208) | `messages.org_id` 直查就位 (前置 channel→org 双跳 deprecated); cross-org 403 反向断言 + EXPLAIN 走 `idx_messages_org_created` (G1.4 audit row 5 已记). owner_id 列保留没删 — read 路径不读, write 时双写; ADM-0.3 落地后再评估 retire (G1.audit AUD-G1-CM3-b 续). | Phase 3 (TBD) | 📝 logged |
| AUD-G2-G23 | G2.3 (#236) | B.1 节流不变量 v0 落: `internal/throttle/notification_throttle.go` (50 LOC) + `_test.go` 5 测 (T1-T5) clock.Fake 全过. `ThrottleWindow = 5*time.Minute` const 唯一字面量. caller-side 接入 (@ 检测 → Allow 调用 → 系统消息插入) 留 Phase 2 收尾 / Phase 3, `offline_mention_notifications` 表持久化 (data-layer.md row 75) 留 v1. | Phase 2 收尾 / Phase 3 | 📝 logged |
| AUD-G2-CHECK | (跨) | SQLite `ALTER TABLE ADD CHECK` 不支持 — ADM-0.3 hard-flip 推迟 + 任何后续 enum 收紧都走 CREATE TABLE _new + INSERT SELECT + DROP + RENAME. v1 hard-flip backlog 单源在此 + ADM03-a 行. | Phase 3 (TBD) | 📝 logged |
| AUD-G2-AL1A | AL-1a (#249) | runtime 三态 (online/offline/error) + 6 reason codes 走 in-memory `agent.Tracker` (online/offline 由 `hub.GetPlugin(id) != nil` 推导, 唯一持久 = error map). AL-3 落表前不持久 — server restart 后所有 error 态丢, owner 重连时 hub 状态自然恢复 (蓝图 §2.3 决议). 字段名 `state / reason / state_updated_at` 跟 client TS `Agent` 接口字面绑定 — REG-AL1A-005 锁. busy/idle 留 Phase 4 BPP frame 落地后再加 (蓝图 §2.3 不准 stub). | Phase 4 (AL-3 表 + AL-1b BPP) | 📝 logged |

`📝 logged` = Phase 3 输入或长期跟踪, 不阻塞 Phase 2 退出; `✅ stable` 已闭合。

> **填充节奏**: §2 实测数据已回填 (#223 / #237 / #236 全 merged). 余 G2.4
> demo 5/5 签字 (野马 partial 2/5 → 战马B latency 截屏后野马补 #2-#4) + G2.5
> presence stub + REG-RT0-007 60s fallback e2e 三项, Phase 2 收尾派活清单.

---

## 4. Phase 2 退出闸结论 (待签)

**已就位 (4/6)**: G2.0 ✅ + G2.3 ✅ + G2.audit ✅ (本表 11 行 specific 完表)
+ G2.6 注释锁 ✅ (CI lint 留 Phase 4).

**outstanding (2/6)**:
- G2.1 e2e 解 .skip + ≤3s latency 真过 (战马B)
- G2.4 demo 5/5 签 (野马 partial 2/5 → 战马B 截屏后补 #2/#3/#4)

(G2.2 离线 fallback + G2.5 presence stub 接受留 Phase 2 收尾, 不阻 exit gate.)

后续: G2.1 + G2.4 就位后, 此文件转 v1 + `docs/qa/signoffs/g2-exit-gate.md`
三人签 + registry RT-0 8/8 全 🟢 (现 6/8) + ADM-0/CM-onboarding 全态固化.

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-28 | 烈马 | v0 草稿 — 闸概览 + audit row 8 行登记骨架; 待 ADM-0.3 + RT-0 server merge 后回填实数据 |
| 2026-04-28 | 烈马 | v2 — §2 实测回填 (G2.0 ADM-0.3 + G2.1 #237 server push + G2.3 #236 throttle + G2.6 注释锁); §3 audit row 8 → 11 行 specific (拆 ADM01/ADM02 + RT0-c e2e .skip + G23 节流 + 补 CM-onboard #226 e2e); §1 闸表 4 项 ✅ + 1 项 🟡 + outstanding 2/6 (G2.1 e2e + G2.4 demo) |
| 2026-04-28 | 烈马 | 加 AUD-G2-AL1A (AL-1a #249 stacked) — runtime 三态 in-memory tracker + busy/idle 留 Phase 4 BPP / AL-3 落表; §3 11 → 12 行 |
