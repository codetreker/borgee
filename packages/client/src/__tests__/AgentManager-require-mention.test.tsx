// AgentManager-require-mention.test.tsx — owner My Agents 页 "仅 @mention 时
// 响应" toggle 行为锁.
//
// 4 case:
//   1. toggle 初始勾选状态从 agent.require_mention 反映 (true → checked)
//   2. 点击 toggle → 调 updateAgentRequireMention(agent.id, !prev)
//      + UI 状态翻动 + 显示 toast
//   3. API 失败 → 状态回滚到 prev (反假装翻动了) + 显示错误 toast
//   4. agent.require_mention=false → toggle 初始未勾

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';

vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<typeof import('../lib/api')>('../lib/api');
  return {
    ...actual,
    fetchAgents: vi.fn(),
    fetchAgent: vi.fn(),
    fetchAgentPermissions: vi.fn(),
    fetchAgentRuntime: vi.fn(),
    rotateAgentApiKey: vi.fn(),
    deleteAgent: vi.fn(),
    addAgentToChannel: vi.fn(),
    updateAgentPermissions: vi.fn(),
    updateAgentRequireMention: vi.fn(),
  };
});

const mockShowToast = vi.fn();
vi.mock('../components/Toast', () => ({
  useToast: () => ({ showToast: mockShowToast }),
  ToastProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

vi.mock('../context/AppContext', () => ({
  useAppContext: () => ({
    state: {
      channels: [{ id: 'ch-1', name: 'general' }],
      currentUser: { id: 'user-owner' },
    },
    dispatch: vi.fn(),
  }),
}));

vi.mock('../components/AgentConfigPanel', () => ({
  AgentConfigPanel: () => <div data-testid="agent-config-panel-stub" />,
}));

vi.mock('./RuntimeCard', () => ({
  default: () => <div data-testid="runtime-card-stub" />,
}));

import AgentManager from '../components/AgentManager';
import * as api from '../lib/api';

const mockAgent = {
  id: 'agent-1',
  display_name: 'Test Agent',
  role: 'agent' as const,
  avatar_url: '',
  owner_id: 'user-owner',
  created_at: 1700000000000,
  api_key: 'bgr_xxxx',
  require_mention: true,
  state: 'online' as const,
} as unknown as api.Agent;

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);

  mockShowToast.mockClear();

  vi.mocked(api.fetchAgent).mockResolvedValue(mockAgent);
  vi.mocked(api.fetchAgentPermissions).mockResolvedValue({ permissions: [], details: [] });
  vi.mocked(api.fetchAgentRuntime).mockResolvedValue(null);
  vi.mocked(api.rotateAgentApiKey).mockResolvedValue('bgr_xxxx');
  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: { writeText: vi.fn(), readText: vi.fn(async () => '') },
  });
});

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  container?.remove();
  root = null;
  container = null;
  vi.restoreAllMocks();
});

async function flush() {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}

function findManageButton(): HTMLButtonElement {
  const buttons = Array.from(container!.querySelectorAll('button'));
  const btn = buttons.find(b => b.textContent === 'Manage');
  if (!btn) throw new Error('Manage button not found');
  return btn as HTMLButtonElement;
}

function findToggle(): HTMLInputElement {
  const el = container!.querySelector('[data-testid="agent-require-mention-toggle"]') as HTMLInputElement;
  if (!el) throw new Error('require_mention toggle not found');
  return el;
}

describe('AgentManager — owner require_mention toggle', () => {
  it('toggle 初始勾选反映 agent.require_mention=true', async () => {
    vi.mocked(api.fetchAgents).mockResolvedValue([mockAgent]);
    await act(async () => { root!.render(<AgentManager onBack={() => {}} />); });
    await flush();
    await act(async () => { findManageButton().click(); });
    await flush();

    const toggle = findToggle();
    expect(toggle.checked).toBe(true);
  });

  it('agent.require_mention=false → toggle 初始未勾', async () => {
    const agentOff = { ...mockAgent, require_mention: false };
    vi.mocked(api.fetchAgents).mockResolvedValue([agentOff]);
    await act(async () => { root!.render(<AgentManager onBack={() => {}} />); });
    await flush();
    await act(async () => { findManageButton().click(); });
    await flush();

    expect(findToggle().checked).toBe(false);
  });

  it('点击 toggle → 调 updateAgentRequireMention(id, false) + 状态翻动 + 显示 toast', async () => {
    vi.mocked(api.fetchAgents).mockResolvedValue([mockAgent]);
    const updatedAgent = { ...mockAgent, require_mention: false };
    vi.mocked(api.updateAgentRequireMention).mockResolvedValue(updatedAgent);

    await act(async () => { root!.render(<AgentManager onBack={() => {}} />); });
    await flush();
    await act(async () => { findManageButton().click(); });
    await flush();

    await act(async () => { findToggle().click(); });
    await flush();

    expect(api.updateAgentRequireMention).toHaveBeenCalledWith('agent-1', false);
    expect(findToggle().checked).toBe(false);
    expect(mockShowToast).toHaveBeenCalledWith('已关闭: 任何消息都响应');
  });

  it('API 失败 → 状态回滚到 prev + 显示错误 toast (反假装翻动了)', async () => {
    vi.mocked(api.fetchAgents).mockResolvedValue([mockAgent]);
    vi.mocked(api.updateAgentRequireMention).mockRejectedValue(new Error('network down'));

    await act(async () => { root!.render(<AgentManager onBack={() => {}} />); });
    await flush();
    await act(async () => { findManageButton().click(); });
    await flush();

    const toggle = findToggle();
    expect(toggle.checked).toBe(true);

    await act(async () => { toggle.click(); });
    await flush();

    // 回滚 — 仍为 true.
    expect(findToggle().checked).toBe(true);
    expect(mockShowToast).toHaveBeenCalledWith('network down');
  });
});
