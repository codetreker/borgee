# Acceptance Template — REFACTOR-1 (CHN 域 helper 抽取 + DM-gate SSOT, 0 schema / 0 endpoint diff)

> Spec brief `refactor-1-spec.md` (飞马 v0). Owner: 战马C 实施 / 飞马 review / 烈马 验收.
>
> **REFACTOR-1 范围**: CHN 域跨 handler 散落 4 helper 抽取到 `internal/api/chn/` 子包, DM-gate 字面 13 处现 inline `channel.Type != "dm"` 检查走 helper SSOT (反约束: 字面行为 byte-identical, 错码字面不变, 0 schema / 0 endpoint / 0 client diff). 立场承袭 BPP-2 7-op + REFACTOR-REASONS 6-dict + DM-9 EmojiPreset + CHN-15 ReadonlyBit + AP-4-enum + DL-1 LastReadAtRepository 跨七+ milestone const SSOT 锁链.

## 验收清单

### §1 数据契约 — 4 helper 抽取 + DM-gate SSOT + 0 diff

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 4 helper 抽取到 `internal/api/chn/helpers.go` 单源 (各 helper 字面 byte-identical 跟原 inline 实现) — `gateDM(channel)` (DM-only path 守门) + `gateChannelMember(ctx, userID, channelID)` (channel-member ACL) + `gateChannelAdmin(ctx, userID, channelID)` (channel admin role) + `gateChannelReadonly(channel)` (readonly bit) | unit + grep | 战马C / 烈马 | `TestREFACTOR1_4HelperByteIdentical` (旧 inline 实现 vs 新 helper 各 truth-table 对比 byte-identical 锁) PASS |
| 1.2 DM-gate 字面 13 处现 inline `channel.Type != "dm"` 替换走 `chn.GateDM(channel)` 单源 — before/after 字面命中数 byte-identical (before=13, after=0 inline + 13 helper call site) | grep | 战马C / 飞马 / 烈马 | `git grep -cE 'channel\\.Type\\s*!=\\s*"dm"' packages/server-go/internal/api/` before=13, after=0 (除 chn/helpers.go 单源定义 1 hit) + `git grep -cE 'chn\\.GateDM\\(' packages/server-go/internal/api/` after=13 |
| 1.3 反约束 0 schema / 0 endpoint / 0 client diff — `git diff main -- internal/migrations/ packages/client/` 0 production lines + endpoint registration 字面 byte-identical (反约束 helper 抽取不动 endpoint shape) | git diff | 飞马 / 烈马 | `git diff main -- packages/server-go/internal/migrations/` 0 行 + `git diff main -- packages/client/` 0 行 + endpoint list reflect 字面 byte-identical |

### §2 行为不变量 — 4 helper 替前 byte-identical + 既有 unit/e2e 全 PASS + 错码字面不变

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 4 helper 替前 byte-identical — 13 个 call site replace 后, 各 endpoint response (200/400/403/404 path) JSON deep-equal 跟 main pre-refactor 对比锁 | integration | 战马C / 烈马 | `internal/api/refactor_1_byte_identical_test.go::TestREFACTOR1_HelperReplaceByteIdentical` (13 endpoint × happy/reject path, response JSON deep-equal 锁) PASS |
| 2.2 错码字面 byte-identical 不变 — `dm.edit_only_in_dm` / `pin.dm_only_path` / `dm_search.q_required` / `chn.readonly` / `chn.member_only` / `chn.admin_only` 等错码字面 0 改 (反约束 helper 抽取不动 error code body) | grep + unit | 飞马 / 烈马 | `git grep -cE '"dm\\.edit_only_in_dm"\\|"pin\\.dm_only_path"\\|"dm_search\\.q_required"\\|"chn\\.readonly"\\|"chn\\.member_only"\\|"chn\\.admin_only"' packages/server-go/internal/api/` before vs after 字面 hits byte-identical |
| 2.3 既有 server-go ./... + client vitest 全绿不破 (Wrapper 立场 — REFACTOR-1 是抽取不是行为改) | full test | 战马C / 烈马 | `go test -tags sqlite_fts5 -timeout=180s ./...` 25+ packages 全 PASS + `pnpm exec vitest run --testTimeout=10000` 全 PASS |
| 2.4 既有 e2e 不破 — `pnpm exec playwright test --timeout=30000` 全 PASS (反约束 helper 抽取不动 endpoint 行为) | E2E | 战马C / 烈马 | playwright 全 PASS |

