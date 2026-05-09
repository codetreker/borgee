// RuntimeCard.test.tsx — AL-4.3 (#379 §3 + #321 §2) DOM 字面锁单测.
//
// 闭环 acceptance §3.1-§3.4 + content-lock §2 grep 检查:
//   §3.1 设计 ② 4 态 data-runtime-status DOM lock — 'registered' /
//        'running' / 'stopped' / 'error' 严闭 (反约束: 不准
//        'starting' / 'stopping' / 'restarting' 中间态 v0)
//   §3.2 设计 ② owner-only btn DOM omit 反向断言 — 非 owner 视图无
//        start/stop btn (不仅 disabled, 直接 omit; 反约束:
//        disabled.*owner_id 0 hit)
//   §3.3 error 态 reason badge byte-identical 跟 lib/agent-state.ts
//        REASON_LABELS 同源 (改 = 改三处, AL-1a #249 设计 ④)
//   §3.4 反约束 — 不显示 endpoint_url / last_heartbeat_at 原始时间戳
//        (#321 §2 反约束 — 沉默胜于假精确)

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import RuntimeCard, { RUNTIME_STATUS_LABELS, RUNTIME_STATUS_TONES } from '../components/RuntimeCard';
import { REASON_LABELS } from '../lib/agent-state';
import type { Agent, AgentRuntime, AgentRuntimeStatus } from '../lib/api';

// Mock the api module so RuntimeCard's onClick handlers don't hit the
// network in unit tests. We also let tests inspect call args.
vi.mock('../lib/api', async (orig) => {
  const actual = await orig<typeof import('../lib/api')>();
  return {
    ...actual,
    startAgentRuntime: vi.fn(),
    stopAgentRuntime: vi.fn(),
  };
});

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
  if (container) document.body.removeChild(container);
  container = null;
  root = null;
});

const ownerID = 'u-owner';
const otherID = 'u-other';

const agent: Agent = {
  id: 'a-1',
  display_name: 'Agent Alpha',
  role: 'agent',
  avatar_url: null,
  owner_id: ownerID,
  created_at: 1700000000000,
};

function makeRuntime(overrides: Partial<AgentRuntime> = {}): AgentRuntime {
  return {
    id: 'rt-1',
    agent_id: 'a-1',
    endpoint_url: 'ws://shouldnotleak:9000/secret-token',
    process_kind: 'openclaw',
    status: 'running',
    last_error_reason: null,
    last_heartbeat_at: 1700000099999,
    created_at: 1700000000000,
    updated_at: 1700000050000,
    ...overrides,
  };
}

function render(props: React.ComponentProps<typeof RuntimeCard>) {
  act(() => {
    root!.render(<RuntimeCard {...props} />);
  });
}

