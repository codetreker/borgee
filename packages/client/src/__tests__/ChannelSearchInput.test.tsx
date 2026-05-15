// ChannelSearchInput.test.tsx — CHN-13.3 5 vitest cases pin content-lock.
//
// Cases:
//   ① ChannelSearchInput placeholder `搜索频道` byte-identical
//   ② aria-label `搜索频道` byte-identical
//   ③ debounce 200ms — onChange 不立即触发, 200ms 静默期后才触发 onSearch
//   ④ 空 q 也触发 onSearch("") — grep 检查项 (清空恢复全列表)
//   ⑤ 同义词反向 reject — source grep 0 hit (data-testid + className 例外)

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
// @ts-expect-error — node:module no @types/node
import { createRequire } from 'module';
import { ChannelSearchInput } from '../components/ChannelSearchInput';

const nodeRequire = createRequire(import.meta.url);
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const fs: any = nodeRequire('fs');
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const nodePath: any = nodeRequire('path');
// ESM workaround — __dirname undefined in `tsc -b` ESM emit.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const nodeUrl: any = nodeRequire('url');
const HERE = nodePath.dirname(nodeUrl.fileURLToPath(import.meta.url));

describe('CHN-13.3 ChannelSearchInput content-lock', () => {
  let container: HTMLDivElement | null = null;
  let root: Root | null = null;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    act(() => {
      root?.unmount();
    });
    container?.remove();
    container = null;
    root = null;
  });

  it('① placeholder `搜索频道` byte-identical', () => {
    const onSearch = vi.fn();
    act(() => {
      root!.render(<ChannelSearchInput onSearch={onSearch} />);
    });
    const input = container!.querySelector(
      '[data-testid="channel-search-input-field"]',
    ) as HTMLInputElement;
    expect(input).not.toBeNull();
    expect(input.placeholder).toBe('搜索频道');
  });

  it('② aria-label `搜索频道` byte-identical', () => {
    const onSearch = vi.fn();
    act(() => {
      root!.render(<ChannelSearchInput onSearch={onSearch} />);
    });
    const input = container!.querySelector(
      '[data-testid="channel-search-input-field"]',
    ) as HTMLInputElement;
    expect(input.getAttribute('aria-label')).toBe('搜索频道');
  });

  it('③ debounce 200ms — onSearch 200ms 静默期后触发 (跟 useUserLayout PUT_DEBOUNCE_MS 同源)', async () => {
    vi.useRealTimers();
    const onSearch = vi.fn();
    act(() => {
      root!.render(<ChannelSearchInput onSearch={onSearch} />);
    });
    // Wait for initial debounce to fire onSearch("").
    await new Promise((r) => setTimeout(r, 250));
    expect(onSearch).toHaveBeenCalled();
    onSearch.mockClear();
    // Use React's native input value setter so onChange fires properly.
    const input = container!.querySelector(
      '[data-testid="channel-search-input-field"]',
    ) as HTMLInputElement;
    const setter = Object.getOwnPropertyDescriptor(
      window.HTMLInputElement.prototype,
      'value',
    )!.set!;
    act(() => {
      setter.call(input, 'alpha');
      input.dispatchEvent(new Event('input', { bubbles: true }));
    });
    // 50ms 内不该触发.
    await new Promise((r) => setTimeout(r, 50));
    expect(onSearch).not.toHaveBeenCalled();
    // 250ms 后触发.
    await new Promise((r) => setTimeout(r, 250));
    expect(onSearch).toHaveBeenCalledWith('alpha');
  });

  it('④ 空 q 也触发 onSearch("") — 清空恢复全列表', async () => {
    vi.useRealTimers();
    const onSearch = vi.fn();
    act(() => {
      root!.render(<ChannelSearchInput initialQuery="alpha" onSearch={onSearch} />);
    });
    await new Promise((r) => setTimeout(r, 250));
    onSearch.mockClear();
    const input = container!.querySelector(
      '[data-testid="channel-search-input-field"]',
    ) as HTMLInputElement;
    const setter = Object.getOwnPropertyDescriptor(
      window.HTMLInputElement.prototype,
      'value',
    )!.set!;
    act(() => {
      setter.call(input, '');
      input.dispatchEvent(new Event('input', { bubbles: true }));
    });
    await new Promise((r) => setTimeout(r, 250));
    expect(onSearch).toHaveBeenCalledWith('');
  });

  it('⑤ 同义词反向 reject — source grep 0 hit (data-testid 例外)', () => {
    const p = nodePath.resolve(HERE, '..', 'components', 'ChannelSearchInput.tsx');
    const src: string = fs.readFileSync(p, 'utf8');
    // user-visible Chinese 反向 reject — 我们用 `搜索频道`.
    for (const tok of ['查找', '检索', '查询']) {
      expect(src.includes(tok)).toBe(false);
    }
    // English 同义词反向 reject (data-testid + className 例外, 但本组件
    // 完全没用 search/find/lookup/locate 之 className 或 testid).
    for (const tok of ['Find', 'Lookup', 'Locate']) {
      expect(src.includes(tok)).toBe(false);
    }
  });
});
