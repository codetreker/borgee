// tests/realtime-presence-broadcast.spec.ts — Realtime presence 4 states + multi-device fanout + thinking subject negative constraint.
//
// Status: skipped with follow-up work tracked in gh#716 + gh#724 §1; real UI validation found the mount is missing on 2026-05-11.
//
// Skip reason: RT3PresenceDot currently has no production mount in the client SPA
// (reverse grep for `<RT3PresenceDot\b` in production code returns 0 hits; only `__tests__/RT3PresenceDot.test.tsx` renders it).
// The older AL-1b PresenceDot has a production mount, but its three-state enum conflicts with the RT-3 spec
// (RT-3 §1.1 4 states: online / offline / recently-active / busy-idle).
// The current spec only uses REST + page.screenshot, so it does not exercise the real UI input/click path required by F3.
// After v2 RT3PresenceDot has a production mount, unskip and use the browser UI path (multi-tab rendered presence dot DOM data-attr 4-state switching).
//
// 5 cases to verify after v2 unskip:
//   - multi-device: owner has multiple active tabs, receives fanout frames, and presence dot DOM stays synchronized
//   - subject: thinking state must carry a non-empty subject (blueprint §1.1 rule against generic loading text)
//   - busy-idle: task_started → busy / task_finished → idle state transition renders in the UI
//   - reject: empty subject → 400 thinking.subject_required wire-level reject
//   - offline-fallback: owner offline 时 RT-3 fanout does not leak events (DL-4 push accounting remains)
//
// Related docs:
//   - Blueprint: docs/blueprint/current/realtime.md §1.1 (4-state presence enum single source for v2)
//   - Acceptance: docs/_archive/qa/acceptance-templates/rt-3.md §3.2 + §4.3
//   - Unit test: vitest RT3PresenceDot.test.tsx
//   - Follow-up: gh#724 §1 (RT3PresenceDot mount remains, grouped with ArtifactComments + BundleSelector v2 backlog)
//
// Implementation constraints after unskip:
//   - Browser-driven UI path (two browser.newContext sessions + page.goto Sidebar
//     → observe `[data-rt3-presence-dot]` data-attr 4-state switching + tooltip rendering)
//   - Do not use fs.*, page.evaluate(fetch), API-only checks, or empty placeholder tests.
//   - subject negative constraint: assert the DOM does not show thinking 5-pattern (`processing|responding|thinking|analyzing|planning`) text

import { test, expect } from '@playwright/test';

void expect; // Keeps the import used until the v2 unskip implements assertions.

test.describe.skip('realtime presence broadcast — 4 态 + multi-device + thinking subject (gh#716 + gh#724 §1, mount 待做)', () => {
  test('§1 multi-device fanout — owner 多 tab 真同步 presence dot DOM 4 态', async () => {
    // Implement during the v2 unskip; see the five cases in the header comment.
  });
});
