// tests/realtime-presence-broadcast.spec.ts — Realtime presence 4 states + multi-device fanout + thinking subject negative constraint.
//
// 状态: skipped with follow-up work tracked in gh#716 + gh#724 §1; real UI validation found the mount is missing on 2026-05-11.
//
// 跳过原因: RT3PresenceDot 组件在 client SPA 当前没有 production mount
// (反向 grep `<RT3PresenceDot\b` 在 production code 0 hit, 仅 `__tests__/RT3PresenceDot.test.tsx` 渲染).
// 老 PresenceDot (AL-1b 那套) 有 production mount 但 4 态 enum SSOT conflicts with the RT-3 spec
// (RT-3 §1.1 4 态: online / offline / recently-active / busy-idle vs 老 PresenceDot 3 态).
// The current spec only uses REST + page.screenshot, so it does not exercise the real UI input/click path required by F3.
// After v2 RT3PresenceDot has a production mount, unskip and use the browser UI path (multi-tab rendered presence dot DOM data-attr 4-state switching).
//
// 5 case (v2 unskip 时验):
//   - multi-device: owner 多 tab 同时 active 收 fanout 帧, presence dot DOM 同步
//   - subject: thinking 态必带非空 subject (蓝图 §1.1 关键纪律, 反"假 loading")
//   - busy-idle: task_started → busy / task_finished → idle state transition renders in the UI
//   - reject: empty subject → 400 thinking.subject_required wire-level reject
//   - offline-fallback: owner offline 时 RT-3 fanout does not leak events (DL-4 push accounting remains)
//
// 关联文档:
//   - 蓝图: docs/blueprint/current/realtime.md §1.1 (4 态 presence enum SSOT v2)
//   - 验收: docs/_archive/qa/acceptance-templates/rt-3.md §3.2 + §4.3
//   - 单测: vitest RT3PresenceDot.test.tsx
//   - 后续: gh#724 §1 (RT3PresenceDot mount 待做, 跟 ArtifactComments + BundleSelector cluster 一起 v2 backlog)
//
// 实施约束 (unskip 后):
//   - Browser-driven UI path (two browser.newContext sessions + page.goto Sidebar
//     → observe `[data-rt3-presence-dot]` data-attr 4-state switching + tooltip rendering)
//   - 不允许 fs.* / page.evaluate(fetch) / 只打 API / empty placeholder tests
//   - subject negative constraint: assert the DOM does not show thinking 5-pattern (`processing|responding|thinking|analyzing|planning`) text

import { test, expect } from '@playwright/test';

void expect; // 避免 unused; v2 unskip 时实施.

test.describe.skip('realtime presence broadcast — 4 态 + multi-device + thinking subject (gh#716 + gh#724 §1, mount 待做)', () => {
  test('§1 multi-device fanout — owner 多 tab 真同步 presence dot DOM 4 态', async () => {
    // v2 unskip 时实施 — 见头部注释 5 case.
  });
});
