import React from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';

import HelperStatusPanel from '../components/HelperStatusPanel';
import { NavigationProvider } from '../components/Navigation/NavigationContext';
import type {
  CreateHelperEnrollmentResponse,
  HelperEnrollmentStatusView,
} from '../lib/api';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  if (root) {
    act(() => root!.unmount());
    root = null;
  }
  if (container) {
    document.body.removeChild(container);
    container = null;
  }
});

async function render(node: React.ReactElement) {
  await act(async () => {
    root!.render(<NavigationProvider initial="helper-status">{node}</NavigationProvider>);
  });
  await act(async () => {
    await Promise.resolve();
  });
}

const rows: HelperEnrollmentStatusView[] = [
  {
    enrollment_id: 'connected-1',
    host_label: 'Dev Mac',
    helper_device_id: 'device-1',
    allowed_categories: ['openclaw_config', 'status_collect'],
    status: 'connected',
    fresh: true,
    last_seen_at: 1778840000000,
    created_at: 1778839900000,
  },
  {
    enrollment_id: 'offline-1',
    host_label: 'Linux Host',
    allowed_categories: ['helper_lifecycle'],
    status: 'offline',
    fresh: false,
    last_seen_at: 1778830000000,
    created_at: 1778820000000,
  },
  {
    enrollment_id: 'revoked-1',
    host_label: 'Old Workstation',
    allowed_categories: ['openclaw_lifecycle'],
    status: 'revoked',
    fresh: false,
    last_seen_at: 1778830000000,
    created_at: 1778820000000,
    revoked_at: 1778850000000,
  },
  {
    enrollment_id: 'uninstalled-1',
    host_label: 'Retired Laptop',
    allowed_categories: ['status_collect'],
    status: 'uninstalled',
    fresh: false,
    created_at: 1778820000000,
    uninstalled_at: 1778860000000,
  },
  {
    enrollment_id: 'pending-1',
    host_label: 'Pending Host',
    allowed_categories: ['unknown_future_category'],
    status: 'pending',
    fresh: false,
    created_at: 1778820000000,
  },
];

