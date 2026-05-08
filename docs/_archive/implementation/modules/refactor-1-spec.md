# REFACTOR-1 spec brief — internal/api 重复收口 (≤80 行)

> 飞马 · 2026-04-30 · 用户解冻拍板 (post-internal/api duplication audit) · zhanma-c 主战 (TEST-FIX-3 完空) 或 zhanma-d
> **关联**: internal/api duplication audit (top 13 真值, 本 spec 选 top 4) · TEST-FIX-3 #610 ✅ merged
> **命名**: REFACTOR-1 = 第一件 refactor milestone (跟 INFRA-* 同等级元 milestone)

> ⚠️ Refactor milestone — **0 行为改 / 0 schema / 0 endpoint** + 既有 unit/e2e 全 PASS byte-identical.
> 净减 ~250 LoC, 一 PR 闭, 不开 follow-up.

## 0. 关键约束 (3 条立场)

1. **行为不变量 byte-identical pre/post refactor** (蓝图不破立场): helper 提取**只动调用形式**, 不动 (a) HTTP status code (b) error reason code 字面 (c) DM-gate 字面 (d) audit log 字段 (e) WS broadcast event 名. 反约束: 反向 grep 既有错误码字面 (`layout.dm_not_grouped` / `dm.edit_only_in_dm` / `"DM 不参与个人分组"` / `metadata.target` 字面) **count 不减** (helper 内承载相同字面, content-lock 不破); 既有 unit/e2e (chn_6/7/8/15 + layout + dm_7 + cv_15 + al_7 + hb_5 共 ≥30 test func) 全 PASS 不动.

2. **4 helper 接口 SSOT** (跟 BPP-3 PluginFrameDispatcher / reasons.IsValid / TEST-FIX-3 fixture SSOT 同精神):
   - **helper-1** `requireChannelMember(w, r, opts) (user *User, ch *Channel, ok bool)` — 收 chn_6/7/8/15 + layout 4-step preamble (auth → load channel → DM gate → membership), opts 携 `{rejectDM bool, requireCreator bool}` 兼容 chn_15 creator-only 变体 + layout DM-reject 反向. 落 `internal/api/channel_helpers.go` 新文件 (~80 行)
   - **helper-2** `parseMessageEditHistory(raw string) []EditEntry` — 收 dm_7 ↔ cv_15 byte-identical 11 行. 落 `internal/api/message_edit_history.go` 新 (~30 行 含 godoc); 既有两处 `parseEditHistoryEntries` / `parseCommentEditHistory` 改 import 单源
   - **helper-3** `writeRetentionOverride(w, r, target string, action auth.Action, defaultDays int)` — 收 al_7 ↔ hb_5 45 行 × 2 admin retention skeleton (admin nil 401 → JSON decode → clamp → InsertAdminAction → response). 落 `internal/api/admin_retention_helper.go` 新 (~70 行); al_7 / hb_5 各缩到 ~15 行 wrapper
   - **helper-4** `handleChannelToggle(w, r, field string, value bool, broadcastEvent string)` — 收 chn_6 pin/unpin 重复 ~35 行 × 2 → 60 行 → 走 chn_7/chn_15 既立 toggle 模式 (一 handle*Toggle + 两 thin handle*/handleUn*). 落 chn_6_pin.go 内 (不另起文件, 跟 chn_7 同模式承袭)
   
   反约束: 反向 grep `func handle.*Pin\|func handle.*Unpin` in chn_6_pin.go 各 ≤1 hit (thin wrapper) + helper count 4 (反第 5 个漂)

3. **0 endpoint 加 + 0 schema 改 + caller 列表锁** (refactor 立场, 跟 INFRA-3/INFRA-4/CV-15 wrapper 系列承袭): PR diff 仅 (a) 4 helper 新文件 ~210 行 (b) caller 改 ~150 行净减 (c) 既有 test 不动 (d) 0 migrations / 0 routes.go / 0 schema 改. 反约束: 反向 grep `migrations/refactor_1_` 0 hit + `r.HandleFunc.*refactor` 0 hit + caller 列表 audit 完整 (chn_6 / chn_7 / chn_8 / chn_15 / layout / dm_7 / cv_15 / al_7 / hb_5 共 9 文件 touched, 不 touch 其他).

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| **R1.1** helper-1 + helper-4 (CHN 域) | `internal/api/channel_helpers.go` 新 (~80 行 requireChannelMember) + chn_6/7/8/15/layout 5 文件改走 helper (净减 ~70 行) + chn_6 toggle 模式对齐 (~30 行净减) | 战马 / 飞马 review |
| **R1.2** helper-2 + helper-3 (跨域) | `internal/api/message_edit_history.go` 新 (~30 行 parseMessageEditHistory) + dm_7 / cv_15 单源化 (~22 行净减); `internal/api/admin_retention_helper.go` 新 (~70 行 writeRetentionOverride) + al_7 / hb_5 缩 wrapper (~60 行净减) | 战马 / 飞马 review |
| **R1.3** closure | REG-REFACTOR1-001..006 (6 反向 grep + 行为不变量 + caller 列表锁 + 字面 content-lock + 既有 test PASS + 净减 LoC ≥200) + acceptance + content-lock 守 (DM-gate 13 hits 字面不漂) + 4 件套 spec 第一件 | 战马 / 烈马 |

