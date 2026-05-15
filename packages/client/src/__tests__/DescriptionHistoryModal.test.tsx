// DescriptionHistoryModal.test.tsx — CHN-14.3 5 vitest cases pin content-lock.
//
// Cases:
//   ① title `编辑历史` 4 字 exact-match
//   ② empty `暂无编辑记录` 6 chars exact-match (empty history renders an explicit empty state)
//   ③ history row `: 修改了说明` exact-match (colon prefix plus space)
//   ④ 时间戳 RFC3339 exact-match
//   ⑤ synonym rejection — source grep 0 hits (except data-testid)

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
// @ts-expect-error — node:module no @types/node
import { createRequire } from 'module';
import { DescriptionHistoryModal } from '../components/DescriptionHistoryModal';
import * as api from '../lib/api';

const nodeRequire = createRequire(import.meta.url);
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const fs: any = nodeRequire('fs');
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const nodePath: any = nodeRequire('path');
// ESM workaround — __dirname undefined in `tsc -b` ESM emit.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const nodeUrl: any = nodeRequire('url');
const HERE = nodePath.dirname(nodeUrl.fileURLToPath(import.meta.url));

describe('CHN-14.3 DescriptionHistoryModal content-lock', () => {
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
    vi.restoreAllMocks();
  });

  it('① title `编辑历史` exact-match (跟 DM-7 EditHistoryModal 同源)', async () => {
    vi.spyOn(api, 'getChannelDescriptionHistory').mockResolvedValue({
      history: [{ old_content: 'old-v1', ts: 1700000000000, reason: 'unknown' }],
    });
    await act(async () => {
      root!.render(<DescriptionHistoryModal channelID="ch-1" onClose={() => {}} />);
    });
    await new Promise((r) => setTimeout(r, 50));
    const modal = container!.querySelector(
      '[data-testid="description-history-modal"]',
    );
    expect(modal).not.toBeNull();
    const h3 = modal!.querySelector('h3');
    expect(h3?.textContent).toBe('编辑历史');
  });

  it('② 空 history 显示 `暂无编辑记录` exact-match', async () => {
    vi.spyOn(api, 'getChannelDescriptionHistory').mockResolvedValue({
      history: [],
    });
    await act(async () => {
      root!.render(<DescriptionHistoryModal channelID="ch-2" onClose={() => {}} />);
    });
    await new Promise((r) => setTimeout(r, 50));
    const empty = container!.querySelector(
      '[data-testid="description-history-empty"]',
    );
    expect(empty).not.toBeNull();
    expect(empty!.textContent).toBe('暂无编辑记录');
  });

  it('③ history 行 action `: 修改了说明` exact-match', async () => {
    vi.spyOn(api, 'getChannelDescriptionHistory').mockResolvedValue({
      history: [{ old_content: 'foo', ts: 1700000000000, reason: 'unknown' }],
    });
    await act(async () => {
      root!.render(<DescriptionHistoryModal channelID="ch-3" onClose={() => {}} />);
    });
    await new Promise((r) => setTimeout(r, 50));
    const action = container!.querySelector('.description-history-action');
    expect(action).not.toBeNull();
    expect(action!.textContent).toBe(': 修改了说明');
  });

  it('④ 时间戳 RFC3339 exact-match (跟 DM-7 + CHN-1.2 同源)', async () => {
    const ts = 1700000000000;
    vi.spyOn(api, 'getChannelDescriptionHistory').mockResolvedValue({
      history: [{ old_content: 'foo', ts, reason: 'unknown' }],
    });
    await act(async () => {
      root!.render(<DescriptionHistoryModal channelID="ch-4" onClose={() => {}} />);
    });
    await new Promise((r) => setTimeout(r, 50));
    const time = container!.querySelector('time.description-history-ts');
    expect(time).not.toBeNull();
    const expected = new Date(ts).toISOString();
    expect(time!.textContent).toBe(expected);
    expect(time!.getAttribute('dateTime')).toBe(expected);
  });

  it('⑤ 同义词反向 reject — source grep 0 hit (data-testid + className 例外)', () => {
    const p = nodePath.resolve(HERE, '..', 'components', 'DescriptionHistoryModal.tsx');
    const src: string = fs.readFileSync(p, 'utf8');
    // User-visible Chinese synonym rejection; the approved copy is `编辑历史` /
    // `暂无编辑记录` / `修改了说明`.
    for (const tok of ['记录', '日志', '审计']) {
      // `记录` is part of the approved `暂无编辑记录` copy, so only independent
      // synonym tokens are checked here.
      if (tok === '记录') continue;
      expect(src.includes(tok)).toBe(false);
    }
    // English synonym rejection, with data-testid and className as the only exceptions.
    // The component must not use Audit / Log / History as user-visible copy.
    expect(src.includes('Audit')).toBe(false);
    expect(src.includes('Log')).toBe(false);
    // Reject rollback / restore wording; edit history must stay separate from restoration.
    expect(src.includes('回退')).toBe(false);
    expect(src.includes('恢复')).toBe(false);
  });
});