describe('HelperStatusPanel', () => {
  it('renders connected, offline, revoked, uninstalled, and pending as distinct Helper states', async () => {
    await render(
      <HelperStatusPanel fetchEnrollments={() => Promise.resolve(rows)} />,
    );

    expect(container!.querySelector('[data-helper-status="connected"]')?.textContent).toContain(
      'Helper connected',
    );
    expect(container!.querySelector('[data-helper-status="offline"]')?.textContent).toContain(
      'Helper offline',
    );
    expect(container!.querySelector('[data-helper-status="revoked"]')?.textContent).toContain(
      'Helper revoked',
    );
    expect(container!.querySelector('[data-helper-status="uninstalled"]')?.textContent).toContain(
      'Helper uninstalled',
    );
    expect(container!.querySelector('[data-helper-status="pending"]')?.textContent).toContain(
      'Waiting for local Helper',
    );
    expect(container!.querySelector('[data-helper-status="revoked"]')?.textContent).not.toContain(
      'offline',
    );
    expect(
      container!.querySelector('[data-helper-status="uninstalled"]')?.textContent,
    ).not.toContain('offline');
  });

  it('shows last seen safely and never claims Configure OpenClaw success', async () => {
    await render(
      <HelperStatusPanel fetchEnrollments={() => Promise.resolve(rows)} />,
    );

    const text = container!.textContent ?? '';
    expect(text).toContain('Last seen');
    expect(text).toContain('No last seen yet');
    expect(text).not.toContain('Invalid Date');
    expect(text).not.toContain('Configure OpenClaw succeeded');
    expect(text).not.toContain('OpenClaw connected');
    expect(text).not.toContain('job succeeded');
  });

  it('renders allowed categories as non-actionable categories, including unknown values', async () => {
    await render(
      <HelperStatusPanel fetchEnrollments={() => Promise.resolve(rows)} />,
    );

    expect(
      container!.querySelector('[data-helper-category="openclaw_config"]')?.textContent,
    ).toContain('OpenClaw config');
    expect(
      container!.querySelector('[data-helper-category="unknown_future_category"]')?.textContent,
    ).toContain('unknown_future_category');
    expect(container!.querySelector('[data-helper-category="openclaw_config"]')?.tagName).not.toBe(
      'BUTTON',
    );
    expect(container!.querySelectorAll('[data-helper-action]')).toHaveLength(0);
  });

  it('does not call helper credential endpoints or remote-node fallback while loading status', async () => {
    const fetchEnrollments = vi.fn(() => Promise.resolve(rows));

    await render(
      <HelperStatusPanel fetchEnrollments={fetchEnrollments} />,
    );

    expect(fetchEnrollments).toHaveBeenCalledTimes(1);
    expect(JSON.stringify(fetchEnrollments.mock.calls)).not.toMatch(
      /\/claim|\/status|\/uninstall|remote\/nodes/,
    );
    expect(container!.textContent ?? '').not.toContain('connection_token');
    expect(container!.textContent ?? '').not.toContain('helper_credential');
    expect(container!.textContent ?? '').not.toContain('enrollment_secret');
  });

  it('renders Configure OpenClaw terminal states without claiming partial success', async () => {
    const terminalRows = [
      helperRow('queued-1', 'Queue Host', 'connected', 'queued', 'Configure OpenClaw queued'),
      helperRow('running-1', 'Running Host', 'connected', 'running', 'Configure OpenClaw running'),
      helperRow('success-1', 'Ready Host', 'connected', 'succeeded', 'Configure OpenClaw complete'),
      helperRow(
        'failed-1',
        'Failed Host',
        'connected',
        'failed',
        'Configure OpenClaw failed',
        'execution_failed',
        'OpenClaw restart failed',
        ['log-failed'],
      ),
      helperRow(
        'denied-1',
        'Denied Host',
        'connected',
        'denied',
        'Configure OpenClaw denied',
        'policy_denied',
        'policy handoff denied',
        ['log-denied'],
      ),
      helperRow(
        'revoked-ui-1',
        'Revoked Host',
        'revoked',
        'revoked',
        'Configure OpenClaw revoked',
        'revoked',
      ),
      helperRow(
        'manual-1',
        'Manual Host',
        'connected',
        'manual_debug',
        'Manual debug required',
        'ttl_expired',
        undefined,
        ['log-manual'],
      ),
    ] as HelperEnrollmentStatusView[];

    await render(
      <HelperStatusPanel
        
        fetchEnrollments={() => Promise.resolve(terminalRows)}
      />,
    );

    const text = container!.textContent ?? '';
    expect(text).toContain('Configure OpenClaw queued');
    expect(text).toContain('Configure OpenClaw running');
    expect(text).toContain('Configure OpenClaw complete');
    expect(text).toContain('Configure OpenClaw failed');
    expect(text).toContain('Configure OpenClaw denied');
    expect(text).toContain('Configure OpenClaw revoked');
    expect(text).toContain('Manual debug required');
    expect(text).not.toContain('OpenClaw connected');
    expect(text).not.toContain('Configure OpenClaw succeeded');
    expect(container!.querySelectorAll('[data-configure-openclaw-state="succeeded"]')).toHaveLength(
      1,
    );

    const deniedButton = Array.from(container!.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('Denied Host'),
    ) as HTMLButtonElement;
    await act(async () => {
      deniedButton.click();
    });

    const deniedText = container!.textContent ?? '';
    expect(deniedText).toContain('policy_denied');
    expect(deniedText).toContain('policy handoff denied');
    expect(deniedText).toContain('Log log-denied');
  });
});

function helperRow(
  enrollmentId: string,
  hostLabel: string,
  helperStatus: string,
  state: string,
  label: string,
  failureCode?: string,
  failureMessage?: string,
  logRefs: string[] = [],
) {
  return {
    enrollment_id: enrollmentId,
    host_label: hostLabel,
    allowed_categories: ['openclaw_config', 'openclaw_lifecycle'],
    status: helperStatus,
    fresh: helperStatus === 'connected',
    last_seen_at: 1778840000000,
    created_at: 1778839900000,
    revoked_at: helperStatus === 'revoked' ? 1778850000000 : undefined,
    configure_openclaw: {
      state,
      label,
      failure_code: failureCode,
      failure_message: failureMessage,
      audit_refs: [],
      log_refs: logRefs,
      steps: [
        {
          job_type: state === 'running' ? 'service.lifecycle' : 'openclaw.configure_agent',
          status: state === 'denied' ? 'failed' : state,
          failure_code: failureCode,
          failure_message: failureMessage,
          audit_refs: [],
          log_refs: logRefs,
        },
      ],
    },
  };
}

