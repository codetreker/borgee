// tests/realtime-presence-broadcast.spec.ts — Realtime presence 4 态 + multi-device fanout + thinking subject 反约束.
//
// 状态: SKIP+followup (gh#716 + gh#724 §1, 2026-05-11 真验 mount 缺失发现).
//
// 跳过原因: RT3PresenceDot 组件在 client SPA 当前没有 production mount
// (反向 grep `<RT3PresenceDot\b` 在 production code 0 hit, 仅 `__tests__/RT3PresenceDot.test.tsx` 渲染).
// 老 PresenceDot (AL-1b 那套) 有 production mount 但 4 态 enum SSOT 跟 RT-3 spec 立场冲突
// (RT-3 §1.1 4 态: online / offline / recently-active / busy-idle vs 老 PresenceDot 3 态).
// 现 spec 全走 REST + page.screenshot (反模式 F3 纯 REST), 不是真 UI input/click 路径.
// v2 RT3PresenceDot mount 落地后 unskip + 改真 UI (多 tab 真渲染 presence dot DOM data-attr 4 态切换).
//
// 5 case (v2 unskip 时验):
//   - multi-device: owner 多 tab 同时 active 收 fanout 帧, presence dot DOM 同步
//   - subject: thinking 态必带非空 subject (蓝图 §1.1 关键纪律, 反"假 loading")
//   - busy-idle: task_started → busy / task_finished → idle state transition 真渲染
//   - reject: empty subject → 400 thinking.subject_required wire-level reject
//   - offline-fallback: owner offline 时 RT-3 fanout 不 leak (DL-4 push 留账)
//
// 关联文档:
//   - 蓝图: docs/blueprint/current/realtime.md §1.1 (4 态 presence enum SSOT v2)
//   - 验收: docs/_archive/qa/acceptance-templates/rt-3.md §3.2 + §4.3
//   - 单测: vitest RT3PresenceDot.test.tsx
//   - 后续: gh#724 §1 (RT3PresenceDot mount 待做, 跟 ArtifactComments + BundleSelector cluster 一起 v2 backlog)
//
// 实施约束 (unskip 后):
//   - 真 UI 路径走浏览器 (双 browser.newContext 真 multi-tab + page.goto Sidebar
//     → 真观察 `[data-rt3-presence-dot]` data-attr 4 态切换 + tooltip 渲染)
//   - 不允许 fs.* / page.evaluate(fetch) / 只打 API / noop
//   - subject 反约束: 真断 DOM 不出现 thinking 5-pattern (`processing|responding|thinking|analyzing|planning`) 文案

import { test, expect } from '@playwright/test';

void expect; // 避免 unused; v2 unskip 时实施.

test.describe.skip('realtime presence broadcast — 4 态 + multi-device + thinking subject (gh#716 + gh#724 §1, mount 待做)', () => {
  test('§1 multi-device fanout — owner 多 tab 真同步 presence dot DOM 4 态', async () => {
    // v2 unskip 时实施 — 见头部注释 5 case.
  });
});
