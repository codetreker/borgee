// PluginConnectionsPanel.test.tsx — #1049 client unit tests.
//
// Cover the four states (loading / error / empty / populated) plus
// form validation, submit (add + edit), and delete. No real network —
// `lib/api` is mocked.
//
// Notes (post-iteration run_3):
//   - The form no longer renders a `connection_id` input — the server
//     derives the id from (org|agent|channel), so any user-typed value
//     would be silently discarded. Tests assert the absence of the
//     input.
//   - Edit calls configurePluginConnection only (no remove-then-
//     configure). Server idempotency on (org|agent|channel) overwrites
//     in place; switching channel derives a new connection_id (orphan
//     cleanup is out of scope per acceptance-criteria.md).
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

  it('add form has no connection_id input — server derives the id', async () => {
    vi.mocked(api.fetchPluginConnections).mockResolvedValueOnce([]);
    render({ enrollmentId: 'enroll-1', agentId: 'agent-1' });
    await flushPromises();
    const addBtn = container!.querySelector(
      '[data-testid="plugin-connection-add-btn"]',
    ) as HTMLButtonElement;
    act(() => {
      addBtn.click();
    });
    expect(
      container!.querySelector('[data-testid="plugin-connection-form-connection-id"]'),
    ).toBeNull();
    expect(
      container!.querySelector('[data-testid="plugin-connection-form-channel-id"]'),
    ).not.toBeNull();
  });

  it('disables submit until channel_id is non-empty', async () => {
    vi.mocked(api.fetchPluginConnections).mockResolvedValueOnce([]);
    render({ enrollmentId: 'enroll-1', agentId: 'agent-1' });
    await flushPromises();
    const addBtn = container!.querySelector(
      '[data-testid="plugin-connection-add-btn"]',
    ) as HTMLButtonElement;
    act(() => {
      addBtn.click();
    });
    const chanInput = container!.querySelector(
      '[data-testid="plugin-connection-form-channel-id"]',
    ) as HTMLInputElement;
    const submit = container!.querySelector(
      '[data-testid="plugin-connection-form-submit"]',
    ) as HTMLButtonElement;
    expect(submit.disabled).toBe(true);
    act(() => {
      const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')!
        .set!;
      setter.call(chanInput, 'channel-x');
      chanInput.dispatchEvent(new Event('input', { bubbles: true }));
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
    const chanInput = container!.querySelector(
      '[data-testid="plugin-connection-form-channel-id"]',
    ) as HTMLInputElement;
    const submit = container!.querySelector(
      '[data-testid="plugin-connection-form-submit"]',
    ) as HTMLButtonElement;
    act(() => {
      const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')!
        .set!;
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

  it('edit flow calls configurePluginConnection only (no remove)', async () => {
    vi.mocked(api.fetchPluginConnections).mockResolvedValueOnce([
      {
        connection_id: 'borgee-plugin:abc',
        agent_id: 'agent-1',
        channel_id: 'chan-old',
        last_configured_at: 1234,
      },
    ]);
    vi.mocked(api.configurePluginConnection).mockResolvedValueOnce(undefined);
    vi.mocked(api.fetchPluginConnections).mockResolvedValueOnce([
      {
        connection_id: 'borgee-plugin:abc',
        agent_id: 'agent-1',
        channel_id: 'chan-new',
        last_configured_at: 2000,
      },
    ]);
    render({ enrollmentId: 'enroll-1', agentId: 'agent-1' });
    await flushPromises();
    const editBtn = container!.querySelector(
      '[data-testid="plugin-connection-edit-btn-borgee-plugin:abc"]',
    ) as HTMLButtonElement;
    act(() => {
      editBtn.click();
    });
    const chanInput = container!.querySelector(
      '[data-testid="plugin-connection-form-channel-id"]',
    ) as HTMLInputElement;
    const submit = container!.querySelector(
      '[data-testid="plugin-connection-form-submit"]',
    ) as HTMLButtonElement;
    act(() => {
      const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')!
        .set!;
      setter.call(chanInput, 'chan-new');
      chanInput.dispatchEvent(new Event('input', { bubbles: true }));
    });
    await act(async () => {
      submit.click();
    });
    await flushPromises();
    expect(api.configurePluginConnection).toHaveBeenCalledWith('enroll-1', 'agent-1', 'chan-new');
    expect(api.removePluginConnection).not.toHaveBeenCalled();
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

  it('confirm dialog focuses Cancel button and Escape dismisses', async () => {
    vi.mocked(api.fetchPluginConnections).mockResolvedValueOnce([
      {
        connection_id: 'borgee-plugin:abc',
        agent_id: 'agent-1',
        channel_id: 'chan-1',
        last_configured_at: 1234,
      },
    ]);
    render({ enrollmentId: 'enroll-1', agentId: 'agent-1' });
    await flushPromises();
    const delBtn = container!.querySelector(
      '[data-testid="plugin-connection-delete-btn-borgee-plugin:abc"]',
    ) as HTMLButtonElement;
    act(() => {
      delBtn.click();
    });
    await flushPromises();
    const cancel = container!.querySelector(
      '[data-testid="plugin-connection-cancel-delete-btn"]',
    ) as HTMLButtonElement;
    expect(cancel).not.toBeNull();
    expect(document.activeElement).toBe(cancel);
    // Escape dismisses the dialog.
    act(() => {
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    });
    await flushPromises();
    expect(
      container!.querySelector('[data-testid="plugin-connection-confirm-dialog"]'),
    ).toBeNull();
  });

  // run_4 a11y: focus trap (Tab + Shift+Tab cycle stays inside the
  // dialog) + focus return on close. Without these, `aria-modal="true"`
  // is a lie — keyboard users can Tab into elements behind the
  // confirm-delete dialog and lose context to <body> on dismiss.
  it('confirm dialog traps Tab cycle (Tab from last → first; Shift+Tab from first → last)', async () => {
    vi.mocked(api.fetchPluginConnections).mockResolvedValueOnce([
      {
        connection_id: 'borgee-plugin:abc',
        agent_id: 'agent-1',
        channel_id: 'chan-1',
        last_configured_at: 1234,
      },
    ]);
    render({ enrollmentId: 'enroll-1', agentId: 'agent-1' });
    await flushPromises();
    const delBtn = container!.querySelector(
      '[data-testid="plugin-connection-delete-btn-borgee-plugin:abc"]',
    ) as HTMLButtonElement;
    act(() => {
      delBtn.click();
    });
    await flushPromises();
    const cancel = container!.querySelector(
      '[data-testid="plugin-connection-cancel-delete-btn"]',
    ) as HTMLButtonElement;
    const confirm = container!.querySelector(
      '[data-testid="plugin-connection-confirm-delete-btn"]',
    ) as HTMLButtonElement;
    expect(cancel).not.toBeNull();
    expect(confirm).not.toBeNull();
    // Focus on last → Tab should wrap to first (Cancel).
    act(() => {
      confirm.focus();
    });
    expect(document.activeElement).toBe(confirm);
    act(() => {
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true }));
    });
    expect(document.activeElement).toBe(cancel);
    // Focus on first → Shift+Tab should wrap to last (Delete).
    act(() => {
      cancel.focus();
    });
    expect(document.activeElement).toBe(cancel);
    act(() => {
      document.dispatchEvent(
        new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, bubbles: true }),
      );
    });
    expect(document.activeElement).toBe(confirm);
  });

  it('confirm dialog returns focus to the Delete button after Cancel', async () => {
    vi.mocked(api.fetchPluginConnections).mockResolvedValueOnce([
      {
        connection_id: 'borgee-plugin:abc',
        agent_id: 'agent-1',
        channel_id: 'chan-1',
        last_configured_at: 1234,
      },
    ]);
    render({ enrollmentId: 'enroll-1', agentId: 'agent-1' });
    await flushPromises();
    const delBtn = container!.querySelector(
      '[data-testid="plugin-connection-delete-btn-borgee-plugin:abc"]',
    ) as HTMLButtonElement;
    // Simulate keyboard click (focus naturally lands on the button
    // before the user activates it; this is the real keyboard a11y
    // path the focus-return logic is designed for).
    act(() => {
      delBtn.focus();
      delBtn.click();
    });
    await flushPromises();
    expect(
      container!.querySelector('[data-testid="plugin-connection-confirm-dialog"]'),
    ).not.toBeNull();
    // Cancel → dialog unmounts → focus must return to the Delete
    // button that opened it (asserts ConfirmDeleteDialog cleanup).
    const cancel = container!.querySelector(
      '[data-testid="plugin-connection-cancel-delete-btn"]',
    ) as HTMLButtonElement;
    act(() => {
      cancel.click();
    });
    await flushPromises();
    expect(
      container!.querySelector('[data-testid="plugin-connection-confirm-dialog"]'),
    ).toBeNull();
    expect(document.activeElement).toBe(delBtn);
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
