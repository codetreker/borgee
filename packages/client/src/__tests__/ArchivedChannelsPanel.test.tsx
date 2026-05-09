// ArchivedChannelsPanel.test.tsx — CHN-5.3 archived panel DOM byte-
// identical + 文案 + 同义词grep 检查 + onRestore callback.
//
// Pins:
//   - <details> + <summary>已归档频道</summary> + data-testid="archived-channels-panel"
//   - 行 data-archived="true" + button data-action="restore" + 文案 `恢复`
//   - badge 文案 `已归档` byte-identical 跟 CHN-1.3 #288 SortableChannelItem
//   - 同义词反向: `存档/封存/还原/解档/重启/restore/archive` 0 hit
//   - empty state `暂无已归档频道` 文案
//   - onRestore callback fires when button clicked
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import { ArchivedChannelsPanel } from '../components/ArchivedChannelsPanel';
import * as api from '../lib/api';
import type { Channel } from '../types';

let container: HTMLDivElement | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
});

afterEach(() => {
  if (container) {
    document.body.removeChild(container);
    container = null;
  }
  vi.restoreAllMocks();
});

function makeChannel(id: string, name: string): Channel {
  return {
    id,
    name,
    org_id: 'org-1',
    creator_id: 'user-1',
    visibility: 'public',
    type: 'channel',
    archived_at: 1700000000000,
    created_at: 1700000000000,
  } as unknown as Channel;
}

async function flush() {
  await act(async () => {
    await new Promise(r => setTimeout(r, 0));
  });
}

describe('ArchivedChannelsPanel — CHN-5.3 DOM + 文案锁', () => {
  it('renders 列表 with byte-identical badge + button DOM', async () => {
    vi.spyOn(api, 'listArchivedChannels').mockResolvedValue([
      makeChannel('c-1', 'channel-a'),
      makeChannel('c-2', 'channel-b'),
    ]);
    const root = createRoot(container!);
    await act(async () => {
      root.render(<ArchivedChannelsPanel />);
    });
    await flush();

    const panel = container!.querySelector('[data-testid="archived-channels-panel"]');
    expect(panel).not.toBeNull();
    const summary = panel!.querySelector('summary');
    expect(summary?.textContent).toBe('已归档频道');

    const items = container!.querySelectorAll('[data-archived="true"]');
    expect(items.length).toBe(2);

    const badges = container!.querySelectorAll('.archived-badge');
    badges.forEach(b => expect(b.textContent).toBe('已归档'));

    const buttons = container!.querySelectorAll('button[data-action="restore"]');
    buttons.forEach(b => expect(b.textContent).toBe('恢复'));
  });

  it('empty state 字面 byte-identical', async () => {
    vi.spyOn(api, 'listArchivedChannels').mockResolvedValue([]);
    const root = createRoot(container!);
    await act(async () => {
      root.render(<ArchivedChannelsPanel />);
    });
    await flush();
    const empty = container!.querySelector('.archived-panel-empty');
    expect(empty?.textContent).toBe('暂无已归档频道');
  });

  it('clicking 恢复 button calls archiveChannel(id, false) + onRestore', async () => {
    vi.spyOn(api, 'listArchivedChannels').mockResolvedValue([
      makeChannel('c-1', 'channel-a'),
    ]);
    const archiveSpy = vi
      .spyOn(api, 'archiveChannel')
      .mockResolvedValue({} as Channel);
    const onRestore = vi.fn();
    const root = createRoot(container!);
    await act(async () => {
      root.render(<ArchivedChannelsPanel onRestore={onRestore} />);
    });
    await flush();

    const button = container!.querySelector('button[data-action="restore"]') as HTMLButtonElement;
    await act(async () => {
      button.click();
      await new Promise(r => setTimeout(r, 0));
    });

    expect(archiveSpy).toHaveBeenCalledWith('c-1', false);
    expect(onRestore).toHaveBeenCalledWith('c-1');
  });

  it('反向断言 — 同义词 0 出现在 DOM (`存档/封存/还原/解档/重启/restore-channel/archive-channel`)', async () => {
    vi.spyOn(api, 'listArchivedChannels').mockResolvedValue([
      makeChannel('c-1', 'channel-a'),
    ]);
    const root = createRoot(container!);
    await act(async () => {
      root.render(<ArchivedChannelsPanel />);
    });
    await flush();
    const html = container!.innerHTML;
    const forbidden = ['存档', '封存', '还原', '解档', '重启', 'restore-channel', 'archive-channel', 'unarchive-channel'];
    for (const f of forbidden) {
      expect(html).not.toContain(f);
    }
  });
});
