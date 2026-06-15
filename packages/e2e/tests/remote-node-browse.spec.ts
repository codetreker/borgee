// tests/remote-node-browse.spec.ts — Remote Node browse end-to-end (bf-AC-3).
//
// Drives the FULL operator + browse journey through the real browser UI:
//   1. REST seed (preconditions only, mirroring agent-presence-online-flip):
//      admin login -> invite -> register owner -> create the owner's channel.
//   2. Browser as owner -> Settings -> Remote Nodes.
//   3. Create a node via the real UI (fill machine name, click 创建).
//   4. Read the connection token from the rendered <code class="node-token">
//      in the SAME session, immediately after create, with NO page.reload()
//      before the capture (the token is memory-only — surfaced once in the
//      create response, json:"-" strips it from list/get/status; a reload
//      wipes it). Captured into a JS const; later reloads are then safe.
//   5. Bind the fixture dir to the owner's channel via the real UI.
//   6. Enroll a real borgee daemon INSIDE the privileged systemd dev-vm
//      container, using the UI-obtained token. This is test orchestration of
//      the VM (the operator-CLI half) via docker exec — NOT a UI/feature
//      bypass; every browse action below stays a real UI click.
//   7. Assert the node flips ONLINE in the UI (.node-status-badge == 在线),
//      driven by re-mounting the Remote Nodes view (no realtime node push).
//   8. Open the channel -> Remote tab -> the binding -> assert:
//        - ls:   the fixture entries (readme.txt + the subdir) render.
//        - read: clicking readme.txt opens the viewer with the file content.
//        - stat-via-ls: readme.txt's size renders "14 B" + the subdir's icon
//          renders 📁 (the size + isDirectory fields the daemon's per-entry
//          os.Stat produces, asserted through the UI, no fetch).
//
// Implementation constraints (project rules — e2e_no_curl_only_ui):
//   - Browser-driven UI assertions only — no in-page network call, no
//     API-request-context, no request interception for the feature.
//   - REST is used ONLY for precondition seeding (admin/invite/owner/channel),
//     via the top-level @playwright/test request context (NOT page-scoped).
//   - The daemon-enroll docker exec is VM orchestration, not a feature bypass.
//   - All Playwright waits have explicit timeouts (no infinite loops).
//   - This spec runs ONLY in the dedicated `remote-node-vm` Playwright project
//     (the dev-vm-dependent CI job). It FAILS LOUDLY if the VM/daemon cannot
//     come up — there is no skip path.
import { test, expect, request as apiRequest } from '@playwright/test';
import { execFileSync } from 'node:child_process';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

// ── VM / fixture constants ─────────────────────────────────────────────────
const VM_USER = 'borgee'; // non-root user the daemon runs as (root is refused).
const FIXTURE_DIR = `/home/${VM_USER}/testdir`; // == daemon --dirs AND the bound path.
const FIXTURE_FILE = 'readme.txt';
const FIXTURE_BYTES = 'hello-from-vm\n'; // EXACTLY 14 bytes incl. trailing newline -> UI shows "14 B".
const FIXTURE_SUBDIR = 'sub'; // a directory entry to exercise isDirectory / 📁.
const COMPOSE = 'scripts/dev-vm/docker-compose.yml';
// Per-run overlay: host networking so the daemon inside the dev-vm can dial the
// loopback-bound harness (server-go listens on 127.0.0.1:4901 — see the file
// header in docker-compose.e2e.yml for why the bridge gateway IP can't reach a
// loopback-only listener).
const COMPOSE_OVERRIDE = 'scripts/dev-vm/docker-compose.e2e.yml';
const CONTAINER = 'borgee-vm';
const SERVER_PORT = process.env.E2E_SERVER_PORT ?? '4901';
// With host networking the daemon reaches the host loopback directly — no
// bridge-gateway resolution needed.
const SERVER_HOST = '127.0.0.1';
// repo root is two levels up from packages/e2e (where playwright runs).
const REPO_ROOT = new URL('../../..', import.meta.url).pathname;

