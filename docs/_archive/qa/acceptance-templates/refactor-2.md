# Acceptance Template — REFACTOR-2 (handler boilerplate 收口, **v1 audit 反转**)

> Spec brief `refactor-2-spec.md` v1 (飞马 audit 反转修订). Owner: 战马C 实施 / 飞马 review / 烈马 验收.
>
> **REFACTOR-2 真 scope (v1 校准)**: handler-level 4 helper SSOT 抽出 (mustUser / decodeJSON / loadAgentByPath / fanoutChannelStateMessage) + caller boilerplate 100+ 处 collapse + ACL drift #11 部分收口 (artifact_comments OR 折叠) + #12 fanout 字面差收口. **撤** v0 的 helper-3 (DM-gate 三错码归一 — 字面归一不可能, 语义反向) + helper-4 (双 ACL 收单源 — security correctness 设计不是 drift) + helper-5 (admin-list — 净减 0). 立场承袭 REFACTOR-1 #611 (CHN 4 helper) + post-#612 haystack gate. **0 endpoint 行为改 + 0 migration / 0 schema + LoC 净减 -137 行 (实测真值; spec v0 错估 500-700)**.

## 验收清单

### §1 helper 抽出验收 (4 helper 单源)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 `mustUser(w, r) (*User, bool)` 单源在 `internal/api/auth_helpers.go` (反约束: 反向 grep `^func mustUser\(` ==1 hit) | grep | `grep -cE '^func mustUser\(' packages/server-go/internal/api/auth_helpers.go` ==1 |
| 1.2 `decodeJSON(w, r, &v) bool` 单源在 `internal/api/request_helpers.go` (canonical-shape only — custom-error-code callers 保留 inline 反约束 reason 字面 byte-identical) | grep | `grep -cE '^func decodeJSON\(' packages/server-go/internal/api/request_helpers.go` ==1 |
| 1.3 `loadAgentByPath(w, r, store) (*User, string, bool)` 单源在 `internal/api/agent_helpers.go` (path-id pattern only; body.AgentID 路径不收) | grep | `grep -cE '^func loadAgentByPath\(' packages/server-go/internal/api/agent_helpers.go` ==1 |
| 1.4 `fanoutChannelStateMessage(args)` 单源在 `internal/api/chn_5_archived.go` 内 (fanoutArchive ↔ fanoutUnarchive collapse, 5 字段 caller 传字面严守) | grep | `grep -cE 'func.*fanoutChannelStateMessage\(' packages/server-go/internal/api/chn_5_archived.go` ==1 |

### §2 caller 跟随验收

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 mustUser callsites ≥100 (替换 100+ 处 inline `if user == nil { writeJSONError 401 }` boilerplate) | grep | `grep -rE 'mustUser\(' packages/server-go/internal/api/*.go \| grep -v _test.go \| wc -l` ≥100 |
| 2.2 残留 `if user == nil` ≤15 (variants — comment 在中间 / authenticate* helper / preview public / dm_10_pin 3-return / err shape 不一致) | grep | `grep -rE 'if user == nil' packages/server-go/internal/api/*.go \| grep -v _test.go \| grep -v helpers.go \| wc -l` ≤15 |
| 2.3 decodeJSON callsites ≥5 (canonical-shape callers replaced) + custom-error-code callers (≥6) 保留 inline | grep | `grep -rE 'decodeJSON\(' packages/server-go/internal/api/*.go \| grep -v _test.go \| wc -l` ≥5 + `grep -rE 'json\.NewDecoder.*Decode' packages/server-go/internal/api/*.go \| grep -v _test.go \| wc -l` ~8 (custom-error-code 保留) |
| 2.4 loadAgentByPath callsites ≥8 (path-id pattern replaced) | grep | `grep -rE 'loadAgentByPath\(' packages/server-go/internal/api/*.go \| grep -v _test.go \| wc -l` ≥8 |
| 2.5 LoC 净减 ≥100 行 (实测 -137; spec v0 错估 500-700) | git diff --stat | `git diff origin/main --shortstat packages/server-go/internal/api/` 净减 ≥100 行 |

### §3 drift 收口验收 (audit 反转后真 scope)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 DM-gate 双向各自单源 byte-identical (audit 反转: **不归一**, RejectDM/RequireDM 立场目标已达成) — `layout.dm_not_grouped` ≥19 hits (REFACTOR-1 baseline RejectDM 组单源在 channel_helpers.go) + `dm.edit_only_in_dm` ==7 hits (RequireDM 组单源在 dm_4_message_edit.go, byte-identical 不动) | grep | `grep -rcE 'layout\.dm_not_grouped' packages/server-go/internal/api/*.go \| awk -F: '{s+=$NF}END{print s}'` ≥19 + `grep -rcE 'dm\.edit_only_in_dm' packages/server-go/internal/api/*.go \| awk -F: '{s+=$NF}END{print s}'` ==7 |
| 3.2 ACL drift #11 部分收口 (audit 反转: **双层 fail-closed AND 保留**, security correctness 设计不是 drift) — `artifact_comments.go` ×2 处 OR 折叠 (`!IsChannelMember && !CanAccessChannel` → `!CanAccessChannel`); messages/dm_4/reactions write 路径双层 AND 保留 (TestAP5_*PostRemovalReject 真守) | grep | `grep -nE 'IsChannelMember.*&&.*CanAccessChannel\|CanAccessChannel.*&&.*IsChannelMember' packages/server-go/internal/api/*.go \| grep -v _test.go` 0 hit + `grep -nE '!.*IsChannelMember.*\|\|.*!.*CanAccessChannel' packages/server-go/internal/api/*.go \| grep -v _test.go \| wc -l` ==3 (messages/dm_4/reactions write 路径) |
| 3.3 #12 fanoutArchive ↔ fanoutUnarchive 字面差收口 — fanoutChannelStateMessage 单源 helper, channel_archived/channel_unarchived event 名 + verb `关闭于`/`恢复于` + payload key `archived_at`/`unarchived_at` byte-identical caller 传 | grep | `grep -rE 'fanoutChannelStateMessage\(' packages/server-go/internal/api/*.go \| grep -v _test.go \| wc -l` ==2 (archive + unarchive 各 1 调用) |
| 3.4 飞马 audit #4 / #5 / #9 / #11 部分 / #12 闭; #6 / #11 双层 / #7 audit 反转**撤** spec — 一次做干净不留尾 (用户铁律) | inspect | spec v1 §0 + §3 反向断言 audit 反转 3 处校准 + helper-7 推 REFACTOR-3 新 audit 范畴 (scope 在 internal/ws 不在 internal/api) |

