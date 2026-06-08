// NodeDetail-connection-token.test.tsx
//
// Locks the client side of the "connection token shown once at create" fix:
//   ① with a token prop, "显示 Token" reveals the literal token value
//   ② the start command uses the CURRENT CLI form — `borgee install` with the
//      install subcommand + --server/--token/--dirs (the old subcommand-less
//      `npx … --server … --token …` form would fail with "unknown subcommand")
//   ③ without a token prop (e.g. after a page refresh — server never re-sends
//      it), the UI shows the "shown once, re-create to get a new one" hint and
//      no token value
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';

vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<typeof import('../lib/api')>('../lib/api');
  return {
    ...actual,
    fetchRemoteBindings: vi.fn().mockResolvedValue([]),
    createRemoteBinding: vi.fn(),
    deleteRemoteBinding: vi.fn(),
  };
});

import { NodeDetail } from '../components/NodeManager';
import { _clearUnsavedGuardsForTest } from '../hooks/useUnsavedChangesGuard';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

const fakeNode = {
  id: 'n-1',
  user_id: 'u-1',
  machine_name: 'my-server',
  last_seen_at: null,
  created_at: 1,
} as unknown as Parameters<typeof NodeDetail>[0]['node'];

const fakeChannels = [{ id: 'ch-1', name: 'general' }];

function renderWith(token?: string) {
  act(() => {
    root!.render(
      <NodeDetail
        node={fakeNode}
        token={token}
        online={true}
        channels={fakeChannels}
        onDelete={() => {}}
      />,
    );
  });
}

beforeEach(() => {
  _clearUnsavedGuardsForTest();
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => { root?.unmount(); });
  container?.remove();
  root = null;
  container = null;
  _clearUnsavedGuardsForTest();
  vi.restoreAllMocks();
});

describe('NodeDetail — connection token surfaced once at create', () => {
  it('① "显示 Token" reveals the token value passed from the create response', async () => {
    renderWith('tok-deadbeef');
    await act(async () => { await Promise.resolve(); });

    // hidden by default
    expect(container!.querySelector('.node-token')).toBeNull();

    const buttons = Array.from(container!.querySelectorAll('button')) as HTMLButtonElement[];
    const showBtn = buttons.find(b => b.textContent === '显示 Token');
    expect(showBtn).toBeDefined();
    act(() => showBtn!.click());

    const tokenEl = container!.querySelector('.node-token');
    expect(tokenEl).not.toBeNull();
    expect(tokenEl!.textContent).toBe('tok-deadbeef');
  });

  it('② start command uses the `install` subcommand with the real token', async () => {
    renderWith('tok-deadbeef');
    await act(async () => { await Promise.resolve(); });
    const cmd = container!.querySelector('.node-cmd-box code')!.textContent ?? '';
    // current CLI shape — install subcommand REQUIRED
    expect(cmd).toContain('@codetreker/borgee-remote-agent install');
    expect(cmd).toMatch(/--server\s+\S+\s+--token\s+\S+\s+--dirs\s+\S+/);
    // reveal so the real token lands in the command, then assert it
    const showBtn = Array.from(container!.querySelectorAll('button'))
      .find(b => b.textContent === '显示 Token') as HTMLButtonElement;
    act(() => showBtn.click());
    const shown = container!.querySelector('.node-cmd-box code')!.textContent ?? '';
    expect(shown).toContain('--token tok-deadbeef');
    // guard against regressing to the broken subcommand-less form
    expect(shown).not.toMatch(/borgee-remote-agent --server/);
  });

  it('③ without a token (post-refresh) shows the shown-once hint, no token value', async () => {
    renderWith(undefined);
    await act(async () => { await Promise.resolve(); });
    expect(container!.querySelector('.node-token-unavailable')).not.toBeNull();
    // no "显示 Token" toggle when there is nothing to show
    const showBtn = Array.from(container!.querySelectorAll('button'))
      .find(b => b.textContent === '显示 Token');
    expect(showBtn).toBeUndefined();
    // command still uses the install subcommand with a placeholder token
    const cmd = container!.querySelector('.node-cmd-box code')!.textContent ?? '';
    expect(cmd).toContain('@codetreker/borgee-remote-agent install');
    expect(cmd).toContain('--token <token>');
  });
});
