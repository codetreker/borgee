# NAMING-1 — milestone-prefix 全清 + Go 社区命名规范 (≤80 行)

> 落地: PR feat/naming-1 · N1.1 (A 文件 + B struct/handler + C test func) + N1.2 (E modules→architecture) + N1.3 closure
> 蓝图出处: naming convention 元 milestone (跟 REFACTOR-1/2 / INFRA-3/4 同等级)
> 设计沿用: [`naming-1-spec.md`](../../implementation/modules/naming-1-spec.md) §0 ① 0 行为改 + ② 5 类 rename 一次全清 + ③ 0 schema/endpoint/v 号

## 1. 5 类 rename scope

| 类 | 处数 | 实施 | 模式 |
|---|---|---|---|
| **A. server-go .go 文件名** | 169 git mv | git mv 保 history; 同 package 文件 rename 不破 import path, 0 caller 改 | `<prefix>_<num>(_<sub>)?_<feature>.go` → `<domain>_<feature>.go` (dm→message/chn→channel/cv→canvas/bpp→plugin/hb→host/rt→realtime/al→agent/ap→permission/adm→admin/cm→community/cs→collab/dl→datalayer/infra→infra/test_fix→testfix); 冲突回退 `<prefix><num>_<feature>` |
| **B. struct/handler + migration var** | 18 + 32 | Python regex 跨 .go 文件批替换 | ADM2Handler→AdminEndpointsHandler, DM10PinHandler→MessagePinHandler, dm101MessagesPinnedAt→messagesPinnedAt 等 (Python collision 检测无 dup) |
| **C. Go test 函数名** | 924 处全 unique (856 真改 strip + 90 file-prefix functionality suffix 接合规) | Python regex 三 pass: `Test<DOMAIN>\d+_X` / `Test<DOMAIN>\d+[A-Z]\d+_X` (sub-letter+digit) / `Test<DOMAIN>\d+<CamelCase>` (no underscore) | 飞马 audit 反转后续 commit c2d192c4 全 unique 化 (file-prefix functionality suffix); 同 file dup 自动 _N suffix |
| **D. TSX 测试命名归一** | 0 | 检测 dot-variant 0 hit, 已是 kebab/PascalCase 一致 | (无需 rename) |
| **E. modules/ → docs/architecture/** | 11 git mv | git mv 保 history, 跨 doc 引用 11 处 `implementation/modules/<arch>` → `architecture/<arch>` 全更 | admin-model / agent-lifecycle / auth-permissions / canvas-vision / channel-model / client-shape / concept-model / data-layer / host-bridge / plugin-protocol / realtime |

## 2. caller 列表 (~210 文件 touched)

- A 类 git mv: 169 .go 文件 (internal/api/ + internal/migrations/ + internal/store/ + internal/auth/ + internal/ws/ + internal/bpp/ + internal/server/ + sdk/bpp/)
- B 类 (struct + migration var): 89 文件 (handler 调用 caller 跟随 + migration registry)
- C 类 (test func): 137 文件 (test func 调用 + 注释引用 literal 跟随)
- E 类: 11 + 11 跨 doc ref 文件
- 5 处 hardcoded 路径修 (channel_history_test / message_history_test / host_agent_state_log_archived_at_test / canvas_edit_history_test / lifecycle_audit_test)
- 二次清理 awkward dup-domain (message_messages_edit_history → messages_edit_history / channel_channels_description_edit_history → channels_description_edit_history)

## 3. 行为不变量 byte-identical 反查

| 字面 | baseline | 当前 | 反查 |
|---|---|---|---|
| HTTP status code literals | unchanged | unchanged ✅ | rename 仅动文件路径 / 类型符号 / 测试函数名 |
| error reason code 字面 (`layout.dm_not_grouped` ≥19 / `dm.edit_only_in_dm` 7 / `pin.dm_only_path` 等) | baseline | baseline ✅ | 跟 REFACTOR-1/2 守护链一致 |
| audit log 字段 + WS broadcast event 名 | unchanged | unchanged ✅ | rename 不动 audit/ws |
| migration Version 数值 (v=1..v=45) | 不动 | 不动 ✅ | git diff origin/main -- migrations/ \| grep '^+.*Version:' 0 hit |
| DB schema column 名 | 不动 | 不动 ✅ | rename 仅 var 名不动 column |
| TestAP5_*PostRemovalReject 3 测试 | PASS | PASS ✅ | C 类 test func rename 0 行为改 |

## 4. 跨 milestone byte-identical 守护链

- INFRA-3 #594 git mv rename detection 同模式 (PROGRESS 子文件迁同精神)
- REFACTOR-1 #611 / REFACTOR-2 #613 字面 content-lock 必修条件沿用
- BPP-3 / reasons.IsValid source pattern (rename 后不 drift)
- TEST-FIX-3-COV #612 haystack gate 三轨 (Func=50/Pkg=70/Total=85, rename 后必过)
- 0-行为-改 wrapper 决策树**变体** (跟 REFACTOR-1/2 / INFRA-3/4 / CV-15 / TEST-FIX-3 同源)

## 5. 遗留透明

- ❌ Test func collision-keep — 飞马 audit 反转: 90 处全 unique 化 (file-prefix functionality suffix, 不留 NAMING-2). 实施 commit c2d192c4: `Test<Domain>\d+_X` → `Test<FilePrefixCamel>_X` (e.g. TestCHN51_NoSchemaChange → TestChn5archived_NoSchemaChange). grep 检查 `Test(CHN|DM|RT|AL|CV|BPP|HB|AP|ADM|CM|CS|DL)[0-9]+` ==0.
- ❌ REFACTOR-3 (cursor envelope 深化 / messages.go 长函数拆 / store query helper 整合) — 留 REFACTOR-3 议程, 跟本 NAMING-1 不同 concern
- ❌ DOM data-attr / CSS class 命名归一 (e.g. `data-cv7-comment-input`) — content-lock 绑, 留 v3+ 议程
- ❌ DB column 名改 — 0 schema 改铁律
- ❌ migration v 号字面改 — 0 migration v 号铁律

## 6. Tests + verify

- `go build -tags sqlite_fts5 ./...` ✅
- `go test -tags sqlite_fts5 -timeout=300s ./...` 24 包全 PASS (含 TestAP5_*PostRemovalReject literal behavior) ✅
- `go vet ./...` 0 redeclared ✅
- post-#613 haystack gate TOTAL 85.5%, no func<50%, no pkg<70% ✅

## 7. grep 守门

- 文件 milestone-prefix 0 残留: `find packages/server-go/internal -name '*.go' | grep -cE '/(al|chn|dm|cv|bpp|hb|rt|ap|adm|cm|cs|dl|infra|test_fix)_[0-9]'` ==0
- struct/handler milestone-prefix 0 残留: `grep -rE 'type [A-Z]+[0-9]+[A-Z][a-zA-Z]*Handler'` 0 hit
- migration var milestone-prefix 0 残留: `grep -rE 'var (dm|al|chn|cv|bpp|hb|ap|adm|cm)[0-9]+[a-zA-Z]+ = Migration'` 0 hit
- migration Version 字面不动: `git diff origin/main -- migrations/ | grep -cE '^\\+\\s*Version:'` ==0
- modules/ 游离 doc 0 残留: `ls docs/implementation/modules/ | grep -cE '^(admin-model|agent-lifecycle|auth-permissions|canvas-vision|channel-model|client-shape|concept-model|data-layer|host-bridge|plugin-protocol|realtime)\\.md$'` ==0
- docs/architecture/ 11 文件就位: `ls docs/architecture/*.md | wc -l` ==11
