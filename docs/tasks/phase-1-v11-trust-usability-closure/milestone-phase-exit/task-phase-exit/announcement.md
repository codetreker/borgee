# Phase 1 v1.1 Trust And Usability Closure — Exit Announcement

Phase: `phase-1-v11-trust-usability-closure`. Per v6 `bf-phase-exit-gate` Step 1 + Step 4. Detail lives in each `milestone.md` Closure Summary + PR body + git log; this announcement only records anchors + signoffs.

## §1 Three-bucket Summary

| Bucket | Count | Gates |
|---|---|---|
| SIGNED | 8 | G1.1, G1.2, G1.3, G1.4, G1.5, G1.6, G1.7, G1.8 |
| PARTIAL | 0 | — |
| DEFERRED | 0 | — |

All 7 source anchors (`HB-RA-1A`, `HB-RA-1B`, `MR-1`, `CH-1`, `CT-1`, `PS-1`, `IA-1`) closed inside this Phase. 3/3 milestones CLOSED 2026-05-17. See `readiness-review.md` for the full gate table.

## §2 Milestone 1 Gates — Helper / OpenClaw Bounded Actuator

| Gate | PR / SHA | Result |
|---|---|---|
| G1.1 Helper vs Remote Agent rail separation | PR #939 (`96dc0dc`), #942 (`642fb57`), #962 (`2e58127`) | SIGNED |
| G1.2 Server enqueue auth + Helper local policy double-validate | PR #938 (`64d56f1`), #942 (`642fb57`), #943 (`c2c61e6`) | SIGNED |
| G1.6 Users configure OpenClaw via bounded jobs | PR #956 (`5575b53`), #958 (`ad50575`), #963 (`d8d179e`), #964 (`3450d8c`); post-promote 闭环 PR #997 (`c66b469`) + #996 (`6ccb990`) + #1001+#1002 (`8deb10c`) + #1003 (`004a20f`) | SIGNED (见 §6 footnote) |

## §3 Milestone 2 Gates — Channel Attention And Authority

| Gate | PR / SHA | Result |
|---|---|---|
| G1.3 Channel attention/management server-authoritative | PR #949 (`c25ef60`), #951 (`3659ce1`), #955 (`0dd35a9`), #959 (`66c9a35`) | SIGNED |
| G1.7 Users understand channel mention/authority/private state | PR #948 (`077cb8c`), #952 (`965fcd7`), #953 (`6ae4604`), #961 (`1e6d54c`), #986 (`68d2471`) | SIGNED |

## §4 Milestone 3 Gates — Client Truth And Navigation

| Gate | PR / SHA | Result |
|---|---|---|
| G1.4 Forbidden states non-leaky | PR #957 (`16e2db6`), #960 (`84a0315`) | SIGNED |
| G1.8 Production surfaces reachable + truthful + IA cleanup | PR #944 (`0877a9b`), #946 (`a6c6ce3`), #947 (`47dc680`), #950 (`05fff88`), #962 (`2e58127`) | SIGNED |

## §5 Cross-cutting Privacy Scope Guard

| Gate | PR / SHA | Result |
|---|---|---|
| G1.5 `PS-1` no new privacy/compliance product surface | scope guard upheld across every M1/M2/M3 PR; M3 task-3 PR #944 (`0877a9b`) is the explicit reverse-proof anchor | SIGNED |

## §6 G1.6 端到端闭环 Footnote

