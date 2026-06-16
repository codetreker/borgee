#!/usr/bin/env bash
# scripts/e2e-backend-off-proof.sh — #974 backend-off proof driver.
#
# Boots ONLY the vite client (backend genuinely OFF — see playwright.config.ts
# E2E_BACKEND_OFF), runs the @backend-required tagged subset, and asserts the run
# FAILED for backend-unreachability. The job is GREEN **iff** the tagged specs
# went RED (exit code inverted by scripts/e2e-backend-off-assert.cjs).
#
# Robustness (this is the crux #974 hammers):
#   • Fake-green catch — a tagged spec that PASSES backend-off (doesn't really
#     depend on the backend) ⇒ asserter exits 1 ⇒ job RED.
#   • Silent-skip catch — a tagged spec that SKIPS instead of failing ⇒ a skip is
#     not a pass ⇒ asserter exits 1 ⇒ job RED.
#   • Count check — EXPECTED_COUNT is DERIVED here from `--list` (never hardcoded),
#     and the asserter verifies all N tagged tests produced a failing result.
#
# Run from repo root. Requires deps + Playwright chromium already installed.
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT/packages/e2e"

GREP_TAG='@backend-required'
REPORT_JSON="$REPO_ROOT/packages/e2e/backend-off-report.json"
LIST_JSON="$REPO_ROOT/packages/e2e/backend-off-list.json"
rm -f "$REPORT_JSON" "$LIST_JSON"

echo "==> [#974] Counting @backend-required tagged tests (--list, backend ON, no webServer side effects)"
# --list does not run tests or boot webServers; it only enumerates the selection.
PLAYWRIGHT_JSON_OUTPUT_NAME="$LIST_JSON" \
  pnpm exec playwright test --project=chromium --grep "$GREP_TAG" --list --reporter=json >/dev/null 2>&1 || true

if [[ ! -s "$LIST_JSON" ]]; then
  echo "FAIL: could not enumerate @backend-required tests (no --list JSON produced)." >&2
  exit 1
fi

EXPECTED_COUNT="$(node -e '
  const r = JSON.parse(require("fs").readFileSync(process.argv[1], "utf8"));
  let n = 0;
  const walk = (s) => { for (const sp of s.specs ?? []) n += (sp.tests ?? []).length; for (const c of s.suites ?? []) walk(c); };
  for (const s of r.suites ?? []) walk(s);
  process.stdout.write(String(n));
' "$LIST_JSON")"

echo "==> [#974] EXPECTED_COUNT=$EXPECTED_COUNT tagged @backend-required test(s)"
if [[ "$EXPECTED_COUNT" -le 0 ]]; then
  echo "FAIL: zero @backend-required tests selected — the tag/grep is wrong or the specs were renamed." >&2
  exit 1
fi

echo "==> [#974] Running tagged subset with backend OFF (E2E_BACKEND_OFF=1: only vite boots)"
# The exit code of this run is IGNORED on purpose — we WANT it to fail. The
# asserter decides the job verdict from the JSON report (run-and-fail, not skip).
set +e
E2E_BACKEND_OFF=1 \
PLAYWRIGHT_JSON_OUTPUT_NAME="$REPORT_JSON" \
  pnpm exec playwright test --project=chromium --grep "$GREP_TAG" --reporter=json >/dev/null 2>&1
PW_EXIT=$?
set -e
echo "==> [#974] Playwright exited $PW_EXIT (a non-zero exit is expected; the asserter is authoritative)"

echo "==> [#974] Asserting the tagged subset RAN AND FAILED from backend-unreachability"
node "$REPO_ROOT/scripts/e2e-backend-off-assert.cjs" "$REPORT_JSON" "$EXPECTED_COUNT"
