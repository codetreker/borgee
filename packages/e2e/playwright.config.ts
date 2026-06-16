// playwright.config.ts — INFRA-2 scaffold for Phase 2.
//
// Two-server orchestration:
//   1. server-go (`go run ./cmd/collab`) on PORT=4901, sqlite db in tmp dir
//   2. vite dev server (`pnpm --filter @borgee/client dev`) on 5174
//      with a proxy override pointing /api /ws /uploads to :4901
//
// Why a separate port (4901, not 4900): local dev usually has the real
// collab on 4900; we want CI + local `pnpm test` to be safe to run without
// stopping the dev server.
//
// Why `webServer` array (not single): Playwright spins both up in
// parallel, waits for both health checks, then runs tests. On teardown
// both are killed.
//
// Auth strategy (this PR is scaffold only — no real auth fixture yet):
//   - Smoke test exercises `/health` (server-go) and `/` (client) only.
//   - CM-onboarding (#42) and RT-0 (#40) will add auth fixtures when they
//     need them. Pattern documented in fixtures/auth.ts (placeholder).
//
// Stopwatch helper for latency assertions (G2.4 requires ≤ 3s) lives in
// fixtures/stopwatch.ts. RT-0 will use it; INFRA-2 only adds the helper.
import { defineConfig, devices } from '@playwright/test';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import fs from 'node:fs';

type TraceMode = 'off' | 'on' | 'retain-on-failure' | 'on-first-retry' | 'retain-on-first-failure';
type VideoMode = 'off' | 'on' | 'retain-on-failure' | 'on-first-retry';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, '..', '..');

const SERVER_PORT = Number(process.env.E2E_SERVER_PORT ?? 4901);
const CLIENT_PORT = Number(process.env.E2E_CLIENT_PORT ?? 5174);
const CI_WORKERS = Number(process.env.E2E_CI_WORKERS ?? 2);
const SERVER_COMMAND =
  process.env.E2E_SERVER_COMMAND ?? 'go run -tags sqlite_fts5 ./cmd/collab';
const TRACE_MODE = (process.env.E2E_TRACE_MODE ?? (process.env.CI ? 'retain-on-failure' : 'on-first-retry')) as TraceMode;
const VIDEO_MODE = (process.env.E2E_VIDEO_MODE ?? 'retain-on-failure') as VideoMode;
const WEB_SERVER_LOG_MODE: 'ignore' | 'pipe' =
  process.env.CI && process.env.E2E_WEB_SERVER_LOGS !== '1' ? 'ignore' : 'pipe';
const SERVER_URL = `http://127.0.0.1:${SERVER_PORT}`;
const CLIENT_URL = `http://127.0.0.1:${CLIENT_PORT}`;

// #974 backend-off proof: when E2E_BACKEND_OFF=1 the server-go webServer entry
// is OMITTED — the backend is genuinely unreachable. Only vite (5174) boots, so
// the SPA still LOADS (vite serves the HTML/JS) but every backend data fetch and
// REST seed call hits a dead 4901 and fails. The `e2e-backend-off` CI job runs
// the @backend-required tagged subset under this flag and asserts the tagged
// tests RUN AND FAIL — proving they are genuinely backend-wired (a spec that
// passed here would not depend on the backend = a fake-green surface).
//
// We must NOT keep the server-go webServer entry in this mode: Playwright
// health-gates every webServer URL before running ANY test, so a /health gate
// against a dead backend would abort the whole run before a single test
// executes (an infra abort, not a test failure) and the "N tagged tests ran AND
// failed" assertion could never be evaluated.
const BACKEND_OFF = process.env.E2E_BACKEND_OFF === '1';

// One temp data dir per run keeps the sqlite db from leaking between
// suites; CI also wipes the runner's workspace, but local runs benefit.
// Server-go opens the sqlite file directly (no auto-mkdir), so we have
// to materialize the dir before webServer boots.
const dataDir = path.join(__dirname, '.playwright-data');
if (process.env.CI && process.env.TEST_WORKER_INDEX === undefined) {
  fs.rmSync(dataDir, { recursive: true, force: true });
}
fs.mkdirSync(path.join(dataDir, 'uploads'), { recursive: true });
fs.mkdirSync(path.join(dataDir, 'workspaces'), { recursive: true });

