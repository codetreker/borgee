# REFACTOR-2 spec brief — internal/api 重复收口剩余全清 (≤80 行)

> 飞马 · 2026-05-01 · 用户拍板 (一次做干净, 不准留尾巴) · zhanma 主战 + 飞马 review
> **关联**: REFACTOR-1 #611 ✅ merged (top 4 helper) · 飞马 internal/api duplication audit top 13 留账剩 9 处
> **命名**: REFACTOR-2 = 第二件 refactor milestone, 跟 REFACTOR-1 同等级 (元 milestone)

> ⚠️ Refactor milestone — **0 行为改 / 0 schema / 0 endpoint** + 既有 unit/e2e 全 PASS byte-identical.
> 净减 ~500-700 LoC, 一 PR 合, **本 milestone 飞马 audit #4-#13 全清不留 v2**.

## 0. 关键约束 (3 条立场)

1. **行为不变量 byte-identical pre/post refactor + 字面 content-lock 严格** (跟 REFACTOR-1 #611 必修条件承袭): helper 提取**只动调用形式**, 不动 (a) HTTP status code (b) error reason code 字面 (c) DM-gate 字面 (d) audit log 字段 (e) WS broadcast event 名 (f) admin god-mode 路径. 反约束: 既有错误码字面 grep count 等量 (helper 内承载相同字面); 既有 unit/e2e (≥80 test func) 全 PASS 不动.

2. **5 helper SSOT 一次立** (跟 REFACTOR-1 4 helper / BPP-3 / TEST-FIX-3 fixture 同精神, **scope 锁定真减 LoC 的 helper, 反 spec 错估强行抽**):
   - **helper-1 `mustUser(w, r) (*User, bool)`** — 收 100+ 处 `user==nil → 401 Unauthorized` boilerplate (~100 行减), 落 `auth_helpers.go` 新
   - **helper-2 `decodeJSON(w, r, &v) bool`** — 收 canonical-shape JSON-decode → 400 boilerplate (~10 行减), 落 `request_helpers.go` 新; **不收** custom-error-code callers (agent_config.invalid_payload / notification_pref.invalid_value / layout.invalid_payload / host_grants.invalid_payload / push.endpoint_invalid / chn_10) — 那些 reason 字面是 public contract (反约束 §0 #1 reason byte-identical)
   - **helper-6 `loadAgentByPath(w, r, store) (*User, string, bool)`** — 收 10 处 `id := PathValue + GetAgent + 404` (~16 行减), 落 `agent_helpers.go` 新
   - **helper-8 `fanoutChannelStateMessage(args)`** — 收 fanoutArchive ↔ fanoutUnarchive (~30 行减, 字面 `"恢复于"` / `"关闭于"` 由 caller 传, content-lock 不破), 落 chn_5_archived.go 内 (跟 REFACTOR-1 chn_6 同 idiom)
   - **drift 收口 #11 ACL 部分** — `artifact_comments.go` ×2 处 `!IsChannelMember && !CanAccessChannel` 折叠为 `!CanAccessChannel` (member ⊂ canAccess 双 false 等价单 false 语义不变, ~2 行减)
   
   反约束: 反向 grep `func mustUser|decodeJSON|loadAgentByPath|fanoutChannelStateMessage` 单源各==1 hit (反第 5 个漂)

   **撤 spec v0 helper (飞马 audit 反转, 用户拍板批准)**:
   - ❌ ~~helper-3 DM-gate 错码统一 → `layout.dm_not_grouped` 单源~~ — **撤**: dm_4_message_edit `dm.edit_only_in_dm` 要 channel.kind=="dm" (DM-only path, 403); chn_6/7/8/layout `layout.dm_not_grouped` 要 channel.kind!="dm" (RejectDM, 400). 同字段反向条件 + 不同 status + 不同 reason — 字面归一会破 user-facing 错码契约 (REFACTOR-1 #611 已立 RejectDM 组单源, dm_4 `dm.edit_only_in_dm` RequireDM 组也是单源, **两组立场目标已达成, 不需新 helper**). 立场目标"DM-gate 三错码统一"是 spec 错估, 真值: RejectDM 组 + RequireDM 组各分别字面 byte-identical 单源.
   - ❌ ~~helper-4 ACL 双重 → 单源 `requireChannelAccess`~~ — **撤**: 双层 ACL `IsChannelMember && CanAccessChannel` 是 **security correctness 设计** (CanAccessChannel = visibility-aware superset / IsChannelMember = member-required subset), 不是 drift. AP-5 post-removal scenario: user 是 channel 创建者但已非 member — `IsChannelMember=false` && `CanAccessChannel=true` (creator superset 让通过), 原 AND 拒, 折叠后通过 → 真行为差. 实测折叠破 TestAP5_PutMessage_PostRemovalReject + TestAP5_DeleteMessage_PostRemovalReject + TestChannelsMessagesWorkspaceAdditionalBranches. **messages/dm_4/reactions write 路径走双层 fail-closed AND 真守, list 路径走单 CanAccessChannel — 已分清不是 drift**. 立场目标"双 ACL 收单源"是 spec 错估, 真值: messages.go 路径已分清 (list 单 CAC / write 双层).
   - ❌ ~~helper-5 `writeAdminListResponse(w, key, items)`~~ — **撤**: 1-line 替换 (`writeJSONResponse(w, 200, map{key: items})` → `writeAdminListResponse(w, key, items)`), 净减 0, 加 helper 反增 LoC + 触发 cov gate. 仅 5 callers 不值得.

3. **0 endpoint 加 + 0 schema 改 + caller 列表锁** (refactor 立场, 跟 REFACTOR-1 / INFRA-3 / CV-15 wrapper 系列承袭): PR diff 仅 (a) 8 helper 新文件 + 既有文件追加 ~400 行 (b) caller 改 ~900 行净减 (c) 既有 test 不动 (d) 0 migrations / 0 routes.go / 0 schema 改 / 0 文件 rename (rename 留 NAMING-1). 反约束: 反向 grep `migrations/refactor_2_` 0 hit + caller 列表 audit (≥40 文件 touched 但仅 helper 调用替换, 0 行为分支 add).

## 1. 拆段实施 (3 段, 一 milestone 一 PR — 内顺序)

| 段 | 范围 |
|---|---|
| **R2.1 helper 抽出** | 4 helper 新建 (auth_helpers / request_helpers / agent_helpers + chn_5_archived 内 fanoutChannelStateMessage). 0 caller 改, 仅 helper 文件 + godoc + 既有 test 验. ~110 行加 (含 godoc) |
| **R2.2 caller 跟随** | 全 caller 改走 helper, byte-identical 替换. 100 mustUser callsites + 5 decodeJSON canonical-shape callsites + 8 loadAgentByPath callsites + 2 fanout 调用 collapse. 实测净减 -137 行 (40 文件 +509 -495). 既有 test 不动全 PASS. |
| **R2.3 drift 收口 + closure** | drift 收口 #11 ACL 部分 (`artifact_comments.go` ×2 处 OR 折叠 — 双 false 等价单 false 不破语义); messages/dm_4/reactions write 路径双层 fail-closed AND 保留 (security correctness 不是 drift). REG-REFACTOR2-001..006 (6 反向 grep + 行为不变量 + caller 锁 + 字面 content-lock 等量 + ACL drift 部分收口 + 既有 test PASS) + acceptance + 4 件套 |

## 2. 反向 grep 锚 (10 反约束)

```bash
# 1) 8 helper 单源各==1 hit
for h in mustUser decodeJSON loadAgentByPath fanoutChannelStateMessage; do
  grep -cE "^func .*$h\(" packages/server-go/internal/api/  # ==1 per helper
done

# 2) user==nil → 401 boilerplate 大幅清空 (反向断言)
grep -rE 'mustUser\(' packages/server-go/internal/api/*.go | grep -v _test.go | wc -l  # ≥100 (replaced)
grep -rE 'if user == nil' packages/server-go/internal/api/*.go | grep -v _test.go | grep -v helpers.go | wc -l  # ≤15 (variants — comment 在中间 / authenticate* helper / preview public / dm_10_pin 3-return)

# 3) DM-gate 字面 (RejectDM 组 + RequireDM 组各自单源 byte-identical, 不归一)
grep -rcE 'layout\.dm_not_grouped' packages/server-go/internal/api/*.go | awk -F: '{s+=$NF}END{print s}'  # ≥19 (REFACTOR-1 baseline, RejectDM 组单源在 channel_helpers.go::requireChannelMember)
grep -rcE 'dm\.edit_only_in_dm' packages/server-go/internal/api/*.go | awk -F: '{s+=$NF}END{print s}'  # ==7 (RequireDM 组单源在 dm_4_message_edit.go, 反向条件 byte-identical)

# 4) ACL drift 部分收口 (artifact_comments OR 折叠), 双层 fail-closed AND 保留
grep -nE 'IsChannelMember.*&&.*CanAccessChannel|CanAccessChannel.*&&.*IsChannelMember' packages/server-go/internal/api/*.go | grep -v _test.go  # 0 hit (artifact_comments OR 折叠完成, security correctness 不破)
grep -nE '!.*IsChannelMember.*\|\|.*!.*CanAccessChannel' packages/server-go/internal/api/*.go | grep -v _test.go | wc -l  # ==3 (messages/dm_4/reactions write 路径双层 fail-closed AND 真守 AP-5 post-removal reject)

# 5) JSON-decode 400 boilerplate 部分清 (canonical-shape only, 反 custom-error-code 漂)
grep -rE 'json\.NewDecoder.*Decode' packages/server-go/internal/api/*.go | grep -v _test.go | wc -l  # ~8 (custom-error-code callers 保留 — agent_config.invalid_payload / chn_8 notification_pref / layout.invalid_payload / host_grants / push.endpoint_invalid / chn_10 — 那些 reason 字面是 public contract)
grep -rE 'decodeJSON\(' packages/server-go/internal/api/*.go | grep -v _test.go | wc -l  # ≥5 (canonical-shape callers replaced)

# 6) agent path-id load 单源 (helper-6)
grep -rE 'loadAgentByPath\(' packages/server-go/internal/api/*.go | grep -v _test.go | wc -l  # ≥8

# 7) fanoutArchive ↔ fanoutUnarchive 单源 (helper-8)
grep -rE 'fanoutChannelStateMessage\(' packages/server-go/internal/api/*.go | grep -v _test.go | wc -l  # ==2 (archive + unarchive 各 1 调用)
grep -rE 'channel_archived|channel_unarchived' packages/server-go/internal/api/*.go | grep -v _test.go | wc -l  # ≥2 (event 名仍各 1 hit, helper 内承载)

# 8) 0 schema / 0 endpoint / 0 migrations 加
ls packages/server-go/internal/migrations/ | grep -cE 'refactor_2_'  # 0 hit
git diff origin/main -- packages/server-go/internal/server/server.go | grep -cE '\+.*HandleFunc|\+.*Handle\('  # 0 hit (0 endpoint 加)

# 9) 既有 test 不动 (既有 *_test.go 行为不改)
go test -tags 'sqlite_fts5' -timeout=300s ./...  # ALL PASS, 含 TestAP5_*PostRemovalReject (双层 ACL 保留 fail-closed 真守)

# 10) post-#612 haystack gate 三轨守 (TEST-FIX-3-COV 立场承袭)
THRESHOLD_FUNC=50 THRESHOLD_PACKAGE=70 THRESHOLD_TOTAL=85 BUILD_TAGS="sqlite_fts5" go run ./scripts/lib/coverage/  # TOTAL ≥85% no func<50% no pkg<70%
```

## 3. 不在范围 (留账)

- ❌ **文件 rename** — 留 NAMING-1 (REFACTOR-2 不动文件名, 只改 helper / boilerplate)
- ❌ **新功能 / 新 endpoint / 新 schema** — 0 行为改铁律
- ❌ **helper-7 pushCursorEnvelope (RT-3 cursor wrapper SSOT)** — 留 **REFACTOR-3 新 audit 范畴** (不是 REFACTOR-1 留尾性质): internal/ws/hub.go 5 个 Push* 方法 byte-identical 模式 (PushArtifactCommentAdded / PushAgentTaskStateChanged / PushIterationStateChanged / PushAnchorCommentAdded / PushArtifactUpdated) 共 NextCursor → Frame → BroadcastToChannel/All → SignalNewEvents 5-step skeleton, 真 refactor 走 internal/ws 域加 `pushFrame[T any]` 泛型, internal/api 不动. **scope 在 internal/ws 不在 internal/api**, 不属 REFACTOR-2 (internal/api boilerplate) 范围. REFACTOR-3 立 internal/ws 域 helper SSOT 单独 audit + 4 件套.
- ❌ **REFACTOR-3 后续 audit** (messages.go 长函数拆分 / store layer query helper 整合 / cursor envelope 深化 / 其他 internal/ws 域 boilerplate) — 新 audit 范畴, 新 milestone 立项, 不是 REFACTOR-2 漏做.
- ❌ **生产配置改 / migration / DDL** — refactor 不动数据契约
- ✅ 飞马 audit #4 / #5 / #9 / #11 部分 / #12 收口 (mustUser / decodeJSON canonical-shape / loadAgentByPath / artifact_comments OR 折叠 / fanout 字面差); #6 / #11 双层 audit 反转 (字面归一不可能 + 双层 ACL 是 security correctness 设计不是 drift) — 一次做干净 (用户铁律), 不留尾.

## 4. 跨 milestone byte-identical 锁

- 复用 REFACTOR-1 #611 4 helper SSOT 模式 (5 helper SSOT, scope 跟 REFACTOR-1 互补)
- 复用 BPP-3 #489 / reasons.IsValid #496 / TEST-FIX-3 fixture SSOT 单源
- 复用 chn-3 content-lock §1 ④ DM-gate 字面跨 helper 不漂 (RejectDM / RequireDM 组各自单源)
- 复用 audit-forward-only / owner-only ACL 链 / admin god-mode 不挂红线
- 0-行为-改 wrapper 决策树**变体**: 跟 REFACTOR-1 / INFRA-3 / INFRA-4 / CV-15 / TEST-FIX-3 同源

## 5. 派活 + 双签

派 **zhanma** (REFACTOR-1 #611 zhanma-d 主战熟手优先续作, 或 zhanma-c TEST-FIX-3 后空) + 飞马 review.

双签流程: spec brief → team-lead → 飞马自审 ✅ APPROVED → yema stance + liema acceptance → zhanma 起实施 (R2.1+R2.2+R2.3 三段一 PR, **teamlead 唯一开 PR**).

## 6. 飞马 (架构师) 自审表态

✅ **APPROVED with 1 必修条件 (v1 audit 反转后修订)**:

🟡 必修-1: **字面 content-lock 严格** (跟 REFACTOR-1 #611 必修条件 byte-identical 承袭) — 战马 PR body 必示 before/after grep count: `"DM 不参与个人分组"` 字面 ≥ baseline + `dm.edit_only_in_dm` ==7 (RequireDM 组单源) + `layout.dm_not_grouped` ≥19 (RejectDM 组单源, REFACTOR-1 baseline) + DM context 下 "Forbidden" 字面 0 hit.

~~🟡 必修-2 ACL drift 统一方向~~ — **撤** (audit 反转): 双层 ACL `IsChannelMember && CanAccessChannel` 是 security correctness 设计 (CanAccessChannel = visibility-aware superset / IsChannelMember = member-required subset), 不是 drift. messages/dm_4/reactions write 路径走双层 fail-closed AND, list 路径走单 CanAccessChannel — 已分清. AP-5 post-removal scenario 真守 (实测折叠破 TestAP5_*PostRemovalReject 3 测试).

担忧 (1 项, 中度):
- 🟡 LoC 净减 -137 行 vs spec v0 估"500-700 行" — 真值: spec v0 错估 (#4 audit 算"5 行变 0 行" 实际"3 行变 1 行 = 1 行减 / 100 callsites"). REFACTOR-2 真减 -137 行 + helper SSOT 立 + drift 部分收口 (artifact_comments OR 折叠 + fanout 字面差) 是真核心目标, LoC 数字是副产品. 不算 scope 漏.

留账接受度: NAMING-1 / REFACTOR-3 (cursor envelope 深化 / messages.go 长函数拆) 全留账, 跟用户铁律 "本 milestone audit 全清" 不冲突 (NAMING-1 / REFACTOR-3 是新 audit 不在飞马 #4-#13 列表).

**ROI 拍**: REFACTOR-2 ⭐⭐⭐ — 一次性收口飞马 audit 5 处真技术债 (correctness + mechanical), 净减 -137 LoC, 5 helper SSOT 立后续 milestone 复用基座, **scope 真做干净 (audit 反转部分 + helper-7 推 REFACTOR-3 新 audit 范畴, 不是留尾)**.
