// IterationTimeline.test.tsx — 4 vitest cases pin CV-4.2 设计 ②+③.
//
// Cases:
//   ① renders 4-state badges (pending/running/completed/failed) + intent_text
//   ② thumbnail src 复用 versionPreviewMap (设计 ② — 不缓存历史 thumbnail)
//   ③ empty state + onJump callback fires with versionID
//   ④ DoesNotWriteOwnCursor — 反向断言 sessionStorage borgee.cv4.cursor:* 0 hit
//      (设计 ③ — cursor 复用 RT-1.1 不写独立, 跟 DM-4 useDMEdit 同精神)

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import IterationTimeline from '../components/IterationTimeline';
import type { ArtifactIteration, IterationState } from '../lib/api';

let container: HTMLDivElement | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  if (typeof window !== 'undefined' && window.sessionStorage) {
    window.sessionStorage.clear();
  }
});

afterEach(() => {
  if (container) {
    container.remove();
    container = null;
  }
});

function makeRow(
  id: string,
  state: IterationState,
  intent: string,
  versionID: number | null,
): ArtifactIteration {
  return {
    id,
    artifact_id: 'art-1',
    requested_by: 'user-1',
    intent_text: intent,
    target_agent_id: 'agent-1',
    state,
    created_artifact_version_id: versionID,
    error_reason: null,
    created_at: 1700000000000,
    completed_at: state === 'completed' || state === 'failed' ? 1700000060000 : null,
  };
}

describe('IterationTimeline (CV-4 v2)', () => {
  it('① renders 4-state badges (pending/running/completed/failed) + intent_text', async () => {
    const rows: ArtifactIteration[] = [
      makeRow('it-1', 'pending', 'add login button', null),
      makeRow('it-2', 'running', 'tweak palette', null),
      makeRow('it-3', 'completed', 'fix typo', 42),
      makeRow('it-4', 'failed', 'broken render', null),
    ];
    const root = createRoot(container!);
    await act(async () => {
      root.render(React.createElement(IterationTimeline, { iterations: rows }));
    });
    for (const state of ['pending', 'running', 'completed', 'failed']) {
      const badge = container!.querySelector(`[data-cv4v2-badge="${state}"]`);
      expect(badge, `badge ${state} rendered`).toBeTruthy();
      expect(badge?.textContent).toBe(state);
    }
    expect(container!.textContent).toContain('add login button');
    expect(container!.textContent).toContain('broken render');
    await act(async () => { root.unmount(); });
  });

  it('② thumbnail src 复用 versionPreviewMap (设计 ② thumbnail history 不存)', async () => {
    const rows: ArtifactIteration[] = [makeRow('it-1', 'completed', 'rev1', 7)];
    const versionPreviewMap = { '7': 'https://cdn.example/preview-7.png' };
    const root = createRoot(container!);
    await act(async () => {
      root.render(
        React.createElement(IterationTimeline, { iterations: rows, versionPreviewMap }),
      );
    });
    const img = container!.querySelector('img[data-cv4v2-thumbnail="true"]') as HTMLImageElement | null;
    expect(img, 'thumbnail rendered').toBeTruthy();
    expect(img?.src).toBe('https://cdn.example/preview-7.png');
    expect(img?.getAttribute('loading')).toBe('lazy');
    await act(async () => { root.unmount(); });
  });

  it('③ empty state + onJump callback fires with versionID', async () => {
    // Empty state.
    const root1 = createRoot(container!);
    await act(async () => {
      root1.render(React.createElement(IterationTimeline, { iterations: [] }));
    });
    expect(container!.querySelector('[data-cv4v2-timeline="empty"]')).toBeTruthy();
    expect(container!.textContent).toContain('暂无 iteration 历史');
    await act(async () => { root1.unmount(); });

    // onJump callback.
    const onJump = vi.fn();
    const rows: ArtifactIteration[] = [makeRow('it-9', 'completed', 'go', 99)];
    const root2 = createRoot(container!);
    await act(async () => {
      root2.render(
        React.createElement(IterationTimeline, { iterations: rows, onJump }),
      );
    });
    const btn = container!.querySelector('button[data-cv4v2-jump="true"]') as HTMLButtonElement;
    expect(btn).toBeTruthy();
    await act(async () => { btn.click(); });
    expect(onJump).toHaveBeenCalledTimes(1);
    expect(onJump).toHaveBeenCalledWith('99');
    await act(async () => { root2.unmount(); });
  });

  it('④ DoesNotWriteOwnCursor — 设计 ③ 反向断言 borgee.cv4.cursor:* 未写', async () => {
    const rows: ArtifactIteration[] = [makeRow('it-1', 'completed', 'x', 1)];
    const root = createRoot(container!);
    await act(async () => {
      root.render(React.createElement(IterationTimeline, { iterations: rows }));
    });
    if (typeof window !== 'undefined' && window.sessionStorage) {
      for (let i = 0; i < window.sessionStorage.length; i++) {
        const key = window.sessionStorage.key(i);
        expect(key).not.toMatch(/^borgee\.cv4\.cursor:/);
      }
    }
    await act(async () => { root.unmount(); });
  });
});
