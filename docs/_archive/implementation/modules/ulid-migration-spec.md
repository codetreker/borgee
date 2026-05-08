# ULID-MIGRATION spec brief — 核心表 PK UUID → ULID (≤80 行)

> 飞马 · 2026-05-01 · 第三方 audit P2 v0→v1 必修兑现 (蓝图 §4.A.1 字面 "ID 方案 = ULID 所有业务表主键, 禁 INTEGER PK")
> **关联**: 蓝图 data-layer.md §4.A.1+§v0 代码债 audit 表 line 238 字面 "ULID 全表 - INTEGER PK 大量使用 - 大改 v1 必修" · DL-2 #615 channel_events / global_events lex_id ULID ✅ · 核心表 (users/channels/messages/admins/agents) 仍 UUID TEXT
> **命名**: ULID-MIGRATION = v0→v1 核心 schema 切 ULID 大 milestone (跟 INFRA-1a #169 schema_migrations 框架同等级 infra wave)

> ⚠️ **大 migration milestone (高风险) ⚠️** — 核心表 PK 类型切 + ~50 callsite uuid.NewString() → ulid.Make() + cross-table FK 跟随 + backfill cron + 既有数据兼容 (TEXT 字段值随 PK 改).
> 一 PR 一 milestone 但 PR 巨大. 反 admin merge bypass — 必走完整 review + cov gate + e2e 全过.

## 0. 关键约束 (3 条立场)

1. **核心表 PK 切 ULID + 既有 UUID 数据 backfill 兼容 + FK 跟随** (蓝图 §4.A.1 字面立场承袭): 
   - **核心 PK 表**: `users` / `channels` / `messages` / `admins` / `agents` / `artifacts` / `iterations` / `anchors` / `host_grants` / 等 ~30 表 PK 改 (TEXT type 不动, 只改值生成器 uuid.NewString() → ulid.Make().String())
   - **既有数据 backfill**: migration v=N+1 复用 ULID 库 (`github.com/oklog/ulid/v2`) — 既有 UUID 不转换 (forward-compat 立场, 新行 ULID + 旧行 UUID 共存; ULID 字典序 > UUID 字典序保单调). 反向断言: 反向 grep `uuid.NewString` in production .go 0 hit (post-rename), `ulid.Make()` 字面 ≥30 hit per migrated table.
   - **FK 跟随**: 跨表 FK 字段值类型不变 (TEXT), 仅插入新行时 FK 引用值是 ULID. 既有 UUID FK 关系不破 (forward-compat).
   - **cursor 协议 byte-identical**: RT-1 #290 cursor `kind+ulid` 协议蓝图 §4.A.4 字面已 ULID, 改 PK 后 cursor 跟 PK 值 monotonic 一致.
   反约束: 反向 grep `uuid\.NewString\(\)` in `internal/api/*.go` `internal/store/*.go` 0 hit (post-migration); `ulid\.Make` 字面 ≥30 hit; FK 字段值类型 TEXT 不动 (反 ALTER COLUMN type).

2. **0 endpoint URL 改 + 0 schema column 名改 + ~50 callsite 字面改 + DB 数据兼容**:
   - **0 endpoint URL 改**: ID 路径参数仍 `{id}` 接受 UUID + ULID 两种字面 (TEXT 字段值 polymorphism)
   - **0 column 名改**: id 字段名仍 `id`, type 仍 TEXT
   - **~50 callsite uuid.NewString() → ulid.Make().String()**: anchors/artifacts/channels/capability_grant/iterations/mention_dispatch/host_grants/admin/users/messages/etc 全改
   - **DB 数据兼容**: 既有 UUID 行不动 (反向 grep 既有 UUID 字面 ≥1 hit per 表, post-migration), 新行 ULID
   反约束: 反向 grep migration v=N+1 仅加 1 行 + Version 字面 byte-identical + 0 ALTER COLUMN type / RENAME column.

3. **post-#621 haystack gate 三轨守 + 既有 test 全 PASS + e2e 跨设备 backfill cron 真测** (大 milestone 风险高, 测试守门必严): cov 三轨 (Func=50/Pkg=70/Total=85) 不破; 既有 ~250+ unit + Playwright e2e 全 PASS post-rename; backfill cron (走 internal/datalayer 既有 sweeper 模式) 真测 既有 UUID 行不丢 + 新 ULID 行真插. 反约束: 0 production 行为分支 add (仅 ID 生成器换); admin merge bypass 0 hit (跟用户铁律永不降覆盖度承袭).

## 1. 拆段实施 (3 段, 一 milestone 一 PR — 大 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| **UM.1 ULID 库 + ID 生成器单源** | `packages/server-go/internal/idgen/idgen.go` 新 (~50 行 NewID() helper 走 ulid.Make() SSOT, 反 inline 字面散落) + go.mod 加 `github.com/oklog/ulid/v2` | 战马 / 飞马 review |
| **UM.2 ~50 callsite 改 + migration v=N+1 backfill cron** | `internal/api/*.go` ~50 callsite `uuid.NewString()` → `idgen.NewID()` (跟 BPP-3 SSOT 模式承袭, 改 = 改一处生成器); migration v=N+1 `ulid_backfill_cron.go` 加 `cron.go` sweeper SSOT (跟 DL-2 retention sweeper 同精神 ctx-aware Start(ctx)); 既有 UUID 行不转换 (forward-compat); ~30 unit test 真测 ULID 字典序 monotonic + 既有 UUID 不破 | 战马 / 飞马 review |
| **UM.3 closure** | REG-ULID-001..015 (15 反向 grep + ID 生成器单源 + 0 endpoint URL 改 + 0 column 名改 + ~50 callsite 真改 + ULID monotonic 真测 + 既有 UUID 兼容 + post-#621 haystack 三轨过 + 既有 test 全 PASS + e2e 跨表 ID 真测) + acceptance + content-lock §1 (ULID 26 字符 + UUID 36 字符 字面锚) + 4 件套 spec 第一件 | 战马 / 烈马 |

## 2. 反向 grep 锚 (15 反约束)

```bash
# 1) ID 生成器单源 (反 uuid.NewString() inline 散落)
grep -rE 'uuid\.NewString\(\)' packages/server-go/internal/api/ packages/server-go/internal/store/ | grep -v _test  # 0 hit (post-rename)
grep -rE 'idgen\.NewID|ulid\.Make' packages/server-go/internal/  | wc -l  # ≥30 hit (per table)

# 2) ULID 库引入
grep -nE 'github.com/oklog/ulid' packages/server-go/go.mod  # ≥1 hit

# 3) idgen.NewID() helper 单源
grep -nE '^func NewID' packages/server-go/internal/idgen/idgen.go  # ==1 hit (SSOT)

# 4) 0 endpoint URL 改
git diff origin/main -- packages/server-go/internal/server/server.go | grep -cE '^\+.*HandleFunc|^\+.*Handle\('  # 0 hit

# 5) 0 column 名改 (TEXT type 不动, 仅值生成器换)
git diff origin/main -- packages/server-go/internal/migrations/ | grep -cE '^\+.*RENAME COLUMN|^\+.*ALTER COLUMN'  # 0 hit

# 6) migration v=N+1 仅加 1 个
ls packages/server-go/internal/migrations/ulid_backfill_*.go  | wc -l  # ==1 hit

# 7) backfill cron ctx-aware (跟 DL-2 sweeper 同精神)
grep -nE 'Start\(ctx\)|ctx\.Done\(\)' packages/server-go/internal/datalayer/ulid_backfill_cron.go  # ≥2 hit

# 8) ULID monotonic 真测
grep -nE 'TestULID.*Monotonic|TestULIDOrder' packages/server-go/internal/idgen/idgen_test.go  # ≥1 hit

# 9-12) 跨表 ID 真测 (~30 表)
for tbl in users channels messages admins agents artifacts iterations anchors host_grants; do
  grep -rE "$tbl.*idgen\.NewID|idgen\.NewID.*$tbl" packages/server-go/internal/api/  | wc -l  # ≥1 hit per table (8 hit total ≥)
done

# 13) 0 ALTER TABLE column type 改 (TEXT 不动)
grep -rE 'ALTER TABLE.*MODIFY COLUMN|ALTER COLUMN.*TYPE' packages/server-go/internal/migrations/  # 0 hit

# 14) admin merge bypass 0 hit (跟用户铁律承袭)
gh pr view <N> --json mergeStateStatus  | jq '.mergeStateStatus'  # not "BYPASSED"

# 15) post-#621 haystack gate + 既有 test
THRESHOLD_FUNC=50 THRESHOLD_PACKAGE=70 THRESHOLD_TOTAL=85 BUILD_TAGS="sqlite_fts5" go run ./scripts/lib/coverage/  # ALL ≥阈值
go test -tags 'sqlite_fts5' -timeout=600s ./... && pnpm vitest run && pnpm exec playwright test  # ALL PASS
```

## 3. 不在范围 (留账)

- ❌ **既有 UUID 行 backfill 转 ULID** — forward-compat 立场, 既有 UUID 字面不动 (反破坏 + 反 ID 引用断)
- ❌ **cursor 协议改** — 蓝图 §4.A.4 已 ULID 字面, RT-1 #290 cursor 不动
- ❌ **DL-2 events lex_id 不动** — DL-2 #615 已 ULID, 不重做
- ❌ **plugin manifest signing key rotation** — 跟 HB-1 #491 ed25519 不撞
- ❌ **Snowflake / KSUID / UUIDv7** — 蓝图 §4.A.1 line 206 字面禁
- ❌ **跨 db cluster ID 协调** — 单机 SQLite ULID 够用, 留 v2+ NATS 切

## 4. 跨 milestone byte-identical 锁

- 蓝图 §4.A.1 ULID lock-in + §4.A.4 cursor lex_id ULID byte-identical
- DL-2 #615 channel_events / global_events lex_id ULID byte-identical 不动
- BPP-3 / reasons.IsValid SSOT 模式 (idgen.NewID 单源 helper)
- DL-2 / DL-3 retention sweeper / ThresholdMonitor / EventsArchiveOffloader ctx-aware 模式承袭
- 0-endpoint-改 wrapper 决策树**变体**: 跟 INFRA-3/4 / REFACTOR-1/2 / NAMING-1 / RT-3 / DL-2/3 / HB-2 v0(D) / WIRE-1 同源
