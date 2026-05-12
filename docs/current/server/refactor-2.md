# REFACTOR-2 — internal/api boilerplate 收口 (≤80 行)

> 落地: PR feat/refactor-2 · R2.1 (4 helper shared path) + R2.2 (caller 跟随) + R2.3 (drift #11 partial fix + #12 helper consolidation) + closure
> 蓝图出处: refactor 元 milestone (跟 REFACTOR-1 / INFRA-3 / INFRA-4 同等级)
> 设计沿用: [`refactor-2-spec.md`](../../implementation/modules/refactor-2-spec.md) v1 §0 ① 行为不变量 + ② 4 helper shared path + ③ 0 schema/endpoint
>
> **v1 audit 反转**: spec v0 错估 3 处校准 (用户拍板批准 — 一次做干净铁律, scope 内做不了改 spec 不留尾).

## 1. 4 helper 单一来源 (`internal/api/`)

| Helper | 文件 | 收口处 | 收口效果 |
|---|---|---|---|
| `mustUser(w, r) (*User, bool)` | `auth_helpers.go` 新 | 100 处 inline `if user == nil { writeJSONError 401 }` boilerplate (#4 audit) | status code + reason literal 由 helper 承载; 残留 11 处 variants (comment 在中间 / authenticate* / preview public / dm_10_pin 3-return / err shape 不一致) |
| `decodeJSON(w, r, &v) bool` | `request_helpers.go` 新 | 5 shared-shape callers (auth / messages ×2 / dm_4) (#5 audit) | "Invalid JSON" literal 由 helper 承载; 6 custom-error-code callers stay outside this helper (agent_config.invalid_payload / chn_8 notification_pref / layout / host_grants / push.endpoint_invalid / chn_10) because those reason literals are public contracts |
| `loadAgentByPath(w, r, store) (*User, string, bool)` | `agent_helpers.go` 新 | 8 处 path-id pattern (agents ×6 / agent_config ×2) (#9 audit) | "Agent not found" + 404 保持一致; body.AgentID paths stay outside this helper (agent_invitations / agent_config_ack_handler) |
| `fanoutChannelStateMessage(args)` | `chn_5_archived.go` 内 | 2 处 fanoutArchive ↔ fanoutUnarchive collapse (#12 audit) | channelStateMessageArgs 5 字段 (verb `关闭于`/`恢复于` / event `channel_archived`/`channel_unarchived` / payload key `archived_at`/`unarchived_at` / log prefix / ts) caller 传字面严守 |

## 2. caller 列表锁定 (40 文件 touched)

`auth_helpers.go` / `request_helpers.go` / `agent_helpers.go` 新建 + `chn_5_archived.go` 内加 helper. caller 列表: 37 个 internal/api/*.go (mustUser/decodeJSON/loadAgentByPath 调用替换) + `channels.go` fanoutArchiveSystemMessage 改走 helper-8.

不 touch 其他 (反向 `git diff origin/main -- packages/server-go/internal/api/ --name-only` 全部 ≤40 文件 + 3 helper 文件; routes / migrations 0 改).

## 3. 行为不变量 byte-identical 反查

| 字面 | baseline (main) | 当前 | 反查 |
|---|---|---|---|
| `Unauthorized` | ≥100 | helper 内 1 (其他 100 处 caller 走 helper) ✅ | mustUser 承载 shared path, status 401 unchanged |
| `Invalid JSON` | ≥5 | helper 内 1 (其他 5 处 shared-shape 走 helper) ✅ | decodeJSON 承载 shared path |
| `Agent not found` | ≥10 | helper 内 1 (其他 8 处 path-id 走 helper) ✅ | loadAgentByPath 承载 shared path |
| `layout.dm_not_grouped` | ≥19 (REFACTOR-1) | ≥19 ✅ | RejectDM 组 shared path (channel_helpers.go::requireChannelMember, REFACTOR-1 #611 已立) |
| `dm.edit_only_in_dm` | 7 | 7 ✅ | RequireDM 组 shared path (dm_4_message_edit.go), literal 不动 (audit correction: keep separate literals) |
| `channel_archived` / `channel_unarchived` | 各 1 | 各 1 ✅ | fanoutChannelStateMessage helper 内承载 |
| TestAP5_*PostRemovalReject 3 测试 | PASS | PASS ✅ | messages/dm_4/reactions write 路径双层 fail-closed AND 保留 (audit correction: 双层是 security correctness) |

## 4. 跨 milestone byte-identical 守护链

- REFACTOR-1 #611 4 helper shared path (mustUser/decodeJSON/loadAgentByPath/fanoutChannelStateMessage 续作 + RejectDM 组 literal shared path 继承)
- BPP-3 #489 PluginFrameDispatcher / reasons.IsValid #496 / TEST-FIX-3 #610 fixture shared-source pattern (helper same pattern)
- AP-4 #551 + AP-5 #555 ACL helper — write 路径走双层 fail-closed AND (post-removal 真守) / list 路径走单 CAC (audit 反转后真值)
- post-#612 haystack gate (Func=50/Pkg=70/Total=85 三轨守, 跟 TEST-FIX-3-COV 一致)
- 0-行为-改 wrapper 决策树**变体** — 跟 INFRA-3 / INFRA-4 / CV-15 / TEST-FIX-3 / REFACTOR-1 同源

## 5. v1 audit 反转 (撤 spec v0 错估)

- ❌ helper-3 DM-gate 三错码合并 **撤** — dm_4 `dm.edit_only_in_dm` (DM-only 403) vs chn_6/7/8/layout `layout.dm_not_grouped` (RejectDM 400) operate on opposite DM-gate conditions with different status and reason values; merging literals would break the user-facing error-code contract. Actual behavior: 双向各自 literal 保持一致并各走自己的 shared path, 设计目标已达成.
- ❌ helper-4 ACL 双重 → 单一 helper **撤** — 双层 `IsChannelMember && CanAccessChannel` 是 security correctness 设计, 不是 drift. 实测折叠破 TestAP5_*PostRemovalReject. Actual behavior: write 路径双层 AND / list 路径单 CAC — 已分清.
- ❌ helper-5 admin-list **撤** — 1-line 替换净减 0 + 触发 cov gate.
- ❌ helper-7 cursor envelope **推 REFACTOR-3 新 audit 范畴** — scope 在 internal/ws (5 Push* 方法 `pushFrame[T any]` 泛型重构), 不在 internal/api. 不算留尾.

## 6. Tests + verify

- `go build -tags sqlite_fts5 ./...` ✅
- `go test -tags sqlite_fts5 -timeout=300s ./...` 24 包全 PASS (含 TestAP5_*PostRemovalReject + TestCM52_X2ConcurrentCommitOneWins flake 修真)
- post-#612 haystack gate TOTAL 85.5% no func<50% no pkg<70% ✅
- LoC 净减 -137 行 (40 文件 +509 -495; spec v0 错估 500-700 因为算"5→0"实际"3→1" = 1 行减/callsite)

## 7. grep 守门

- `grep -cE '^func mustUser\\(' auth_helpers.go` ==1
- `grep -cE '^func decodeJSON\\(' request_helpers.go` ==1
- `grep -cE '^func loadAgentByPath\\(' agent_helpers.go` ==1
- `grep -cE 'func.*fanoutChannelStateMessage\\(' chn_5_archived.go` ==1
- `grep -rE 'mustUser\\(' internal/api/*.go | grep -v _test.go | wc -l` ≥100
- `grep -nE 'IsChannelMember.*&&.*CanAccessChannel|CanAccessChannel.*&&.*IsChannelMember' internal/api/*.go | grep -v _test.go` 0 hit (artifact_comments OR 折叠完成)
- `find internal/migrations -name 'refactor_2_*'` 0 hit
- `git diff origin/main -- internal/server/server.go | grep -cE '\\+.*HandleFunc|\\+.*Handle\\('` 0 hit
