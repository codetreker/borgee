# COL-B24 Code Review — CC2 (Independent)

Reviewer: Claude Opus 4 | Date: 2026-04-23 | Scope: 12 integration test files + setup.ts + ws-helpers.ts

---

## CRITICAL

### C1: `ws.js` mock breaks all WS-based tests that use `buildFullApp`

**Files**: plugin-comm, file-link, remote-explorer (all `buildFullApp` users)

Every test file globally mocks `../ws.js` with no-op stubs (`broadcastToChannel: vi.fn()`, etc.), **including files that then call `buildFullApp()` and open real WS connections**. The `buildFullApp` function imports `ws-plugin.js`, `ws-remote.js`, etc. — route handlers that internally call `broadcastToChannel` and friends. Since the mock replaces the real module, event push tests (e.g., `plugin-comm.integration.test.ts:152` "message event pushed to connected plugin WS") **cannot actually verify that events are pushed** — the broadcast function is a no-op. The test currently only checks that the `api_response` comes back (status 201), not that an event was broadcast, making the assertion tautological.

**Impact**: False confidence — WS event fan-out is not tested despite appearing to be.

**Recommendation**: Tests using `buildFullApp` + real WS should NOT mock `ws.js`. Either remove the mock in those files, or use `vi.mock` conditionally with `vi.importActual` to preserve real broadcast behavior for the WS integration suite.

---

### C2: `buildFullApp` accepts `testDb` parameter but never injects it into routes

**File**: `setup.ts:266`

`buildFullApp(testDb)` takes a `Database` parameter, but the function body never passes it to any route registrar. All routes obtain their DB via `import { getDb } from '../db.js'`. The only reason tests work is the top-level `vi.mock('../db.js', () => ({ getDb: () => testDb }))` in each test file. This means:

1. The `testDb` parameter is misleading — it creates the illusion of explicit injection when the real mechanism is a global mock.
2. If any test file forgets the `vi.mock` for `db.js`, it will silently hit the real (or undefined) DB.

**Impact**: Fragile test infrastructure that silently depends on mock hoisting order.

**Recommendation**: Either remove the `testDb` parameter (make the mock the documented contract) or actually inject the DB into routes (e.g., via Fastify `decorate`).

---

### C3: Test isolation violation — `auth-flow.integration.test.ts` tests depend on execution order

**File**: `auth-flow.integration.test.ts`

- Test `'register → already used invite code → 404'` (line 68) queries for `FlowUser` created by the first test (line 54). If test order changes or the first test fails, this test breaks.
- Test `'login → correct password → 200'` (line 82) depends on the user registered in test 1.

All tests in this file share a single DB without `beforeEach` reset, and later tests read data created by earlier ones.

**Impact**: Any reorder or `.only` on a downstream test will cause false failures.

**Recommendation**: Each test should seed its own prerequisite data. The "already used invite code" test should seed its own used code directly rather than relying on test 1's side effect.

---

## HIGH

### H1: `concurrency.integration.test.ts` — invite code test is not actually concurrent

**File**: `concurrency.integration.test.ts:43-65`

`app.inject()` in Fastify runs requests in-process synchronously on the event loop. `Promise.all` of 5 `inject()` calls does **not** produce true concurrency — each request runs to completion before the next starts (single-threaded, no I/O interleaving on an in-memory SQLite). The test name says "5 并发只 1 成功" but in practice requests execute sequentially, so the first always wins and the test always passes trivially.

To actually test concurrency, you'd need a real HTTP server (`app.listen`) with 5 parallel `fetch` calls, or use worker threads. With in-memory SQLite, true concurrency testing is limited regardless.

**Recommendation**: Either rename to clarify it tests "sequential uniqueness constraint enforcement" or use a real HTTP server.

### H2: Massive boilerplate duplication across all test files

**Files**: All 10 inject-mode test files

Every file repeats the exact same pattern:
```ts
let testDb: Database.Database;
vi.mock('../db.js', () => ({ getDb: () => testDb, closeDb: () => {} }));
vi.mock('../ws.js', () => ({ broadcastToChannel: vi.fn(), ... }));

function inject(method, url, userId, payload) { ... }
```

This is ~20 lines duplicated 10 times. The `TestContext` class in `setup.ts` was designed to eliminate this, yet **no test file uses `TestContext`**. Every file manually creates Fastify, registers routes, seeds users — exactly what `TestContext.create()` does.

**Recommendation**: Either use `TestContext` (it's already built) or delete it. Having infrastructure that's never used is confusing.

### H3: `closeWsAndWait` helper duplicated in 3 files

**Files**: `plugin-comm.integration.test.ts:35`, `file-link.integration.test.ts:35`, `remote-explorer.integration.test.ts:64`

Identical function copy-pasted in each file. Should be in `ws-helpers.ts`.

### H4: `require-mention.integration.test.ts` — scope mismatch with design doc

**File**: `require-mention.integration.test.ts`

The task breakdown (T4.1) specifies: "SSE + Poll 两个路径, describe.each 覆盖", "WS 路径过滤", "DM 不受限制". The actual implementation tests only the admin API for toggling the `require_mention` flag via DB — it doesn't test message filtering at all. This is a CRUD test for a user attribute, not a requireMention behavior test.

**Impact**: The core feature (agent doesn't receive messages unless @mentioned) is untested.

### H5: `remote-explorer.integration.test.ts` — `addRemoteTables` duplicates schema

**File**: `remote-explorer.integration.test.ts:28-48`

The test manually creates `remote_nodes` and `remote_bindings` tables with inline SQL. If the production schema for these tables changes, this test's schema will drift silently. The `createTestDb()` in setup.ts should contain all tables, or the test should import the schema from the same source as production.

### H6: `workspace-flow.integration.test.ts` — test ordering dependency via `uploadedFileId`

**File**: `workspace-flow.integration.test.ts:89`

`uploadedFileId` is set by the "upload" test (line 98) and consumed by "rename" (line 109). If "upload" is skipped or fails, "rename" will fail with an undefined ID. Same pattern as C3 but less severe since these tests are more naturally sequential.

### H7: Migration test doesn't test actual migrations

**File**: `migration.integration.test.ts`

The "idempotent migration" test (line 37) just re-runs `CREATE TABLE IF NOT EXISTS` for 2 of 10 tables. This doesn't test the actual migration system — it tests that SQLite's `IF NOT EXISTS` works. A real migration test would apply migration scripts in sequence against an older schema and verify the result.

---

## Summary

| Severity | Count | Key Theme |
|----------|-------|-----------|
| CRITICAL | 3 | WS mock invalidates integration tests; DB injection is fake; test ordering dependency |
| HIGH | 7 | False concurrency test; unused TestContext; duplicated code; scope mismatch; schema drift |

The biggest systemic issue is the tension between "integration test" and "everything is mocked." The `ws.js` mock means WS event fan-out is never truly tested, and the `db.js` mock via hoisting is fragile. The tests are well-structured mechanically but several test names promise more than the assertions verify.