// docker(...) runs a docker subcommand from the repo root, returning stdout.
function docker(args: string[], opts: { timeout?: number } = {}): string {
  return execFileSync('docker', args, {
    cwd: REPO_ROOT,
    encoding: 'utf8',
    timeout: opts.timeout ?? 60_000,
  });
}

// dexec runs a shell snippet inside the dev-vm as root (for setup/seed).
function dexec(script: string): string {
  return docker(['exec', CONTAINER, 'bash', '-lc', script]);
}

test.describe('bf-AC-3 remote-node browse (UI create -> daemon-in-VM -> ls/read/stat)', () => {
  // ── beforeAll: bring up the dev-vm, wait healthy, seed the fixture ────────
  test.beforeAll(async () => {
    // The image borgee-vm-base:latest is built by scripts/dev-vm/build-image.sh
    // (CI / local) BEFORE this runs. beforeAll only brings the container UP.
    // The e2e overlay adds host networking (loopback reach — see its header).
    docker(['compose', '-f', COMPOSE, '-f', COMPOSE_OVERRIDE, 'up', '-d'], {
      timeout: 180_000,
    });

    // Wait for systemd healthy (bounded poll). The dev-vm masks the always-
    // failing units so it reaches `running` (accept `degraded` too).
    let healthy = false;
    for (let i = 0; i < 30; i++) {
      let state = '';
      try {
        state = docker(['exec', CONTAINER, 'systemctl', 'is-system-running']).trim();
      } catch (err: unknown) {
        // is-system-running exits non-zero while `initializing`/`starting`;
        // execFileSync throws but still captures stdout on the error object.
        const e = err as { stdout?: Buffer | string };
        state = (e.stdout ?? '').toString().trim();
      }
      if (state === 'running' || state === 'degraded') {
        healthy = true;
        break;
      }
      await new Promise((r) => setTimeout(r, 3_000));
    }
    if (!healthy) {
      throw new Error(
        `dev-vm ${CONTAINER} never reached systemd running/degraded — ` +
          `aborting (no fake green). Last journal:\n` +
          docker(['logs', '--tail', '40', CONTAINER]),
      );
    }

    // Create the non-root user (idempotent).
    dexec(`id ${VM_USER} >/dev/null 2>&1 || useradd -m -s /bin/bash ${VM_USER}`);

    // Seed the fixture as that user with EXACT bytes (printf, not echo).
    dexec(
      `mkdir -p ${FIXTURE_DIR}/${FIXTURE_SUBDIR} && ` +
        `printf '${FIXTURE_BYTES}' > ${FIXTURE_DIR}/${FIXTURE_FILE} && ` +
        `chown -R ${VM_USER}:${VM_USER} ${FIXTURE_DIR}`,
    );
    // Fail loud if the fixture is not exactly 14 bytes (the "14 B" assertion).
    const size = dexec(`stat -c %s ${FIXTURE_DIR}/${FIXTURE_FILE}`).trim();
    if (size !== '14') {
      throw new Error(`fixture ${FIXTURE_FILE} is ${size} bytes, expected 14 (trailing newline missing?)`);
    }

    // Health gate (fail loud): the daemon's server must be reachable from
    // inside the VM BEFORE we enroll, else "node never goes online" is opaque.
    // With host networking the harness's 127.0.0.1:4901 listener is reachable
    // directly from the container.
    docker([
      'exec',
      CONTAINER,
      'curl',
      '-fsS',
      `http://${SERVER_HOST}:${SERVER_PORT}/health`,
    ]);
  });

  // ── afterAll: always tear down (no leak across runs) ──────────────────────
  test.afterAll(async () => {
    // Stop the daemon best-effort, then remove the privileged container + vols.
    try {
      docker(['exec', CONTAINER, 'pkill', '-u', VM_USER, '-f', 'borgee']);
    } catch {
      // daemon may already be gone — ignore.
    }
    docker(['compose', '-f', COMPOSE, '-f', COMPOSE_OVERRIDE, 'down', '-v'], {
      timeout: 120_000,
    });
  });

  test('owner creates node in UI -> daemon connects -> online -> browse ls/read/stat', async ({
    browser,
    baseURL,
  }) => {
    test.setTimeout(120_000);
    const serverURL = `http://127.0.0.1:${SERVER_PORT}`;

    // ── REST seed (preconditions only — NOT the feature) ────────────────────
    const adminCtx = await apiRequest.newContext({ baseURL: serverURL });
    const loginRes = await adminCtx.post('/admin-api/auth/login', {
      data: { login: ADMIN_LOGIN, password: ADMIN_PASSWORD },
    });
    expect(loginRes.ok(), `admin login: ${loginRes.status()}`).toBe(true);

    const inviteRes = await adminCtx.post('/admin-api/v1/invites', {
      data: { note: 'bf-ac3-remote-browse' },
    });
    expect(inviteRes.ok(), `mint invite: ${inviteRes.status()}`).toBe(true);
    const inviteCode = ((await inviteRes.json()) as { invite: { code: string } }).invite.code;

    const stamp = Date.now();
    const ownerCtx = await apiRequest.newContext({ baseURL: serverURL });
    const ownerReg = await ownerCtx.post('/api/v1/auth/register', {
      data: {
        invite_code: inviteCode,
        email: `rnb-owner-${stamp}@example.test`,
        password: 'p@ssw0rd-rnb',
        display_name: `RNB Owner ${stamp}`,
      },
    });
    expect(ownerReg.ok(), `owner register: ${ownerReg.status()}`).toBe(true);

    // Create the owner's channel via the OWNER context so the owner is a member
    // (the remote tab is gated isMember). Default visibility is fine.
    const channelName = `rnb-${stamp.toString(36)}`;
    const chRes = await ownerCtx.post('/api/v1/channels', {
      data: { name: channelName, visibility: 'private' },
    });
    expect(chRes.ok(), `create channel: ${chRes.status()} ${await chRes.text()}`).toBe(true);

    // ── Browser as owner ────────────────────────────────────────────────────
    const ownerStorage = await ownerCtx.storageState();
    const tokenCookie = ownerStorage.cookies.find((c) => c.name === 'borgee_token');
    expect(tokenCookie, 'borgee_token cookie should exist').toBeTruthy();

    const page = await browser.newPage();
    const url = new URL(baseURL!);
    await page.context().addCookies([
      {
        name: 'borgee_token',
        value: tokenCookie!.value,
        domain: url.hostname,
        path: '/',
        httpOnly: true,
        secure: false,
        sameSite: 'Lax',
      },
    ]);
    await page.goto('/');
    await expect(page.locator('.sidebar-title')).toBeVisible({ timeout: 15_000 });

    // ── Create node (real UI) ───────────────────────────────────────────────
    await page.locator('[data-action="open-settings"]').click();
    await page.locator('[data-runtime-entry="remote-nodes"]').click();
    await page.getByRole('button', { name: '+ 添加 Node' }).click();
    const machineName = `vm-node-${stamp}`;
    await page.locator('.node-create-form input[type="text"]').fill(machineName);
    await page.locator('.node-create-form').getByRole('button', { name: '创建' }).click();

    // The new node auto-selects -> NodeDetail header shows the name.
    await expect(page.locator('.node-detail-header h3')).toHaveText(machineName, { timeout: 10_000 });
    await expect(page.locator('.node-list-item', { hasText: machineName })).toBeVisible();

    // ── Read the token PURELY FROM THE UI — same session, NO reload yet ──────
    // The post-fix token is memory-only (create response -> NodeManager state).
    // Reveal it and read the rendered <code class="node-token"> text — a real
    // DOM read of a rendered element, not a fetch.
    await page.getByRole('button', { name: '显示 Token' }).click();
    const tokenEl = page.locator('.node-token');
    await expect(tokenEl).toBeVisible({ timeout: 10_000 });
    const token = (await tokenEl.innerText()).trim();
    expect(token, 'node connection token must be surfaced in the UI').toBeTruthy();

    // ── Bind the fixture dir to the owner's channel (real UI) ────────────────
    await page.getByRole('button', { name: '+ 绑定' }).click();
    await page
      .locator('.node-bind-form select')
      .selectOption({ label: `#${channelName}` });
    await page
      .locator('.node-bind-form input[placeholder^="远程路径"]')
      .fill(FIXTURE_DIR);
    await page.locator('.node-bind-form').getByRole('button', { name: '确定' }).click();
    await expect(page.locator('.node-binding-item')).toBeVisible({ timeout: 10_000 });

    // ── Enroll the daemon in the VM with the UI-obtained token ───────────────
    // Test orchestration (operator-CLI half) — the browse below stays UI.
    // daemon-direct (not install + systemctl --user): the dev-vm masks logind
    // so there is no user session bus. Run detached so the test proceeds.
    // --server ws://<host>:4901 -> the client dials /ws/remote and sends the
    // token on the Authorization: Bearer header (not in the URL).
    // host networking makes 127.0.0.1 the host harness from inside the VM.
    docker([
      'exec',
      '-d',
      CONTAINER,
      'sudo',
      '-u',
      VM_USER,
      'borgee-remote-agent',
      'daemon',
      '--server',
      `ws://${SERVER_HOST}:${SERVER_PORT}`,
      '--token',
      token,
      '--dirs',
      FIXTURE_DIR,
    ]);

    // ── Settle the daemon connect, then drive an online re-fetch (real UI) ───
    // There is no realtime node-status push; NodeManager fetches status only on
    // mount. Give the reverse-WS register a brief bounded settle, then re-mount
    // the Remote Nodes view (page.reload is SAFE now — token already captured).
    await page.waitForTimeout(3_000);
    await page.reload();
    await expect(page.locator('.sidebar-title')).toBeVisible({ timeout: 15_000 });
    await page.locator('[data-action="open-settings"]').click();
    await page.locator('[data-runtime-entry="remote-nodes"]').click();
    await page.locator('.node-list-item', { hasText: machineName }).click();

    const badge = page.locator('.node-status-badge');
    await expect(badge).toHaveText('在线', { timeout: 30_000 });
    await expect(badge).toHaveClass(/online/);

    // ── Open the channel from the sidebar -> Remote tab -> the binding ───────
    await page.locator('.channel-item', { hasText: channelName }).click();
    await expect(page.locator('.channel-view-tabs')).toBeVisible({ timeout: 15_000 });
    await page.locator('[data-tab="remote"]').click();
    await page.locator('.remote-binding-item').first().click();

    // ── ls: the fixture entries render (the .workspace-file-item list IS ls) ─
    const readmeEntry = page.locator('.workspace-file-item', { hasText: FIXTURE_FILE });
    await expect(readmeEntry).toBeVisible({ timeout: 15_000 });
    await expect(
      page.locator('.workspace-file-item', { hasText: FIXTURE_SUBDIR }),
    ).toBeVisible();

    // ── stat-via-ls: size + isDirectory rendered through the UI (no fetch) ───
    // readme.txt's size == "14 B" (the byte size from the stat-shaped DirEntry).
    await expect(readmeEntry.locator('.workspace-file-size')).toHaveText('14 B');
    // the subdir's icon == 📁 (the isDirectory field from the DirEntry).
    await expect(
      page.locator('.workspace-file-item', { hasText: FIXTURE_SUBDIR }).locator('.workspace-file-icon'),
    ).toHaveText('📁');

    // ── read: clicking the file opens the viewer with the file content ───────
    await readmeEntry.click();
    await expect(page.locator('.file-viewer-panel')).toBeVisible({ timeout: 10_000 });
    await expect(page.locator('.file-viewer-body')).toContainText('hello-from-vm');
    // the viewer header also renders the stat size.
    await expect(page.locator('.file-viewer-size')).toHaveText('14 B');
  });
});
