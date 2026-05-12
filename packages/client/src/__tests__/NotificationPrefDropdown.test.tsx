// NotificationPrefDropdown.test.tsx — CHN-8.2 dropdown DOM byte-identical checks,
// three-option labels, synonym rejection, change → API call, and NotifPref alignment.
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import { NotificationPrefDropdown } from '../components/NotificationPrefDropdown';
import * as api from '../lib/api';
import {
  NOTIF_PREF_SHIFT,
  NOTIF_PREF_MASK,
  NOTIF_PREF_ALL,
  NOTIF_PREF_MENTION,
  NOTIF_PREF_NONE,
  getNotifPref,
} from '../lib/notif_pref';

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
  vi.restoreAllMocks();
});

describe('NotificationPrefDropdown — CHN-8.2 DOM and locked labels', () => {
  it('three options remain byte-identical (所有消息 / 仅@提及 / 不打扰)', () => {
    const root = createRoot(container!);
    act(() => {
      root.render(
        <NotificationPrefDropdown channelId="c-1" pref="all" />,
      );
    });
    const sel = container!.querySelector('[data-testid="notification-pref-dropdown"]') as HTMLSelectElement;
    expect(sel).not.toBeNull();
    const opts = container!.querySelectorAll('option');
    expect(opts.length).toBe(3);

    const map: Record<string, string> = {};
    opts.forEach(o => { map[o.getAttribute('value')!] = o.textContent || ''; });
    expect(map.all).toBe('所有消息');
    expect(map.mention).toBe('仅@提及');
    expect(map.none).toBe('不打扰');
  });

  it('change → setNotificationPref(id, "mention") + onChange callback', async () => {
    const spy = vi
      .spyOn(api, 'setNotificationPref')
      .mockResolvedValue({ collapsed: 4, pref: 'mention' });
    const onChange = vi.fn();
    const root = createRoot(container!);
    act(() => {
      root.render(
        <NotificationPrefDropdown
          channelId="c-1"
          pref="all"
          onChange={onChange}
        />,
      );
    });
    const sel = container!.querySelector('select') as HTMLSelectElement;
    await act(async () => {
      sel.value = 'mention';
      sel.dispatchEvent(new Event('change', { bubbles: true }));
      await new Promise(r => setTimeout(r, 0));
    });
    expect(spy).toHaveBeenCalledWith('c-1', 'mention');
    expect(onChange).toHaveBeenCalledWith('mention');
  });

  it('synonym rejection: forbidden notification labels do not appear in user-visible options', () => {
    const root = createRoot(container!);
    act(() => {
      root.render(
        <NotificationPrefDropdown channelId="c-1" pref="all" />,
      );
    });
    const html = container!.innerHTML;
    const forbidden = ['subscribe', 'unsubscribe', 'follow', 'snooze', '订阅', '关注', '取消订阅'];
    for (const f of forbidden) {
      expect(html).not.toContain(f);
    }
  });

  it('NotifPref constants stay byte-identical with getNotifPref behavior', () => {
    expect(NOTIF_PREF_SHIFT).toBe(2);
    expect(NOTIF_PREF_MASK).toBe(3);
    expect(NOTIF_PREF_ALL).toBe(0);
    expect(NOTIF_PREF_MENTION).toBe(1);
    expect(NOTIF_PREF_NONE).toBe(2);
    expect(getNotifPref(0)).toBe(NOTIF_PREF_ALL);
    expect(getNotifPref(4)).toBe(NOTIF_PREF_MENTION); // bit 2 set
    expect(getNotifPref(8)).toBe(NOTIF_PREF_NONE); // bit 3 set
    expect(getNotifPref(null)).toBe(NOTIF_PREF_ALL);
    expect(getNotifPref(undefined)).toBe(NOTIF_PREF_ALL);
    // Bitmap isolation: bits 0 and 1 do not affect notification preference.
    expect(getNotifPref(1 | 2)).toBe(NOTIF_PREF_ALL); // collapsed + mute, no pref
    expect(getNotifPref(1 | 4)).toBe(NOTIF_PREF_MENTION); // collapsed + mention
  });
});
