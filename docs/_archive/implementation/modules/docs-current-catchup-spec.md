# DOCS-CURRENT-CATCHUP — spec brief v0 (cross-PR docs/current 累积补)

> **不属于功能 milestone — 是规则 6 (代码改 → docs/current 必同步) 的累积补救 PR**. 5 PR 落地后 docs/current 真没跟上, 一次清不留尾, 按用户立场 docs/current = 代码现状 (不是历史归档).
>
> 锚: `docs/blueprint/_meta/blueprint-audit-rotation.md` L13 "PR body `## Current 同步` 段是否填 (规则 6 lint)" + § "docs/current 字面 const verify (PR #242 lessons)" + 用户立场 (2026-05-06 修正): docs/current 不是历史, 偏差就修.

## §0. 三铁律 (硬约束)

1. **一 milestone 一 PR** — DOCS-CURRENT-CATCHUP 整 milestone 一 PR (不拆 5 sub-PR), 跟 NAMING-1 #614 / REFACTOR-2 #613 "一次做干净不留尾" 立场承袭.
2. **文件命名按功能不按 milestone** — 新建 docs/current 文件按功能命名 (`packages/borgee-helper.md` / `packages/borgee-installer.md` / `client/app-shell.md` / `server/api/dm-search.md`), 反 milestone 前缀 (memory `file_naming_no_milestone_prefix.md` 铁律).
3. **docs/current 真反映代码现状** — 文件存在 ↔ 包/组件/endpoint 存在 byte-identical; const/path/route 字面对账 (跟 blueprint-audit-rotation §2.1 PR #242 lessons 同精神). 反"docs 重复 spec brief / PR body narrative" (CLAUDE.md changelog-slim 立场).

## §1. Scope (5 项, 用户拍 A 一次清)

### §1.1 HB-2 (#617 / commit f680e37) — `packages/borgee-helper/` Go daemon

**漂状态** (5 处实测):
- `docs/current/server/api/host-grants.md` L20 + L124 字面 stale: "HB-2 daemon (Rust crate, 待 HB-2 真实施 PR)" — 真值 = Go daemon 已 merged #617 (5-01)
- `docs/current/server/dl-2.md` L81/L83: "HB-2 v0(D) Borgee Helper SQLite consumer 留 HB-2 单 milestone" stale
- `docs/current/server/dl-3.md` L79: "HB-2 v0(D) Borgee Helper 阈值哨 留 HB-2 follow-up" stale
- `docs/current/server/wire-1.md` L75: "HB-2 v0(D) ... 留 HB-2 v1" stale
- `packages/borgee-helper/` 整包无 docs/current/ 入口

**修法**:
- 新建 `docs/current/borgee-helper.md` (按功能, 不按 milestone): 真包路径 `packages/borgee-helper/`, 7 internal/ 子包 (acl/audit/sandbox/ipc/reasons/grants/fileio), cmd/borgee-helper main, e2e, install/ systemd+launchd unit. **跟 PR #617 + #622 真值锚, 不重述 PR narrative**
- 修 `docs/current/server/api/host-grants.md` L20 + L124 字面 "Rust crate, 待 HB-2 真实施 PR" → "Go daemon (#617 已 merged, packages/borgee-helper/)"
- 修 dl-2.md / dl-3.md / wire-1.md 4 处 "留 HB-2 follow-up" → "已落 #617 + 真包路径"

### §1.2 HB-1B (#627 / commit 573abb3) — `packages/borgee-installer/` Go installer

**漂状态**: `packages/borgee-installer/` 整包 (cmd/borgee-installer-{linux,darwin} + internal/{deploy,manifest,dialog} + install/) 无 docs/current/ 入口.

**修法**: 新建 `docs/current/borgee-installer.md` (按功能命名), 锚 PR #627 真包路径 + 双平台 cmd 入口 + ed25519 manifest verify.

### §1.3 CS-1 (#601 / commit 1687527) — 三栏布局 + Artifact 4 态

**漂状态** (3 真组件无 docs/current/client/ 入口):
- `packages/client/src/components/AppShell.tsx` (三栏布局 SSOT)
- `packages/client/src/components/ArtifactDrawer.tsx` (4 态 right column)
- `packages/client/src/lib/use_artifact_panel.ts` (4-state state machine: closed/peek/split/full)

**修法**: 新建 `docs/current/client/app-shell.md`, 锚 3 文件路径 + 4 态 state machine 真实施 + transition 谓词 (反向 grep `closed → split` 直接 reject 跟 useArtifactPanel guard byte-identical).

### §1.4 DM-11 (#600 / commit 1df15dd) — `/api/v1/dm/search` endpoint

**漂状态**: 真 endpoint = `GET /api/v1/dm/search?q=<query>&limit=<N>` (实测 message_search.go), 但 `docs/current/server/api/` 无 dm-search 入口.

**修法**: 新建 `docs/current/server/api/dm-search.md` (按功能, 不叫 dm-11), 锚 PR #600 真 endpoint + 4 立场 (0 schema / DM-only scope / channel-member ACL / admin god-mode 不挂) + 错码 `dm_search.q_required` / `dm_search.q_too_short` 字面.

### §1.5 DM-12 (#603 / commit 88d355e) — DM reaction picker [LOW, 可不做]

**漂状态**: `packages/client/src/components/DMMessageReactionPicker.tsx` + ReactionBar.tsx 无 docs/current/client/ 入口.

**修法**: client-only composite (0 server, 0 schema), 真值 = 复用 reactions endpoint. 写 docs/current/client/dm-reaction-picker.md ≤30 行锚组件路径就够. **如时间紧可不做留下次** (用户拍 LOW).

## §2. 反约束 (硬不在范围)

- **不写 §5+ 派活/自审/changelog 段** (CLAUDE.md changelog-slim, 跟 REFACTOR-2 spec v1 同模式)
- **不重述 PR narrative** — docs/current 锚组件/endpoint/包路径真值, 不复制 PR body
- **不动 docs/blueprint/** — 蓝图是真值 SOT, 这次仅补 docs/current
- **不动 spec brief** — 5 PR 的 spec/acceptance/REG 都已合, 不重写
- **不开 admin god-mode 旁路** — DM-11 / HB-2 / host-grants 字面写明 admin god-mode 不挂 (ADM-0 §1.3 红线承袭)
- **0 代码改 / 0 schema / 0 endpoint** — docs-only PR
- **不归一 const 字面** — docs/current 字面 const verify (引用代码) 不重写代码字面 (跟 blueprint-audit-rotation §2.1 PR #242 lessons 同精神)
- **跨 PR 累积补 ≠ 重做 PR** — 不动 #617/#627/#601/#600/#603 已合代码, 仅补 docs/current 同步段

## §3. 反向 grep 锚 (5 候选每个一个最小锚)

| § | 候选 | 反向 grep 锚 (改后必为 0 hit) |
|---|---|---|
| §1.1 | HB-2 host-grants.md "Rust crate" stale | `grep -n 'Rust crate' docs/current/server/api/host-grants.md` ==0 |
| §1.1 | HB-2 dl-2/dl-3/wire-1 "留 HB-2 follow-up" stale | `grep -nE '留 HB-2 (follow-up\|v1\|单 milestone)' docs/current/server/{dl-2,dl-3,wire-1}.md` ==0 |
| §1.1 | borgee-helper 包入口 | `ls docs/current/borgee-helper.md` 存在 (新建) |
| §1.2 | borgee-installer 包入口 | `ls docs/current/borgee-installer.md` 存在 (新建) |
| §1.3 | CS-1 三栏 AppShell 入口 | `ls docs/current/client/app-shell.md` 存在 (新建); 内含 `useArtifactPanel` + `ArtifactDrawer` + `AppShell` 三 component path 字面 |
| §1.4 | DM-11 endpoint 入口 | `ls docs/current/server/api/dm-search.md` 存在 (新建); 内含 `/api/v1/dm/search` route 字面 + 错码 `dm_search.q_required` 字面 |
| §1.5 | DM-12 reaction picker | `ls docs/current/client/dm-reaction-picker.md` 存在 (新建, 可选) |

## §4. 留尾 (真不在本 milestone)

- **deferred 段不动** — `docs/current/server/dl-2.md` L83 "events fanout 接 RT-3 留 follow-up" / `docs/current/client/search-box.md` L113 "ChannelView sidebar 集成 留 follow-up" — 这两条是真 follow-up scope 不是 stale, **不动**
- **HB-2.0 #605 / HB-2 v0(C) #606** — 已合的中间态 PR, 真包 dir 已清成 v0(D) #617 真值, 不专门补 docs/current 入口 (单包入口锚 v0(D) 真值就够)
- **REFACTOR-3 cursor envelope** — REFACTOR-2 #613 audit 反转推, scope internal/ws, 不在本 milestone
- **F 类 PROGRESS.md L24 Phase 3 数字格式** — 用户拍"不死磕历史归档", PR #651 truth-sync 后已留档 `/tmp/truth-sync-extended-fulltable.md`, 不在本 milestone

---

> 飞马 v0 spec brief — 4 段 ≤80 行 changelog-slim 守, 等 yema/liema 4 件套补齐 + zhanma 真实施落 PR.