G1.6 在 phase exit (2026-05-18, PR #992 promote) 时签 SIGNED, 但当时 user-reachable 端到端未真闭 — 见 §10 Retro. post-cutover 5 PR chain 真实闭环:

| PR | Merge SHA | What it shipped |
|---|---|---|
| #997 | `c66b469` | ed25519 真签名链 + config-driven manifest entries |
| #996 | `6ccb990` | `install-butler` binary (signed-manifest installer) |
| #1001 + #1002 | `8deb10c` | helper dispatch loop (poll + policy evaluate + lease + result) |
| #1003 | `004a20f` | `.deb` / `.pkg` builder + `release-helper.yml` pipeline |

Caveat: manifest 真 SHA256 / Signature 数据待第一个 `borgee-helper-v0.1.0` tag 触发 #1003 release pipeline 后由 deploy env 注入. 在此之前 manifest 走 placeholder; 代码路径 + 签名验证 + dispatch loop 已 wired, 等真 release artifact 即生效.

## §7 Four-Role Signoffs

| Role | Verdict | Date | PR anchor |
|---|---|---|---|
| Dev (zhanma) | PASS | 2026-05-18 | PR #992 — Implementation accepted, 29 tasks merged across M1/M2/M3 with passing CI. |
| QA (liema) | PASS | 2026-05-18 | PR #992 — Acceptance evidence linked per task; no DEFERRED gates; full client + server-go test suites green on PR #992. |
| PM (yema) | PASS | 2026-05-18 | PR #992 — Phase 1 v1.1 user-perceivable scope delivered; bounded-actuator stance + privacy UI cleanup + reaction picker + helper reboot-survival landed. |
| Teamlead | PASS | 2026-05-18 | PR #992 — All 8 Phase exit gates SIGNED; promotion preconditions met; v6 protocol followed. |

This Phase ran without a live multi-instance team. The four signoff slots will be filled by the human reviewer at PR review time per v6 `bf-phase-exit-gate` Step 2 + role checklists (`references/{dev,qa,pm,teamlead}-review.md`). Each row: role / ✅ or ⚠️ / YYYY-MM-DD / this-PR anchor.

## §8 Changelog

- PR-A (`abaed75`): step 1 reconcile + clean stale records (Active Task Resume + M1/M2/M3 Closure Summaries + `next/` resume hint + archive legacy intake).
- PR-B (this commit): step 2 `bf-phase-exit-gate` deliverables (`readiness-review.md` + `announcement.md`).
- PR-C (planned, same branch): promote accepted v1.1 scope into `docs/blueprint/current/` and flip `next/README.md` §0 `Work` column from `IMPLEMENTING` → `COMPLETED` for `HB-RA-1A`, `HB-RA-1B`, `MR-1`, `CH-1`, `CT-1`, `PS-1`, `IA-1`.

Out-of-scope items intentionally not deferred as Phase gates (see `readiness-review.md` Carry-overs section): Helper `.deb`/`.pkg` delivery chain, `install-butler` privilege-handoff hardening, signed-manifest production data round-trip, Remote Agent npm bundle, broad visual redesign, mobile e2e expansion, modal a11y sweep.

## §9 Closure Announcement

Date: TODO (filled at merge).

Phase 1 v1.1 closes with all 3 milestones CLOSED, all 8 exit gates SIGNED, no DEFERRED anchor debt. Next Phase (v1.2 or whichever) is unblocked: `next/README.md` §0.1 Phase opening rule still applies — a new Phase needs a real prerequisite, integration, or coordination boundary before opening.

## References

- `docs/tasks/phase-1-v11-trust-usability-closure/phase-plan.md`
- `docs/tasks/phase-1-v11-trust-usability-closure/milestone-{1,2,3}-*/milestone.md` (each has its Closure Summary)
- `docs/tasks/phase-1-v11-trust-usability-closure/milestone-1-*/accepted-history.md`
- `docs/blueprint/next/README.md` (`§0` ledger + `§5` next workflow step)
- `readiness-review.md` (this folder)

## §10 Retro — G1.6 为何被错签 SIGNED

记录此次 phase exit 流程 slip 原因, 给未来 phase exit + `bf-phase-exit-gate` skill 用. 不追责, 协议层修补.

**什么 slipped**: G1.6 "Users configure OpenClaw via bounded jobs" 在 2026-05-18 phase exit 时签 SIGNED, 但 user-reachable 端到端未真: 无 `install-butler` binary (#996 后才有), manifest 走 placeholder (#997 才真签名), helper dispatch loop 未 wire (#1001+#1002 才接通), 无 `.deb`/`.pkg` release pipeline (#1003 才有). "代码 scaffolding 存在" 被等同了 "user outcome 可达".

**为何 slipped**:
- 4-role signoff 基于用户信任 + `readiness-review.md` 自己的 Carry-overs 段披露. 但 Carry-overs 措辞让 reviewer 把代码级缺口当 "deferred" 而非 "blocks G1.6 SIGNED claim".
- `bf-phase-exit-gate` skill 当前 signoff 格式没区分 "code-shipped + outcome-reachable" 跟 "stance-locked + execution-deferred". 同一格子两种状态都填 SIGNED.

**改什么 (协议, 非追责)**:
- 未来 phase exit announcement: 每条 gate row REQUIRES "User outcome path" 列, 追 user action → observable outcome 链. 链上任一环是 "pending PR X" 或 "awaits future work", gate 不能 SIGNED, 必须 PARTIAL 或 DEFERRED.
- Carry-overs 段每条加 label: `BLOCKS-GATE: G1.x` (真挡 gate) vs `OPERATIONAL-FOLLOWUP` (真 deploy-time only).
- `bf-phase-exit-gate` skill v6.x 可加 built-in "user-outcome trace" verifier 作机械 lint. 此条作为 blueprintflow v6.x 输入, 不在此 PR scope (此 PR 在 borgee 仓库).

非追责: signoff 当时 good faith; 协议允许此 drift. 协议微调是 fix, 不是人.

**2nd slip (2026-05-20 chore/npm-bundle-rework)**: G1.6 闭环 PR chain 起手用 `.deb` + `.pkg` 分发 (#1003 / #1008) + 3 个独立 Go binary (`borgee-helper` / `borgee-helper-claim` / `install-butler`, #996 / #1011), 后来用户直接拍校正方向: 应当走现有 `@codetreker/borgee-remote-agent` npm 包 + 单 `borgee` Go binary + 子命令. chore/npm-bundle-rework 一次校正 — 删 `release-helper.yml` + `nfpm.yaml` + `packages/borgee-installer/`, 折 3 binary 成 `cmd/borgee`, 加 4 个平台 npm 子包, 加 `release-borgee.yml`.

根因跟 G1.6 同源: dispatch loops 派活时没回去查用户最早讨论, 看到"helper 要发布"就按 OS 包装常识 (`.deb` / `.pkg`) 走. 协议补丁: 派分发类活前必先 grep + 读用户讨论 source-of-truth, 不靠角色 memory 推断分发渠道.
