# REFACTOR-2 stance checklist — internal/api boilerplate 收口 (refactor only, **v1 audit 反转**)

> 5 立场 byte-identical 跟 refactor-2-spec.md v1 §0+§2. **真有 prod code refactor 但 0 行为改 / 0 schema / 0 endpoint / 0 client UI**. 跟 REFACTOR-1 #611 + DL-1 #609 + INFRA-3 #594 + TEST-FIX-1/2/3 同模式. content-lock 不需 (server-only). **scope 已校准 — audit 反转 3 处 spec 错估 (helper-3/-4/-5) + helper-7 推 REFACTOR-3 新 audit 范畴 (不是留尾, scope 在 internal/ws 不在 internal/api)**.

## 1. 0 行为改 (refactor only)
- [ ] 0 endpoint shape 改 — `git diff origin/main -- internal/api/server.go | grep -E '\\+.*Method|\\+.*Register'` 0 hit
- [ ] 0 response body / 0 error code 字面改 — 既有错码 (`dm.*`/`pin.*`/`chn.*`/`auth.*`) before/after byte-identical
- [ ] 0 SLO 收紧 — 反"为绕 boilerplate 改 endpoint shape"
- [ ] 既有 unit + e2e 全 PASS byte-identical (反 race-flake, 跟 #612 cov 85% 协议承袭)

## 2. 0 schema / 0 endpoint / 不动 v 号
- [ ] `git diff origin/main -- internal/migrations/` 0 改
- [ ] `currentSchemaVersion` 不动 + `migrations/refactor_2_` 0 hit
- [ ] server.go register 新 endpoint 0 hit

## 3. helper 单源不复制 (mustUser / decodeJSON / loadAgentByPath / fanoutChannelStateMessage)
- [ ] 4 helper 各 internal/api/ 单源一份, 反 spam 多文件本地复制
- [ ] **黑名单 grep 真测**: `grep -rE 'mustUser\\(' packages/server-go/internal/api/*.go | grep -v _test.go | wc -l` ≥100 + `grep -rE 'if user == nil' packages/server-go/internal/api/*.go | grep -v _test.go | grep -v helpers.go | wc -l` ≤15 (variants only)
- [ ] 0 新 `*_helpers.go` 复制 (单源 4 helper 文件 = auth_helpers + request_helpers + agent_helpers + chn_5_archived 内, 跟 REFACTOR-1 4 helper SSOT 承袭)
- [ ] Go idiom 命名锁 — `mustUser` / `decodeJSON` / `loadAgentByPath` / `fanoutChannelStateMessage` byte-identical (反 `getUser` / `parseJSON` / `fetchAgent` 漂)

## 4. DM-gate 错码 RejectDM/RequireDM 双向各自单源 (audit 反转, **不归一**)
- [ ] `layout.dm_not_grouped` (RejectDM 组, 400, channel.kind!="dm") 单源在 channel_helpers.go::requireChannelMember (REFACTOR-1 #611 已立, baseline ≥19 hits)
- [ ] `dm.edit_only_in_dm` (RequireDM 组, 403, channel.kind=="dm") 单源在 dm_4_message_edit.go::handlePatchMessage (==7 hits, byte-identical 不动)
- [ ] **撤"三错码归一"立场** — 同字段反向条件 + 不同 status + 不同 reason 字面不能共错码 (会破 user-facing 错码契约). 真值: 双向各自字面 byte-identical 单源, 立场目标已达成.

## 5. ACL 双层 fail-closed AND 是 security correctness (audit 反转, **不收单源**)
- [ ] messages.go / dm_4_message_edit.go / reactions.go write 路径走双层 fail-closed AND `!IsChannelMember || !CanAccessChannel` 真守 AP-5 post-removal reject (实测折叠破 TestAP5_*PostRemovalReject 3 测试)
- [ ] artifact_comments.go OR 折叠 (`!IsChannelMember && !CanAccessChannel` → `!CanAccessChannel`) — 双 false 等价单 false 不破语义, 真减 LoC
- [ ] list 路径走单 CanAccessChannel (visibility-aware superset, 反 member-only 漏 public)
- [ ] **撤"双 ACL 收单源"立场** — 双层是 security correctness 设计不是 drift: CanAccessChannel = visibility-aware superset / IsChannelMember = member-required subset, write 路径需 AND 真守 (creator 但已非 member 不能 write). 真值: messages.go 路径已分清 (list 单 CAC / write 双层).
- [ ] anchor #360 owner-only 锁链 22+ PRs 承袭 + REG-INV-002 fail-closed
- [ ] admin god-mode 不挂 helper (反向 grep `admin.*IsChannelMember|admin.*CanAccessChannel` 0 hit)

## 6. scope 一次做干净 (audit 反转后真 scope, 不留尾)
- [ ] spec v1 #4 / #5 / #9 / #11 部分 / #12 真闭一次合 (不留 v2 / 不留 follow-up)
- [ ] **#6 / #11 双层** spec 错估 audit 反转**撤** (字面归一不可能 + 双层 ACL 是 security correctness), 不算留尾
- [ ] **#7 cursor envelope** 推 REFACTOR-3 新 audit 范畴 (scope 在 internal/ws 不在 internal/api), 不算留尾
- [ ] 跟 user memory `strict_one_milestone_one_pr` + `progress_must_be_accurate` 铁律承袭

## 7. 测试全 PASS (0 改, 0 race-flake)
- [ ] 既有 unit + e2e 0 改 byte-identical (反 refactor 顺手改测试)
- [ ] 0 race-flake — 跟 TEST-FIX-2 #608 + TEST-FIX-3 #610 + #612 deterministic 协议承袭
- [ ] cov ≥85% 不降 (#612 协议, user memory `no_lower_test_coverage` 铁律) + post-#612 haystack gate Func=50/Pkg=70/Total=85 三轨过
- [ ] server-go ./... 全 24+ packages 全绿 (+sqlite_fts5)

## 反约束 — 真不在范围
- ❌ 文件名重命名 / 结构体名 audit (留 NAMING-1)
- ❌ 改 endpoint shape / response body / error code 字面
- ❌ 0 schema / 0 migration / 0 client / 0 acceptance / 0 content-lock 改
- ❌ 加新 CI step (跟 REFACTOR-1 + INFRA-3 + TEST-FIX-* 同精神)
- ❌ helper-3 DM-gate 三错码归一 (撤, 字面归一不可能)
- ❌ helper-4 ACL 双层收单源 (撤, security correctness 设计不是 drift)
- ❌ helper-5 writeAdminListResponse (撤, 1-line 净减 0 + 触发 cov gate)
- ❌ helper-7 pushCursorEnvelope (推 REFACTOR-3 新 audit 范畴, scope 在 internal/ws)
- ❌ admin god-mode 加挂 (永久不挂, ADM-0 §1.3 红线)

## 跨 milestone byte-identical 锁链 (5 链)
- **REFACTOR-1 #611** 4 helper SSOT 承袭 + RejectDM 组字面单源继承
- **DL-1 #609** 4 interface 抽象同精神 (refactor 真有 prod code 类别)
- **AP-4 #551 + AP-5 #555** ACL helper 复用 — write 路径走双层 fail-closed AND (post-removal 真守) / list 路径走单 CAC
- **anchor #360** owner-only ACL 锁链 22+ PRs 立场延伸
- **#612 cov 85% deterministic + TEST-FIX-1/2/3** race-flake 协议承袭 + post-#612 haystack gate (Func=50/Pkg=70/Total=85)

## PM 拆死决策 (3 段, audit 反转后修订)
- **REFACTOR-2 #4 / #5 / #9 / #11 部分 / #12 scope 全清 vs REFACTOR-1 留尾拆死** — 一次合不分 v0/v1/v2 (用户铁律). #6 / #11 双层 / #7 audit 反转**撤 spec** 不算留尾 (用户拍板批准 spec 错估校准, 真值: 字面归一不可能 / 双层是 security correctness / cursor envelope scope 在 internal/ws).
- **helper SSOT vs spam 复制拆死** — 4 helper 单源 count==1 各 (反 N+ 散布 / 反 *_helpers.go 复制 / 反改 endpoint shape 绕)
- **DM-gate 双向各自单源 vs 错码归一拆死** — RejectDM 组 + RequireDM 组各自字面 byte-identical 单源, 不归一字面 (反"统一字面 = 破错码契约 = security correctness 红线")

## 用户主权红线 (5 项)
- ✅ 0 行为改 (e2e + unit 全 PASS byte-identical, 含 TestAP5_*PostRemovalReject 双层 ACL 真守)
- ✅ 既有 ACL gate 字面 + 行为 byte-identical (anchor #360 + REG-INV-002 守 + 双层 fail-closed 不折叠)
- ✅ 0 user-facing change (server-only refactor)
- ✅ 0 SLO 收紧 vs 0 endpoint shape 改 (反"为绕 boilerplate 改")
- ✅ admin god-mode 不挂 helper (ADM-0 §1.3 红线)

## PR 出来 5 核对疑点
1. 黑名单 grep — `mustUser(` ≥100 + `if user == nil` ≤15 (variants only) (SSOT 真兑现)
2. 0 schema / 0 endpoint / 0 migration (`git diff` 反向断言)
3. 既有 unit + e2e 全 PASS + post-#612 haystack gate 三轨 PASS (Func=50/Pkg=70/Total=85)
4. DM-gate 双向各自单源 byte-identical (`layout.dm_not_grouped` ≥19 + `dm.edit_only_in_dm` ==7, 不归一)
5. scope #4/#5/#9/#11部分/#12 闭 0 留尾 + #6/#11双层/#7 audit 反转**撤 spec** 不算留尾 (PR description 写明)

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-05-01 | yema | v0 stance checklist — 7 立场 + 3 段 + 5 跨链 + scope 全清铁律. |
| 2026-05-01 | yema (audit 反转后修订) | v1 — 撤 helper-3/-4/-5 立场 (audit 反转 3 处 spec 错估), 撤"DM-gate 三错码归一"+"双 ACL 收单源" 改 "RejectDM/RequireDM 双向各自单源" + "双层 ACL 是 security correctness 设计不是 drift". helper-7 cursor envelope 推 REFACTOR-3 新 audit 范畴 (scope 在 internal/ws 不在 internal/api), 不算留尾. 5 helper SSOT (撤 helper-3/-4/-5/-7 留 mustUser/decodeJSON/loadAgentByPath/fanoutChannelStateMessage 共 4 helper + drift #11 部分收口). 立场承袭 REFACTOR-1 + post-#612 haystack gate + AP-5 post-removal reject 实测真守. |
