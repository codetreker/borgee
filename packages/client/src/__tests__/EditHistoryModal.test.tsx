// EditHistoryModal.test.tsx — DM-7.3 5 vitest cases pin content-lock §1+§2.
//
// Cases:
//   ① title `编辑历史` 4 字 + count `共 N 次编辑` exact-match
//   ② RFC3339 timestamp exact-match (ISO string)
//   ③ 空 history (length === 0) → return null (modal 不渲染)
//   ④ 加载失败 toast `加载编辑历史失败` exact-match
//   ⑤ 同义词反向 reject — 文件源 grep history(English text) / changes /
//      revisions / 版本 / 修订 / 变更 / 回退 0 hit (data-testid 例外)

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
// @ts-expect-error — node:module no @types/node
import { createRequire } from 'module';
import { EditHistoryModal } from '../components/EditHistoryModal';
import * as api from '../lib/api';

const nodeRequire = createRequire(import.meta.url);
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const fs: any = nodeRequire('fs');
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const nodePath: any = nodeRequire('path');
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const url: any = nodeRequire('url');
const HERE = nodePath.dirname(url.fileURLToPath(import.meta.url));

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  vi.restoreAllMocks();
});

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  container?.remove();
  container = null;
  root = null;
});

async function flushAsync() {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}

describe('DM-7.3 EditHistoryModal content lock', () => {
  it('① title `编辑历史` + count `共 N 次编辑` exact-match', async () => {
    vi.spyOn(api, 'getEditHistory').mockResolvedValue({
      history: [
        { old_content: 'v1', ts: 1700000000000, reason: 'unknown' },
        { old_content: 'v2', ts: 1700000060000, reason: 'unknown' },
      ],
    });
    act(() => {
      root!.render(
        <EditHistoryModal channelID="c1" messageID="m1" onClose={() => {}} />,
      );
    });
    await flushAsync();
    const modal = container!.querySelector('[data-testid="edit-history-modal"]');
    expect(modal).not.toBeNull();
    expect(modal!.querySelector('h3')!.textContent).toBe('编辑历史');
    expect(modal!.querySelector('.edit-history-count')!.textContent).toBe(
      '共 2 次编辑',
    );
  });

  it('② RFC3339 timestamp exact-match', async () => {
    const ts = 1700000000000;
    vi.spyOn(api, 'getEditHistory').mockResolvedValue({
      history: [{ old_content: 'old', ts, reason: 'unknown' }],
    });
    act(() => {
      root!.render(
        <EditHistoryModal channelID="c1" messageID="m1" onClose={() => {}} />,
      );
    });
    await flushAsync();
    const time = container!.querySelector('time.edit-history-ts')!;
    const want = new Date(ts).toISOString();
    expect(time.getAttribute('datetime')).toBe(want);
    expect(time.textContent).toBe(want);
    expect(want).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/);
  });

  it('③ empty history returns null (modal 不渲染)', async () => {
    vi.spyOn(api, 'getEditHistory').mockResolvedValue({ history: [] });
    act(() => {
      root!.render(
        <EditHistoryModal channelID="c1" messageID="m1" onClose={() => {}} />,
      );
    });
    await flushAsync();
    expect(container!.querySelector('[data-testid="edit-history-modal"]')).toBeNull();
  });

  it('④ load failure → `加载编辑历史失败` exact-match', async () => {
    vi.spyOn(api, 'getEditHistory').mockRejectedValue(new Error('boom'));
    act(() => {
      root!.render(
        <EditHistoryModal channelID="c1" messageID="m1" onClose={() => {}} />,
      );
    });
    await flushAsync();
    const err = container!.querySelector('[data-testid="edit-history-modal-error"]');
    expect(err).not.toBeNull();
    expect(err!.textContent).toBe('加载编辑历史失败');
  });

  it('⑤ 同义词反向 reject — source grep 0 hit', () => {
    const compPath = nodePath.resolve(
      HERE,
      '..',
      'components',
      'EditHistoryModal.tsx',
    );
    const src: string = fs.readFileSync(compPath, 'utf8');
    // forbidden Chinese tokens (permanently separated — 我们用 `编辑`).
    for (const tok of ['版本', '修订', '变更', '回退']) {
      expect(src.includes(tok)).toBe(false);
    }
    // English visible text reject — strip data-testid + className + import paths
    // (these legitimately use `history` literal). Test 字面 grep on rendered
    // user-visible text only.
    const visibleText = src
      .replace(/data-testid="[^"]*"/g, '')
      .replace(/className="[^"]*"/g, '')
      .replace(/from\s+['"][^'"]*['"]/g, '')
      .replace(/import[^;]*;/g, '')
      .replace(/getEditHistory|EditHistoryModal|DM7EditHistoryEntry|edit-history-[a-z-]+|edit_history|EditHistory|useState|useEffect/g, '')
      .replace(/\/\/.*$/gm, '')
      .replace(/\/\*[\s\S]*?\*\//g, '');
    for (const tok of ['changes', 'revisions', 'revs']) {
      expect(visibleText.toLowerCase().includes(tok)).toBe(false);
    }
  });
});