describe('HelperStatusPanel — Create enrollment (Add host) UI', () => {
  const fakeToken = 'enr-newhost-1.super-secret-shown-once-9f3b';
  const fakeInstallCommand = `sudo npx @codetreker/borgee-remote-agent install --server wss://borgee.example.com --token ${fakeToken}`;

  // React tracks controlled-input values via a hidden value tracker; setting
  // .value directly bypasses it. Use the native setter so React's onChange
  // actually fires (mirrors ArtifactCommentSearchBox.test.tsx::setReactInputValue).
  function setReactInputValue(input: HTMLInputElement, value: string) {
    const setter = Object.getOwnPropertyDescriptor(
      window.HTMLInputElement.prototype,
      'value',
    )!.set!;
    setter.call(input, value);
    input.dispatchEvent(new Event('input', { bubbles: true }));
  }

  function fakeCreateResponse(): CreateHelperEnrollmentResponse {
    return {
      enrollment_id: 'enr-newhost-1',
      host_label: 'stage2-test-host',
      allowed_categories: ['openclaw_config', 'status_collect'],
      enrollment_token: fakeToken,
      enrollment_secret: 'super-secret-shown-once-9f3b',
      enrollment_secret_expires_at: 1778839900000 + 15 * 60 * 1000,
      install_command: fakeInstallCommand,
    };
  }

  async function flush() {
    await act(async () => {
      await Promise.resolve();
    });
    await act(async () => {
      await Promise.resolve();
    });
  }

  it('clicks Add host, fills the form, mints a token, shows it once, and refreshes on Done', async () => {
    const createEnrollment = vi.fn(async () => fakeCreateResponse());
    const fetchEnrollments = vi
      .fn<() => Promise<HelperEnrollmentStatusView[]>>()
      // initial load: empty
      .mockResolvedValueOnce([])
      // after Done: now includes the new host (proves the list-refresh fired)
      .mockResolvedValueOnce([
        {
          enrollment_id: 'enr-newhost-1',
          host_label: 'stage2-test-host',
          allowed_categories: ['openclaw_config', 'status_collect'],
          status: 'pending',
          fresh: false,
          created_at: Date.now(),
        },
      ]);

    await render(
      <HelperStatusPanel
        fetchEnrollments={fetchEnrollments}
        createEnrollment={createEnrollment}
      />,
    );
    await flush();

    // Open the modal.
    const addBtn = container!.querySelector(
      '[data-action="add-helper-host"]',
    ) as HTMLButtonElement | null;
    expect(addBtn, 'Add host button visible').toBeTruthy();
    await act(async () => {
      addBtn!.click();
    });

    expect(container!.querySelector('[data-helper-create-modal]')).toBeTruthy();

    // Fill the host label.
    const label = container!.querySelector(
      '[data-helper-host-label]',
    ) as HTMLInputElement;
    await act(async () => {
      setReactInputValue(label, 'stage2-test-host');
    });

    // The two default-on categories are openclaw_config + status_collect; keep
    // them as-is so the POST payload matches what the test asserts below.

    // Submit.
    const submit = container!.querySelector(
      '[data-action="submit-helper-create"]',
    ) as HTMLButtonElement;
    await act(async () => {
      submit.click();
    });
    await flush();

    expect(createEnrollment).toHaveBeenCalledTimes(1);
    expect(createEnrollment).toHaveBeenCalledWith({
      host_label: 'stage2-test-host',
      allowed_categories: ['openclaw_config', 'status_collect'],
    });

    // Reveal view: token + install_command + warning all visible.
    const installEl = container!.querySelector(
      '[data-helper-install-command]',
    ) as HTMLTextAreaElement | null;
    const tokenEl = container!.querySelector(
      '[data-helper-enrollment-token]',
    ) as HTMLTextAreaElement | null;
    const warning = container!.querySelector(
      '[data-helper-create-warning]',
    ) as HTMLElement | null;

    expect(installEl?.value).toBe(fakeInstallCommand);
    expect(tokenEl?.value).toBe(fakeToken);
    expect(warning?.textContent ?? '').toContain('shown ONCE');

    // Done closes the modal AND triggers a list refresh.
    const done = container!.querySelector(
      '[data-action="close-helper-create-modal"]',
    ) as HTMLButtonElement;
    await act(async () => {
      done.click();
    });
    await flush();

    expect(container!.querySelector('[data-helper-create-modal]')).toBeNull();
    expect(fetchEnrollments).toHaveBeenCalledTimes(2);
    // The newly-created host now shows up in the list (proves the refresh,
    // not just the close).
    const text = container!.textContent ?? '';
    expect(text).toContain('stage2-test-host');
  });

  it('only renders the token in the reveal-view textareas — no other DOM surface leaks it', async () => {
    const fetchEnrollments = vi.fn(async () => [] as HelperEnrollmentStatusView[]);
    const createEnrollment = vi.fn(async () => fakeCreateResponse());

    await render(
      <HelperStatusPanel
        fetchEnrollments={fetchEnrollments}
        createEnrollment={createEnrollment}
      />,
    );
    await flush();

    (
      container!.querySelector('[data-action="add-helper-host"]') as HTMLButtonElement
    ).click();
    await flush();
    const label = container!.querySelector(
      '[data-helper-host-label]',
    ) as HTMLInputElement;
    await act(async () => {
      setReactInputValue(label, 'stage2-test-host');
    });
    await act(async () => {
      (
        container!.querySelector('[data-action="submit-helper-create"]') as HTMLButtonElement
      ).click();
    });
    await flush();

    // Count token occurrences. The token MUST appear exactly twice: once in
    // the install_command textarea (where it's embedded after --token), and
    // once in the enrollment_token textarea on its own. No other rendered
    // surface (no innerText of warnings / labels / status panels / list items)
    // is allowed to reproduce it.
    const tokens = (container!.innerHTML.match(/enr-newhost-1\.super-secret-shown-once-9f3b/g) ?? [])
      .length;
    expect(tokens, 'token should appear only in the two reveal textareas').toBe(2);

    // After Done: token is gone from the DOM entirely.
    await act(async () => {
      (
        container!.querySelector(
          '[data-action="close-helper-create-modal"]',
        ) as HTMLButtonElement
      ).click();
    });
    await flush();
    expect(container!.innerHTML).not.toContain('super-secret-shown-once');
  });

  it('disables Create until host_label is non-empty and at least one category is picked', async () => {
    const fetchEnrollments = vi.fn(async () => [] as HelperEnrollmentStatusView[]);
    const createEnrollment = vi.fn(async () => fakeCreateResponse());

    await render(
      <HelperStatusPanel
        fetchEnrollments={fetchEnrollments}
        createEnrollment={createEnrollment}
      />,
    );
    await flush();

    (
      container!.querySelector('[data-action="add-helper-host"]') as HTMLButtonElement
    ).click();
    await flush();

    const submit = container!.querySelector(
      '[data-action="submit-helper-create"]',
    ) as HTMLButtonElement;
    expect(submit.disabled, 'disabled when host_label empty').toBe(true);

    const label = container!.querySelector(
      '[data-helper-host-label]',
    ) as HTMLInputElement;
    await act(async () => {
      setReactInputValue(label, '   '); // whitespace only — still disabled
    });
    expect(submit.disabled, 'disabled when host_label is whitespace only').toBe(true);

    await act(async () => {
      setReactInputValue(label, 'My Host');
    });
    expect(submit.disabled, 'enabled once host_label is non-empty + default categories set').toBe(
      false,
    );

    // Uncheck both default categories → disabled again.
    const cb1 = container!.querySelector(
      '[data-helper-category-checkbox="openclaw_config"] input',
    ) as HTMLInputElement;
    const cb2 = container!.querySelector(
      '[data-helper-category-checkbox="status_collect"] input',
    ) as HTMLInputElement;
    await act(async () => {
      cb1.click();
      cb2.click();
    });
    expect(submit.disabled, 'disabled when zero categories selected').toBe(true);
  });
});