export default defineConfig({
  testDir: './tests',
  // CI keeps per-file ordering deterministic but runs two files at a time.
  // E2E_CI_WORKERS lets CI dial this back without changing committed config.
  // Local path: Playwright default workers, no retry, trace only on failure.
  fullyParallel: !process.env.CI,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? CI_WORKERS : undefined,
  reporter: process.env.CI
    ? [['github']]
    : [['list'], ['html', { open: 'never' }]],

  use: {
    baseURL: CLIENT_URL,
    trace: TRACE_MODE,
    screenshot: 'only-on-failure',
    video: VIDEO_MODE,
    // Attach SERVER_URL into test context so fixtures can hit the
    // server directly (e.g. seed users via REST) instead of clicking
    // through the UI for every preconditon.
    extraHTTPHeaders: {
      'X-E2E-Test': '1',
    },
  },

  projects: [
    {
      // Vanilla project: every spec EXCEPT the VM-dependent remote-node browse
      // spec. testIgnore keeps remote-node-browse.spec.ts out of the default
      // (no-VM) run so the vanilla CI `e2e` job never sweeps it VM-less.
      // NOTE: Playwright with no --project runs ALL projects, so the vanilla
      // job MUST pass --project=chromium (else remote-node-vm's testMatch would
      // re-include the VM spec). See .github/workflows/ci.yml.
      name: 'chromium',
      testIgnore: /remote-node-browse\.spec\.ts$/,
      use: { ...devices['Desktop Chrome'] },
    },
    {
      // VM-dependent project: ONLY the remote-node browse spec. Runs in the
      // dedicated `e2e-remote-node` CI job (which builds + brings up the
      // dev-vm). Longer per-test budget — the spec also sets
      // test.setTimeout(120_000).
      name: 'remote-node-vm',
      testMatch: /remote-node-browse\.spec\.ts$/,
      timeout: 120_000,
      use: { ...devices['Desktop Chrome'] },
    },
  ],

  // Order matters: server first (vite proxies to it). Playwright's
  // built-in `webServer` health check waits on each URL before starting
  // tests, so the server has to be reachable before the client tries
  // to proxy. In BACKEND_OFF mode the server-go entry is dropped so only
  // vite boots (see BACKEND_OFF note above).
  webServer: [
    // server-go backend — OMITTED when E2E_BACKEND_OFF=1.
    ...(BACKEND_OFF
      ? []
      : [
          {
            // CI can provide a prebuilt binary to avoid paying go run compile/startup
            // cost inside the Playwright webServer phase.
            command: SERVER_COMMAND,
            cwd: path.join(repoRoot, 'packages/server-go'),
            url: `${SERVER_URL}/health`,
            timeout: 60_000,
            reuseExistingServer: !process.env.CI,
            env: {
              PORT: String(SERVER_PORT),
              HOST: '127.0.0.1',
              NODE_ENV: 'development',
              DEV_AUTH_BYPASS: 'false',
              DATABASE_PATH: path.join(dataDir, 'collab-e2e.db'),
              SQLITE_TXLOCK: 'immediate',
              UPLOAD_DIR: path.join(dataDir, 'uploads'),
              WORKSPACE_DIR: path.join(dataDir, 'workspaces'),
              CLIENT_DIST: path.join(repoRoot, 'packages/client/dist'),
              JWT_SECRET: 'e2e-test-secret-not-for-prod',
              ADMIN_USER: 'e2e-admin',
              ADMIN_PASSWORD: 'e2e-admin-password-12345',
              // ADM-0.1 bootstrap is intentionally fail-fast: missing env vars
              // panic at startup. Without these the Playwright webServer
              // panics on boot and downstream PRs' e2e jobs all fail. The
              // password is bcrypt('e2e-admin-pass-12345', cost=10) — committed
              // because this is e2e-only data, never reachable from prod
              // (DATABASE_PATH is the .playwright-data tmp dir).
              // See docs/current/e2e/README.md §3.
              BORGEE_ADMIN_LOGIN: 'e2e-admin',
              BORGEE_ADMIN_PASSWORD_HASH:
                '$2a$10$4Qtu/ZynUPfAMPXPCtPa2uY7B04RVGK6V1gQfyihHgnW4LYvcY01i',
              BORGEE_TEST_FAST_BCRYPT: '1',
              BORGEE_TEST_FAST_ADMIN_PASSWORD: 'e2e-admin-pass-12345',
            },
            stdout: WEB_SERVER_LOG_MODE,
            stderr: WEB_SERVER_LOG_MODE,
          },
        ]),
    {
      // vite dev server with overridden proxy target. We can't edit
      // vite.config.ts at runtime, so we rely on the env var read by
      // vite.config.ts (added in this PR). Falls back to 4900 in normal
      // dev so existing devs aren't broken. In BACKEND_OFF mode vite still
      // boots and serves the SPA; its /api proxy just hits a dead 4901.
      command: `pnpm --filter @borgee/client dev --host 127.0.0.1 --port ${CLIENT_PORT} --strictPort`,
      cwd: repoRoot,
      url: CLIENT_URL,
      timeout: 60_000,
      reuseExistingServer: !process.env.CI,
      env: {
        VITE_E2E_API_TARGET: SERVER_URL,
        VITE_E2E_WS_RECONNECT_DELAY_MS: '50',
      },
      stdout: WEB_SERVER_LOG_MODE,
      stderr: WEB_SERVER_LOG_MODE,
    },
  ],
});
