# REFACTOR-1 — internal/api 重复收口 (≤80 行)

> 落地: PR feat/refactor-1 · R1.1 (4 helper-1 + helper-4 CHN 域) + R1.2 (helper-2 + helper-3 跨域) + R1.3 closure
> 蓝图出处: refactor 元 milestone (跟 INFRA-3/INFRA-4 同等级)
> 设计沿用: [`refactor-1-spec.md`](../../implementation/modules/refactor-1-spec.md) §0 ① 行为不变量 + ② 4 helper 单一来源 + ③ 0 schema/endpoint

## 1. 4 helper 单一来源 (`internal/api/`)

| Helper | 文件 | 收口处 | 收口效果 |
|---|---|---|---|
| `requireChannelMember(w, r, store, channelID, opts)` | `channel_helpers.go` (89 行) | chn_6 / chn_7 / chn_8 / chn_15 / layout 5 处 | 4-step preamble (auth → load → DM gate → member/creator) 单一来源; ChannelACLOpts{RejectDM, RequireCreator} 携 5 处变体 |
| `parseMessageEditHistory(raw)` | `message_edit_history.go` (34 行) | dm_7 + cv_15 2 处 | 11 行 byte-identical duplicate 合一; 旧 parseEditHistoryEntries / parseCommentEditHistory 删 |
| `writeRetentionOverride(w, r, store, logger, logTag, action, extraMeta, responseExtra)` | `admin_retention_helper.go` (113 行) | al_7 + hb_5 2 处 | 5-step skeleton (admin nil 401 → JSON decode → clamp → InsertAdminAction → response); al_7/hb_5 各缩为 thin wrapper |
| `handlePinToggle(w, r, pin)` | `chn_6_pin.go` 内 | chn_6 pin/unpin | thin wrapper 模式跟 chn_7 handleMuteToggle / chn_15 handleReadonlyToggle 对齐 (脱节收口) |

## 2. caller 列表锁定 (9 文件 touched)

CHN 域: `chn_6_pin.go` / `chn_7_mute.go` / `chn_8_notif_pref.go` / `chn_15_readonly.go` / `layout.go`
跨域: `dm_7_edit_history.go` / `cv_15_comment_edit_history.go` / `al_7_audit_retention_override.go` / `hb_5_heartbeat_retention_override.go`

不 touch 其他 internal/api/*.go (反向 `git diff origin/main --name-only` 仅 9 caller + 3 helper).

## 3. 行为不变量 byte-identical 反查

| 字面 | baseline (main) | 当前 | 反查 |
|---|---|---|---|
| `DM 不参与个人分组` | 5 | 5 ✅ | 4 处 inline → helper-1 内承载 + 4 处 doc-comment 引用保留 count |
| `layout.dm_not_grouped` | 19 | 19 ✅ | 4 处 inline → helper-1 内承载 + 4 处 doc-comment 引用 |
| `dm.edit_only_in_dm` | 7 | 7 ✅ | byte-identical 不动 |
| `metadata.target` | 6 | 7 | helper-3 docstring +1 ≥ baseline ✅ |
| `parseEditHistoryEntries` / `parseCommentEditHistory` | 各 1 (合 2) | 0 hit ✅ | 合一到 parseMessageEditHistory |
| `func handle{Pin,Unpin}Channel` | 各 1 (full body) | 各 1 thin wrapper (≤4 行) ✅ | toggle 模式对齐 |

## 4. 跨 milestone byte-identical 守护链

- BPP-3 #489 PluginFrameDispatcher / reasons.IsValid #496 / TEST-FIX-3 #610 fixture 单一来源 (helper 单一来源模式沿用)
- chn_7 #523 / chn_15 既立 toggle 模式 (chn_6 对齐脱节收口)
- content-lock §1 字面绑 (DM-gate 4 字面跨 helper 不脱节)
- audit-forward-only / owner-only ACL 链 (RetentionOverride helper audit byte-identical)
- 0-行为-改 wrapper 决策树**变体** — 跟 INFRA-3 / INFRA-4 / CV-15 / TEST-FIX-3 "wrapper 真有 prod code 0 schema/0 行为改" 同源

## 5. 反向约束 / 不在范围

- ❌ #4 user==nil → 401 boilerplate 100+ 处收 — 留 REFACTOR-2 (单 PR 风险高)
- ❌ #5 JSON-decode → 400 boilerplate 30 处 — 留 REFACTOR-2
- ❌ #6 DM-gate 13 hits 三种错误码统一 — 留 REFACTOR-2 (需先讨论 error code 单一来源 设计原则)
- ❌ #8/#9/#10/#11 admin-list / loadAgent / cursor-push / ACL 脱节 — 留 REFACTOR-2/3
- ❌ #12/#13 fanout 字面差 / agents 双形状 decoder — content-lock 绑 / 不够痛, 不动

## 6. Tests + verify

- `go build -tags sqlite_fts5 ./...` ✅
- `go test -tags sqlite_fts5 -timeout=180s ./internal/api/` 全 PASS ✅ (含 9 域 ≥30 既有 test func 不动)
- 字面 4 grep count == baseline (`DM 不参与个人分组` + `layout.dm_not_grouped` + `dm.edit_only_in_dm` + `metadata.target`)