### §3 蓝图行为对照 — DM-only path 立场 + 跨七 milestone const SSOT 锁链

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 DM-only path 立场承袭 (跟 dm_4_message_edit.go #549 / dm_10_pin.go #597 / dm_11_search.go #600 / dm-12 reaction picker #603 跨四 milestone DM-only 立场承袭) — REFACTOR-1 抽取后立场字面 byte-identical | inspect + grep | 飞马 / 烈马 | spec §0 立场 ① 字面 + DM-only path 4 处 byte-identical 现 chn.GateDM SSOT |
| 3.2 channel-member ACL helper 立场承袭 (跟 AP-4 #551 + AP-5 #555 + DM-10/11 channel-member ACL helper 复用立场承袭) — REFACTOR-1 把 `gateChannelMember` 抽到 chn 子包, 反平行实施 | grep | 飞马 / 烈马 | 反向 grep `IsChannelMember\\|CanAccessChannel` body 在 internal/api/ 除 chn/ 0 hit (helper 单源闸) |
| 3.3 跨 milestone const SSOT 锁链承袭 (跟 BPP-2 7-op + REFACTOR-REASONS 6-dict + DM-9 EmojiPreset + CHN-15 ReadonlyBit + AP-4-enum + DL-1 LastReadAtRepository 同精神) — REFACTOR-1 是第 N 处 SSOT 锁链延伸 | inspect | 飞马 / 烈马 | spec §0 立场 ② 字面 + 锁链编号 |

### §4 反向断言 — drift 守门 + CI step + 反平行实施

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 反向 grep DM-gate 字面 13 hits before/after byte-identical (before=13 inline, after=0 inline + 13 chn.GateDM call site) — CI step `refactor-1-dm-gate-ssot` 守门 | CI grep | 飞马 / 烈马 | `release-gate.yml::refactor-1-dm-gate-ssot` step + run PASS (`git grep -cE 'channel\\.Type\\s*!=\\s*"dm"' packages/server-go/internal/api/ -- ':!*chn/helpers.go'` count==0) |
| 4.2 反平行 helper 实施 — 反向 grep `func gateDM\\|func gateChannelMember\\|func gateChannelAdmin\\|func gateChannelReadonly` 在 internal/api/ 除 chn/ count==0 (单一 SSOT, 反平行漂) | CI grep | 飞马 / 烈马 | reverse grep test |
| 4.3 反 admin god-mode 直 bypass helper — 反向 grep `admin.*chn\\.GateDM\\|admin.*chn\\.GateChannelMember` 在 internal/api/admin*.go count==0 (admin 走 /admin-api/* 单独路径, ADM-0 §1.3 红线承袭) | CI grep | 飞马 / 烈马 | reverse grep test |
| 4.4 0 production behavior 改 — `git diff main -- internal/migrations/ packages/client/ internal/server/` 0 行 (除 server.go 加 chn 包注册 1-2 行 wire-up) + 反向断 endpoint shape 字面 byte-identical | git diff | 飞马 / 烈马 | git diff stat verify |

## REG-REFACTOR1-* 占号 (initial ⚪)

- REG-REFACTOR1-001 ⚪ 4 helper 抽取到 internal/api/chn/helpers.go 单源 (gateDM / gateChannelMember / gateChannelAdmin / gateChannelReadonly truth-table byte-identical)
- REG-REFACTOR1-002 ⚪ DM-gate 字面 13 hits before/after byte-identical (before=13 inline, after=0 inline + 13 chn.GateDM call site) + CI step `refactor-1-dm-gate-ssot` 守门
- REG-REFACTOR1-003 ⚪ 4 helper 替前 byte-identical (13 endpoint × happy/reject path response JSON deep-equal 锁) + 错码字面 byte-identical 不变
- REG-REFACTOR1-004 ⚪ 既有 server-go ./... + client vitest + e2e 全绿不破 + 反平行 helper 实施 (gateDM/gateChannelMember/gateChannelAdmin/gateChannelReadonly 在 internal/api/ 除 chn/ 0 hit)
- REG-REFACTOR1-005 ⚪ 0 schema / 0 endpoint / 0 client diff (`git diff main` 0 行) + 反 admin god-mode 直 bypass (ADM-0 §1.3)
- REG-REFACTOR1-006 ⚪ 跨 milestone const SSOT 锁链承袭 (BPP-2 + REFACTOR-REASONS + DM-9 + CHN-15 + AP-4-enum + DL-1 + REFACTOR-1 跨八 milestone) + DM-only path 立场承袭 (dm-4/dm-10/dm-11/dm-12 跨四 milestone)

## 边界

- BPP-2 #485 7-op 白名单 const SSOT (跨层锁源)
- REFACTOR-REASONS 6-dict const SSOT (反平行立场源)
- DM-9 #585 EmojiPreset const SSOT (单源闸 立场承袭)
- CHN-15 #587 ReadonlyBit const SSOT
- AP-4-enum #591 reaction enum const SSOT
- DL-1 LastReadAtRepository interface 抽象 (跟 REFACTOR-1 同期推 SSOT 立场)
- AP-4 #551 + AP-5 #555 channel-member ACL helper (REFACTOR-1 把 IsChannelMember + CanAccessChannel 抽到 chn 子包反平行)
- dm_4_message_edit.go #549 / dm_10_pin.go #597 / dm_11_search.go #600 / dm-12 #603 DM-only path 立场承袭跨四 milestone
- ADM-0 §1.3 admin god-mode 红线 (admin 不直 bypass helper)
- 0 schema / 0 endpoint / 0 client diff Wrapper 立场 (REFACTOR-1 是 server 内部抽取不是行为改)

## 退出条件

- §1 (3) + §2 (4) + §3 (3) + §4 (4) 全绿 — 一票否决
- 4 helper 抽取到 internal/api/chn/helpers.go 单源 (truth-table byte-identical)
- DM-gate 字面 13 hits before/after byte-identical (before=13 inline, after=0 inline + 13 helper call site)
- 4 helper 替前 byte-identical (13 endpoint × happy/reject path response JSON deep-equal)
- 错码字面 byte-identical 不变 (`dm.edit_only_in_dm` / `pin.dm_only_path` / `dm_search.q_required` / `chn.readonly` / `chn.member_only` / `chn.admin_only` 6 字面)
- 既有 server-go ./... + client vitest + e2e 全绿不破
- 0 schema / 0 endpoint / 0 client diff (`git diff main` 0 行)
- CI step `refactor-1-dm-gate-ssot` 守门
- 反平行 helper 实施 + 反 admin god-mode 直 bypass
- 登记 REG-REFACTOR1-001..006

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-30 | 烈马 | v0 — acceptance template 草稿 (4 选 1 验收框架 + REG-REFACTOR1-001..006 6 行占号 ⚪). 战马C worktree base 1df15ddd, 飞马 spec brief v0 待落, 实施 PR 出来时直接验. **抽取立场关键**: 0 production behavior 改 (helper 抽取不是行为改) + DM-gate 字面 13 hits before/after byte-identical (反向 grep 守门) + 错码字面 byte-identical 不变 (反约束 helper 抽取不动 error code body). 跨 milestone byte-identical 锁链: const SSOT 立场承袭跨八 milestone (BPP-2 + REFACTOR-REASONS + DM-9 + CHN-15 + AP-4-enum + DL-1 + REFACTOR-1) + DM-only path 立场承袭跨四 milestone (dm-4/dm-10/dm-11/dm-12) + AP-4/AP-5 channel-member ACL helper 复用 + ADM-0 §1.3 admin god-mode 红线. |
