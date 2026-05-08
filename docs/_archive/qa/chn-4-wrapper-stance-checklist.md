# CHN-4 e2e flake wrapper 立场反查 (战马D v1)

> 战马D · 2026-04-29 · ≤80 行 · 跟 spec brief `chn-4-spec-v1-wrapper.md` §0 立场 3 项配套
> 烈马 v0 `chn-4-stance-checklist.md` 立场不动, 此 wrapper 加 e2e refactor 反查.

---

## 1. 立场反查表 (5 项, 烈马 v0 + 加固)

| # | 立场 | 反向断言 | 守门 |
|---|---|---|---|
| ① | 7 源 byte-identical 反向断言不动 (硬条件) — `#354 ④ + #353 §3.1 + #357 ② + #364 + #371 + #374 + chn-4 stance` 7 源全保留 | 反 grep `data-tab="workspace".*data-channel-type="dm"\|dmShowsWorkspace\|enableDMWorkspace` 在 packages/ count==0 — 7 源同根, 不增不减 | server-side grep CI hook + astscan helper (PERF-AST-LINT #506) |
| ② | e2e 重写 fixture-based, 删 timing 死等 (真根因) — Playwright `test.beforeAll` REST-driven seed + assertion auto-retry 替代 `page.waitForTimeout` | 反 grep `waitForTimeout\|setTimeout\|page\.evaluate.*setTimeout` 在 packages/e2e/tests/chn-4-* count==0 | e2e refactor + 反 grep CI hook |
| ③ | DM 视图反约束仍硬条件 — `toHaveCount(0)` retry 兜底, 7 源同根不靠 timing | e2e DM 视图 `[data-tab="workspace"]` toHaveCount(0) 默认 5s retry; 反向: 不允许 `expect.poll` 模式 (Playwright auto-retry 已足够) | e2e 真跑 + Playwright report |
| ④ | server production 0 行变更 — 复用 PERF-AST-LINT astscan helper 验 grep audit | git diff `packages/server-go/internal/` + `packages/client/src/` 仅含新 _test.go / e2e / fixture, 0 行 production code 变更 | git diff line count + CI hook |
| ⑤ | fixture file 是 SSOT — owner + agent + DM channel + public channel REST seed 单 entry, 不在 spec 内部重复 setup | `packages/e2e/fixtures/chn-4-fixtures.ts` 存在 + spec import 单源 | exists check + import grep |

---

## 2. 反约束 grep 清单 (CI lint hooks)

```bash
# A. e2e 死等 0 hit (立场 ②)
grep -rnE 'waitForTimeout|setTimeout' packages/e2e/tests/chn-4-* | grep -v '\.md:'   # 0 hit

# B. DM 视图无 workspace tab — 7 源 byte-identical (立场 ① 不动)
grep -rn 'data-tab="workspace".*data-channel-type="dm"\|dmShowsWorkspace' packages/   # 0 hit

# C. e2e fixture file 存在 (立场 ⑤)
test -f packages/e2e/fixtures/chn-4-fixtures.ts   # exists

# D. 复用 astscan helper (立场 ④)
grep -rn 'astscan.AssertNoForbidden' packages/server-go/internal/api/chn_4_*_test.go   # ≥1 hit

# E. server production 0 行变更 (立场 ④)
git diff origin/main -- packages/server-go/internal/ packages/client/src/ | grep -E '^\+' | grep -v '_test.go' | grep -v '^\+\+\+' | grep -v 'fixtures/'   # 0 行
```

---

## 3. 跨 milestone byte-identical 锁

- ① 7 源 同根 (#354 ④ + #353 §3.1 + #357 ② + #364 + #371 + #374 + chn-4 stance)
- ② Playwright auto-retry 跟 RT-1.2 #292 latency CI 时序敏感修法同精神 (反死等)
- ④ astscan helper 跟 PERF-AST-LINT #506 同源 (改 = 改 helper 一处)
- ⑤ fixture-based 跟 chn-2-3-dm-flow.spec.ts (#413) registerOwner / adminLogin 同模式

---

## 4. 不在范围 (反约束)

- 不开新 entity / 新表 (烈马 v0 spec 立场)
- 不开新 endpoint
- 不动 client production renderer
- E2E 套件外其他 flake (留 PERF-* 后续 PR)

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-29 | 战马D | v1 — CHN-4 e2e flake wrapper 立场反查 5 项 (7 源不动 / 死等删除 / retry 兜底 / production 0 行 / fixture SSOT). 烈马 v0 立场承袭加固. CI lint hooks 5 grep 全 0/≥1 hit 守门. |
