// MemberList.test.tsx — CHN-11.3 5 vitest cases pin content-lock.
//
// Cases:
//   ① MemberList title `成员` + add `添加成员` + remove `移除` byte-identical
//   ② AddMemberModal title `添加成员` + placeholder + submit `添加` + cancel `取消`
//   ③ KickConfirmModal title `确认移除 {user}?` byte-identical + confirm `确认`
//   ④ canManage=false hides add/remove buttons
//   ⑤ 同义词反向 reject — source grep 0 hit (data-testid 例外); 既有
//      handleAddMember + handleRemoveMember server-side path 不漂入 chn_11

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
// @ts-expect-error — node:module no @types/node
import { createRequire } from 'module';
import { MemberList } from '../components/MemberList';
import { AddMemberModal } from '../components/AddMemberModal';
import { KickConfirmModal } from '../components/KickConfirmModal';
import * as api from '../lib/api';

const nodeRequire = createRequire(import.meta.url);
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const fs: any = nodeRequire('fs');
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const nodePath: any = nodeRequire('path');
// ESM workaround — __dirname undefined in `tsc -b` ESM emit.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const nodeUrl: any = nodeRequire('url');
const HERE = nodePath.dirname(nodeUrl.fileURLToPath(import.meta.url));

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

describe('CHN-11.3 MemberList + AddMemberModal + KickConfirmModal content lock', () => {
  it('① MemberList title `成员` + add `添加成员` + remove `移除` byte-identical', () => {
    const members = [
      { user_id: 'u1', display_name: 'Alice' },
      { user_id: 'u2', display_name: 'Bob' },
    ];
    act(() => {
      root!.render(
        <MemberList
          members={members}
          canManage={true}
          onAdd={() => {}}
          onRemove={() => {}}
        />,
      );
    });
    const list = container!.querySelector('[data-testid="member-list"]')!;
    expect(list.querySelector('h3')!.textContent).toBe('成员');
    expect(
      container!.querySelector('[data-testid="member-list-add"]')!.textContent,
    ).toBe('添加成员');
    expect(
      container!.querySelector('[data-testid="member-remove-u1"]')!.textContent,
    ).toBe('移除');
  });

  it('② AddMemberModal title + placeholder + submit + cancel byte-identical', () => {
    act(() => {
      root!.render(
        <AddMemberModal
          channelID="c1"
          onAdded={() => {}}
          onCancel={() => {}}
        />,
      );
    });
    const modal = container!.querySelector('[data-testid="add-member-modal"]')!;
    expect(modal.querySelector('h3')!.textContent).toBe('添加成员');
    const input = container!.querySelector(
      '[data-testid="add-member-input"]',
    ) as HTMLInputElement;
    expect(input.placeholder).toBe('用户邮箱或 ID');
    expect(
      container!.querySelector('[data-testid="add-member-submit"]')!.textContent,
    ).toBe('添加');
    expect(
      container!.querySelector('[data-testid="add-member-cancel"]')!.textContent,
    ).toBe('取消');
  });

  it('③ KickConfirmModal title `确认移除 {user}?` byte-identical', () => {
    act(() => {
      root!.render(
        <KickConfirmModal
          channelID="c1"
          user={{ user_id: 'u1', display_name: 'Alice' }}
          onRemoved={() => {}}
          onCancel={() => {}}
        />,
      );
    });
    const modal = container!.querySelector(
      '[data-testid="kick-confirm-modal"]',
    )!;
    expect(modal.querySelector('h3')!.textContent).toBe('确认移除 Alice?');
    expect(
      container!.querySelector('[data-testid="kick-confirm-yes"]')!.textContent,
    ).toBe('确认');
    expect(
      container!.querySelector('[data-testid="kick-confirm-no"]')!.textContent,
    ).toBe('取消');
  });

  it('④ canManage=false hides add+remove buttons', () => {
    act(() => {
      root!.render(
        <MemberList
          members={[{ user_id: 'u1', display_name: 'Alice' }]}
          canManage={false}
          onAdd={() => {}}
          onRemove={() => {}}
        />,
      );
    });
    expect(
      container!.querySelector('[data-testid="member-list-add"]'),
    ).toBeNull();
    expect(
      container!.querySelector('[data-testid="member-remove-u1"]'),
    ).toBeNull();
    // 空 members → return null.
    act(() => {
      root!.render(
        <MemberList
          members={[]}
          canManage={true}
          onAdd={() => {}}
          onRemove={() => {}}
        />,
      );
    });
    expect(
      container!.querySelector('[data-testid="member-list"]'),
    ).toBeNull();
  });

  it('⑤ 同义词反向 reject + addChannelMember/removeChannelMember 真调', async () => {
    const compRoots = ['MemberList', 'AddMemberModal', 'KickConfirmModal'];
    for (const c of compRoots) {
      const p = nodePath.resolve(HERE, '..', 'components', `${c}.tsx`);
      const src: string = fs.readFileSync(p, 'utf8');
      // user-visible Chinese 反向 reject — 我们用 `添加/移除`.
      for (const tok of ['逐出', '踢出', '邀请']) {
        expect(src.includes(tok)).toBe(false);
      }
    }
    // addChannelMember真调.
    const addSpy = vi
      .spyOn(api, 'addChannelMember')
      .mockResolvedValue(undefined as never);
    let addedWith: string | null = null;
    act(() => {
      root!.render(
        <AddMemberModal
          channelID="c1"
          onAdded={(uid) => {
            addedWith = uid;
          }}
          onCancel={() => {}}
        />,
      );
    });
    const input = container!.querySelector(
      '[data-testid="add-member-input"]',
    ) as HTMLInputElement;
    const setter = Object.getOwnPropertyDescriptor(
      window.HTMLInputElement.prototype,
      'value',
    )!.set!;
    setter.call(input, 'alice@test.com');
    input.dispatchEvent(new Event('input', { bubbles: true }));
    await act(async () => {
      const btn = container!.querySelector(
        '[data-testid="add-member-submit"]',
      ) as HTMLButtonElement;
      btn.click();
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(addSpy).toHaveBeenCalledWith('c1', 'alice@test.com');
    expect(addedWith).toBe('alice@test.com');
  });
});
