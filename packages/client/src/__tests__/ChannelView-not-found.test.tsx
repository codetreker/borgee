// ChannelView-not-found.test.tsx — #973 access-denied / not-found marker.
//
// Grounded fact: navigation is an in-app stack (NavigationContext), there are
// NO URL routes, so a forbidden CHANNEL (absent from the membership-scoped
// state.channels) and a forbidden DM (absent from state.dmChannels → isDm
// false) BOTH hit the `if (!channel && !isDm)` guard. Conflating forbidden
// with not-found is the SECURE choice — the copy must not leak whether the
// resource exists. This test pins:
//   1. the stable `data-channel-not-found` testability marker is present, so
//      the access-denied state is assertable in e2e;
//   2. the user-facing copy stays the single non-leaky "频道未找到" string
//      (no "you are forbidden" / existence leak).
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';

// The not-found branch returns before any heavy child renders; stub them so
// the import graph stays light and the test exercises only the guard.
vi.mock('../components/MessageList', () => ({ default: () => null }));
vi.mock('../components/MessageInput', () => ({ default: () => null }));
vi.mock('../components/ConnectionStatus', () => ({ default: () => null }));
vi.mock('../components/ChannelMembersModal', () => ({ default: () => null }));
vi.mock('../components/WorkspacePanel', () => ({ default: () => null }));
vi.mock('../components/RemotePanel', () => ({ default: () => null }));
vi.mock('../components/ArtifactPanel', () => ({ default: () => null }));

vi.mock('../lib/api', () => ({
  leaveChannel: vi.fn(),
  joinChannel: vi.fn(),
  fetchChannelPreview: vi.fn().mockResolvedValue({ messages: [] }),
  listCommands: vi.fn().mockResolvedValue({ agent: [] }),
}));

const appState = vi.hoisted(() => ({
  state: {
    channels: [] as Array<{ id: string }>,
    dmChannels: [] as Array<{ id: string; peer?: unknown }>,
    connectionState: 'connected',
    currentUser: { id: 'u-self', role: 'member' },
  },
  actions: {
    loadMessages: vi.fn(),
    loadChannels: vi.fn(),
  },
  sendWsMessage: vi.fn(),
}));

vi.mock('../context/AppContext', () => ({
  useAppContext: () => appState,
}));

vi.mock('../hooks/useVisualViewport', () => ({
  useVisualViewport: () => 0,
}));

import ChannelView from '../components/ChannelView';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  appState.state.channels = [];
  appState.state.dmChannels = [];
});

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  root = null;
  if (container) {
    document.body.removeChild(container);
    container = null;
  }
  vi.clearAllMocks();
});

async function render(node: React.ReactElement) {
  root = createRoot(container!);
  await act(async () => {
    root!.render(node);
  });
}

describe('ChannelView not-found / access-denied marker (#973)', () => {
  it('a forbidden / non-existent channel id renders the stable not-found marker', async () => {
    // `forbidden-channel-id` is absent from the membership-scoped channels and
    // dmChannels lists — exactly what a non-member would see for a private
    // channel they cannot access.
    await render(<ChannelView channelId="forbidden-channel-id" />);

    const marker = container!.querySelector('[data-channel-not-found="true"]');
    expect(marker, 'access-denied / not-found state must carry the testability marker').toBeTruthy();
    expect(marker?.textContent).toContain('频道未找到');
  });

  it('does not leak resource existence: copy is the single non-leaky string', async () => {
    await render(<ChannelView channelId="forbidden-channel-id" />);

    const text = container!.textContent ?? '';
    // The forbidden=not-found conflation must NOT distinguish "this exists but
    // you are forbidden" from "this does not exist" — no existence leak.
    expect(text).not.toMatch(/无权|禁止|forbidden|没有权限|access denied/i);
    expect(text).not.toContain('forbidden-channel-id');
  });
});