### §4 全清不留账验收 (反约束 + 行为不变量守门)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 4.1 0 endpoint 行为改 — 既有 server-go ./... 全绿 byte-identical (含 TestAP5_*PostRemovalReject 双层 ACL 真守) | full test | `go test -tags sqlite_fts5 -timeout=300s ./...` 24 packages 全 PASS |
| 4.2 0 migration / 0 schema 改 (`git diff main -- internal/migrations/` 0 行) | git diff | 0 行 |
| 4.3 post-#612 haystack gate 三轨过 — Func=50 / Pkg=70 / Total=85 (TEST-FIX-3-COV #612 立场承袭) | CI verify | `THRESHOLD_FUNC=50 THRESHOLD_PACKAGE=70 THRESHOLD_TOTAL=85 go run ./scripts/lib/coverage/` TOTAL ≥85% no func<50% no pkg<70% |
| 4.4 反平行 helper / 反 admin god-mode bypass — 4 helper 单源各==1 hit + admin god-mode 反向 grep `admin.*mustUser\|admin.*loadAgentByPath` 在 admin*.go 0 hit (ADM-0 §1.3 红线) | CI grep | 反向 grep tests PASS |

## REG-REFACTOR2-* 占号 (audit 反转后修订)

- REG-REFACTOR2-001 🟢 4 helper 抽出 (mustUser / decodeJSON / loadAgentByPath / fanoutChannelStateMessage) 全单源各==1 hit
- REG-REFACTOR2-002 🟢 caller 跟随 boilerplate (mustUser ≥100 callsites + decodeJSON canonical-shape ≥5 + loadAgentByPath ≥8) + LoC 净减 -137 行 (40 文件 +509 -495)
- REG-REFACTOR2-003 🟢 DM-gate 双向各自单源 byte-identical (`layout.dm_not_grouped` ≥19 baseline + `dm.edit_only_in_dm` ==7, **audit 反转: 不归一字面**)
- REG-REFACTOR2-004 🟢 ACL drift #11 部分收口 (artifact_comments OR 折叠, messages/dm_4/reactions write 双层 fail-closed AND 保留 — **audit 反转: 双层是 security correctness 设计不是 drift, TestAP5_*PostRemovalReject 真守**) + #12 fanout 字面差收口
- REG-REFACTOR2-005 🟢 0 endpoint 行为改 + 0 migration / 0 schema + 既有 24 包 test 全绿不破 (含 TestAP5_*PostRemovalReject)
- REG-REFACTOR2-006 🟢 post-#612 haystack gate Func=50/Pkg=70/Total=85 三轨 PASS (TOTAL 85.5%) + 反平行 helper + 反 admin god-mode bypass (ADM-0 §1.3) + 跨 milestone const SSOT 锁链承袭

## 退出条件

- §1 (4) + §2 (5) + §3 (4) + §4 (4) 全绿 — 一票否决
- 4 helper 全单源 (mustUser / decodeJSON / loadAgentByPath / fanoutChannelStateMessage)
- caller boilerplate 替换 (mustUser ≥100 + decodeJSON ≥5 canonical-shape + loadAgentByPath ≥8) + LoC 净减 ≥100 行 (实测 -137)
- DM-gate 双向各自单源 byte-identical (audit 反转: 不归一) + ACL drift #11 部分收口 (audit 反转: 双层保留 fail-closed)
- 飞马 audit #4 / #5 / #9 / #11 部分 / #12 闭; #6 / #11 双层 / #7 audit 反转**撤** spec (用户拍板批准)
- 既有全包 unit 全绿不破 + post-#612 haystack gate 三轨 PASS
- 0 endpoint 行为改 + 0 migration / 0 schema
- 登记 REG-REFACTOR2-001..006

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-05-01 | 烈马 | v0 — acceptance template 草稿 (4 选 1 验收框架 + REG-REFACTOR2-001..006 6 行占号 ⚪). |
| 2026-05-01 | 战马C | flip — REG-REFACTOR2-001..006 6 ⚪→🟢 实施验收 PASS. |
| 2026-05-01 | 烈马 (audit 反转后修订) | v1 — 撤"DM-gate 三错码归一" + "双 ACL 收单源" 立场 (audit 反转 spec 错估 3 处), 改 "DM-gate 双向各自单源 byte-identical 不归一" + "ACL 双层 fail-closed AND 是 security correctness 设计不是 drift". 验收清单 §3 重写 (drift 收口 #11 部分 + #12 fanout). LoC 真值 -137 行 (spec v0 错估 500-700 因 #4 audit 算"5→0"实际"3→1"). helper-7 cursor envelope 推 REFACTOR-3 新 audit 范畴. 退出条件按 v1 真 scope 校准. |
