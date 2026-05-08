# Acceptance Template — E2E-SCENARIOS-ESTABLISHMENT

> Spec brief `e2e-scenarios-establishment-spec.md` (烈马 v0). Owner: 烈马 dev / 飞马 architect review / 野马 PM cross-review.
>
> **范围**: 新建 `docs/qa/e2e-scenarios.md` 单文件 QA SSOT (305 行 / 103 场景 / 12 模块全覆盖). 0 production / test / schema / endpoint 改.

## 验收清单

### §1 行为不变量 (用户铁律 + 立场承袭)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 e2e 仅真 UI + click + screenshot 字面承袭 (用户 2026-05-01 铁律) | grep + inspect | `grep -E 'cURL\|fetch\|page\.evaluate.*\(' docs/qa/e2e-scenarios.md` 仅出现在禁规 / blocked-by-UI-coverage 反 fetch 顶替段; 0 hit 当 e2e 证据 ✅ |
| 1.2 17 Smoke + 86 Regression = 103 场景 | line count | `grep -cE '^\| SMK-' docs/qa/e2e-scenarios.md` = 17 + `grep -cE '^\| REG-' docs/qa/e2e-scenarios.md` = 86 ✅ |
| 1.3 12 模块全覆盖 (AL/BPP/CHN/CV/DM/HB/RT/AP/ADM/CM/DL/INFRA) | grep 12 §header | `grep -E '^### §(AL\|BPP\|CHN\|CV\|DM\|HB\|RT\|AP\|ADM\|CM\|DL\|INFRA)' docs/qa/e2e-scenarios.md` = 12 hit ✅ |

### §2 飞马 4 维 review 兑现

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 ① 锚 milestone PR # (4+1): SMK-03/04/05/06/08 全锚 | grep PR # | SMK-04 含 #197/#229; SMK-05 含 #290/#616; SMK-06 含 #226/#228/#233; SMK-08 改 4 source enum 锚 #619/#626 ✅ |
| 2.2 ② 跨 milestone 锁链断言 +3 (SMK-15 cookie / SMK-16 capability dot UI / SMK-17 ULID) | grep SMK-15..17 | 3 行真在 + SMK-16 标 blocked-by-UI-coverage + 明示 "不允许 fetch 顶替" ✅ |
| 2.3 ③ 字面 stale 修 2 处 (SMK-08 banner + REG-ADM-05 archived UI 凿实) | grep client 真值 | SMK-08 含 "BannerImpersonate.tsx" client 真值锚; REG-ADM-05 标 blocked-by-#633-client-followup ✅ |
| 2.4 ④ Budget 重分类 3 处 (HB-01/04 + DL-03 → deferred-to-host-deploy-verify) | grep | `grep -cE 'deferred-to-host-deploy-verify' docs/qa/e2e-scenarios.md` ≥ 3 hit ✅ |

### §3 野马 PM review 兑现

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 5 文案修 (SMK-04 / REG-CHN-08 / REG-CV-04 / REG-DM-03 / REG-DM-04) | grep 真值 | 5 处字面修 byte-identical 跟 client 真 UI 路径 ✅ |
| 3.2 5 真缺漏 v2 (SMK-11/12/13/14 + REG-CV-11/12) | grep ID | 6 行真新增 ✅ |
| 3.3 3 反向断 (REG-RT-03 / REG-RT-05 / REG-AP-07 capability `*` admin-only) | grep 反向 | 3 反向断字面真在 ✅ |

### §4 反向断 (反 production drift + 反 fetch 顶替)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 4.1 0 production / packages 改 | git diff stat | `git diff main --stat -- packages/` = 0 行 ✅ |
| 4.2 0 schema / endpoint / cookie / routes 改 | git diff | 同上 ✅ |
| 4.3 5 状态码透明留账 (✅/🟡/⏸/⚠️/❌) + blocked / deferred 不顶替铁规 | inspect §5 | 5 状态码 + "blocked / deferred 不允许 fetch / cURL 顶替" 明文真挂 §5 ✅ |
| 4.4 立场承袭锁链 (用户铁律 + memory + 飞马 + 野马 + liema-633) | grep §6 | §6 立场承袭 5 行真挂 byte-identical ✅ |

### §5 closure (REG + Phase-4 entry)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 5.1 REG-E2ESCN-001..006 6 行 🟢 active 真加 registry | grep | `grep -cE '^\| REG-E2ESCN-' docs/qa/regression-registry.md` = 6 ✅ |
| 5.2 phase-4.md entry E2E-SCENARIOS-ESTABLISHMENT ✅ | grep | phase-4.md 加 entry ✅ |

## 退出条件

- §1 (3) + §2 (4) + §3 (3) + §4 (4) + §5 (2) 全绿 — 一票否决
- 17 Smoke + 86 Regression = 103 场景, 12 模块全覆盖
- 用户 2026-05-01 铁律 byte-identical 字面承袭 + 飞马 4 维 + 野马 review 全收
- 0 production / test / schema / endpoint 改
- REG-E2ESCN-001..006 全 🟢 真加 registry (跟 #624/#625/#628 同精神, 不 PR body 单 claim)
- 登记 Phase-4 entry

## 留账 (透明 v2+)

- Phase 3 真跑 smoke (等用户回 user 凭据)
- 后续每 milestone acceptance 直引此 SSOT 场景 ID
- regression 完整跑通
- blocked-by-* 真补 (P3 admin-spa-ui-coverage + admin-spa-archived-ui-followup)
