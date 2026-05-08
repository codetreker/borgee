# E2E-SCENARIOS-ESTABLISHMENT spec brief — QA SSOT 固化 (≤80 行)

> 烈马 v0 · 2026-05-01 · post-#635 admin-password-plain-env wave 后启动
> **关联**: #633 admin-spa-shape-fix browser verify (D6 fetch 暴露铁规) · 用户 2026-05-01 铁律 (e2e 仅真 UI input + click + screenshot 算; 完整 e2e 验证浏览器真模拟) · 飞马 4 维 review · 野马 PM review
> **命名**: E2E-SCENARIOS-ESTABLISHMENT = QA e2e 真验证场景固化 SSOT 文档 (Smoke + Regression 分类, 12 模块全覆盖)

> ⚠️ 本 milestone 非业务功能, 是 **QA infra 真值** — 把 Borgee 全模块 e2e 验证场景从"散在 acceptance template + 各 PR review" 变成单源 SSOT 文档, 后续每 milestone acceptance 直引此文件场景 ID, 反 scope 散漂.

## 0. 关键约束 (4 条立场)

1. **e2e 真验证仅真 UI input + click + screenshot 算 (用户 2026-05-01 铁律)**: 文档内每场景"操作步骤"列字面 byte-identical 跟用户真路径 (浏览器 keyboard.type / mouse.click / page.goto / page.screenshot); 禁: cURL / fetch / page.evaluate(API) 当 e2e 证据. 反约束: 每场景"期望"列含 DOM/字面/截屏锚, 0 hit "API 200" 单调状态码.

2. **12 模块全覆盖 (蓝图字面承袭)**: AL/BPP/CHN/CV/DM/HB/RT/AP/ADM/CM/DL/INFRA 12 模块平均 ≥5 场景 (CM/DL 边缘 4 合理). 反约束: 飞马 architect review 抽查全 milestone covered (跨 #199 ADM-0.1 → #635 admin-password 全在).

3. **Smoke / Regression 拆死 + 5 状态码透明留账**: smoke ≤15 min (deploy 后必跑) / regression 1-2h (weekly + release 前). 5 状态码 (✅ done / 🟡 partial / ⏸ blocked-by-* (含 UI-coverage / #633-client-followup) / ⏸ deferred-to-host-deploy-verify / ⚠️ todo / ❌ failed). 反约束: blocked / deferred 不允许 fetch / cURL 顶替 (铁律承袭).

4. **0 production code 改 / 0 test 改 / 0 schema / 0 endpoint**: 仅新建 `docs/qa/e2e-scenarios.md` 单文件 SSOT. 反约束: `git diff main --stat -- packages/` = 0 行.

## 1. scope (1 件套, 305 行)

### 1.1 docs/qa/e2e-scenarios.md (新建)

- 17 Smoke 场景 (登录/登出 admin+user + 6 真用 path + 4 跨 milestone 锁链断言 [cookie SSOT / capability dot UI / ULID 字面] + mention/三搜/notification/settings)
- 86 Regression 场景 (12 模块: AL 8 + BPP 6 + CHN 10 + CV 12 + DM 8 + HB 6 + RT 8 + AP 7 + ADM 10 + CM 4 + DL 4 + INFRA 3)
- §3 总数 + v3 双 review 变更日志 (飞马 4 维 + 野马 PM 5+5+3)
- §4 三段汇总 (blocked-by-UI-coverage 11 / blocked-by-#633-client-followup 1 / deferred-to-host-deploy-verify 3)
- §5 退出条件 5 状态码 + 铁规重申
- §6 立场承袭 (跟 docs/evidence/liema-633-browser-verify/README.md byte-identical)

## 2. 立场承袭锁链

- 用户 2026-05-01 铁律 (e2e 仅真 UI + 完整验证)
- liema-633-browser-verify §0 (D6 fetch 暴露后凿实)
- 飞马 4 维 architect review (锚 PR # / 跨 milestone 锁链断言 / 字面 stale 修 / Budget 重分类)
- 野马 PM review (5 文案修 + 5 真缺漏 + 3 反向断)
- progress_must_be_accurate 用户 memory (blocked / deferred 留账透明不删)
- no_admin_merge_bypass 用户 memory (走 PR 不直 push main)

## 3. 留账 (透明 v2+)

- Phase 3 真跑 smoke (等用户回 user 凭据)
- 后续每 milestone acceptance template 直引此 SSOT 场景 ID (反 scope 散漂)
- regression 完整跑通 (1-2h, weekly)
- blocked-by-* 真补 (P3 admin-spa-ui-coverage backlog + admin-spa-archived-ui-followup milestone)

## 4. 不在范围

- Playwright spec 真跑实施 (此 milestone 仅 SSOT 固化, Phase 3 跑是下一活)
- 既有 packages/e2e/tests/ 38 spec 重写
- CI integration 把 e2e-scenarios.md 跟 packages/e2e/tests/ 真校对 (留 v2 INFRA milestone)
- prod deploy verify 路径 (REG-HB-01/04 + REG-DL-03 deferred-to-host-deploy-verify)

## 5. 退出条件

- 4 件套全闭 (spec brief + stance + acceptance + REG-E2ESCN-001..006)
- e2e-scenarios.md 305 行 / 103 场景 / 12 模块全覆盖
- 0 production / test / schema / endpoint 改
