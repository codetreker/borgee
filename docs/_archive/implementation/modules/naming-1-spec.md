# NAMING-1 spec brief — milestone-prefix 全清 + Go 社区命名规范 (≤80 行)

> 飞马 · 2026-05-01 · 用户拍板 (一次做干净, 不准挑 top-N) · zhanma 主战 + 飞马 review
> **关联**: REFACTOR-2 #613 ✅ merged · 文件 rename 留账接口
> **命名**: NAMING-1 = 第一件 naming convention milestone (元 milestone)

> ⚠️ Naming milestone — **0 行为改 / 0 schema / 0 endpoint / 0 migration v 号** + 既有 unit/e2e 全 PASS byte-identical.
> 大批量 git mv 保 history + gofmt/goimports 跑全, 一 PR 合, **本 milestone milestone-prefix 全清不留 v2**.

## 0. 关键约束 (3 条立场)

1. **0 行为改 + 字面 byte-identical 严格** (跟 REFACTOR-1/2 必修条件承袭): rename **只动文件路径 / 类型符号 / 测试函数名**, 不动 (a) HTTP status code (b) error reason code 字面 (`layout.dm_not_grouped` / `dm.edit_only_in_dm` / `pin.dm_only_path` 等) (c) audit log 字段 (d) WS broadcast event 名 (e) **migration Version 数值** (字面 v=1..v=45 不动) (f) DB schema column 名. 反约束: 既有错误码字面 grep count 等量; 既有 unit/e2e (≥80 test func) 全 PASS 不动 (除 test func 名 rename 自身).

