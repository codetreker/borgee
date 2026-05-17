// tests/agent-permission-bundle.spec.ts - Agent permission bundle UI tests (transparent capability expansion).
//
// Status: skipped with follow-up work tracked in gh#716 + gh#724 §1; real mount verification found the missing mount on 2026-05-11.
//
// Skip reason: BundleSelector / PermissionsView components currently have no production
// mount in the client SPA (reverse grep for `<BundleSelector\b` / `<PermissionsView\b` in production code
// returns 0 hits; only `__tests__/` renders them in unit tests). The real UI path is unreachable. The old spec used direct REST
// plus visual evidence (F3 pure REST anti-pattern), not a real UI input/click path.
// After v2 BundleSelector / PermissionsView mount lands, unskip and convert to real UI: user clicks
// bundle -> capability checkbox expands -> confirm -> server receives N PUT calls -> DOM assertions.
//
// 4 cases to verify after v2 unskip:
//   - capability response shape - /api/v1/me/permissions includes capabilities array
//   - reverse-check bundle endpoint drift - POST /api/v1/bundles does not exist (reuse AP-1 PUT)
//   - real UI rendering - capability expands transparently (RBAC 8 terms stay absent from body)
//   - admin privilege UI isolation path - admin login does not include bundle UI text (ADM-0 §1.3 boundary)
//
// Related docs:
//   - Blueprint: docs/blueprint/current/auth-permissions.md §4 (capability bundle UI)
//   - Acceptance: docs/_archive/qa/acceptance-templates/ap-2.md §3.2
//   - Unit tests: vitest ap-2-capabilities.test.ts + ap-2-reverse-grep.test.ts
//   - Follow-up: gh#724 §1 (BundleSelector / PermissionsView mount remains)
//
// Implementation constraints after unskip:
//   - Real UI path uses the browser (page.goto Settings page -> page.click bundle row -> expand
//     capability checkboxes -> click confirm -> assert server receives PUT calls)
//   - Do not use fs.* / page.evaluate(fetch) / API-only / noop tests
//   - admin privilege isolation path case uses REWRITE-NAV (page.goto admin URL + assert bundle UI text has 0 hits)

import { test, expect } from '@playwright/test';

void expect; // Keeps the import used until the v2 unskip implements assertions.

test.describe.skip('agent permission bundle UI — capability 透明 (gh#716 + gh#724 §1, mount 待做)', () => {
  test('§3.2 capability response shape — /api/v1/me/permissions 含 capabilities 数组', async () => {
    // v2 unskip 时实施 — 见头部注释 4 case.
  });
});
