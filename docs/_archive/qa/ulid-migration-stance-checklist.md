# ULID-MIGRATION stance checklist — uuid → ULID 字面迁移 (server-only)

> 7 立场 byte-identical 跟 ulid-migration-spec.md (飞马待 commit). **真有 prod code (uuid → ULID 真改 + migration 工具脚本) + 0 行为改 / 0 既有 endpoint shape 改 / 0 既有 ACL 改**. 跟 NAMING-1 #614 全清模式 + REFACTOR-1/2 + DL-1 #609 4 interface 同精神承袭. content-lock 不需 (server-only ID format, 0 user-visible 字面改). **scope 全清 — 跟 user 铁律 strict_one_milestone_one_pr 承袭, 反 NAMING-2 留尾教训**.

## 1. ULID 字面 byte-identical (k-sortable lexicographic)
- [ ] 26-char ULID `01ARZ3NDEKTSV4RRFFQ69G5FAV` (Crockford base32, 时间戳前缀 lexicographic sortable)
- [ ] 反向 grep uuid 字面 (`[0-9a-f]{8}-[0-9a-f]{4}-...`) 在新 schema field 0 hit (字面真清)
- [ ] ULID 命名锁 `ULID` byte-identical (反 `KSUID / Snowflake / NanoID / CUID` 同义词漂)

## 2. scope 全清 (一次切完, 不留 v2)
- [ ] 反"留 top-N / 留 v2-migration / 留 ULID-2" 字面 (反 REFACTOR-1 留尾教训, 用户铁律)
- [ ] **黑名单 grep 真测**: 反向 grep `uuid\\.NewString\\(\\)|uuid\\.New\\(\\)` 在 packages/server-go/internal/ 0 hit (uuid 调用真清)
- [ ] 跨表全 ID 字段真切 ULID (channels / messages / artifacts / users / agents / audit_events / ...)

## 3. 0 行为改 (字面迁移 + 0 endpoint shape 改)
- [ ] endpoint shape / response body / error code 字面 byte-identical (仅 ID 字面值改)
- [ ] 既有 unit + e2e byte-identical 不破 (反 race-flake + 反 mask 5 模式)
- [ ] cov ≥85% (#613 gate, user memory `no_lower_test_coverage` 铁律)

## 4. migration 工具真启 (uuid → ULID 双向兼容期)
- [ ] migration 脚本 `internal/migrations/<v>_uuid_to_ulid.go` 真启 (跟 NAMING-1 git mv 同精神承袭)
- [ ] 兼容期: GET endpoint 接受 uuid + ULID 双 format → 写入 ULID (反 break user 既有 bookmarks / share links)
- [ ] migration v 号顺序锚 (post-#612 v 号字面单调递增)

## 5. 跨 milestone 锁链不破 (字面 byte-identical)
- [ ] DM-gate / error code / audit field byte-identical (跟 REFACTOR-1/2 + NAMING-1 字面锁延伸)
- [ ] 5-field audit JSON-line schema `actor / action / target / when / scope` byte-identical 跨六源 (HB-1+HB-2+BPP-4+HB-4+HB-3+ADM-3)
- [ ] anchor #360 owner-only + REG-INV-002 fail-closed + ADM-0 §1.3 红线 byte-identical

## 6. 既有测试 0 改 + 0 race-flake
- [ ] 既有 unit + e2e 0 改测试 body (仅 ID 字面值生成端 mock 适配 ULID)
- [ ] 0 race-flake — 跟 TEST-FIX-1/2/3 + #612/#613 deterministic 协议承袭
- [ ] server-go ./... 全 25+ packages 全绿 (+sqlite_fts5 tag)

## 7. admin god-mode 不挂 ULID migration (ADM-0 §1.3 红线)
- [ ] 反向 grep `admin.*ulid|admin.*migration` 在 packages/server-go/ 0 hit
- [ ] migration 走 system-internal, 反 admin override (anchor #360 立场延伸)

## 反约束 — 真不在范围
- ❌ 改 endpoint shape / response body / error code 字面 (反 0 行为改)
- ❌ KSUID / Snowflake / NanoID / CUID 同义词漂入 (反 ULID 单源)
- ❌ 加新 endpoint / 改既有 endpoint shape / 0 client UI 改
- ❌ 留尾 v2 (用户铁律, 反 REFACTOR-1 留尾教训)
- ❌ admin god-mode 加挂 (永久不挂, ADM-0 §1.3 红线)

## 跨 milestone byte-identical 锁链 (5 链)
- NAMING-1 #614 milestone-prefix 全清模式承袭 (字面 byte-identical, 全清不留 top-N)
- REFACTOR-1 #611 + REFACTOR-2 字面锁延伸 (DM-gate / error code byte-identical 不破)
- DL-1 #609 4 interface byte-identical 不破 (Repository helper baseline N=108)
- 5-field audit schema 跨六源 byte-identical (HB-1+HB-2+BPP-4+HB-4+HB-3+ADM-3)
- anchor #360 owner-only ACL 22+ PRs + ADM-0 §1.3 红线

## PM 拆死决策 (3 段)
- **ULID 全清 vs top-N 拆死** — 一次全切 (本 PR 选, 用户铁律, 反留尾)
- **ULID vs uuid/KSUID/Snowflake/NanoID/CUID 拆死** — ULID byte-identical (本 PR, k-sortable + Crockford), 反同义词漂
- **0 行为改 vs 顺手改 endpoint 拆死** — 仅 ID 字面值改 (本 PR), 反"为绕迁移改 endpoint shape"

## 用户主权红线 (5 项)
- ✅ 0 行为改既有 endpoint (e2e + unit byte-identical 不破)
- ✅ 兼容期 uuid + ULID 双 format read (反 break user bookmarks / share links)
- ✅ 0 既有 ACL 改 (anchor #360 + REG-INV-002 守)
- ✅ 0 user-visible 字面改 (server-only ID format)
- ✅ admin god-mode 不挂 (ADM-0 §1.3 红线)

## PR 出来 5 核对疑点
1. 黑名单 grep `uuid.NewString\\(\\)|uuid.New\\(\\)` 在 internal/ count==0 (uuid 真清)
2. ULID 字面 26-char Crockford base32 byte-identical (反向 grep ≥M hit)
3. 0 endpoint shape / 0 response body 改 (`git diff` 反向断言)
4. migration 兼容期双 format read 真验 (e2e GET uuid + ULID 双 PASS)
5. cov ≥85% (#613 gate) + 0 race-flake + admin grep 0 hit
