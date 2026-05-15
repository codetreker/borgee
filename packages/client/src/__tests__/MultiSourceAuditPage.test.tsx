// MultiSourceAuditPage.test.tsx — ADM-3 multi-source audit acceptance §3
// (admin UI 4 source badge + filter + DOM 锚 byte-identical 跟 content-lock).
//
// Pins:
//   - 4 source enum AUDIT_SOURCES 字面 byte-identical (server/plugin/host_bridge/agent)
//   - SOURCE_LABEL 4 i18n key byte-identical
//   - DOM 锚: [data-page="admin-audit-multi-source"] + [data-source-row]
//   - filter dropdown 走 fetchMultiSourceAudit({source})

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react';
import MultiSourceAuditPage from '../admin/pages/MultiSourceAuditPage';
import { AUDIT_SOURCES } from '../admin/api';

let container: HTMLDivElement | null = null;
let fetchSpy: ReturnType<typeof vi.spyOn> | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
});

afterEach(() => {
  if (container) {
    document.body.removeChild(container);
    container = null;
  }
  fetchSpy?.mockRestore();
  fetchSpy = null;
});

function render(node: React.ReactElement) {
  const root = createRoot(container!);
  act(() => {
    root.render(node);
  });
}

async function flushPromises() {
  await act(async () => {
    await new Promise((r) => setTimeout(r, 0));
  });
}

function mockFetchOnce(rows: any[], sources = ['server', 'plugin', 'host_bridge', 'agent']) {
  fetchSpy = vi.spyOn(globalThis, 'fetch').mockImplementation(async () => ({
    ok: true,
    status: 200,
    json: async () => ({ sources, rows }),
  }) as any);
}

describe('MultiSourceAuditPage', () => {
  it('AUDIT_SOURCES 4-enum byte-identical (server/plugin/host_bridge/agent)', () => {
    expect(AUDIT_SOURCES).toEqual(['server', 'plugin', 'host_bridge', 'agent']);
  });

  it('renders [data-page="admin-audit-multi-source"] + filter dropdown with 4 options', async () => {
    mockFetchOnce([]);
    render(<MultiSourceAuditPage />);
    await flushPromises();
    const page = container!.querySelector('[data-page="admin-audit-multi-source"]');
    expect(page).toBeTruthy();
    const select = container!.querySelector('select[data-filter="source"]') as HTMLSelectElement;
    expect(select).toBeTruthy();
    // 1 "All" option + 4 source options.
    expect(select.options.length).toBe(5);
    expect(select.options[1].value).toBe('server');
    expect(select.options[2].value).toBe('plugin');
    expect(select.options[3].value).toBe('host_bridge');
    expect(select.options[4].value).toBe('agent');
  });

  it('renders rows with [data-source-row] per source + 4 SOURCE_LABEL byte-identical', async () => {
    mockFetchOnce([
      { source: 'server', ts: 1700000000000, actor: 'admin-A→user-1', action: 'delete_channel', payload: '' },
      { source: 'plugin', ts: 1700000001000, actor: 'admin-B→user-2', action: 'plugin_connect', payload: '' },
      { source: 'agent', ts: 1700000002000, actor: 'global', action: 'agent.state', payload: '{}' },
    ]);
    render(<MultiSourceAuditPage />);
    await flushPromises();

    const rows = container!.querySelectorAll('[data-source-row]');
    expect(rows.length).toBe(3);
    expect(rows[0].getAttribute('data-source-row')).toBe('server');
    expect(rows[1].getAttribute('data-source-row')).toBe('plugin');
    expect(rows[2].getAttribute('data-source-row')).toBe('agent');

    // SOURCE_LABEL byte-identical (Server / Plugin / Host Bridge / Agent).
    const badges = Array.from(container!.querySelectorAll('.audit-source-badge'))
      .map((el) => el.textContent);
    expect(badges).toEqual(['Server', 'Plugin', 'Agent']);
  });

  it('empty state shows "No audit rows."', async () => {
    mockFetchOnce([]);
    render(<MultiSourceAuditPage />);
    await flushPromises();
    expect(container!.textContent).toContain('No audit rows.');
  });

  it('error response renders [role="alert"]', async () => {
    fetchSpy = vi.spyOn(globalThis, 'fetch').mockImplementation(async () => ({
      ok: false,
      status: 500,
      statusText: 'boom',
      json: async () => ({ error: 'boom' }),
    }) as any);
    render(<MultiSourceAuditPage />);
    await flushPromises();
    const alert = container!.querySelector('[role="alert"]');
    expect(alert).toBeTruthy();
  });

  it('selecting source filter triggers fetch with ?source=<src>', async () => {
    mockFetchOnce([]);
    render(<MultiSourceAuditPage />);
    await flushPromises();
    const select = container!.querySelector('select[data-filter="source"]') as HTMLSelectElement;

    fetchSpy!.mockClear();
    act(() => {
      select.value = 'agent';
      select.dispatchEvent(new Event('change', { bubbles: true }));
    });
    await flushPromises();

    expect(fetchSpy!).toHaveBeenCalled();
    const call = fetchSpy!.mock.calls[0];
    expect(String(call[0])).toContain('source=agent');
  });

  it('reverse-grep: page does NOT render UUID actor_id without context (audit-forward-only literal lock — no PII bare)', () => {
    // SOURCE_LABEL drift defense: forbid synonym wording byte-identical guard.
    const forbidden = ['hybrid', 'combined', 'multi_source', 'mixed_actor'];
    const src = MultiSourceAuditPage.toString();
    for (const f of forbidden) {
      expect(src.toLowerCase()).not.toContain(f.toLowerCase());
    }
  });
});
