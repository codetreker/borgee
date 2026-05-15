// DescriptionEditor.test.tsx — CHN-10.3 5 vitest cases pin content-lock.
//
// Cases:
//   ① title `频道说明` + save `保存` + cancel `取消` exact-match
//   ② counter `{n}/500` 字面 + maxLength=500 exact-match
//   ③ click save → setChannelDescription called + onSaved fired
//   ④ length > 500 → error `频道说明不能超过 500 字符` exact-match
//   ⑤ 同义词反向 reject — source grep 0 hit (data-testid + className 例外)
//   + ChannelHeader empty description → null (不渲染)

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
// @ts-expect-error — node:module no @types/node
import { createRequire } from 'module';
import { DescriptionEditor } from '../components/DescriptionEditor';
import { ChannelHeader } from '../components/ChannelHeader';
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

describe('CHN-10.3 DescriptionEditor content lock', () => {
  it('① title `频道说明` + save `保存` + cancel `取消` exact-match', () => {
    act(() => {
      root!.render(
        <DescriptionEditor
          channelID="c1"
          initial=""
          onSaved={() => {}}
          onCancel={() => {}}
        />,
      );
    });
    const modal = container!.querySelector('[data-testid="description-editor"]');
    expect(modal).not.toBeNull();
    expect(modal!.querySelector('h3')!.textContent).toBe('频道说明');
    expect(
      container!.querySelector('[data-testid="description-save"]')!.textContent,
    ).toBe('保存');
    expect(
      container!.querySelector('[data-testid="description-cancel"]')!.textContent,
    ).toBe('取消');
  });

  it('② counter `{n}/500` 字面 + maxLength=500 exact-match', () => {
    act(() => {
      root!.render(
        <DescriptionEditor
          channelID="c1"
          initial="abc"
          onSaved={() => {}}
          onCancel={() => {}}
        />,
      );
    });
    const counter = container!.querySelector('.description-editor-counter');
    expect(counter!.textContent).toBe('3/500');
    const ta = container!.querySelector(
      '[data-testid="description-editor-input"]',
    ) as HTMLTextAreaElement;
    expect(ta.maxLength).toBe(500);
    // DESCRIPTION_MAX_LENGTH exact-match 跟 server const + GORM size:500.
    expect(api.DESCRIPTION_MAX_LENGTH).toBe(500);
  });

  it('③ click save → setChannelDescription called + onSaved fired', async () => {
    const setSpy = vi
      .spyOn(api, 'setChannelDescription')
      .mockResolvedValue({} as never);
    let savedWith: string | null = null;
    act(() => {
      root!.render(
        <DescriptionEditor
          channelID="c1"
          initial="hello"
          onSaved={(v) => {
            savedWith = v;
          }}
          onCancel={() => {}}
        />,
      );
    });
    const btn = container!.querySelector(
      '[data-testid="description-save"]',
    ) as HTMLButtonElement;
    act(() => {
      btn.click();
    });
    await flushAsync();
    expect(setSpy).toHaveBeenCalledWith('c1', 'hello');
    expect(savedWith).toBe('hello');
  });

  it('④ length > 500 → error 文案 exact-match', async () => {
    const setSpy = vi
      .spyOn(api, 'setChannelDescription')
      .mockResolvedValue({} as never);
    act(() => {
      root!.render(
        <DescriptionEditor
          channelID="c1"
          initial=""
          onSaved={() => {}}
          onCancel={() => {}}
        />,
      );
    });
    const ta = container!.querySelector(
      '[data-testid="description-editor-input"]',
    ) as HTMLTextAreaElement;
    // Bypass maxLength clamp for the test by setting value directly via React state path.
    // Simulate user paste over the limit via fireEvent change.
    const setter = Object.getOwnPropertyDescriptor(
      window.HTMLTextAreaElement.prototype,
      'value',
    )!.set!;
    setter.call(ta, 'a'.repeat(501));
    ta.dispatchEvent(new Event('input', { bubbles: true }));
    await flushAsync();
    const btn = container!.querySelector(
      '[data-testid="description-save"]',
    ) as HTMLButtonElement;
    act(() => {
      btn.click();
    });
    await flushAsync();
    // The textarea maxLength=500 prevents the >500 input from sticking, so the
    // clamp path is unreachable from the DOM. Force the boundary by directly
    // testing the error literal via a programmatic over-limit save:
    // Instead, assert error literal is reachable when value.length > 500 (set via React state directly).
    // We reload the component with initial > 500 to exercise the validation path.
    act(() => {
      root!.render(
        <DescriptionEditor
          channelID="c1"
          initial={'b'.repeat(501)}
          onSaved={() => {}}
          onCancel={() => {}}
        />,
      );
    });
    const btn2 = container!.querySelector(
      '[data-testid="description-save"]',
    ) as HTMLButtonElement;
    act(() => {
      btn2.click();
    });
    await flushAsync();
    const err = container!.querySelector(
      '[data-testid="description-editor-error"]',
    );
    expect(err).not.toBeNull();
    expect(err!.textContent).toBe('频道说明不能超过 500 字符');
    // setSpy may or may not have been called depending on browser maxLength
    // clamp; the exact literal is what's locked.
    void setSpy;
  });

  it('⑤ 同义词反向 reject + ChannelHeader empty → null', () => {
    // Source-level reverse-grep on user-visible Chinese tokens (data-testid +
    // className legitimately use English; we only check Chinese synonyms).
    const compPath = nodePath.resolve(
      HERE,
      '..',
      'components',
      'DescriptionEditor.tsx',
    );
    const headerPath = nodePath.resolve(
      HERE,
      '..',
      'components',
      'ChannelHeader.tsx',
    );
    const editorSrc: string = fs.readFileSync(compPath, 'utf8');
    const headerSrc: string = fs.readFileSync(headerPath, 'utf8');
    for (const tok of ['简介', '主题', '关于', '介绍']) {
      expect(editorSrc.includes(tok)).toBe(false);
      expect(headerSrc.includes(tok)).toBe(false);
    }
    // ChannelHeader empty → return null.
    act(() => {
      root!.render(<ChannelHeader description="" />);
    });
    expect(
      container!.querySelector('[data-testid="channel-header-description"]'),
    ).toBeNull();
    act(() => {
      root!.render(<ChannelHeader description={null} />);
    });
    expect(
      container!.querySelector('[data-testid="channel-header-description"]'),
    ).toBeNull();
    // Non-empty → renders + edit trigger 文案 `编辑`.
    act(() => {
      root!.render(<ChannelHeader description="hi" onEdit={() => {}} />);
    });
    expect(
      container!.querySelector('[data-testid="channel-header-description"]')!
        .textContent,
    ).toContain('hi');
    expect(
      container!.querySelector('[data-testid="description-edit-trigger"]')!
        .textContent,
    ).toBe('编辑');
  });
});
