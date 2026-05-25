// Vitest coverage for issue #1050 — Install OpenClaw trigger UI.
//
// Surface under test: the "Install OpenClaw" button + confirmation modal +
// progress badge that HelperStatusPanel renders inside its detail section
// when the selected helper enrollment does not yet show a succeeded
// `openclaw.install_from_manifest` step.
//
// We mirror the harness used by HelperStatusPanel.test.tsx (createRoot +
// act + native value setters) so the two suites stay aligned and we do
// not bring in @testing-library here.

import React from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';

import HelperStatusPanel from '../components/HelperStatusPanel';
import { NavigationProvider } from '../components/Navigation/NavigationContext';
import type {
  HelperEnrollmentStatusView,
  InstallOpenClawJob,
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

async function flush() {
  await act(async () => {
    await Promise.resolve();
  });
  await act(async () => {
    await Promise.resolve();
  });
}

function aliveEnrollment(
  enrollmentId: string,
  hostLabel: string,
  overrides: Partial<HelperEnrollmentStatusView> = {},
): HelperEnrollmentStatusView {
  return {
    enrollment_id: enrollmentId,
    host_label: hostLabel,
    helper_device_id: 'device-1',
    allowed_categories: ['openclaw_lifecycle', 'openclaw_config'],
    status: 'connected',
    fresh: true,
    last_seen_at: 1778840000000,
    created_at: 1778839900000,
    ...overrides,
  };
}

function fakeJob(overrides: Partial<InstallOpenClawJob> = {}): InstallOpenClawJob {
  return {
    job_id: 'job-install-1',
    status: 'queued',
    job_type: 'openclaw.install_from_manifest',
    category: 'openclaw_lifecycle',
    enrollment_id: 'enr-alive-1',
    created_at: 1778840000123,
    ...overrides,
  };
}

describe('HelperStatusPanel — Install OpenClaw button visibility', () => {
  it('shows the button when helper is alive and OpenClaw is not yet installed', async () => {
    await render(
      <HelperStatusPanel
        fetchEnrollments={() => Promise.resolve([aliveEnrollment('enr-alive-1', 'Alive Host')])}
        installOpenClaw={async () => fakeJob()}
      />,
    );

    const btn = container!.querySelector('[data-action="install-openclaw"]');
    expect(btn, 'install-openclaw button is visible').toBeTruthy();
    expect(btn?.textContent).toContain('Install OpenClaw');
  });

  it('hides the button and shows an installed badge once an install step has succeeded', async () => {
    const installed = aliveEnrollment('enr-done-1', 'Done Host', {
      configure_openclaw: {
        state: 'succeeded',
        label: 'Configure OpenClaw complete',
        audit_refs: [],
        log_refs: [],
        steps: [
          {
            job_type: 'openclaw.install_from_manifest',
            status: 'succeeded',
            audit_refs: [],
            log_refs: [],
          },
        ],
      },
    });
    await render(
      <HelperStatusPanel
        fetchEnrollments={() => Promise.resolve([installed])}
        installOpenClaw={async () => fakeJob()}
      />,
    );

    expect(container!.querySelector('[data-action="install-openclaw"]')).toBeNull();
    expect(
      container!.querySelector('[data-helper-openclaw-badge="installed"]')?.textContent,
    ).toContain('OpenClaw installed');
  });

  it('hides the button while a job is queued/running (in-flight progress badge replaces it)', async () => {
    const running = aliveEnrollment('enr-running-1', 'Running Host', {
      configure_openclaw: {
        state: 'running',
        label: 'Configure OpenClaw running',
        audit_refs: [],
        log_refs: [],
        steps: [
          {
            job_type: 'openclaw.install_from_manifest',
            status: 'running',
            audit_refs: [],
            log_refs: [],
          },
        ],
      },
    });
    await render(
      <HelperStatusPanel
        fetchEnrollments={() => Promise.resolve([running])}
        installOpenClaw={async () => fakeJob()}
      />,
    );

    expect(container!.querySelector('[data-action="install-openclaw"]')).toBeNull();
    expect(
      container!.querySelector('[data-helper-openclaw-badge="progress"]')?.textContent,
    ).toContain('Installing OpenClaw');
  });

  it('hides the button when the helper is offline (not alive)', async () => {
    await render(
      <HelperStatusPanel
        fetchEnrollments={() =>
          Promise.resolve([
            aliveEnrollment('enr-offline-1', 'Offline Host', {
              status: 'offline',
              fresh: false,
            }),
          ])
        }
        installOpenClaw={async () => fakeJob()}
      />,
    );
    expect(container!.querySelector('[data-action="install-openclaw"]')).toBeNull();
  });

  it('hides the button when the helper does not allow the openclaw_lifecycle category', async () => {
    await render(
      <HelperStatusPanel
        fetchEnrollments={() =>
          Promise.resolve([
            aliveEnrollment('enr-no-cat-1', 'No Lifecycle Host', {
              allowed_categories: ['openclaw_config', 'status_collect'],
            }),
          ])
        }
        installOpenClaw={async () => fakeJob()}
      />,
    );
    expect(container!.querySelector('[data-action="install-openclaw"]')).toBeNull();
  });
});

describe('HelperStatusPanel — Install OpenClaw modal interaction', () => {
  it('opens the modal on click and Cancel closes without invoking the install API', async () => {
    const installOpenClaw = vi.fn<(enrollmentId: string) => Promise<InstallOpenClawJob>>(
      async () => fakeJob(),
    );
    await render(
      <HelperStatusPanel
        fetchEnrollments={() => Promise.resolve([aliveEnrollment('enr-alive-1', 'Alive Host')])}
        installOpenClaw={installOpenClaw}
      />,
    );

    const btn = container!.querySelector('[data-action="install-openclaw"]') as HTMLButtonElement;
    await act(async () => {
      btn.click();
    });

    const modal = container!.querySelector('[data-helper-install-openclaw-modal]');
    expect(modal, 'install modal opened').toBeTruthy();
    expect(modal?.getAttribute('role')).toBe('dialog');
    expect(modal?.getAttribute('aria-modal')).toBe('true');

    // Body surfaces the read-only facts that the operator should confirm.
    expect(
      container!.querySelector('[data-helper-install-openclaw-plugin-id]')?.textContent,
    ).toBe('openclaw');
    expect(
      container!.querySelector('[data-helper-install-openclaw-target-path]')?.textContent,
    ).toContain('/usr/local/lib/borgee/openclaw');
    expect(
      container!.querySelector('[data-helper-install-openclaw-host]')?.textContent,
    ).toBe('Alive Host');

    const cancel = container!.querySelector(
      '[data-action="cancel-install-openclaw"]',
    ) as HTMLButtonElement;
    await act(async () => {
      cancel.click();
    });

    expect(container!.querySelector('[data-helper-install-openclaw-modal]')).toBeNull();
    expect(installOpenClaw).not.toHaveBeenCalled();
  });

  it('Confirm calls installOpenClaw with the enrollment id and closes the modal on success', async () => {
    const installOpenClaw = vi.fn<(enrollmentId: string) => Promise<InstallOpenClawJob>>(
      async (id) => fakeJob({ enrollment_id: id }),
    );
    const fetchEnrollments = vi
      .fn<() => Promise<HelperEnrollmentStatusView[]>>()
      .mockResolvedValueOnce([aliveEnrollment('enr-alive-1', 'Alive Host')])
      // After enqueue → onEnqueued triggers a reload. Echo back the same
      // row so the panel keeps the same selected enrollment.
      .mockResolvedValueOnce([
        aliveEnrollment('enr-alive-1', 'Alive Host', {
          configure_openclaw: {
            state: 'queued',
            label: 'Configure OpenClaw queued',
            audit_refs: [],
            log_refs: [],
            steps: [
              {
                job_type: 'openclaw.install_from_manifest',
                status: 'queued',
                audit_refs: [],
                log_refs: [],
              },
            ],
          },
        }),
      ]);

    await render(
      <HelperStatusPanel
        fetchEnrollments={fetchEnrollments}
        installOpenClaw={installOpenClaw}
      />,
    );
    await flush();

    (container!.querySelector('[data-action="install-openclaw"]') as HTMLButtonElement).click();
    await flush();

    const confirm = container!.querySelector(
      '[data-action="confirm-install-openclaw"]',
    ) as HTMLButtonElement;
    await act(async () => {
      confirm.click();
    });
    await flush();

    expect(installOpenClaw).toHaveBeenCalledTimes(1);
    expect(installOpenClaw).toHaveBeenCalledWith('enr-alive-1');

    // Modal closed.
    expect(container!.querySelector('[data-helper-install-openclaw-modal]')).toBeNull();
    // The list refresh ran, picking up the new queued step.
    expect(fetchEnrollments).toHaveBeenCalledTimes(2);
    expect(
      container!.querySelector('[data-helper-openclaw-badge="progress"]')?.textContent,
    ).toContain('Installing OpenClaw');
  });

  it('renders an inline error if the install POST fails and keeps the modal open', async () => {
    const installOpenClaw = vi.fn(async () => {
      throw new Error('helper job enqueue forbidden');
    });
    await render(
      <HelperStatusPanel
        fetchEnrollments={() => Promise.resolve([aliveEnrollment('enr-alive-1', 'Alive Host')])}
        installOpenClaw={installOpenClaw}
      />,
    );
    await flush();

    (container!.querySelector('[data-action="install-openclaw"]') as HTMLButtonElement).click();
    await flush();

    await act(async () => {
      (
        container!.querySelector('[data-action="confirm-install-openclaw"]') as HTMLButtonElement
      ).click();
    });
    await flush();

    expect(container!.querySelector('[data-helper-install-openclaw-modal]')).toBeTruthy();
    const err = container!.querySelector('[data-helper-install-openclaw-error]');
    expect(err, 'inline error rendered').toBeTruthy();
    expect(err?.textContent).toContain('helper job enqueue forbidden');
  });

  it('reflects WS-driven configure_openclaw state changes in the progress badge after refresh', async () => {
    // Simulates the WS subscription delivering a transition by re-fetching
    // the enrollment list with the new aggregate state — same effect path
    // the real WS handler uses (it calls back into fetchHelperEnrollments).
    const states: HelperEnrollmentStatusView[][] = [
      [aliveEnrollment('enr-ws-1', 'WS Host')],
      [
        aliveEnrollment('enr-ws-1', 'WS Host', {
          configure_openclaw: {
            state: 'queued',
            label: 'Configure OpenClaw queued',
            audit_refs: [],
            log_refs: [],
            steps: [
              {
                job_type: 'openclaw.install_from_manifest',
                status: 'queued',
                audit_refs: [],
                log_refs: [],
              },
            ],
          },
        }),
      ],
      [
        aliveEnrollment('enr-ws-1', 'WS Host', {
          configure_openclaw: {
            state: 'succeeded',
            label: 'Configure OpenClaw complete',
            audit_refs: [],
            log_refs: [],
            steps: [
              {
                job_type: 'openclaw.install_from_manifest',
                status: 'succeeded',
                audit_refs: [],
                log_refs: [],
              },
            ],
          },
        }),
      ],
    ];
    let call = 0;
    const fetchEnrollments = vi.fn(async () => {
      const result = states[Math.min(call, states.length - 1)];
      call += 1;
      return result;
    });

    await render(
      <HelperStatusPanel
        fetchEnrollments={fetchEnrollments}
        installOpenClaw={async () => fakeJob({ enrollment_id: 'enr-ws-1' })}
      />,
    );
    await flush();

    expect(container!.querySelector('[data-action="install-openclaw"]')).toBeTruthy();

    // Trigger refresh that brings the queued step.
    (container!.querySelector('button.btn-sm:not([data-action="install-openclaw"])') as HTMLButtonElement);
    // Use the refresh button by data-action lookup that survives label changes.
    const buttons = Array.from(container!.querySelectorAll('button')) as HTMLButtonElement[];
    const refreshBtn = buttons.find((b) => (b.textContent ?? '').trim() === 'Refresh');
    expect(refreshBtn, 'refresh button present').toBeTruthy();
    await act(async () => {
      refreshBtn!.click();
    });
    await flush();
    expect(
      container!.querySelector('[data-helper-openclaw-badge="progress"]')?.textContent,
      'queued → progress badge',
    ).toContain('Installing OpenClaw');

    // Trigger refresh that brings the succeeded step.
    await act(async () => {
      refreshBtn!.click();
    });
    await flush();
    expect(
      container!.querySelector('[data-helper-openclaw-badge="installed"]')?.textContent,
      'succeeded → installed badge',
    ).toContain('OpenClaw installed');
    expect(container!.querySelector('[data-action="install-openclaw"]')).toBeNull();
  });
});
