import React from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';

import HelperStatusPanel from '../components/HelperStatusPanel';
import type { HelperEnrollmentStatusView } from '../lib/api';

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
    root!.render(node);
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
    await render(<HelperStatusPanel onBack={() => undefined} fetchEnrollments={() => Promise.resolve(rows)} />);

    expect(container!.querySelector('[data-helper-status="connected"]')?.textContent).toContain('Helper connected');
    expect(container!.querySelector('[data-helper-status="offline"]')?.textContent).toContain('Helper offline');
    expect(container!.querySelector('[data-helper-status="revoked"]')?.textContent).toContain('Helper revoked');
    expect(container!.querySelector('[data-helper-status="uninstalled"]')?.textContent).toContain('Helper uninstalled');
    expect(container!.querySelector('[data-helper-status="pending"]')?.textContent).toContain('Waiting for local Helper');
    expect(container!.querySelector('[data-helper-status="revoked"]')?.textContent).not.toContain('offline');
    expect(container!.querySelector('[data-helper-status="uninstalled"]')?.textContent).not.toContain('offline');
  });

  it('shows last seen safely and never claims Configure OpenClaw success', async () => {
    await render(<HelperStatusPanel onBack={() => undefined} fetchEnrollments={() => Promise.resolve(rows)} />);

    const text = container!.textContent ?? '';
    expect(text).toContain('Last seen');
    expect(text).toContain('No last seen yet');
    expect(text).not.toContain('Invalid Date');
    expect(text).not.toContain('Configure OpenClaw succeeded');
    expect(text).not.toContain('OpenClaw connected');
    expect(text).not.toContain('job succeeded');
  });

  it('renders allowed categories as non-actionable categories, including unknown values', async () => {
    await render(<HelperStatusPanel onBack={() => undefined} fetchEnrollments={() => Promise.resolve(rows)} />);

    expect(container!.querySelector('[data-helper-category="openclaw_config"]')?.textContent).toContain('OpenClaw config');
    expect(container!.querySelector('[data-helper-category="unknown_future_category"]')?.textContent).toContain('unknown_future_category');
    expect(container!.querySelector('[data-helper-category="openclaw_config"]')?.tagName).not.toBe('BUTTON');
    expect(container!.querySelectorAll('[data-helper-action]')).toHaveLength(0);
  });

  it('does not call helper credential endpoints or remote-node fallback while loading status', async () => {
    const fetchEnrollments = vi.fn(() => Promise.resolve(rows));

    await render(<HelperStatusPanel onBack={() => undefined} fetchEnrollments={fetchEnrollments} />);

    expect(fetchEnrollments).toHaveBeenCalledTimes(1);
    expect(JSON.stringify(fetchEnrollments.mock.calls)).not.toMatch(/\/claim|\/status|\/uninstall|remote\/nodes/);
    expect(container!.textContent ?? '').not.toContain('connection_token');
    expect(container!.textContent ?? '').not.toContain('helper_credential');
    expect(container!.textContent ?? '').not.toContain('enrollment_secret');
  });
});
