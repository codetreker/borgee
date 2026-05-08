# E2E-SCENARIOS-ESTABLISHMENT stance checklist (≤30 行)

> 烈马 v0 · 2026-05-01 · QA SSOT 固化立场

## 立场 (4 条, 跟 spec brief §0 一致)

1. ✅ **e2e 仅真 UI + click + screenshot** — 用户 2026-05-01 铁律字面承袭, 禁 cURL/fetch 顶替
2. ✅ **12 模块全覆盖** — AL/BPP/CHN/CV/DM/HB/RT/AP/ADM/CM/DL/INFRA, 平均 ≥5 (CM/DL 4 边缘合理), 全 milestone covered (#199 → #635)
3. ✅ **Smoke / Regression 拆死 + 5 状态码留账透明** — blocked / deferred 不顶替, 真账透明 (跟 progress_must_be_accurate 铁律承袭)
4. ✅ **0 production / test / schema / endpoint 改** — 单文件 SSOT 固化

## 反向断 (4 条)

- ❌ 0 hit "API 200" 期望列单调 (必须含 DOM/字面/截屏锚)
- ❌ 0 hit fetch / cURL / page.evaluate(API) 当 e2e 证据
- ❌ 0 hit blocked-by-* 用 fetch 顶替 (留账透明不偷懒)
- ❌ 0 production / packages/ 改 (`git diff main --stat -- packages/` = 0)

## 锁链承袭

- 用户 2026-05-01 铁律 + memory `progress_must_be_accurate` + memory `no_admin_merge_bypass`
- 飞马 4 维 review (锚 PR # / 跨 milestone 锁链断言 / 字面 stale / Budget)
- 野马 5 文案修 + 5 真缺漏 + 3 反向断
- liema-633-browser-verify §0 D6 fetch 暴露后真凿实
