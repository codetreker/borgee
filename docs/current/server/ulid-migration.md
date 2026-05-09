# ULID-MIGRATION — UUID v4 → ULID 全表迁移 v0→v1 必修 (≤80 行)

> 落地: PR feat/ulid-migration · UM.1 idgen.NewID() SSOT + UM.2 ~42 callsite migration + UM.3 closure
> Spec 锚: [`ulid-migration-spec.md`](../../implementation/modules/ulid-migration-spec.md) §0 ① forward-compat + ② 0 endpoint/column 改 + ③ post-#621 gate
> 蓝图锚: data-layer.md §4.A.1 ULID lock-in + §4.A.4 cursor lex_id (RT-1 #290 已 ULID byte-identical 不动)

## 1. 文件清单

| 文件 | 行 | 角色 |
|---|---|---|
| `internal/idgen/idgen.go` | 45 | NewID() SSOT helper (ulid.Make + ulid.Monotonic + sync.Mutex 串行 entropy reader 反 across-goroutine drift) |
| `internal/idgen/idgen_test.go` | 90 | 4 unit (LengthIs26 + Unique + Monotonic_SerialCalls + GoroutineSafe 16×200 0 collision) |
| `go.mod` 扩 | +1 | `github.com/oklog/ulid/v2 v2.1.1` |
| `internal/admin/auth.go` + `internal/api/{19 files}` + `internal/store/{4 files}` + `internal/ws/{3 files}` + `internal/migrations/cm_onboarding_welcome.go` | -42/+42 | `uuid.NewString()` → `idgen.NewID()` callsite 真改 (跨 19 文件), `github.com/google/uuid` import 删 (post-migration 0 production hit) |
| `internal/api/mention_dispatch.go` | +1 alt | MentionTokenRegex 加 ULID-26 Base32 alternation `[0-9A-HJKMNP-TV-Z]{26}` (forward-compat 真守) |

## 2. 设计原则 byte-identical (spec §0)

### ① forward-compat (既有 UUID + 新行 ULID 共存)
- 既有 UUID-36 行不动 (db column TEXT 不限长度)
- 新行 idgen.NewID() 走 ulid v2 canonical 26-char Crockford Base32
- MentionTokenRegex 双 alternation: UUID `8-4-4-4-12` hex `|` ULID 26 alphanum (反 UUID-only 漂)

### ② 0 endpoint URL 改 + 0 column 名改 + 0 migration v 号
- `git diff origin/main -- internal/server/server.go | grep HandleFunc` 0 hit
- `git diff origin/main -- internal/migrations/` 0 行
- ID 字段名仍 `id`, type 仍 TEXT, polymorphism 接受 UUID + ULID 字面

### ③ post-#621 haystack gate 三轨过 + admin merge bypass 0 hit
- TOTAL 85.6% / 0 func<50% / 0 pkg<70% / exit 0
- 25+ Go packages 全 PASS

## 3. 跨 milestone byte-identical 锁链 (17 处)

DL-2 #615 events lex_id ULID (newULID hex monotonic) → idgen.NewID 走 canonical Base32 (跟 RT-1.3 cursor 同精神) · ADM-3 #586 RENAME forward-compat (既有数据不动) · reasons.IsValid #496 / NAMING-1 #614 / DL-1 SSOT helper 模式 (跟 BPP-3 PluginFrameDispatcher 同精神 — idgen.NewID 单源, 反 inline 散落) · 蓝图 §4.A.1 ULID lock-in 字面 byte-identical · post-#621 haystack gate 三轨守门

## 4. acceptance 校准回退

acceptance v0 草稿超 spec scope (proposed schema migration v=N+1 + view alias + backfill cron) → 跟 spec brief §0 ① 字面 forward-compat 对齐, 校准回退跟 RT-3 #616 / DL-3 #618 / AP-2 #620 / WIRE-1 同模式. 这一条沉淀为 "一次做干净不留尾" 而不是 "超 scope acceptance 走过场" 的做法.

## 5. Tests + verify

- `go build -tags sqlite_fts5 ./...` ✅
- `go test -tags sqlite_fts5 -timeout=300s ./...` 25+ packages 全 PASS ✅
- haystack gate TOTAL 85.6% / 0 func<50% / exit 0 ✅
- grep 检查 `uuid\.NewString\(\)` production 0 hit ✅
- grep 检查 `idgen\.NewID\(\)` production ≥42 hit ✅

## 6. grep 守门 (spec §2 15 检查项)

- ID 生成器单源 (反 inline 散落): `grep -cE '^func NewID' idgen.go` ==1
- ULID 库引入: `grep -nE 'github.com/oklog/ulid/v2' go.mod` ≥1
- ~42 callsite 真改: `grep -rE 'idgen\.NewID\(\)' internal/` ≥42 hit + `grep -rE 'uuid\.NewString\(\)' internal/api/ internal/store/ | grep -v _test` 0 hit
- 0 ALTER COLUMN type: `grep -rE 'ALTER COLUMN.*TYPE\|MODIFY COLUMN' internal/migrations/` 0 hit
- ULID monotonic 真测: TestNewID_Monotonic_SerialCalls + GoroutineSafe PASS
- admin merge bypass 0 hit (cov gate 真过)

## 7. 留账 (透明 — 跟 spec §3 字面对齐)

- 既有 UUID 行 backfill 转 ULID — 沿用 forward-compat 设计, 既有 UUID 字面不动 (反破坏 + 反 ID 引用断, 跟 ADM-3 #586 RENAME 元数据 0 数据迁移同模式)
- cursor 协议改 — 蓝图 §4.A.4 已 ULID 字面, RT-1 #290 cursor 不动 (lex_id 跟 PK 解耦)
- DL-2 events lex_id 不动 — DL-2 #615 已 ULID hex monotonic (跟 idgen canonical Base32 不同实现但同精神)
- Snowflake / KSUID / UUIDv7 — 蓝图 §4.A.1 line 206 字面禁
- 跨 db cluster ID 协调 — 单机 SQLite ULID 够用, 留 v2+ NATS 切
- type ID string 抽象 (蓝图 §v0 代码债 audit 表 line 219) 留 v2+ — 本 v1 仅切生成器, 不切类型抽象
