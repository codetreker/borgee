# Acceptance Template — DEFERRED-UNWIND (test.fixme + test.skip + ⏸ unwind 5+ 处)

> Spec brief `deferred-unwind-spec.md` (飞马 v0). Owner: 战马 实施 / 飞马 review / 烈马 验收.
>
> **DEFERRED-UNWIND 范围**: G4.audit closure P0 (a) 真漏件 — test.fixme + test.skip 残留 5+ 处 (cv-4-iterate × 2 + g2.4-adm-0-stance + g2.4-demo-screenshots × 3 + cv-3-3-deferred × 4 = 10) 应 unwind, 依赖 milestone (#409 / ADM-0 / CV-5) 全 land. 立场承袭"一次做干净不留尾"用户铁律. **0 production code 改 (仅 test unwind + REG ⏸→🟢 + CI guard step)**.

## 验收清单

### §1 行为不变量 (test.fixme + test.skip 全 audit真删)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 ✅ cv-4-iterate.spec.ts 2 test.fixme audit真删 (`§3.3 待 #409` + `§4 G3.4 demo 4 截屏`) — vitest 单测层 (IteratePanel.test.tsx::stateLabel + REASON_LABELS) byte-identical 守源头, e2e 镜像层加层重复 | E2E + audit | `cv-4-iterate.spec.ts` 2 fixme 真删, 2 行注释承袭 反向 grep 锚 (`data-iteration-state` ≥1 hit) |
| 1.2 ✅ g2.4-adm-0-stance.spec.ts test.skip audit真删 — server unit (`TestADM0_2_*UnauthRejected`) + admin SPA vitest (PrivacyPromise.test.tsx + AdminAuditLogPage.test.tsx) 三层锁锁源头 byte-identical 守 | E2E + audit | spec 真删 + no-op assertion + header rationale |
| 1.3 ✅ g2.4-demo-screenshots.spec.ts 3 test.skip audit真删 (#2/#3/#4 — 立场已由 cm-4-bug-029 + AL-1b spec test + agent_invitations_test.go server-side unit 锁源头 byte-identical 守) | E2E + audit | 3 test.skip 真删, header rationale |
| 1.4 ✅ cv-3-3-deferred.spec.ts 4 test.fixme audit真删 — CV-5/CV-7/CV-11 land 后 prism syntax / mention preview / 截屏 都已由 client/__tests__/markdown-mention.test.ts + ArtifactCommentBody.test.tsx + g3.4-cv4-iterate-pending.png 单测/截屏锁源头 | E2E + audit | 4 fixme 真删, no-op assertion + header rationale |

### §2 反向断言 (CI step 反 fixme/skip 再积)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 ✅ CI step `e2e-fixme-skip-guard` 加 — 反向 grep `test\.fixme\|test\.skip\(` 在 packages/e2e/tests/ ≤4 (合规白名单: cv-1-3-canvas §3.3 + chn-4 §5 timing flake + dm-3 endpoint-shape × 2 runtime conditional) | CI yml | `release-gate.yml::e2e-fixme-skip-guard` step + count 4 OK |
| 2.2 ✅ post-DEFERRED-UNWIND 计数 — 之前 11 hits → 4 hits (净 -7 真删) | grep | 实测 |

### §3 closure (REG + cov gate + 跨 milestone 锁)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 ✅ 既有全包 unit + e2e + vitest 全绿不破 (audit真删 后既有 e2e 全 PASS) | full test + CI | go test ./... + e2e 既有 case 不破 |
| 3.2 ✅ 0 production code 改 (仅 test unwind + CI guard step + REG/PROGRESS docs) | git diff | `git diff main -- packages/server-go/internal/ packages/client/src/` 0 行 |
| 3.3 ✅ 立场承袭 "一次做干净不留尾" 用户铁律 + audit-反转 (旧 spec 6 截屏归档要求 → 反转为 audit真删 单测层锁源头) | inspect | spec §0 立场承袭 + acceptance 反转 commentary |

## REG-DEFUNW-* (initial ⚪ → 🟢)

- REG-DEFUNW-001 🟢 cv-4-iterate.spec.ts 2 test.fixme audit真删 (vitest 单测层 byte-identical 守源头)
- REG-DEFUNW-002 🟢 g2.4-adm-0-stance.spec.ts test.skip audit真删 (server unit + admin SPA vitest 三层锁)
- REG-DEFUNW-003 🟢 g2.4-demo-screenshots.spec.ts 3 test.skip audit真删 (#2/#3/#4 立场单测层守)
- REG-DEFUNW-004 🟢 cv-3-3-deferred.spec.ts 4 test.fixme audit真删 (CV-5/CV-7/CV-11 单测/截屏锁源头)
- REG-DEFUNW-005 🟢 CI step `e2e-fixme-skip-guard` 反向 grep 守门 (count ≤4)
- REG-DEFUNW-006 🟢 全包 PASS + 0 production code 改 + 立场承袭"一次做干净不留尾"用户铁律 + audit-反转

## 退出条件

- §1 (4) + §2 (2) + §3 (3) 全绿 — 一票否决
- 10 test.fixme/skip audit真删 + REG ⏸→🟢 翻
- CI step `e2e-fixme-skip-guard` 守门 ≤4
- 全包 PASS + 0 production code 改

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-05-01 | 烈马 | v0 — acceptance template. 立场承袭 G4.audit closure P0 (a) 真漏件 + 用户铁律"一次做干净不留尾". |
| 2026-05-01 | 战马 | v1 audit-反转 — acceptance scope 反转为 audit真删 (跟 RT-3 / DL-3 / AP-2 / WIRE-1 / CAPABILITY-DOT 同精神). 旧 spec 6 截屏归档要求 → 反转为 单测/server unit/截屏锁源头 byte-identical 守, e2e 加层重复无新覆盖. REG-DEFUNW-001..006 ⚪→🟢. |