2. **5 类 rename 一次全清** (用户铁律一次做干净):
   - **A. server .go 文件名** (87 + 70 + 12 = ~169 文件): `internal/api/<prefix>_<num>_<feature>.go` → `<feature>.go` (e.g. `dm_10_pin.go` → `message_pin.go`, `chn_6_pin.go` → `channel_pin.go`); `internal/migrations/<prefix>_<num>_<feature>.go` 同模式 (e.g. `dm_10_1_messages_pinned_at.go` → `messages_pinned_at.go`); `internal/{store,auth,ws,bpp,helper}/<prefix>_*.go` 同模式. 冲突时加 domain 前缀 (e.g. `pin.go` 冲突 → `dm_message_pin.go` 跟 `channel_pin.go` 各自分开)
   - **B. 结构体名 + handler 名**: `DM10PinHandler` → `MessagePinHandler` / `CHN6PinHandler` → `ChannelPinHandler` 等; `dm101MessagesPinnedAt` migration var → `messagesPinnedAt`; 反 milestone-prefix 跟功能命名混 (e.g. `RT4PresenceTracker` → `PresenceTracker`)
   - **C. Go test 函数名**: `TestCHN5_*` / `TestDM10_*` / `TestRT4_*` → `TestChannel*` / `TestMessagePin*` / `TestPresence*` 等; 反向 grep `TestCHN[0-9]+|TestDM[0-9]+|TestRT[0-9]+|TestAL[0-9]+|TestCV[0-9]+|TestBPP[0-9]+|TestHB[0-9]+|TestAP[0-9]+|TestADM[0-9]+|TestCM[0-9]+|TestCS[0-9]+|TestDL[0-9]+` 0 hit
   - **D. TSX 测试命名归一**: `packages/client/src/__tests__/` 当前混 3 体 (kebab `channel-groups-ui.test.ts` / dot-variant `cv-2-v2-media-preview.test.ts` / PascalCase `FailureCenter.test.tsx`) → 统一 **kebab-case for utility / PascalCase for component test** (跟既有 Go 文件命名 + React 组件命名习惯承袭, 反 dot-variant)
   - **E. modules/ 11 游离架构概览迁 docs/architecture/**: `admin-model.md` / `agent-lifecycle.md` / `auth-permissions.md` / `canvas-vision.md` / `channel-model.md` / `client-shape.md` / `concept-model.md` / `data-layer.md` / `host-bridge.md` / `plugin-protocol.md` / `realtime.md` — 跟 milestone spec (`<milestone>-spec.md`) 区分; `git mv` 保 history; 跨 doc 引用 grep 全更 (反 stale link)
   
   反约束: 5 类**全清不留**, 反向 grep milestone-prefix 文件 0 hit (除 acceptance template / progress phase 文档 / regression-registry / spec brief 自身字面引用 — 那些是 milestone 编号上下文必留).

3. **0 schema / 0 endpoint / 0 migration v 号 / 0 行为改** (Naming milestone 立场, 跟 REFACTOR-1/2 / INFRA-3/4 wrapper 系列承袭): PR diff 仅 (a) git mv 文件路径 (b) struct/handler/var rename (c) caller 跟随 (gofmt/goimports 跑全) (d) test func rename + caller 跟随 (e) 跨 doc link 更. 反约束: `git diff origin/main -- 'packages/server-go/internal/migrations/' | grep -cE '^\+\s*Version:\s*[0-9]+' = 0` (Version 字面不动); 0 schema column 名改 + 0 endpoint URL 改 + 0 reason code 字面改.

## 1. 拆段实施 (3 段, 一 milestone 一 PR — 内顺序)

| 段 | 范围 |
|---|---|
| **N1.1 server-go rename** | A 类 (~169 文件 git mv) + B 类 (struct/handler/var rename, gofmt/goimports 跑全 caller 跟随) + C 类 (Go test 函数名 rename, 跟 caller 自动跟随). 走机械 sed + git mv 批量, 单 milestone 域逐个域跑. ~169 文件 rename 净 0 LoC (字面替换). |
| **N1.2 client-go + docs** | D 类 TSX test 命名归一 (kebab/PascalCase, ~30 文件 rename) + E 类 modules/ 11 文件迁 docs/architecture/ (git mv) + 跨 doc link 更 (grep `modules/<arch-name>` 全更). |
| **N1.3 closure** | REG-NAMING1-001..010 (10 反向 grep + 0 行为改 + 字面 byte-identical + caller 跟随真过 + post-#613 haystack gate Func=50/Pkg=70/Total=85 三轨真过 + 既有 test 全 PASS + git history 保 (≥70 similarity rename detection)) + acceptance + 4 件套 spec 第一件 |

## 2. 反向 grep 锚 (10 反约束)

```bash
# 1) milestone-prefix 文件全清 (server-go)
find packages/server-go/internal -name '*.go' | grep -cE '/(al|chn|dm|cv|bpp|hb|rt|ap|adm|cm|cs|dl|infra|test_fix)_[0-9]'  # 0 hit

# 2) Go test 函数 milestone-prefix 0 残留
grep -rE 'func Test(CHN|DM|RT|AL|CV|BPP|HB|AP|ADM|CM|CS|DL)[0-9]+' packages/server-go/  # 0 hit

# 3) struct/handler milestone-prefix 0 残留
grep -rE 'type [A-Z]+[0-9]+[A-Z][a-zA-Z]*Handler' packages/server-go/internal/api/  # 0 hit (DM10PinHandler 类全清)

# 4) migration var 名 milestone-prefix 0 残留
grep -rE 'var (dm|al|chn|cv|bpp|hb|ap|adm|cm)[0-9]+[a-zA-Z]+ = Migration' packages/server-go/internal/migrations/  # 0 hit

# 5) migration Version 字面不动 (反约束守 v 号)
git diff origin/main -- packages/server-go/internal/migrations/ | grep -cE '^\+\s*Version:\s*[0-9]+'  # 0 hit (Version 字面 v=1..v=45 不动)
git diff origin/main -- packages/server-go/internal/migrations/registry.go | grep -cE '^\+|^-' | awk '$0 !~ /var name/'  # ≤2*N (rename only, 顺序不变)

# 6) error code / DM-gate 字面 byte-identical (REFACTOR-1/2 必修条件承袭)
old_dmnotgr=$(git show origin/main:docs/qa/regression-registry.md | grep -c 'layout.dm_not_grouped' || echo 0)
new_dmnotgr=$(grep -rE 'layout\.dm_not_grouped' packages/server-go/internal/api/ docs/ | wc -l)
[ "$new_dmnotgr" -ge "$old_dmnotgr" ]  # 字面 count 不少
grep -rE 'dm\.edit_only_in_dm|pin\.dm_only_path|layout\.dm_not_grouped' packages/server-go/internal/api/*.go | grep -v _test.go | wc -l  # ≥ baseline

# 7) TSX test 命名归一 (反 dot-variant 残留)
find packages/client/src/__tests__ -name '*.test.ts*' | grep -cE '\.[a-z][^/]*-v[0-9]\.test\.|\.[a-z][^/]*\.[a-z][^/]*\.test\.'  # 0 hit (dot-variant 清)

# 8) modules/ 游离 doc 迁 architecture/
test -d docs/architecture/ && [ "$(ls docs/architecture/*.md 2>/dev/null | wc -l)" -ge 11 ]  # 11 架构概览迁入
ls docs/implementation/modules/ | grep -cE '^(admin-model|agent-lifecycle|auth-permissions|canvas-vision|channel-model|client-shape|concept-model|data-layer|host-bridge|plugin-protocol|realtime)\.md$'  # 0 hit (全迁出)

# 9) git rename detection (history 保) ≥70 similarity
gh pr view <N> --json files | jq '[.files[] | select(.status=="renamed")] | length'  # ≥150 (≥169 大致一致)

# 10) post-#613 haystack gate 三轨守 + 既有 test 全 PASS
THRESHOLD_FUNC=50 THRESHOLD_PACKAGE=70 THRESHOLD_TOTAL=85 BUILD_TAGS="sqlite_fts5" go run ./scripts/lib/coverage/  # ALL 三轨 ≥阈值
go test -tags 'sqlite_fts5' -timeout=300s ./...  # ALL PASS
pnpm vitest run --testTimeout=10000  # ALL PASS
```

## 3. 不在范围 (留账)

- ❌ **行为改 / endpoint 加 / schema 改 / migration v 号改** — 0 行为改铁律
- ❌ **REFACTOR-3** (cursor envelope 深化 / messages.go 长函数拆 / store query helper 整合) — 留 REFACTOR-3 议程, 跟本 NAMING-1 不同 concern
- ❌ **acceptance template / regression-registry / spec brief 内 milestone 编号字面** — milestone 上下文真值必留 (e.g. `dm-10-spec.md` / `REG-DM10-001..006` / phase-4.md changelog 字面)
- ❌ **CSS class 名 / DOM data-attr 命名归一** (e.g. `data-cv7-comment-input`) — content-lock 绑, 留 v3+ 议程
- ❌ **DB column 名改** — 0 schema 改铁律
- ✅ NAMING-1 milestone-prefix 全清 (5 类 A-E), 不留 v2

## 4. 跨 milestone byte-identical 锁

- 复用 INFRA-3 #594 git mv rename detection 模式 (PROGRESS 子文件迁同精神)
- 复用 REFACTOR-1 #611 / REFACTOR-2 #613 字面 content-lock 必修条件承袭
- 复用 BPP-3 / reasons.IsValid SSOT 单源精神 (rename 后单源不漂)
- 复用 TEST-FIX-3-COV #612 haystack gate 三轨 (Func=50/Pkg=70/Total=85) — rename 后必过
- 0-行为-改 wrapper 决策树**变体**: 跟 REFACTOR-1/2 / INFRA-3/4 / CV-15 / TEST-FIX-3 同源

## 5. 派活 + 双签

派 **zhanma-c** (REFACTOR-2 #613 主战熟手, 续作减学习成本) 或 zhanma-d. 飞马 review.

双签流程: spec brief → team-lead → 飞马自审 ✅ APPROVED → yema stance + liema acceptance → zhanma 起实施 (N1.1+N1.2+N1.3 三段一 PR, **teamlead 唯一开 PR**).

## 6. 飞马 (架构师) 自审表态

✅ **APPROVED with 2 必修条件**:

🟡 必修-1: **git mv 真走** (跟 INFRA-3 #594 audit 嫌疑承袭) — `gh pr view files | jq '.[] | .status'` 真值 ≥150 "renamed" + similarity ≥70. zhanma 实施 PR body 必示 rename detection 报数.

🟡 必修-2: **migration Version 字面不动 verify** — 反约束 grep #5 真守, `git diff -- migrations/` Version 行 0 hit (rename 只改 var 名 + 文件名, 不改 Version 数值). zhanma PR body 必示 `git diff origin/main -- migrations/ | grep -E 'Version:'` 输出.

担忧 (1 项, 中度):
- 🟡 scope 巨大 (~169 server-go + ~30 client tsx + 11 modules/ 迁 = ~210 文件 touched), PR review 工作量大. 但 git mv + sed 机械 rename 可批量 verify (10 反向 grep + 既有 test 全 PASS + haystack gate 三轨过).

留账接受度: REFACTOR-3 / DOM data-attr / CSS class 全留账, 跟用户铁律 "本 milestone milestone-prefix 全清" 不冲突 (那些是新 audit 范畴).

**ROI 拍**: NAMING-1 ⭐⭐⭐ — 一次性收 milestone-prefix tech debt 169+ 文件, Go 社区命名规范 + git history 保, 不留 v2.
