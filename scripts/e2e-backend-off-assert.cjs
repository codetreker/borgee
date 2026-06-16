#!/usr/bin/env node
// scripts/e2e-backend-off-assert.cjs — #974 backend-off proof asserter.
//
// Reads a Playwright JSON report (produced by the @backend-required run with the
// backend genuinely OFF) and decides whether the proof HELD. The whole point of
// #974 is that a backend-off condition must be CAUGHT, not silently green-skipped,
// so this asserter is deliberately strict and INVERTS the usual pass/fail logic:
//
//   The tagged @backend-required tests are EXPECTED TO FAIL backend-off (their
//   real product assertions need backend-served data). So:
//
//     proof HOLDS  (exit 0, job GREEN)  IFF
//        • > 0 tagged tests were collected AND ran, AND
//        • every test that ran FAILED (status 'failed'/'timedOut'/'interrupted'), AND
//        • ZERO tests passed              (a pass backend-off = a FAKE-GREEN surface
//                                          that does not really depend on the backend), AND
//        • ZERO tests were skipped        (a skip is NOT a pass — a silent skip
//                                          backend-off is itself a proof failure), AND
//        • ZERO tests were flaky, AND
//        • the number of test results equals EXPECTED_COUNT (every tagged test
//          actually produced a result — none silently dropped/collected-away).
//
//   Otherwise the proof FAILED (exit 1, job RED) with an actionable reason.
//
// Usage: node scripts/e2e-backend-off-assert.cjs <report.json> <EXPECTED_COUNT>

const fs = require('node:fs');

const [reportPath, expectedCountRaw] = process.argv.slice(2);
if (!reportPath || expectedCountRaw === undefined) {
  console.error('usage: e2e-backend-off-assert.cjs <report.json> <EXPECTED_COUNT>');
  process.exit(2);
}
const EXPECTED_COUNT = Number(expectedCountRaw);
if (!Number.isInteger(EXPECTED_COUNT) || EXPECTED_COUNT <= 0) {
  console.error(`FAIL: EXPECTED_COUNT must be a positive integer, got "${expectedCountRaw}"`);
  process.exit(2);
}

let report;
try {
  report = JSON.parse(fs.readFileSync(reportPath, 'utf8'));
} catch (err) {
  console.error(`FAIL: could not read/parse Playwright JSON report at ${reportPath}: ${err.message}`);
  console.error('       (no report usually means the whole run aborted — e.g. a webServer that never came up).');
  process.exit(1);
}

// Walk every spec/test/result in the report tree, regardless of suite nesting.
const tests = []; // { title, status }
function walkSuite(suite) {
  for (const spec of suite.specs ?? []) {
    for (const t of spec.tests ?? []) {
      // A Playwright "test" has 1+ results (retries). The LAST result is the
      // outcome that decides the test. A skipped test has a single 'skipped'
      // result; a flaky test has a failing result followed by a passing retry.
      const results = t.results ?? [];
      const last = results[results.length - 1];
      const status = t.status /* roll-up: expected|unexpected|flaky|skipped */ ?? (last ? last.status : 'unknown');
      tests.push({
        title: `${spec.title}`,
        rollup: t.status,
        lastResult: last ? last.status : 'none',
        status,
      });
    }
  }
  for (const child of suite.suites ?? []) walkSuite(child);
}
for (const suite of report.suites ?? []) walkSuite(suite);

const stats = report.stats ?? {};
console.log('— Playwright stats —', JSON.stringify(stats));
console.log(`— collected ${tests.length} tagged test(s), EXPECTED_COUNT=${EXPECTED_COUNT} —`);
for (const t of tests) {
  console.log(`  • rollup=${t.rollup} lastResult=${t.lastResult} :: ${t.title}`);
}

const failures = [];

// (0) every tagged test must have produced a result — none collected-away.
if (tests.length !== EXPECTED_COUNT) {
  failures.push(
    `expected EXACTLY ${EXPECTED_COUNT} tagged @backend-required test(s) to run, but ${tests.length} produced a result. ` +
      `A missing test = the tag/grep drifted or a test was silently collected-away.`,
  );
}

// Classify each test from its roll-up status (Playwright's own verdict).
//   'expected'   = passed as expected           → FAKE-GREEN backend-off (bad)
//   'unexpected' = failed                        → GOOD (the proof we want)
//   'flaky'      = failed then passed on retry   → not a clean fail (bad)
//   'skipped'    = test.skip / fixme / filtered  → SILENT SKIP (bad)
let failed = 0;
let passed = 0;
let skipped = 0;
let flaky = 0;
let other = 0;
for (const t of tests) {
  switch (t.rollup) {
    case 'unexpected':
      failed += 1;
      break;
    case 'expected':
      passed += 1;
      break;
    case 'skipped':
      skipped += 1;
      break;
    case 'flaky':
      flaky += 1;
      break;
    default:
      other += 1;
  }
}

if (passed > 0) {
  failures.push(
    `${passed} tagged test(s) PASSED backend-off — a fake-green surface that does NOT really depend on the backend. ` +
      `A @backend-required test must assert a backend-RENDERED element so it FAILS when the backend is off.`,
  );
}
if (skipped > 0) {
  failures.push(
    `${skipped} tagged test(s) were SKIPPED backend-off — a skip is NOT a pass. A silent skip is exactly the #974 failure mode; treat it as a proof failure.`,
  );
}
if (flaky > 0) {
  failures.push(`${flaky} tagged test(s) were FLAKY backend-off — not a clean, deterministic backend-unreachability failure.`);
}
if (other > 0) {
  failures.push(`${other} tagged test(s) had an unrecognized status — cannot prove a clean backend-off failure.`);
}
if (failed === 0) {
  failures.push('ZERO tagged tests failed backend-off — the proof requires every tagged test to FAIL when the backend is unreachable.');
}

// Cross-check the Playwright top-level stats with our own count so a malformed
// tree can't sneak past (defence in depth, not the primary signal).
if (typeof stats.expected === 'number' && stats.expected !== 0) {
  failures.push(`Playwright stats report ${stats.expected} expected-pass(es) backend-off (must be 0).`);
}
if (typeof stats.skipped === 'number' && stats.skipped !== 0) {
  failures.push(`Playwright stats report ${stats.skipped} skipped test(s) backend-off (must be 0).`);
}

if (failures.length > 0) {
  console.error('\n❌ #974 backend-off proof FAILED — job is RED:');
  for (const f of failures) console.error(`   - ${f}`);
  console.error(
    '\nWhat this means: the tagged @backend-required specs did NOT all genuinely fail from backend-unreachability. ' +
      'Either a spec is not actually backend-wired (fake-green), a spec silently skipped, or the backend-off boot is wrong.',
  );
  process.exit(1);
}

console.log(
  `\n✅ #974 backend-off proof HELD — all ${failed} tagged @backend-required test(s) ran AND failed from backend-unreachability ` +
    `(0 passed, 0 skipped, 0 flaky). The tagged production surfaces are genuinely backend-wired. Job is GREEN.`,
);
process.exit(0);
