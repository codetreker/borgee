// AgentManager-detail.test.tsx — gh#684 §5 vitest unit 锁
// AgentManager 详情排版重组 + Credentials 卡 mask + 复制 + auto-clear 60s.
//
// 6 case 对应 brief §5 + §3 文案锁:
// 1. mask 模式 `bgr_...{last4}` 字面渲染 (反 Show 按钮 / 反完整 plaintext)
// 2. 复制按钮 aria-label / title byte-identical
// 3. Show 按钮 DOM 不再出现 (反向断言, 防回归)
// 4. 复制按钮点击 → clipboard.writeText 完整 key + toast 文案 byte-identical
// 5. auto-clear 60s setTimeout → readText 比对 → writeText('')
// 6. 反 OpenAI 前缀 `sk-` 不出现 (反向断言)

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
    revealAgentApiKey: vi.fn(),
    deleteAgent: vi.fn(),
    addAgentToChannel: vi.fn(),
    updateAgentPermissions: vi.fn(),
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
      channels: [{ id: 'ch-1', name: 'general' }, { id: 'ch-2', name: 'team' }],
      currentUser: { id: 'user-owner' },
    },
    dispatch: vi.fn(),
  }),
}));

// AgentConfigPanel mock — 内部走 fetchAgentConfig 拉 SSOT 字段, 跟 #684 不
// 相关; 替成 stub 返个 div 防 unhandled fetch 把测试搞崩.
vi.mock('../components/AgentConfigPanel', () => ({
  AgentConfigPanel: () => <div data-testid="agent-config-panel-stub" />,
}));

// RuntimeCard mock — 内部 owner-gated start/stop; #684 不动 runtime 卡内
// 容只动 wrapper, 替 stub 减依赖.
vi.mock('./RuntimeCard', () => ({
  default: () => <div data-testid="runtime-card-stub" />,
}));

import AgentManager from '../components/AgentManager';
import * as api from '../lib/api';

const TEST_KEY = 'bgr_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd';
const TEST_LAST4 = 'abcd';

// F7 (#1108): reads (fetchAgent / fetchAgents) carry only api_key_last4 — the
// full plaintext key is NEVER in the read shape. The component must render the
// mask from api_key_last4 and obtain the full key (for copy) via
// revealAgentApiKey only.
const mockAgent = {
  id: 'agent-1',
  display_name: 'Test Agent',
  role: 'agent' as const,
  avatar_url: '',
  owner_id: 'user-owner',
  created_at: 1700000000000,
  api_key_last4: TEST_LAST4,
  state: 'online' as const,
} as unknown as api.Agent;

let container: HTMLDivElement | null = null;
let root: Root | null = null;

// Stub clipboard API (jsdom 默认没).
const writeTextMock = vi.fn(async (_text: string) => {});
const readTextMock = vi.fn(async () => '');

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);

  vi.useFakeTimers({ shouldAdvanceTime: true });
  writeTextMock.mockClear();
  readTextMock.mockClear();
  mockShowToast.mockClear();
  // F7 (#1108): clear api-mock call history so the per-test "not.toHaveBeenCalled"
  // assertions are not polluted by a previous test's reveal/copy.
  vi.mocked(api.revealAgentApiKey).mockClear();
  vi.mocked(api.fetchAgent).mockClear();

  // jsdom navigator.clipboard 替 stub. 走 defineProperty 反原型链锁.
  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: { writeText: writeTextMock, readText: readTextMock },
  });

  vi.mocked(api.fetchAgents).mockResolvedValue([mockAgent]);
  vi.mocked(api.fetchAgent).mockResolvedValue(mockAgent);
  vi.mocked(api.fetchAgentPermissions).mockResolvedValue({ permissions: [], details: [] });
  vi.mocked(api.fetchAgentRuntime).mockResolvedValue(null);
  vi.mocked(api.rotateAgentApiKey).mockResolvedValue(TEST_KEY);
  // F7 (#1108): copy fetches the full key via revealAgentApiKey (POST), not the
  // redacted read.
  vi.mocked(api.revealAgentApiKey).mockResolvedValue(TEST_KEY);
});

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  container?.remove();
  root = null;
  container = null;
  vi.useRealTimers();
  vi.restoreAllMocks();
});