describe('RuntimeCard — AL-4.3 acceptance §3 + #321 §2', () => {
  it('§3.1 4 态 data-runtime-status DOM lock', () => {
    const states: AgentRuntimeStatus[] = ['registered', 'running', 'stopped', 'error'];
    for (const s of states) {
      render({
        agent,
        runtime: makeRuntime({ status: s, last_error_reason: s === 'error' ? 'unknown' : null }),
        viewerUserID: ownerID,
        onRefresh: vi.fn(),
      });
      const card = container!.querySelector(`[data-runtime-status="${s}"]`);
      expect(card, `data-runtime-status="${s}" missing`).toBeTruthy();
    }
  });

  it('§3.1 反约束 — 不准 starting/stopping/restarting 中间态出现', () => {
    // 字面禁守 — RUNTIME_STATUS_LABELS keys 必须严格 4 态 (CHECK 镜像).
    const allowed = new Set(['registered', 'running', 'stopped', 'error']);
    const got = Object.keys(RUNTIME_STATUS_LABELS);
    expect(got.length).toBe(4);
    for (const k of got) {
      expect(allowed.has(k), `forbidden status "${k}" leaked into UI labels`).toBe(true);
    }
    for (const forbidden of ['starting', 'stopping', 'restarting']) {
      expect(got).not.toContain(forbidden);
    }
  });

  it('§3.2 owner-only — non-owner viewer DOM omit start/stop button', () => {
    // 非 owner 视图: viewerUserID !== agent.owner_id.
    render({
      agent,
      runtime: makeRuntime({ status: 'stopped' }),
      viewerUserID: otherID,
      onRefresh: vi.fn(),
    });
    // 反约束 (CV-1 ⑦ 同模式): button omit 不仅 disabled.
    expect(container!.querySelector('[data-runtime-action="start"]')).toBeNull();
    expect(container!.querySelector('[data-runtime-action="stop"]')).toBeNull();
    // status badge 仍渲染 (let non-owner see state, just not act on it).
    expect(container!.querySelector('[data-runtime-status="stopped"]')).toBeTruthy();
    // No actions wrapper at all (反约束 belt — `disabled.*owner_id` 0 hit).
    expect(container!.querySelector('[data-runtime-actions="owner"]')).toBeNull();
  });

  it('§3.2 owner-only — owner viewer sees start btn for stopped/error/registered', () => {
    for (const s of ['registered', 'stopped', 'error'] as AgentRuntimeStatus[]) {
      render({
        agent,
        runtime: makeRuntime({ status: s, last_error_reason: s === 'error' ? 'unknown' : null }),
        viewerUserID: ownerID,
        onRefresh: vi.fn(),
      });
      expect(container!.querySelector('[data-runtime-action="start"]'), `start btn missing in status=${s}`).toBeTruthy();
      expect(container!.querySelector('[data-runtime-action="stop"]'), `stop btn should be hidden in status=${s}`).toBeNull();
    }
  });

  it('§3.2 owner-only — owner viewer sees stop btn only for running', () => {
    render({
      agent,
      runtime: makeRuntime({ status: 'running' }),
      viewerUserID: ownerID,
      onRefresh: vi.fn(),
    });
    expect(container!.querySelector('[data-runtime-action="stop"]')).toBeTruthy();
    expect(container!.querySelector('[data-runtime-action="start"]')).toBeNull();
  });

  it('§3.2 反约束 — undefined / null viewerUserID 不渲染 start/stop btn', () => {
    // 防 leak — 未登录 / state 加载中 都视作非 owner 路径.
    render({
      agent,
      runtime: makeRuntime({ status: 'stopped' }),
      viewerUserID: null,
      onRefresh: vi.fn(),
    });
    expect(container!.querySelector('[data-runtime-action="start"]')).toBeNull();
    expect(container!.querySelector('[data-runtime-action="stop"]')).toBeNull();
  });

  it('§3.3 error 态 reason badge byte-identical 跟 REASON_LABELS 同源 (#249 设计 ④)', () => {
    for (const reason of Object.keys(REASON_LABELS) as Array<keyof typeof REASON_LABELS>) {
      render({
        agent,
        runtime: makeRuntime({ status: 'error', last_error_reason: reason }),
        viewerUserID: ownerID,
        onRefresh: vi.fn(),
      });
      const badge = container!.querySelector(`[data-error-reason="${reason}"]`);
      expect(badge, `reason badge missing for ${reason}`).toBeTruthy();
      // 字面 byte-identical 跟 REASON_LABELS 同源 (改 = 改三处:
      // server agent/state.go + lib/agent-state.ts + 此).
      expect(badge!.textContent).toBe(REASON_LABELS[reason]);
    }
  });

  it('§3.4 反约束 — 不显示 endpoint_url / last_heartbeat_at 原始时间戳 (#321 §2)', () => {
    // 沉默胜于假精确 (#321 §2 + #11). endpoint_url 是进程内部细节,
    // last_heartbeat_at 时间戳暴露 = 假精确.
    const rt = makeRuntime({
      endpoint_url: 'ws://shouldnotleak:9000/secret-token',
      last_heartbeat_at: 1700000099999,
    });
    render({ agent, runtime: rt, viewerUserID: ownerID, onRefresh: vi.fn() });
    const text = container!.textContent ?? '';
    // 反向断言: endpoint_url 字串不进文本.
    expect(text).not.toContain('shouldnotleak');
    expect(text).not.toContain('secret-token');
    // 反向断言: last_heartbeat_at 原始 Unix ms 不进文本.
    expect(text).not.toContain('1700000099999');
  });

  it('runtime null → graceful degrade omit (设计 ① "Borgee 不带 runtime")', () => {
    render({
      agent,
      runtime: null,
      viewerUserID: ownerID,
      onRefresh: vi.fn(),
    });
    // No card at all when no runtime registered.
    expect(container!.firstChild).toBeNull();
  });

  it('STATUS_TONES — 跟 PresenceDot/AL-1a 三色调一致 (改 = 改两处)', () => {
    // belt: 4 态 tone enum 严闭 ('ok' | 'muted' | 'error') 跟 lib/agent-state.ts
    // AgentStateLabel.tone 字面对齐.
    const allowed = new Set(['ok', 'muted', 'error']);
    for (const [s, t] of Object.entries(RUNTIME_STATUS_TONES)) {
      expect(allowed.has(t), `tone "${t}" outside palette for status "${s}"`).toBe(true);
    }
    expect(RUNTIME_STATUS_TONES.running).toBe('ok');
    expect(RUNTIME_STATUS_TONES.error).toBe('error');
    // registered + stopped 都是 muted (跟 PresenceDot offline 同视觉态).
    expect(RUNTIME_STATUS_TONES.registered).toBe('muted');
    expect(RUNTIME_STATUS_TONES.stopped).toBe('muted');
  });
});
