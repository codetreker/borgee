// PluginConnectionsPanel.test.tsx — #1049 client unit tests.
//
// Cover the four states (loading / error / empty / populated) plus
// form validation and submit. No real network — `lib/api` is mocked.
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';

vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<typeof import('../lib/api')>('../lib/api');
  return {
    ...actual,
    fetchPluginConnections: vi.fn(),
    configurePluginConnection: vi.fn(),
    removePluginConnection: vi.fn(),
  };
});

import { PluginConnectionsPanel } from '../components/PluginConnectionsPanel';
import * as api from '../lib/api';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  container?.remove();
  container = null;
  root = null;
  vi.clearAllMocks();
});

function render(props: { enrollmentId: string; agentId: string }) {
  act(() => {
    root!.render(<PluginConnectionsPanel {...props} />);
  });
}

async function flushPromises() {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}

describe('PluginConnectionsPanel', () => {
  it('shows empty state when there are no connections', async () => {
    vi.mocked(api.fetchPluginConnections).mockResolvedValueOnce([]);
    render({ enrollmentId: 'enroll-1', agentId: 'agent-1' });
    await flushPromises();
    expect(container!.querySelector('[data-testid="plugin-connections-empty"]')).not.toBeNull();
    expect(container!.querySelector('[data-testid="plugin-connections-section"]')).not.toBeNull();
  });

  it('shows error state with retry button when fetch fails', async () => {
    vi.mocked(api.fetchPluginConnections).mockRejectedValueOnce(new Error('boom'));
    render({ enrollmentId: 'enroll-1', agentId: 'agent-1' });
    await flushPromises();
    const errEl = container!.querySelector('[data-testid="plugin-connections-error"]');
    expect(errEl).not.toBeNull();
    expect(errEl!.textContent).toContain('boom');
    expect(container!.querySelector('[data-testid="plugin-connections-retry-btn"]')).not.toBeNull();
  });

  it('renders a row per connection for the agent', async () => {
    vi.mocked(api.fetchPluginConnections).mockResolvedValueOnce([
      {
        connection_id: 'borgee-plugin:abc',
        agent_id: 'agent-1',
        channel_id: 'chan-1',
        last_configured_at: 1234,
      },
      {
        connection_id: 'borgee-plugin:other',
        agent_id: 'agent-other',
        channel_id: 'chan-2',
        last_configured_at: 5678,
      },
    ]);
    render({ enrollmentId: 'enroll-1', agentId: 'agent-1' });
    await flushPromises();
    const rows = container!.querySelectorAll('[data-testid="plugin-connection-row"]');
    expect(rows.length).toBe(1);
    expect(rows[0].getAttribute('data-connection-id')).toBe('borgee-plugin:abc');
  });

  it('disables the submit button when connection_id does not match the regex', async () => {
    vi.mocked(api.fetchPluginConnections).mockResolvedValueOnce([]);
    render({ enrollmentId: 'enroll-1', agentId: 'agent-1' });
    await flushPromises();
    const addBtn = container!.querySelector(
      '[data-testid="plugin-connection-add-btn"]',
    ) as HTMLButtonElement;
    act(() => {
      addBtn.click();
    });
    const connInput = container!.querySelector(
      '[data-testid="plugin-connection-form-connection-id"]',
    ) as HTMLInputElement;
    const chanInput = container!.querySelector(
      '[data-testid="plugin-connection-form-channel-id"]',
    ) as HTMLInputElement;
    const submit = container!.querySelector(
      '[data-testid="plugin-connection-form-submit"]',
    ) as HTMLButtonElement;
    // Simulate bad connection_id input.
    act(() => {
      const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')!
        .set!;
      setter.call(connInput, 'not-valid');
      connInput.dispatchEvent(new Event('input', { bubbles: true }));
      setter.call(chanInput, 'channel-x');
      chanInput.dispatchEvent(new Event('input', { bubbles: true }));
    });
    expect(submit.disabled).toBe(true);
    expect(
      container!.querySelector('[data-testid="plugin-connection-form-connection-id-error"]'),
    ).not.toBeNull();

    // Fix the connection_id so the submit enables.
    act(() => {
      const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')!
        .set!;
      setter.call(connInput, 'borgee-plugin:test-1');
      connInput.dispatchEvent(new Event('input', { bubbles: true }));
    });
    expect(submit.disabled).toBe(false);
  });

  it('submits add by calling configurePluginConnection then reloads', async () => {
    vi.mocked(api.fetchPluginConnections).mockResolvedValueOnce([]);
    vi.mocked(api.configurePluginConnection).mockResolvedValueOnce(undefined);
    vi.mocked(api.fetchPluginConnections).mockResolvedValueOnce([
      {
        connection_id: 'borgee-plugin:server-derived',
        agent_id: 'agent-1',
        channel_id: 'chan-x',
        last_configured_at: 9999,
      },
    ]);
    render({ enrollmentId: 'enroll-1', agentId: 'agent-1' });
    await flushPromises();
    const addBtn = container!.querySelector(
      '[data-testid="plugin-connection-add-btn"]',
    ) as HTMLButtonElement;
    act(() => {
      addBtn.click();
    });
    const connInput = container!.querySelector(
      '[data-testid="plugin-connection-form-connection-id"]',
    ) as HTMLInputElement;
    const chanInput = container!.querySelector(
      '[data-testid="plugin-connection-form-channel-id"]',
    ) as HTMLInputElement;
    const submit = container!.querySelector(
      '[data-testid="plugin-connection-form-submit"]',
    ) as HTMLButtonElement;
    act(() => {
      const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')!
        .set!;
      setter.call(connInput, 'borgee-plugin:test-1');
      connInput.dispatchEvent(new Event('input', { bubbles: true }));
      setter.call(chanInput, 'chan-x');
      chanInput.dispatchEvent(new Event('input', { bubbles: true }));
    });
    await act(async () => {
      submit.click();
    });
    await flushPromises();
    expect(api.configurePluginConnection).toHaveBeenCalledWith('enroll-1', 'agent-1', 'chan-x');
    expect(api.fetchPluginConnections).toHaveBeenCalledTimes(2);
  });

  it('confirms delete then calls removePluginConnection', async () => {
    vi.mocked(api.fetchPluginConnections).mockResolvedValueOnce([
      {
        connection_id: 'borgee-plugin:abc',
        agent_id: 'agent-1',
        channel_id: 'chan-1',
        last_configured_at: 1234,
      },
    ]);
    vi.mocked(api.removePluginConnection).mockResolvedValueOnce(undefined);
    vi.mocked(api.fetchPluginConnections).mockResolvedValueOnce([]);
    render({ enrollmentId: 'enroll-1', agentId: 'agent-1' });
    await flushPromises();
    const delBtn = container!.querySelector(
      '[data-testid="plugin-connection-delete-btn-borgee-plugin:abc"]',
    ) as HTMLButtonElement;
    act(() => {
      delBtn.click();
    });
    const confirm = container!.querySelector(
      '[data-testid="plugin-connection-confirm-delete-btn"]',
    ) as HTMLButtonElement;
    expect(confirm).not.toBeNull();
    await act(async () => {
      confirm.click();
    });
    await flushPromises();
    expect(api.removePluginConnection).toHaveBeenCalledWith(
      'enroll-1',
      'agent-1',
      'borgee-plugin:abc',
    );
  });

  it('shows loading state while the initial fetch is in flight', () => {
    let resolve!: (v: api.PluginConnectionView[]) => void;
    vi.mocked(api.fetchPluginConnections).mockImplementationOnce(
      () =>
        new Promise<api.PluginConnectionView[]>(r => {
          resolve = r;
        }),
    );
    render({ enrollmentId: 'enroll-1', agentId: 'agent-1' });
    expect(container!.querySelector('[data-testid="plugin-connections-loading"]')).not.toBeNull();
    // resolve to settle the act() warning
    act(() => {
      resolve([]);
    });
  });
});
