# Acceptance Template — DOCS-CURRENT-CATCHUP (5 候选 docs/current/ 偏差修)

> Owner: 战马 实施 / 飞马 review / 烈马 验收.
>
> **范围**: docs/current/ 是代码现状, 5 milestone 偏差修 (HB-2 / HB-1B / CS-1 / DM-11 / DM-12). 仅 docs/current/ 文件 + 文案对齐, 0 production code 改 + 0 schema + 0 endpoint 改. 反 narrative.

## 验收清单

### §1 HB-2 — host-bridge daemon 现状文档

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 `docs/current/server/host-bridge-daemon.md` 存在 | `[ -f docs/current/server/host-bridge-daemon.md ]` | ⚪ pending (当前真值: 不存在) |
| 1.2 `docs/current/server/api/host-grants.md` 反向 grep `Rust crate` 0 hit (HB-2 已切 Go daemon, 反 stale Rust crate 表达) | `grep -c "Rust crate" docs/current/server/api/host-grants.md` == 0 | ⚪ pending (当前真值: 2 hit, L20+L124) |
| 1.3 反向 grep `HB-2 v0(D) 留 follow-up` 在 docs/current/server/wire-1.md / dl-2.md / dl-3.md 0 hit (HB-2 已落 #617, follow-up 留账失效) | `grep -c "HB-2 v0(D) 留 follow-up" docs/current/server/{wire-1,dl-2,dl-3}.md` 各 == 0 | ✅ 当前真值: 0/0/0 已过 |

### §2 HB-1B — host-installer 现状文档

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 `docs/current/server/host-installer.md` 存在 (或合理路径 `docs/current/server/hb-1b-installer.md` / `docs/current/borgee-installer.md`) — owner 拍板路径 | `[ -f docs/current/server/host-installer.md ] \|\| [ -f docs/current/server/hb-1b-installer.md ]` | ⚪ pending (当前真值: 0 文件存在) |
| 2.2 文档锚 `packages/borgee-installer/` 真路径 ≥1 hit (反空 placeholder) | `grep -r "packages/borgee-installer" docs/current/server/host-installer.md \|\| docs/current/server/hb-1b-installer.md` ≥1 hit | ⚪ pending (依 §2.1) |

### §3 CS-1 — client 三栏 + Artifact 分级 现状文档

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 `docs/current/client/app-shell.md` 存在 | `[ -f docs/current/client/app-shell.md ]` | ⚪ pending (当前真值: 不存在) |
| 3.2 `docs/current/client/artifact-drawer.md` 存在 | `[ -f docs/current/client/artifact-drawer.md ]` | ⚪ pending (当前真值: 不存在) |
| 3.3 `docs/current/client/app-shell.md` ≥1 hit `三栏` 或 `three-pane` (CS-1 立场字面真锚) | `grep -E "三栏\|three-pane" docs/current/client/app-shell.md` ≥1 hit | ⚪ pending (依 §3.1) |
| 3.4 `docs/current/client/artifact-drawer.md` ≥1 hit `Artifact 分级` 或 `iteration` (CS-1 立场字面真锚) | `grep -E "Artifact 分级\|iteration" docs/current/client/artifact-drawer.md` ≥1 hit | ⚪ pending (依 §3.2) |

### §4 DM-11 — DM 搜索 API 现状文档

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 4.1 `docs/current/server/api/dm-search.md` 存在 | `[ -f docs/current/server/api/dm-search.md ]` | ⚪ pending (当前真值: 不存在; api/ 仅 5 文件 artifact-preview/search/thumbnail + channels + host-grants) |
| 4.2 文档锚 endpoint 真路径 ≥1 hit (反空 placeholder) — `GET /api/v1/dm/search` 或类似 | `grep -E "/api/v[0-9]+/dm/search\|/api/v[0-9]+/dm-search" docs/current/server/api/dm-search.md` ≥1 hit | ⚪ pending (依 §4.1) |

### §5 DM-12 [LOW] — 留账或跳

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 5.1 [LOW] DM-12 docs/current/ 锚位置 owner 拍板 (路径 + 范围), 默认留 ⚪ 不阻塞 catch-up milestone | owner 拍板 | ⚪ deferred (LOW 优先级, 不在本 milestone 退出闸) |

### §6 反约束 (catch-up 范围守门)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 6.1 0 production code 改 — `git diff main -- packages/` == 0 行 | `git diff` | ⚪ pending |
| 6.2 0 schema / 0 migration v 号改 — `git diff main -- packages/server-go/internal/migrations/` == 0 行 | `git diff` | ⚪ pending |
| 6.3 0 endpoint 行为改 — `git diff main -- packages/server-go/internal/api/server.go packages/server-go/internal/server/server.go` 0 mux.Handle / `POST /api/v[0-9]+/` / `GET /api/v[0-9]+/` 行改 | `git diff` | ⚪ pending |
| 6.4 既有全包 unit + e2e + vitest 全绿不破 (docs-only catch-up, 不破代码) | full test | ⚪ pending |

### §7 closure (REG)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 7.1 REG-DCC-001..004 ⚪→🟢 翻牌 (HB-2 / HB-1B / CS-1 / DM-11 各一行, DM-12 留 ⚪ deferred) | regression-registry 锚 | ⚪ pending (post-impl flip) |

## REG-DCC-* 占号 (initial ⚪)

- REG-DCC-001 ⚪ HB-2 catch-up: `docs/current/server/host-bridge-daemon.md` 存在 + `host-grants.md` `Rust crate` 0 hit + 3 文件 `HB-2 v0(D) 留 follow-up` 0 hit
- REG-DCC-002 ⚪ HB-1B catch-up: `docs/current/server/host-installer.md` (或合理路径) 存在 + `packages/borgee-installer` 锚 ≥1 hit
- REG-DCC-003 ⚪ CS-1 catch-up: `docs/current/client/app-shell.md` + `artifact-drawer.md` 存在 + `三栏` / `Artifact 分级` 锚 ≥1 hit each
- REG-DCC-004 ⚪ DM-11 catch-up: `docs/current/server/api/dm-search.md` 存在 + endpoint 真路径锚 ≥1 hit
- REG-DCC-005 ⚪ deferred DM-12 [LOW] (owner 拍板路径, 不阻塞 catch-up)

## 退出条件

- §1 (3) + §2 (2) + §3 (4) + §4 (2) + §6 (4) + §7 (1) **全 ✅** — 一票否决
- §5 (DM-12 [LOW]) 留 ⚪ deferred 不阻塞
- 0 production code / 0 schema / 0 endpoint 改 (catch-up 范围)
- 既有全包 unit + e2e + vitest 全绿
- REG-DCC-001..004 🟢 active (REG-DCC-005 留 ⚪)

## 当前真值快照 (2026-05-06 初始 audit)

| 候选 | 当前 |
|---|---|
| HB-2 §1.1 host-bridge-daemon.md | ⚪ 不存在 |
| HB-2 §1.2 host-grants.md `Rust crate` | 🔴 2 hit (L20+L124) |
| HB-2 §1.3 wire-1/dl-2/dl-3 stale follow-up | ✅ 0/0/0 |
| HB-1B §2.1 host-installer.md | ⚪ 不存在 |
| CS-1 §3.1 app-shell.md | ⚪ 不存在 |
| CS-1 §3.2 artifact-drawer.md | ⚪ 不存在 |
| DM-11 §4.1 api/dm-search.md | ⚪ 不存在 |
| DM-12 §5.1 [LOW] | ⚪ deferred |

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-05-06 | 烈马 | v0 草稿 — 5 候选 catch-up acceptance 框架 (§1-§5 候选 + §6 反约束 + §7 closure) + REG-DCC-001..005 占号 ⚪ + 当前真值快照. 反 narrative — 仅锚 + grep + 三状态. 立场: docs/current/ 是代码现状不是历史, 偏差就修. 0 production code / 0 schema / 0 endpoint 改 (catch-up 范围). |
