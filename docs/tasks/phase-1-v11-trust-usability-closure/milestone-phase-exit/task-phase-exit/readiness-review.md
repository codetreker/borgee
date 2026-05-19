# Phase 1 v1.1 Readiness Review

Phase: `phase-1-v11-trust-usability-closure`. Inputs: each milestone's `milestone.md` Closure Summary. This doc only records the gate status + final call + next-Phase prerequisites.

## Scope Closed In This Phase

| Milestone | Status | Canonical doc |
|---|---|---|
| M1 Helper / OpenClaw Bounded Actuator | CLOSED 2026-05-17 | `milestone-1-helper-openclaw-bounded-actuator/milestone.md` |
| M2 Channel Attention And Authority | CLOSED 2026-05-17 | `milestone-2-channel-attention-and-authority/milestone.md` |
| M3 Client Truth And Navigation | CLOSED 2026-05-17 | `milestone-3-client-truth-and-navigation/milestone.md` |

Source anchors covered: `HB-RA-1A`, `HB-RA-1B`, `MR-1`, `CH-1`, `CT-1`, `PS-1`, `IA-1` (all 7 from `phase-plan.md`).

## Phase Exit Gate Status

Phase-plan §Exit Gates → status. Per-gate evidence already lives in each milestone Closure Summary; do not restate.

| Gate (phase-plan §Exit Gates) | Result | Evidence anchor |
|---|---|---|
| G1.1 Helper actuator vs Remote Agent rail separation | SIGNED | M1 Closure Summary + M3 Closure Summary (`IA-1` gate row) |
| G1.2 Server enqueue auth + Helper local policy double-validate | SIGNED | M1 Closure Summary (schema-bound + pull/lease rows) |
| G1.3 Channel attention / management server-authoritative | SIGNED | M2 Closure Summary (`@Everyone` + owner-broaden rows) |
| G1.4 Forbidden states do not leak protected content | SIGNED | M3 Closure Summary (non-leaky row) |
| G1.5 `PS-1` no new privacy/compliance product surface | SIGNED | M3 Closure Summary (PS-1 row) |
| G1.6 Users can configure OpenClaw through bounded jobs | SIGNED | M1 task-12 (PR #964 `3450d8c`); post-promote 端到端闭环 PR #997 (`c66b469`) + #996 (`6ccb990`) + #1001+#1002 (`8deb10c`) + #1003 (`004a20f`) — 见 `announcement.md` §6 footnote + §10 retro |
| G1.7 Users understand channel mention/authority/private state | SIGNED | M2 task-1/3/4/6/8/9/10 |
| G1.8 Production surfaces reachable + truthful + IA cleanup | SIGNED | M3 task-1/2/3/4/5/6/7 |

All 8 gates SIGNED. No PARTIAL. No DEFERRED.

## Carry-overs

None. Each milestone Closure Summary lists `Deferred tasks: None`.

Items intentionally left out of this Phase scope (recorded in announcement §4 for context only, not deferred Phase gates):

- Helper `.deb` / `.pkg` delivery chain (`HB-RA-1B` execution detail). **CLEARED** by PR #1003 (`004a20f`) post-promote: `.deb`/`.pkg` builder + `release-helper.yml` pipeline shipped. 真 release artifact 待第一个 `borgee-helper-v0.1.0` tag 触发.
- `install-butler` short-lived installer privilege handoff. **CLEARED** by PR #996 (`6ccb990`) post-promote: `install-butler` binary shipped, signed-manifest installer 走真路径.
- Remote Agent npm bundle / install-butler split (out of `HB-RA-1A` scope, rail-separation guardrail intact).
- Signed-manifest production data round-trip (`HB-RA-1B` planning scope). 签名链 **CLEARED** by PR #997 (`c66b469`) post-promote (ed25519 + config-driven entries); key rotation cadence / deploy 时 key 注入流程仍 **OPERATIONAL-FOLLOWUP** — 走 `docs/current/host-bridge/manifest-signing.md` 文档 + deploy env, 非代码事.
- Broad visual redesign, mobile e2e expansion, modal a11y sweep (`next/README.md` §4 backlog rules).

These are not DEFERRED Phase gates; they were never in v1.1 boundary (see `phase-plan.md` Out of scope). Post-promote 闭环 PR 见 `announcement.md` §6.

## Risks / Blockers Cleared

- No open blocker. M2 task-10 `Settings channel delete button` last task accepted via PR #986 (`68d2471`); read-only lock on task-5/task-6 narrowed for `delete` only, `leave`/`archive`/`owner-transfer` remain locked out.
- All 8 milestone gates above are SIGNED with merged-PR anchors reachable from `main`.
- `docs/tasks/README.md` Active Task Resume cleaned (PR-A `abaed75`).
- Stale legacy intake `docs/tasks/681-remote-agent-openclaw/` archived (PR-A `abaed75`).

## Promotion Readiness To `docs/blueprint/current/`

| Check | Status |
|---|---|
| All required task PRs merged | yes |
| Milestone gates SIGNED | yes (3/3) |
| Phase exit gates SIGNED | yes (8/8) — see table above |
| Carry-overs anchored | N/A (none) |
| `docs/blueprint/next/README.md` ready to flip Work → COMPLETED | yes (after this gate signs) |
| `current/` sync prepared | PR-C will execute v1.1 → `current/` promote |

## Final Call

GO. All 8 Phase exit gates SIGNED, all 3 milestones CLOSED, no deferred anchor debt. After 4-role signoffs in `announcement.md` §7, PR-C may promote accepted v1.1 scope into `docs/blueprint/current/` and flip the next-ledger `Work` column to `COMPLETED`.

## Next-Phase Prerequisites

- v1.2 (or whatever the next Phase becomes) needs a real prerequisite/integration/coordination reason before opening — see `next/README.md` §0.1 Phase opening rule.
- Reopen `HB-RA-1B` execution-detail items (manifest signing rotation, install-butler privilege handoff, `.deb`/`.pkg` chain) only if v1.2 brainstorm pulls them; not auto-carried.
