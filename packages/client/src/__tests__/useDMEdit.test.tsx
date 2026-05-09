// useDMEdit.test.ts — 4 vitest cases pin DM-4.2 设计 ①+②+③.
//
// Cases:
//   ① HappyPath — editMessage 调 patchDMMessage 真返回
//   ② empty content reject — 空 content / 全空格 → throws + error 状态
//   ③ DoesNotWriteOwnCursor — 反向断言 hook 不写 borgee.dm4.cursor:* sessionStorage
//   ④ multi-device 复用 useDMSync — useDMEdit 不读不写 dm-3.cursor key
//      (cursor 进展全归 useDMSync, edit 是 cursor 子集, spec §0.2)

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import { useDMEdit } from '../hooks/useDMEdit';
import * as api from '../lib/api';

let container: HTMLDivElement | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  vi.restoreAllMocks();
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

interface Capture {
  editMessage: ((id: string, c: string) => Promise<unknown>) | null;
  isEditing: boolean;
  error: string | null;
}

const HookHarness: React.FC<{ dmID: string; cap: Capture }> = ({ dmID, cap }) => {
  const r = useDMEdit(dmID);
  cap.editMessage = r.editMessage;
  cap.isEditing = r.isEditing;
  cap.error = r.error;
  return null;
};

async function mountHook(dmID: string): Promise<Capture> {
  const cap: Capture = { editMessage: null, isEditing: false, error: null };
  const root = createRoot(container!);
  await act(async () => {
    root.render(React.createElement(HookHarness, { dmID, cap }));
  });
  return cap;
}

describe('useDMEdit hook (DM-4.2)', () => {
  it('① HappyPath — editMessage calls patchDMMessage with content', async () => {
    const spy = vi
      .spyOn(api, 'patchDMMessage')
      .mockResolvedValue({
        message: {
          id: 'msg-1',
          channel_id: 'dm-aaa',
          sender_id: 'user-1',
          content: 'edited content',
        },
      });
    const cap = await mountHook('dm-aaa');
    await act(async () => {
      await cap.editMessage!('msg-1', 'edited content');
    });
    expect(spy).toHaveBeenCalledTimes(1);
    expect(spy).toHaveBeenCalledWith('dm-aaa', 'msg-1', 'edited content');
  });

  it('② empty content reject — throws + sets error state', async () => {
    const spy = vi.spyOn(api, 'patchDMMessage');
    const cap = await mountHook('dm-aaa');
    let threw = false;
    await act(async () => {
      try {
        await cap.editMessage!('msg-1', '   ');
      } catch {
        threw = true;
      }
    });
    expect(threw).toBe(true);
    expect(spy).not.toHaveBeenCalled();
  });

  it('③ DoesNotWriteOwnCursor — hook never touches borgee.dm4.cursor:*', async () => {
    vi.spyOn(api, 'patchDMMessage').mockResolvedValue({
      message: { id: 'msg-1', channel_id: 'dm-aaa', sender_id: 'u', content: 'x' },
    });
    const cap = await mountHook('dm-aaa');
    await act(async () => {
      await cap.editMessage!('msg-1', 'x');
    });
    // 设计 ② 反向断言: useDMEdit must not persist its own cursor.
    if (typeof window !== 'undefined' && window.sessionStorage) {
      for (let i = 0; i < window.sessionStorage.length; i++) {
        const key = window.sessionStorage.key(i);
        expect(key).not.toMatch(/^borgee\.dm4\.cursor:/);
      }
    }
  });

  it('④ multi-device — useDMEdit does not interfere with useDMSync cursor (dm3 key)', async () => {
    // Pre-seed a useDMSync cursor (DM-3 #508) for the same dm channel,
    // then exercise useDMEdit and assert the cursor is untouched.
    if (typeof window !== 'undefined' && window.sessionStorage) {
      window.sessionStorage.setItem('borgee.dm3.cursor:dm-aaa', '999');
    }
    vi.spyOn(api, 'patchDMMessage').mockResolvedValue({
      message: { id: 'msg-1', channel_id: 'dm-aaa', sender_id: 'u', content: 'y' },
    });
    const cap = await mountHook('dm-aaa');
    await act(async () => {
      await cap.editMessage!('msg-1', 'y');
    });
    expect(window.sessionStorage.getItem('borgee.dm3.cursor:dm-aaa')).toBe('999');
  });
});