async function flush() {
  // 让 promise microtask + render flush.
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}

// 找 Manage 按钮 — `← Back` 也是 .btn-sm 在它前面, 不能直接拿第一个.
function findManageButton(): HTMLButtonElement {
  const buttons = Array.from(container!.querySelectorAll('button'));
  const btn = buttons.find(b => b.textContent === 'Manage');
  if (!btn) throw new Error('Manage button not found');
  return btn as HTMLButtonElement;
}

describe('#684 — AgentManager Credentials 卡 (mask + 复制 + auto-clear)', () => {
  it('mask 模式 bgr_...{last4} 字面渲染 (反 Show 按钮 / 反完整 plaintext)', async () => {
    await act(async () => {
      root!.render(<AgentManager onBack={() => {}} />);
    });
    await flush();

    // 展开 (Manage 按钮).
    const manageBtn = findManageButton();
    expect(manageBtn.textContent).toBe('Manage');
    await act(async () => {
      manageBtn.click();
    });
    await flush();

    // mask 渲染 byte-identical.
    const mask = container!.querySelector('[data-testid="agent-api-key-mask"]');
    expect(mask).not.toBeNull();
    expect(mask!.textContent).toBe(`bgr_...${TEST_LAST4}`);

    // 反向: 完整 plaintext 不进 DOM.
    const html = container!.innerHTML;
    expect(html).not.toContain(TEST_KEY);
  });

  it('Show 按钮 DOM 不再出现 (反向断言, 防回归)', async () => {
    await act(async () => {
      root!.render(<AgentManager onBack={() => {}} />);
    });
    await flush();
    const manageBtn = findManageButton();
    await act(async () => { manageBtn.click(); });
    await flush();

    // 反向: 没 Show 按钮.
    const buttons = Array.from(container!.querySelectorAll('button'));
    const showBtn = buttons.find(b => b.textContent === 'Show' || b.textContent === 'Hide');
    expect(showBtn).toBeUndefined();
  });

  it('复制按钮 aria-label + title byte-identical (yema brief §3 文案锁)', async () => {
    await act(async () => {
      root!.render(<AgentManager onBack={() => {}} />);
    });
    await flush();
    const manageBtn = findManageButton();
    await act(async () => { manageBtn.click(); });
    await flush();

    const copyBtn = container!.querySelector('button[aria-label="复制 API Key"]') as HTMLButtonElement;
    expect(copyBtn).not.toBeNull();
    expect(copyBtn.getAttribute('aria-label')).toBe('复制 API Key');
    expect(copyBtn.getAttribute('title')).toBe('复制完整 API Key 到剪贴板');
  });

  it('点击复制按钮 → clipboard.writeText 完整 key + toast "API Key 已复制, 60 秒后自动清空"', async () => {
    await act(async () => {
      root!.render(<AgentManager onBack={() => {}} />);
    });
    await flush();
    const manageBtn = findManageButton();
    await act(async () => { manageBtn.click(); });
    await flush();

    const copyBtn = container!.querySelector('button[aria-label="复制 API Key"]') as HTMLButtonElement;
    await act(async () => { copyBtn.click(); });
    await flush();

    // F7 (#1108): copy MUST obtain the full key via revealAgentApiKey (POST),
    // never via the redacted fetchAgent read.
    expect(api.revealAgentApiKey).toHaveBeenCalledWith('agent-1');
    expect(writeTextMock).toHaveBeenCalledWith(TEST_KEY);
    expect(mockShowToast).toHaveBeenCalledWith('API Key 已复制, 60 秒后自动清空');
  });

  it('F7 (#1108) — mask 渲染只靠 api_key_last4, 展开时组件不收完整 key + 复制走 reveal', async () => {
    // 读 shape 已脱敏: fetchAgent 返回的 mockAgent 不带 api_key, 只带
    // api_key_last4. 展开后 mask 仍要 byte-identical 渲染, 且完整 key 不进 DOM.
    await act(async () => {
      root!.render(<AgentManager onBack={() => {}} />);
    });
    await flush();
    const manageBtn = findManageButton();
    await act(async () => { manageBtn.click(); });
    await flush();

    // mask 来自 api_key_last4.
    const mask = container!.querySelector('[data-testid="agent-api-key-mask"]');
    expect(mask!.textContent).toBe(`bgr_...${TEST_LAST4}`);
    // 展开渲染期间不调 reveal (完整 key 不在展开时进组件).
    expect(api.revealAgentApiKey).not.toHaveBeenCalled();
    // 完整 plaintext 不进 DOM.
    expect(container!.innerHTML).not.toContain(TEST_KEY);

    // 点复制才走 reveal.
    const copyBtn = container!.querySelector('button[aria-label="复制 API Key"]') as HTMLButtonElement;
    await act(async () => { copyBtn.click(); });
    await flush();
    expect(api.revealAgentApiKey).toHaveBeenCalledWith('agent-1');
  });

  it('60s 后 auto-clear: readText 比对 + writeText("") + toast "剪贴板已清空"', async () => {
    readTextMock.mockResolvedValue(TEST_KEY);
    await act(async () => {
      root!.render(<AgentManager onBack={() => {}} />);
    });
    await flush();
    const manageBtn = findManageButton();
    await act(async () => { manageBtn.click(); });
    await flush();

    const copyBtn = container!.querySelector('button[aria-label="复制 API Key"]') as HTMLButtonElement;
    await act(async () => { copyBtn.click(); });
    await flush();

    // 复制成功 toast.
    expect(mockShowToast).toHaveBeenCalledWith('API Key 已复制, 60 秒后自动清空');

    // 推进 60s timer.
    await act(async () => {
      vi.advanceTimersByTime(60_000);
    });
    await flush();

    // readText 真的被调.
    expect(readTextMock).toHaveBeenCalled();
    // writeText('') 清剪贴板.
    expect(writeTextMock).toHaveBeenCalledWith('');
    // 二次 toast.
    expect(mockShowToast).toHaveBeenCalledWith('剪贴板已清空 (安全保护)');
  });

  it('60s 后 auto-clear: 用户改了剪贴板 → 不清 (反误清用户内容)', async () => {
    readTextMock.mockResolvedValue('user-pasted-something-else');
    await act(async () => {
      root!.render(<AgentManager onBack={() => {}} />);
    });
    await flush();
    const manageBtn = findManageButton();
    await act(async () => { manageBtn.click(); });
    await flush();

    const copyBtn = container!.querySelector('button[aria-label="复制 API Key"]') as HTMLButtonElement;
    await act(async () => { copyBtn.click(); });
    await flush();

    writeTextMock.mockClear();
    mockShowToast.mockClear();

    await act(async () => {
      vi.advanceTimersByTime(60_000);
    });
    await flush();

    expect(readTextMock).toHaveBeenCalled();
    // writeText 没被调 (反清用户内容).
    expect(writeTextMock).not.toHaveBeenCalledWith('');
    // 没二次 toast (反打扰).
    expect(mockShowToast).not.toHaveBeenCalledWith('剪贴板已清空 (安全保护)');
  });

  it('反 OpenAI 前缀 sk- 不出现 (反向断言, 防有人复制粘贴 OpenAI 文档误抄)', async () => {
    await act(async () => {
      root!.render(<AgentManager onBack={() => {}} />);
    });
    await flush();
    const manageBtn = findManageButton();
    await act(async () => { manageBtn.click(); });
    await flush();

    const html = container!.innerHTML;
    // 反 OpenAI mask `sk-...{last4}` 字面 (跟 brief §2.4 / §3 grep 守卫
    // `['\"]sk-\\.\\.\\.` 同模式 — 只锁字面字符串, 不误伤 CSS task-state).
    expect(html).not.toMatch(/sk-\.\.\./);
  });

  it('Rotate API Key 按钮文案保留 (反误删)', async () => {
    await act(async () => {
      root!.render(<AgentManager onBack={() => {}} />);
    });
    await flush();
    const manageBtn = findManageButton();
    await act(async () => { manageBtn.click(); });
    await flush();

    const buttons = Array.from(container!.querySelectorAll('button'));
    const rotateBtn = buttons.find(b => b.textContent === 'Rotate API Key');
    expect(rotateBtn).not.toBeUndefined();
  });
});
