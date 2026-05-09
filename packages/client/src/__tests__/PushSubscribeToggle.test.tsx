// CS-3.2 — PushSubscribeToggle 单测 (cs-3-stance-checklist 设计 ② + content-lock §2).
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import PushSubscribeToggle from '../components/PushSubscribeToggle';
import * as pushLib from '../lib/pushSubscribe';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  vi.restoreAllMocks();
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

describe('CS-3.2 — PushSubscribeToggle (Web Push UI 复用 DL-4)', () => {
  it('TestCS32_UnsupportedReturnsNull — 浏览器不支持 → return null', async () => {
    vi.spyOn(pushLib, 'isPushSupported').mockReturnValue(false);
    vi.spyOn(pushLib, 'getCurrentSubscriptionState').mockReturnValue('unsupported');
    await render(<PushSubscribeToggle vapidPublicKey="fake-key" />);
    expect(container!.querySelector('[data-cs3-push-toggle]')).toBeNull();
  });

  it('TestCS32_GrantedLabel — granted 三态文案 byte-identical', async () => {
    vi.spyOn(pushLib, 'isPushSupported').mockReturnValue(true);
    vi.spyOn(pushLib, 'getCurrentSubscriptionState').mockReturnValue('granted');
    await render(<PushSubscribeToggle vapidPublicKey="fake-key" />);
    const btn = container!.querySelector('[data-cs3-push-toggle]') as HTMLButtonElement;
    expect(btn).toBeTruthy();
    expect(btn.textContent).toBe('已开启通知');
    expect(btn.getAttribute('data-push-state')).toBe('granted');
    expect(btn.getAttribute('aria-pressed')).toBe('true');
  });

  it('TestCS32_DeniedLabel — denied 文案 byte-identical + disabled', async () => {
    vi.spyOn(pushLib, 'isPushSupported').mockReturnValue(true);
    vi.spyOn(pushLib, 'getCurrentSubscriptionState').mockReturnValue('denied');
    await render(<PushSubscribeToggle vapidPublicKey="fake-key" />);
    const btn = container!.querySelector('[data-cs3-push-toggle]') as HTMLButtonElement;
    expect(btn.textContent).toBe('通知已被浏览器拒绝, 请到浏览器设置开启');
    expect(btn.disabled).toBe(true);
    expect(btn.getAttribute('aria-pressed')).toBe('false');
  });

  it('TestCS32_DefaultLabel — default 文案 byte-identical (toggle off)', async () => {
    vi.spyOn(pushLib, 'isPushSupported').mockReturnValue(true);
    vi.spyOn(pushLib, 'getCurrentSubscriptionState').mockReturnValue('default');
    await render(<PushSubscribeToggle vapidPublicKey="fake-key" />);
    const btn = container!.querySelector('[data-cs3-push-toggle]') as HTMLButtonElement;
    expect(btn.textContent).toBe('开启通知');
    expect(btn.getAttribute('aria-pressed')).toBe('false');
  });

  it('TestCS32_DelegatesToDL4_subscribeToPush — click default → DL-4 subscribeToPush', async () => {
    vi.spyOn(pushLib, 'isPushSupported').mockReturnValue(true);
    vi.spyOn(pushLib, 'getCurrentSubscriptionState').mockReturnValue('default');
    const subSpy = vi.spyOn(pushLib, 'subscribeToPush').mockResolvedValue({} as any);
    await render(<PushSubscribeToggle vapidPublicKey="fake-key" />);
    const btn = container!.querySelector('[data-cs3-push-toggle]') as HTMLButtonElement;
    await act(async () => {
      btn.click();
    });
    expect(subSpy).toHaveBeenCalledWith('fake-key');
  });

  it('TestCS32_DelegatesToDL4_unsubscribeFromPush — click granted → DL-4 unsubscribeFromPush', async () => {
    vi.spyOn(pushLib, 'isPushSupported').mockReturnValue(true);
    vi.spyOn(pushLib, 'getCurrentSubscriptionState').mockReturnValue('granted');
    const unsubSpy = vi.spyOn(pushLib, 'unsubscribeFromPush').mockResolvedValue(undefined as any);
    await render(<PushSubscribeToggle vapidPublicKey="fake-key" />);
    const btn = container!.querySelector('[data-cs3-push-toggle]') as HTMLButtonElement;
    await act(async () => {
      btn.click();
    });
    expect(unsubSpy).toHaveBeenCalled();
  });
});
