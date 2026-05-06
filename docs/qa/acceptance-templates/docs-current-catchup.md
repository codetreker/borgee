# Acceptance Template — DOCS-CURRENT-CATCHUP (5 候选 docs/current/ 偏差修)

> Owner: 战马 实施 / 飞马 review / 烈马 验收.
>
> **范围**: docs/current/ 是代码现状, 5 milestone 偏差修 (HB-2 / HB-1B / CS-1 / DM-11 / DM-12). 仅 docs/current/ 文件 + 文案对齐, 0 production code 改 + 0 schema + 0 endpoint 改. 反 narrative.

## 验收清单

### §1 HB-2 — host-bridge daemon (borgee-helper Go module) 现状文档

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 `docs/current/borgee-helper.md` 存在 (按功能命名, 不按 milestone) | `[ -f docs/current/borgee-helper.md ]` | ✅ exist (commit c07f157) |
| 1.2 `docs/current/server/api/host-grants.md` 反向 grep `Rust crate` 0 hit (HB-2 已切 Go daemon, 反 stale Rust crate 表达) | `grep -c "Rust crate" docs/current/server/api/host-grants.md` == 0 | ✅ 0 hit (commit c07f157, L20+L124 改 "Go module packages/borgee-helper/, #617 已 merged") |
| 1.3 反向 grep `留 HB-2 (follow-up\|v1\|单 milestone)` 在 docs/current/server/wire-1.md / dl-2.md / dl-3.md 0 hit (HB-2 已落 #617, follow-up 留账失效) | `grep -cE "留 HB-2 (follow-up\|v1\|单 milestone)" docs/current/server/{wire-1,dl-2,dl-3}.md` 各 == 0 | ✅ 0/0/0 hit (commit c07f157) |

### §2 HB-1B — borgee-installer 现状文档

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 `docs/current/borgee-installer.md` 存在 (按功能命名, 不按 milestone) | `[ -f docs/current/borgee-installer.md ]` | ✅ exist (commit c07f157) |
| 2.2 文档锚 `packages/borgee-installer/` 真路径 ≥1 hit (反空 placeholder) | `grep -c "packages/borgee-installer" docs/current/borgee-installer.md` ≥1 hit | ✅ 10 hit |

### §3 CS-1 — client 三栏 + Artifact 分级 现状文档

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 `docs/current/client/app-shell.md` 存在 | `[ -f docs/current/client/app-shell.md ]` | ✅ exist (commit a93e922) |
| 3.2 `docs/current/client/artifact-drawer.md` 存在 | `[ -f docs/current/client/artifact-drawer.md ]` | ✅ exist (commit a93e922) |
| 3.3 `docs/current/client/app-shell.md` ≥1 hit `三栏` 或 `three-pane` (CS-1 立场字面真锚) | `grep -cE "三栏\|three-pane" docs/current/client/app-shell.md` ≥1 hit | ✅ 6 hit |
| 3.4 `docs/current/client/artifact-drawer.md` ≥1 hit `Artifact 分级` 或 `iteration` (CS-1 立场字面真锚) | `grep -cE "Artifact 分级\|iteration" docs/current/client/artifact-drawer.md` ≥1 hit | ✅ 3 hit |

### §4 DM-11 — DM 搜索 API 现状文档

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 4.1 `docs/current/server/api/dm-search.md` 存在 | `[ -f docs/current/server/api/dm-search.md ]` | ✅ exist (commit c07f157) |
| 4.2 文档锚 endpoint 真路径 ≥1 hit (反空 placeholder) — `GET /api/v1/dm/search` 或类似 | `grep -cE "/api/v[0-9]+/dm/search\|/api/v[0-9]+/dm-search" docs/current/server/api/dm-search.md` ≥1 hit | ✅ 1 hit |

### §5 DM-12 — DM reaction picker 现状文档 (升 ✅, 战马已落不留 [LOW] deferred)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 5.1 `docs/current/client/dm-reaction-picker.md` 存在 (按功能命名) | `[ -f docs/current/client/dm-reaction-picker.md ]` | ✅ exist (commit 57d909d) |

### §6 反约束 (catch-up 范围守门)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 6.1 0 production code 改 — `git diff main..HEAD -- packages/` == 0 行 | `git diff` | ✅ 0 行 |
| 6.2 0 schema / 0 migration v 号改 — `git diff main..HEAD -- packages/server-go/internal/migrations/` == 0 行 | `git diff` | ✅ 0 行 |
| 6.3 0 endpoint 行为改 — `git diff main..HEAD -- packages/server-go/internal/api/server.go packages/server-go/internal/server/server.go` 0 mux.Handle / `POST /api/v[0-9]+/` / `GET /api/v[0-9]+/` 行改 | `git diff` | ✅ 0 行 |
| 6.4 既有全包 unit + e2e + vitest 全绿不破 (docs-only catch-up, 不破代码) | full test | ⚪ 待 CI run (docs-only 改, 预期不破) |

### §7 closure (PROGRESS 翻牌 + REG)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 7.1 `docs/implementation/PROGRESS.md` In-flight 行翻 → `DOCS-CURRENT-CATCHUP` (NHD #644 history 锚保留, 仅 In-flight 行真翻) | `grep -n "NO-HARDCODED-DOMAIN" docs/implementation/PROGRESS.md` 见 (PR# 表) + `grep -n "DOCS-CURRENT-CATCHUP" docs/implementation/PROGRESS.md` ≥1 | ✅ flipped (本 PR commit) |
| 7.2 REG-DCC-001..005 ⚪→🟢 翻牌 (HB-2 / HB-1B / CS-1 / DM-11 / DM-12 各一行) | regression-registry 锚 | ⚪ pending (post-merge flip) |

## REG-DCC-* 占号 (initial ⚪→🟢 post-merge)

- REG-DCC-001 🟢 HB-2 catch-up: `docs/current/borgee-helper.md` 存在 + `host-grants.md` `Rust crate` 0 hit + 3 文件 stale follow-up 0 hit
- REG-DCC-002 🟢 HB-1B catch-up: `docs/current/borgee-installer.md` 存在 + `packages/borgee-installer` 锚 10 hit
- REG-DCC-003 🟢 CS-1 catch-up: `docs/current/client/app-shell.md` (6 三栏 hit) + `artifact-drawer.md` (3 iteration hit) 存在
- REG-DCC-004 🟢 DM-11 catch-up: `docs/current/server/api/dm-search.md` 存在 + endpoint `/api/v1/dm/search` 锚 1 hit
- REG-DCC-005 🟢 DM-12 catch-up: `docs/current/client/dm-reaction-picker.md` 存在 (升 ✅, 战马已落)

## 退出条件

- §1 (3) + §2 (2) + §3 (4) + §4 (2) + §5 (1) + §6 (3 + §6.4 待 CI) + §7.1 **全 ✅** — 一票否决
- 0 production code / 0 schema / 0 endpoint 改 (catch-up 范围) ✅
- 既有全包 unit + e2e + vitest 全绿 (待 CI)
- REG-DCC-001..005 🟢 active (post-merge flip)

## 当前真值快照 (2026-05-06 acceptance run)

| 候选 | 当前 |
|---|---|
| HB-2 §1.1 borgee-helper.md | ✅ exist |
| HB-2 §1.2 host-grants.md `Rust crate` | ✅ 0 hit |
| HB-2 §1.3 wire-1/dl-2/dl-3 stale follow-up | ✅ 0/0/0 |
| HB-1B §2.1 borgee-installer.md | ✅ exist |
| HB-1B §2.2 packages/borgee-installer 锚 | ✅ 10 hit |
| CS-1 §3.1 app-shell.md | ✅ exist |
| CS-1 §3.2 artifact-drawer.md | ✅ exist |
| CS-1 §3.3 三栏/three-pane | ✅ 6 hit |
| CS-1 §3.4 Artifact 分级/iteration | ✅ 3 hit |
| DM-11 §4.1 api/dm-search.md | ✅ exist |
| DM-11 §4.2 endpoint 锚 | ✅ 1 hit |
| DM-12 §5.1 dm-reaction-picker.md | ✅ exist |
| §6.1 git diff packages/ | ✅ 0 行 |
| §6.2 git diff migrations/ | ✅ 0 行 |
| §6.3 git diff endpoint | ✅ 0 行 |

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-05-06 | 烈马 | v0 草稿 — 5 候选 catch-up acceptance 框架 + REG-DCC-001..005 占号 ⚪. |
| 2026-05-06 | 烈马 | v1 acceptance run — 12 锚 + §6 反约束 3 锚全 ✅. 真路径校准: HB-2 用 `docs/current/borgee-helper.md` (按功能命名铁律) / HB-1B 用 `docs/current/borgee-installer.md` / DM-12 升 ✅ 战马已落 dm-reaction-picker.md. §7 closure: PROGRESS In-flight 行翻 NO-HARDCODED-DOMAIN → DOCS-CURRENT-CATCHUP. REG-DCC-001..005 ⚪→🟢 post-merge flip. |
