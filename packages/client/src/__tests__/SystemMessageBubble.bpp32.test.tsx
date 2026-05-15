// SystemMessageBubble.bpp32.test.tsx — BPP-3.2.2 三按钮 DOM 字面锁
// + content-lock §3 同义词grep 检查 + data-action enum 锁.
//
// 锚: docs/qa/bpp-3.2-content-lock.md §3 byte-identical:
//   <button data-bpp32-button="primary" data-action="grant">授权</button>
//   <button data-bpp32-button="danger"  data-action="reject">拒绝</button>
//   <button data-bpp32-button="ghost"   data-action="snooze">稍后</button>

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react';
import SystemMessageBubble, {
  isBPP32GrantPayload,
  type BPP32GrantPayload,
} from '../components/SystemMessageBubble';

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
});

const validBPP32: BPP32GrantPayload = {
  action: 'grant',
  agent_id: 'agent-x',
  capability: 'artifact.commit',
  scope: 'artifact:art-1',
  request_id: 'req-1',
};

function render(props: { bpp32?: BPP32GrantPayload | null; fallback?: any; onGrant?: any }) {
  act(() => {
    createRoot(container!).render(
      <SystemMessageBubble
        bodyHTML="AgentX 想 commit_artifact 但缺权限 commit_artifact"
        bpp32={props.bpp32}
        fallback={props.fallback}
        onGrant={props.onGrant}
      />,
    );
  });
}

describe('BPP-3.2.2 SystemMessageBubble three-button DOM lock (content-lock §3)', () => {
  it('renders three buttons byte-identical 跟 content-lock §3', () => {
    render({ bpp32: validBPP32 });
    const buttons = container!.querySelectorAll('[data-bpp32-grant="true"] button');
    expect(buttons.length).toBe(3);

    const want = [
      { label: '授权', action: 'grant', kind: 'primary' },
      { label: '拒绝', action: 'reject', kind: 'danger' },
      { label: '稍后', action: 'snooze', kind: 'ghost' },
    ];
    want.forEach((w, i) => {
      const b = buttons[i] as HTMLButtonElement;
      expect(b.textContent).toBe(w.label);
      expect(b.getAttribute('data-action')).toBe(w.action);
      expect(b.getAttribute('data-bpp32-button')).toBe(w.kind);
    });
  });

  it('反约束 §3 — 12 同义词禁词不出现在按钮 label', () => {
    render({ bpp32: validBPP32 });
    const labels = Array.from(container!.querySelectorAll('button')).map(b => b.textContent ?? '');
    const allText = labels.join(' ');
    const banned = [
      '批准', '授予', '同意', '许可',     // 替 "授权"
      '驳回', '拒接', '否决', '不允许', // 替 "拒绝"
      '稍候', '延后', '推迟', '暂缓', '过会儿', // 替 "稍后"
    ];
    for (const word of banned) {
      expect(allText).not.toContain(word);
    }
  });

  it('反约束 §3 — data-action enum 仅 3 值 (grant/reject/snooze)', () => {
    render({ bpp32: validBPP32 });
    const actions = Array.from(container!.querySelectorAll('[data-bpp32-grant="true"] button'))
      .map(b => b.getAttribute('data-action'));
    actions.forEach(a => {
      expect(['grant', 'reject', 'snooze']).toContain(a);
    });
  });

  it('button click → calls onGrant with the matching action enum', async () => {
    const onGrant = vi.fn().mockResolvedValue(undefined);
    render({ bpp32: validBPP32, onGrant });
    const grantBtn = container!.querySelector('button[data-action="grant"]') as HTMLButtonElement;
    const rejectBtn = container!.querySelector('button[data-action="reject"]') as HTMLButtonElement;
    const snoozeBtn = container!.querySelector('button[data-action="snooze"]') as HTMLButtonElement;

    await act(async () => { grantBtn.click(); });
    expect(onGrant).toHaveBeenCalledWith(expect.objectContaining({ action: 'grant', agent_id: 'agent-x' }));

    await act(async () => { rejectBtn.click(); });
    expect(onGrant).toHaveBeenLastCalledWith(expect.objectContaining({ action: 'reject' }));

    await act(async () => { snoozeBtn.click(); });
    expect(onGrant).toHaveBeenLastCalledWith(expect.objectContaining({ action: 'snooze' }));
  });

  it('does NOT render BPP-3.2 buttons when bpp32 prop is null (CM-onboarding fallback path)', () => {
    render({
      bpp32: null,
      fallback: { kind: 'button', label: '创建 agent', action: 'open_agent_manager' },
    });
    expect(container!.querySelectorAll('[data-bpp32-grant="true"]').length).toBe(0);
    const fb = container!.querySelectorAll('button[data-action="open_agent_manager"]');
    expect(fb.length).toBe(1);
    expect(fb[0].textContent).toBe('创建 agent');
  });
});

describe('BPP-3.2.2 isBPP32GrantPayload type guard', () => {
  it('accepts valid 3-enum action + 4 required fields', () => {
    expect(isBPP32GrantPayload(validBPP32)).toBe(true);
  });
  it('rejects null / non-object', () => {
    expect(isBPP32GrantPayload(null)).toBe(false);
    expect(isBPP32GrantPayload(undefined)).toBe(false);
    expect(isBPP32GrantPayload('grant')).toBe(false);
    expect(isBPP32GrantPayload(42)).toBe(false);
  });
  it('rejects CM-onboarding shape (kind=button)', () => {
    expect(isBPP32GrantPayload({ kind: 'button', label: '创建 agent', action: 'open_agent_manager' })).toBe(false);
  });
  it('rejects action 同义词 (approve/defer/allow)', () => {
    expect(isBPP32GrantPayload({ ...validBPP32, action: 'approve' })).toBe(false);
    expect(isBPP32GrantPayload({ ...validBPP32, action: 'defer' })).toBe(false);
  });
  it('rejects payload missing required fields', () => {
    expect(isBPP32GrantPayload({ ...validBPP32, agent_id: '' })).toBe(false);
    expect(isBPP32GrantPayload({ ...validBPP32, capability: '' })).toBe(false);
    expect(isBPP32GrantPayload({ ...validBPP32, scope: '' })).toBe(false);
    expect(isBPP32GrantPayload({ ...validBPP32, request_id: '' })).toBe(false);
  });
});
