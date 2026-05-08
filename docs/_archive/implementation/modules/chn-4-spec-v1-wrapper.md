# CHN-4 e2e flake 真根因 wrapper — spec brief v1 (战马D)

> 战马D · 2026-04-29 · ≤80 行 · CHN-4 收口 wrapper, e2e flake 真根因修
> 关联: 烈马 v0 spec `chn-4-spec.md` ✅ (协作场骨架立场不动) / CHN-4 实施 #411+#423+#428 (e2e timing 死等是 flake 源头) / G3.4 退出闸 #442 evidence
> Owner: 战马D 实施 (e2e refactor 主战) + 烈马 v0 spec 立场守

---

## 0. 立场 (3 项, 烈马 v0 立场承袭 + 加固)

### 立场 ① — 7 源 byte-identical 反向断言**不动** (硬条件)
- 现状: `#354 ④ + #353 §3.1 + #357 ② + #364 + #371 + #374 + chn-4-collab-skeleton stance` 7 源已锁 "DM 视图永不含 workspace tab" 反向 grep
- 此 PR 不动 server / client production 代码, 仅重写 e2e
- 反约束: server-side grep CI hook 数量 7 → 7 (反 grep `data-tab="workspace".*data-channel-type="dm"` count==0)

### 立场 ② — e2e 重写为 fixture-based, 删 timing 死等 (真根因)
- 现状真因: e2e 用 `page.waitForTimeout(N ms)` 等 server WS push, CI runner 慢时 N ms 不够 → flake (跟 RT-1.2 latency CI 时序敏感同模式)
- 修法: Playwright `test.beforeAll` REST-driven seed (auth + DM/workspace channel) + assertion auto-retry (`toHaveCount` / `toBeVisible` 默认 5s timeout) 替代死等
- 依据: Playwright [auto-waiting](https://playwright.dev/docs/actionability) 是 reliable 模式

### 立场 ③ — DM 视图反约束**仍硬条件**, 用 retry assertion 兜底
- DM 视图永不含 workspace tab — 7 源 byte-identical 锁不变
- e2e 断言: `await expect(page.locator('[data-tab="workspace"]')).toHaveCount(0)` Playwright 默认 retry 5s

---

## 1. 拆 ≤3 段

### CHN-4.1 — e2e refactor (fixture-based, 删 timing 死等)
- 替换 `packages/e2e/tests/chn-4-collab-skeleton.spec.ts` 内所有 `page.waitForTimeout`
- 加 fixture file: `packages/e2e/fixtures/chn-4-fixtures.ts` (REST-driven seed: owner + agent + DM channel + public channel)
- assertion 全用 Playwright auto-retry (`toBeVisible` / `toHaveCount` / `toContainText`)
- 反向 grep: `waitForTimeout|setTimeout` 在 packages/e2e/tests/chn-4-* count==0

### CHN-4.2 — server-side 反约束 grep CI 守 (复用 PERF-AST-LINT #506)
- 7 源 byte-identical 反向断言保留, 不动 server production 代码
- 加 `internal/api/chn_4_grep_audit_test.go` 用 astscan helper #506 反向断言 `data-tab="workspace"` 不出现在 DM 视图相关 production *.go

### CHN-4.3 — closure (acceptance + signoff + REG flip)
- `acceptance-templates/chn-4.md` 翻 ✅ + 烈马 signoff e2e refactor 行 + REG-CHN4-001..005 flip 🟢

---

## 2. 反约束 (5 grep)

```bash
# A. 死等 0 hit / B. DM 无 workspace tab / C. fixture exists
# D. astscan ≥1 hit / E. server prod 0 行变更
```

详见 stance checklist (CHN-4.1 实施时落地).

---

## 3. REG-CHN4-001..005 占号

| ID | 锚 | Test |
|---|---|---|
| 001 | e2e fixture-based, 0 timing 死等 | `chn-4-collab-skeleton.spec.ts` 重写 + 反 grep |
| 002 | DM 视图无 workspace tab assertion | Playwright `toHaveCount(0)` retry |
| 003 | 7 源 byte-identical 反向断言不动 | server-side grep CI hook 数量验 |
| 004 | astscan helper 复用 (#506 同模式) | server unit + 反 grep |
| 005 | server production 0 行变更 | git diff 0 行 |

## 4. 不在范围

- 协作场新 entity / 新表 / 新 endpoint (烈马 v0 立场守) / E2E 套件外其他 flake (留 PERF-* 后续 PR)
