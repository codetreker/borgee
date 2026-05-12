// ArtifactCommentSearchBox.test.tsx — CV-12.2 vitest acceptance.
//
// 锚: cv-12-stance-checklist.md §4 + content-lock §1+§2.
// 4 case: input DOM anchor / no_result 文案 byte-identical / result list /
// empty_query_no_api_call 反向断.

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react-dom/test-utils';

vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<typeof import('../lib/api')>('../lib/api');
  return {
    ...actual,
    searchArtifactComments: vi.fn(),
  };
});

import ArtifactCommentSearchBox from '../components/ArtifactCommentSearchBox';
import * as api from '../lib/api';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  vi.clearAllMocks();
});

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  if (container) {
    document.body.removeChild(container);
    container = null;
  }
});

async function render(node: React.ReactElement) {
  root = createRoot(container!);
  await act(async () => {
    root!.render(node);
  });
}

function setReactInputValue(input: HTMLInputElement, value: string) {
  // React tracks input values via a hidden value tracker; set via the
  // native setter so React's onChange fires properly.
  const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')!.set!;
  setter.call(input, value);
  input.dispatchEvent(new Event('input', { bubbles: true }));
}

describe('ArtifactCommentSearchBox — CV-12.2 client', () => {
  it('设计 ④ DOM data-cv12-search-input anchor (artifactId)', async () => {
    await render(<ArtifactCommentSearchBox artifactId="art-1" artifactChannelId="ch-1" />);
    const input = container!.querySelector('[data-cv12-search-input]') as HTMLInputElement;
    expect(input).not.toBeNull();
    expect(input.getAttribute('data-cv12-search-input')).toBe('art-1');
    expect(input.placeholder).toBe('搜索评论...');
  });

  it('设计 ④ 0 result 文案 "未找到匹配评论" byte-identical', async () => {
    (api.searchArtifactComments as ReturnType<typeof vi.fn>).mockResolvedValue({ messages: [] });
    await render(<ArtifactCommentSearchBox artifactId="art-2" artifactChannelId="ch-2" />);
    const input = container!.querySelector('[data-cv12-search-input]') as HTMLInputElement;
    await act(async () => {
      setReactInputValue(input, 'absent');
    });
    const submit = container!.querySelector('[data-testid="cv12-search-submit"]') as HTMLButtonElement;
    await act(async () => {
      submit.click();
    });
    for (let i = 0; i < 5; i++) {
      await act(async () => {
        await Promise.resolve();
      });
    }
    const noResult = container!.querySelector('[data-testid="cv12-no-result"]');
    expect(noResult).not.toBeNull();
    expect(noResult!.textContent).toBe('未找到匹配评论');
  });

  it('设计 ④ result list — 渲染 data-cv12-search-result-id 锚', async () => {
    (api.searchArtifactComments as ReturnType<typeof vi.fn>).mockResolvedValue({
      messages: [
        { id: 'msg-1', content: 'first match', sender_id: 'u-1', created_at: 1700000000000 },
        { id: 'msg-2', content: 'second match', sender_id: 'u-2', created_at: 1700000001000 },
      ],
    });
    await render(<ArtifactCommentSearchBox artifactId="art-3" artifactChannelId="ch-3" />);
    const input = container!.querySelector('[data-cv12-search-input]') as HTMLInputElement;
    await act(async () => {
      setReactInputValue(input, 'match');
    });
    const submit = container!.querySelector('[data-testid="cv12-search-submit"]') as HTMLButtonElement;
    await act(async () => {
      submit.click();
    });
    for (let i = 0; i < 5; i++) {
      await act(async () => {
        await Promise.resolve();
      });
    }
    const rows = container!.querySelectorAll('[data-cv12-search-result-id]');
    expect(rows.length).toBe(2);
    expect(rows[0].getAttribute('data-cv12-search-result-id')).toBe('msg-1');
    expect(rows[1].getAttribute('data-cv12-search-result-id')).toBe('msg-2');
  });

  it('设计 ④ 空 query 不调 API (反向断)', async () => {
    const spy = api.searchArtifactComments as ReturnType<typeof vi.fn>;
    await render(<ArtifactCommentSearchBox artifactId="art-4" artifactChannelId="ch-4" />);
    const submit = container!.querySelector('[data-testid="cv12-search-submit"]') as HTMLButtonElement;
    // Empty query keeps the submit button disabled; this is the reverse check for API calls.
    expect(submit.disabled).toBe(true);
    // Even if we force-click the button, no API call.
    await act(async () => {
      submit.click();
    });
    expect(spy).not.toHaveBeenCalled();
  });
});