## 2. 反向 grep 锚 (6 反约束, count==0 OR 等量)

```bash
# 1) 4 helper 新文件存在 (反漏)
for f in channel_helpers.go message_edit_history.go admin_retention_helper.go; do
  test -f packages/server-go/internal/api/$f || echo MISS_$f
done

# 2) DM-gate 字面 content-lock (count 等量, 反漂)
old=$(git show origin/main:packages/server-go/internal/api/layout.go | grep -c 'DM 不参与个人分组')
new=$(grep -rcE 'DM 不参与个人分组' packages/server-go/internal/api/ | awk -F: '{s+=$NF}END{print s}')
[ "$new" -eq "$old" ]  # 字面在 helper 内承载, 总 count 不变

# 3) 既有 test 字面 byte-identical 不破 (反 helper 改行为)
go test -tags sqlite_fts5 -run 'TestCHN_6|TestCHN_7|TestCHN_8|TestCHN_15|TestLayout|TestDM_7|TestCV_15|TestAL_7|TestHB_5' ./internal/api/...  # ALL PASS

# 4) chn_6 toggle 模式对齐 (handle*/handleUn* thin wrapper)
grep -cE '^func \(h \*ChannelHandler\) handle(Pin|Unpin)Channel' packages/server-go/internal/api/chn_6_pin.go  # ==2 (各 1 thin wrapper, ≤10 行)

# 5) parseMessageEditHistory 单源 (dm_7/cv_15 各自 func 删)
grep -cE 'func parseEditHistoryEntries|func parseCommentEditHistory' packages/server-go/internal/api/  # 0 hit (合一)

# 6) RetentionOverride helper 单源 (writeRetentionOverride 真挂)
grep -cE 'func writeRetentionOverride' packages/server-go/internal/api/admin_retention_helper.go  # ==1
grep -cE 'writeRetentionOverride\(' packages/server-go/internal/api/al_7_audit_retention_override.go packages/server-go/internal/api/hb_5_heartbeat_retention_override.go  # ≥2 hit (al_7 + hb_5 都调)
```

## 3. 不在范围 (留账)

- ❌ #4 user==nil → 401 boilerplate 100+ 处收 (`mustUser(r)`) — 留 REFACTOR-2 (scope 太大, 单 PR 风险高)
- ❌ #5 JSON-decode → 400 boilerplate 30 处 (`decodeJSON(w,r,&v)`) — 留 REFACTOR-2
- ❌ #6 DM-gate 13 hits 三种错误码统一 — 留 REFACTOR-2 (需先讨论 error code SSOT 立场)
- ❌ #8/#9/#10/#11 admin-list / loadAgent / cursor-push / ACL drift — 留 REFACTOR-2/3
- ❌ #12/#13 fanout 字面差 / agents 双形状 decoder — content-lock 绑 / 不够痛, 不动

## 4. 跨 milestone byte-identical 锁

- 复用 BPP-3 #489 / reasons.IsValid #496 / TEST-FIX-3 fixture SSOT 模式 (helper 单源)
- 复用 chn_7 #523 / chn_15 既立 toggle 模式 (chn_6 对齐, drift 收口)
- 复用 content-lock §1 字面绑 (DM-gate 字面跨 helper 不漂)
- 复用 audit-forward-only / owner-only ACL 链 (RetentionOverride helper 内 audit log byte-identical)
- 0-行为-改 wrapper 决策树**变体**: 跟 INFRA-3 / INFRA-4 / CV-15 / TEST-FIX-3 同 "wrapper 真有 prod code 0 schema/0 行为改" 类别同源

## 5. 派活 + 双签

派 **zhanma-c** (TEST-FIX-3 #610 主战完空, 续作减学习成本) 或 **zhanma-d** (CS-* 域熟手). 飞马 review.

双签流程: spec brief → team-lead → 飞马自审 ✅ APPROVED → yema stance + liema acceptance → zhanma 起 worktree `.worktrees/refactor-1` 实施 (R1.1+R1.2+R1.3 三段一 PR).

## 6. 飞马 (架构师) 自审表态

✅ **APPROVED with 1 必修条件** (字面 content-lock):

🟡 必修: helper 内字面**严格 byte-identical** 跟既有 (`"DM 不参与个人分组"` / `layout.dm_not_grouped` / `dm.edit_only_in_dm` / `metadata.target` 等), 反向 grep count 等量 (反约束 grep #2). 战马实施 PR body 必示 before/after grep count 一致.

担忧 (1 项, 轻度):
- 🟡 helper-1 opts 携 `{rejectDM, requireCreator}` 两 flag 是 over-engineer 嫌疑, 但收 5 处 + chn_15 creator-only / layout DM-reject 真值变体确实需要 — 接受

留账接受度全 ✅: REFACTOR-2/3 candidates (user-nil / JSON-decode / DM-gate 错误码 / ACL drift) 全留账, 不强塞本 PR.

**ROI 拍**: REFACTOR-1 ⭐ 高 ROI — 净减 ~250 LoC + 跨 5 milestone drift 收口 + helper SSOT 立 (后续 CHN-16+ / DL-* / DM-1x 复用), 一 PR 闭不留 follow-up.
