# NAMING-1 stance checklist — milestone-prefix 全去 + 路径迁移 (rename only)

> 7 立场 byte-identical 跟 naming-1-spec.md §0+§2. **真有 git mv (path rename + symbol rename) 但 0 行为改 / 0 schema / 0 endpoint / 0 user-facing change / 0 测试 body 改**. 跟 REFACTOR-1 #611 + REFACTOR-2 + DL-1 #609 + INFRA-3 #594 真有 prod refactor 类别同模式. content-lock 不需 (server / TSX 0 user-visible 字面改). **scope 全清 — REFACTOR-1 留尾教训承袭, 所有 milestone-prefix 一次全去不挑 top-N (用户铁律)**.

## 1. 0 行为改 (rename only)
- [ ] 0 endpoint shape 改 — `git diff origin/main -- internal/api/server.go | grep -E '\\+.*Method|\\+.*Register'` 0 hit
- [ ] 0 response body / 0 error code 字面改 — 既有错码 (`dm.*`/`pin.*`/`chn.*`/`auth.*`) before/after byte-identical
- [ ] 0 helper / handler 行为改 (REFACTOR-2 已收, 反"rename 顺手改" — 反 REFACTOR-3 提前)
- [ ] 既有 unit + e2e 全 PASS byte-identical (反 race-flake, 跟 #612/#613 cov 85% 协议承袭)

## 2. 0 schema / 0 migration v 号 (文件名改 v 号字面不动)
- [ ] migration 文件名 `al_*_*.go` → 新命名规范 (但 v 号字面 byte-identical 不动)
- [ ] `currentSchemaVersion` 不动 + schema version 字面 byte-identical
- [ ] `git diff origin/main -- internal/migrations/` 仅文件名改 + 内容 v 号字面不动 (反向断言)
- [ ] 反向 grep `\\+.*currentSchemaVersion =` 0 hit

## 3. scope 全清 (所有 milestone-prefix 一次全去, 不挑 top-N)
- [ ] 反 "留 top-N / 留 NAMING-2" 字面 (反 REFACTOR-1 留尾教训, 用户铁律)
- [ ] **黑名单 grep 真测**: `find packages/server-go -name 'al_*.go' -o -name 'chn_*.go' -o -name 'dm_*.go' -o -name 'rt_*.go' -o -name 'hb_*.go' -o -name 'ap_*.go' -o -name 'cv_*.go' -o -name 'adm_*.go' -o -name 'cm_*.go' -o -name 'cs_*.go' -o -name 'dl_*.go' -o -name 'infra_*.go'` 应 0
- [ ] **测试函数前缀全去**: `grep -rE '^func Test(CHN|DM|RT|HB|AP|AL|CV|ADM|CM|CS|DL|INFRA)[0-9]_' packages/server-go/` 应 0
- [ ] 跟 user memory `strict_one_milestone_one_pr` + `progress_must_be_accurate` 铁律承袭
- [ ] PR description 反向断言无 "留 v2 / 留 NAMING-2 / 留 follow-up" 字面

## 4. caller 全跟随 (gofmt / goimports 验 0 broken import)
- [ ] `gofmt -l packages/server-go/` 0 hit (rename 后格式 byte-identical)
- [ ] `goimports -l packages/server-go/` 0 hit (import 路径全跟随 git mv)
- [ ] `go build ./...` PASS (0 broken import)
- [ ] 0 dangling reference (反向 grep 旧路径 `internal/migrations/al_1a_` 等 0 hit)

## 5. 既有测试全 PASS (0 改测试 body / 0 race-flake)
- [ ] 0 改测试 body — git mv 仅 path rename + 函数名 rename, 测试逻辑 byte-identical (反"顺手改测试")
- [ ] 0 race-flake — 跟 TEST-FIX-2 #608 + TEST-FIX-3 #610 + #612/#613 deterministic 协议承袭 (反 mask)
- [ ] **cov ≥85% post-#613 gate 真过** (#613 cov 阈值 85% 协议, user memory `no_lower_test_coverage` 铁律)
- [ ] server-go ./... 全 25+ packages 全绿 (+sqlite_fts5 tag) 含 CHN-* / DM-* / AL-* / HB-* 既有验证

## 6. 跨 milestone 锁链不破 (字面 byte-identical)
- [ ] **DM-gate 字面 byte-identical** — `DM 不参与个人分组` / `dm.edit_only_in_dm` / `metadata.target` / REFACTOR-2 三错码统一 v0 选定字面 byte-identical 不漂
- [ ] **error code 字面 byte-identical** — `dm.*` / `pin.*` / `chn.*` / `dm_search.*` / `auth.*` / `layout.*` 跨已合 milestone 不漂
- [ ] **audit field 字面 byte-identical** — `actor / action / target / when / scope` 5-field SSOT 跨 HB-1 + HB-2 + BPP-4 + HB-4 + HB-3 byte-identical (改一处 = 改五处单测锁)
- [ ] **CHN-* / DM-* / AL-* / HB-* 立场承袭** — 既有锁链不破, anchor #360 owner-only + REG-INV-002 + ADM-0 §1.3 红线 byte-identical

## 7. history 保留 (git mv 不重写)
- [ ] **`git mv` 真用** — 反 `rm + add` 重写 (反 git history 断链)
- [ ] `git log --follow <newpath>` 应跨 rename 边界连续可见
- [ ] PR commit 用 `git mv` 单步 + caller 跟随单独 commit (反 squash merge 模糊 rename 边界, 建议 merge commit 保 rename 步骤)

## 反约束 — 真不在范围
- ❌ helper / handler 行为改 (REFACTOR-2 已收, REFACTOR-3 留)
- ❌ 文档内容改 (只迁路径 `modules/` → `architecture/`, 内容 byte-identical)
- ❌ 0 schema 字段改 / 0 migration v 号改 / 0 endpoint shape 改 / 0 client UI 改
- ❌ 加新 CI step (跟 REFACTOR-1/2 + INFRA-3 + TEST-FIX-* 同精神)
- ❌ 引入新 milestone-prefix 为 namespace conflict (反 scope 全清)
- ❌ 留尾 top-N (用户铁律 REFACTOR-1 教训)
- ❌ admin god-mode 加挂 / 改 (永久不挂, ADM-0 §1.3 红线)

## 跨 milestone byte-identical 锁链 (5 链)
- **REFACTOR-1 #611 + REFACTOR-2 字面锁延伸** — DM-gate 字面 + 错码字面 byte-identical, NAMING-1 仅 path/symbol rename, 字面值 0 漂
- **DL-1 #609 4 interface 同精神** — interface name (Storage/PresenceStore/EventBus/Repository) byte-identical 不动, 仅可能 path 迁
- **#612/#613 cov 85% deterministic 协议** — NAMING-1 测试 body 0 改, cov 真守 ≥85%
- **anchor #360 owner-only ACL 锁链 22+ PRs** — NAMING-1 ACL helper 字面 + 行为 byte-identical 不破
- **5-field audit JSON-line schema 锁链** — `actor/action/target/when/scope` 跨 HB-1/HB-2/BPP-4/HB-4/HB-3 byte-identical 不动

## PM 拆死决策 (3 段)
- **NAMING-1 全清 vs top-N 拆死** — 一次全去 (本 PR 选, 用户铁律, 反 REFACTOR-1 留尾教训)
- **rename only vs REFACTOR-3 行为改拆死** — git mv + caller 跟随 (本 PR 选), 行为改留 REFACTOR-3 (反"rename 顺手改")
- **git mv vs rm+add 拆死** — git mv 保 history (本 PR 选), 反 rm+add 断链

## 用户主权红线 (5 项)
- ✅ 0 行为改 (e2e + unit 全 PASS byte-identical)
- ✅ 既有 ACL gate 字面 + 行为 byte-identical (anchor #360 + REG-INV-002 守)
- ✅ 0 user-facing change (server / TSX 0 user-visible 字面改)
- ✅ 0 schema 字段 / 0 endpoint shape / 0 error code 字面改
- ✅ admin god-mode 不挂 / 不改 (ADM-0 §1.3 红线 + PR #571 §2 ⑥ 精神延伸)

## PR 出来 5 核对疑点
1. 黑名单 grep `find ... -name 'al_*.go'` 等 count==0 + `^func Test(CHN|DM|...)[0-9]_` count==0
2. 0 schema / 0 endpoint / migration v 号字面 byte-identical (`git diff` 反向断言)
3. 既有 unit + e2e 全 PASS + cov ≥85% (#612/#613 协议 + gofmt/goimports 0 hit)
4. DM-gate / error code / audit field 字面 byte-identical (跨已合 milestone grep count baseline 等同)
5. `git log --follow` 跨 rename 边界连续 (history 保留真验)
