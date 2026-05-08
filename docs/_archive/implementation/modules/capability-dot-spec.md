# CAPABILITY-DOT spec brief — 14 const snake_case → dot-notation (≤80 行)

> 飞马 · 2026-05-01 · 第三方 audit P1 真 spec drift 兑现 (蓝图字面 dot-notation vs 代码 snake_case)
> **关联**: AP-1 #493 14 capability const · AP-2 #620 LABEL_MAP 14 字面 · AP-4-enum #591 reflect-lint · 蓝图 auth-permissions.md "命名遵循 `<domain>.<verb>` 风格"
> **命名**: CAPABILITY-DOT = AP-1 14 capability rename to blueprint dot-notation (跟 NAMING-1 #614 milestone-prefix 全清同精神)

> ⚠️ Cross-layer rename milestone — **0 行为改 / 0 schema column 名改** (保留 user_permissions.capability TEXT) + ~80 callsite 字符串 byte-identical 字面改 + DB 数据 backfill (UPDATE existing rows 字符串).
> 蓝图字面 14 const: `read_channel` → `channel.read` / `write_channel` → `channel.write` / `mention_user` → `user.mention` / `read_dm` → `dm.read` / etc.

## 0. 关键约束 (3 条立场)

1. **蓝图 auth-permissions.md `<domain>.<verb>` 风格 byte-identical 兑现** (蓝图字面立场承袭): 14 const 字符串值改 (Go const 名保留 ReadChannel / WriteChannel 等 — Go 命名规范不漂; 仅 const 字符串值改 snake_case → dot-notation). 反约束: 反向 grep server `read_channel|write_channel|...` 在 production .go 0 hit (除 migration backfill SQL); reflect-lint AP-4-enum #591 自动跟随 ALL 数组顺序.

2. **DB backfill migration v=N+1 + AP-2 LABEL_MAP rekey + content-lock byte-identical 改 4 处**:
   - **server const**: `internal/auth/capabilities.go` 14 const 字符串值改 (Go const 名不动)
   - **DB backfill**: migration v=N+1 `UPDATE user_permissions SET capability = REPLACE(capability, '_', '.')` 但需逆向: `read_channel` → `channel.read` (字段顺序换), 走 14 行 UPDATE 字面 (不能机械 REPLACE — read_channel 跟 channel.read 字面 verb-noun 顺序对调)
   - **AP-2 LABEL_MAP rekey**: `packages/client/src/lib/capabilities.ts::CAPABILITY_TOKENS` 14 字面改 + `LABEL_MAP` rekey 跟随
   - **AP-2 capability-bundles.ts 3 bundle**: `workspace/reader/mention` 内 capability token 全 rekey
   - **content-lock**: `docs/qa/ap-2-content-lock.md` §1 14 字面改
   反约束: 反向 grep `read_channel|write_channel|delete_channel|read_artifact|write_artifact|commit_artifact|iterate_artifact|rollback_artifact|mention_user|read_dm|send_dm|manage_members|invite_user|change_role` in production 0 hit (post-rename); 新格式 14 字面 ≥14 hit per 文件 (server const + AP-2 LABEL_MAP + content-lock).

3. **0 schema column 名改 + 0 endpoint URL 改 + AP-4-enum reflect-lint byte-identical 不破** (跟 REFACTOR-1/2 / NAMING-1 / RT-3 / DL-2 / HB-2 v0(D) wrapper 立场承袭): PR diff 仅 (a) capabilities.go 14 const 字符串值 (b) 1 migration v=N+1 backfill (c) capabilities.ts + capability-bundles.ts (d) content-lock §1 (e) AP-4-enum reflect-lint 自动验. 反约束: 0 column 名改 (user_permissions.capability TEXT 字段名不动, 只改值) + 0 endpoint URL 改 + 0 routes.go.

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| **CD.1 server const + DB backfill** | `internal/auth/capabilities.go` 14 const 字符串值改 (e.g. `ReadChannel = "channel.read"`) + AP-4-enum reflect-lint 不破; migration v=N+1 `capabilities_dot_notation_backfill.go` 14 行 UPDATE 字面 (per-token 字符串映射, 反 REPLACE 机械) + idempotent guard | 战马 / 飞马 review |
| **CD.2 client + content-lock rekey** | `packages/client/src/lib/capabilities.ts::CAPABILITY_TOKENS` 14 字面改 + `LABEL_MAP` rekey + `capability-bundles.ts` 3 bundle 内 capability rekey + `docs/qa/ap-2-content-lock.md` §1 14 字面改 + AP-2 ap-2-reverse-grep.test.ts 反向 grep 锚改 | 战马 / 飞马 review |
| **CD.3 closure** | REG-CD-001..006 (6 反向 grep + 14 const 字面 byte-identical 跟蓝图 + AP-4-enum reflect-lint 不破 + DB backfill idempotent + 0 schema column 改 + haystack 三轨过 + 既有 test 全 PASS) + acceptance + 4 件套 spec 第一件 | 战马 / 烈马 |

## 2. 反向 grep 锚 (6 反约束)

```bash
# 1) snake_case 14 字面 0 hit (post-rename, 除 migration backfill SQL)
for tok in read_channel write_channel delete_channel read_artifact write_artifact commit_artifact iterate_artifact rollback_artifact mention_user read_dm send_dm manage_members invite_user change_role; do
  grep -rE "$tok" packages/server-go/internal/auth/ packages/client/src/lib/ docs/qa/ap-2-content-lock.md  # 0 hit per token
done

# 2) dot-notation 14 字面真补
for tok in 'channel.read' 'channel.write' 'channel.delete' 'artifact.read' 'artifact.write' 'artifact.commit' 'artifact.iterate' 'artifact.rollback' 'user.mention' 'dm.read' 'dm.send' 'channel.manage_members' 'channel.invite' 'channel.change_role'; do
  grep -rE "\"$tok\"" packages/server-go/internal/auth/ packages/client/src/lib/ docs/qa/  | wc -l  # ≥1 hit per token
done

# 3) AP-4-enum reflect-lint byte-identical 不破
grep -nE 'capabilities_lint_test|TestAP4_' packages/server-go/internal/auth/capabilities_lint_test.go  # ≥1 hit (test 自动跟随 ALL)

# 4) DB migration backfill 真挂
ls packages/server-go/internal/migrations/capabilities_dot_notation_backfill.go  # exists
grep -cE 'UPDATE user_permissions SET capability' packages/server-go/internal/migrations/capabilities_dot_notation_backfill.go  # ≥14 hit (per-token UPDATE)

# 5) user_permissions.capability column 名不改
git diff origin/main -- packages/server-go/internal/migrations/ | grep -cE '^\+.*ALTER TABLE.*capability\|^\+.*RENAME COLUMN'  # 0 hit

# 6) post-#621 haystack gate + 既有 test
THRESHOLD_FUNC=50 THRESHOLD_PACKAGE=70 THRESHOLD_TOTAL=85 BUILD_TAGS="sqlite_fts5" go run ./scripts/lib/coverage/  # ALL ≥阈值
go test -tags 'sqlite_fts5' -timeout=300s ./... && pnpm vitest run  # ALL PASS (含 ap-2-reverse-grep.test.ts post-rekey)
```

## 3. 不在范围 (留账)

- ❌ **新 capability 加** (e.g. `agent.invite` / `audit.read`) — 留 v2+ scope expansion
- ❌ **capability scope grammar 扩展** (currently 14 + scope 二维) — 留 v3+
- ❌ **i18n LABEL_MAP 14 字面 重译** — 中文字面跟当前 byte-identical (反 LABEL_MAP 漂)
- ❌ **AP-1 expires_at 列 backfill** — 留 AP-1.bis 续作

## 4. 跨 milestone byte-identical 锁

- AP-1 #493 14 const 字面立场 (Go const 名不动, 仅字符串值改)
- AP-2 #620 LABEL_MAP / capability-bundles.ts / content-lock byte-identical 跟随 rekey
- AP-4-enum #591 reflect-lint 自动跟随 ALL 数组顺序
- 蓝图 auth-permissions.md `<domain>.<verb>` 风格字面 byte-identical
- ADM-0 §1.3 admin god-mode 路径独立不破
