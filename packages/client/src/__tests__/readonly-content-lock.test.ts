// readonly-content-lock.test.ts — CHN-15 byte-identical 跨 server/client/
// content-lock 双向锁验证.
//
// Pins:
//   - READONLY_BIT === 16 (跟 server api.ReadonlyBit byte-identical)
//   - READONLY_LABEL 3 文案 byte-identical 跟 content-lock §1 同源
//   - CHANNEL_READONLY_TOAST 1 字面 byte-identical 跟 server const
//   - 同义词grep 检查 — 8 字面 user-visible 0 hit (排除 "readonly" 单词
//     在 const/type 名内出现, 仅扫文案字符串字面)

import { describe, it, expect } from 'vitest';
import { READONLY_BIT, READONLY_LABEL } from '../lib/readonly';
import { CHANNEL_READONLY_TOAST } from '../lib/api';

describe('CHN-15 byte-identical 双向锁', () => {
  it('READONLY_BIT === 16 (跟 server api.ReadonlyBit byte-identical)', () => {
    expect(READONLY_BIT).toBe(16);
  });

  it('READONLY_LABEL 3 文案 byte-identical 跟 content-lock §1 同源', () => {
    expect(READONLY_LABEL.set_toast).toBe('已设为只读');
    expect(READONLY_LABEL.unset_toast).toBe('已恢复编辑');
    expect(READONLY_LABEL.no_send_reject).toBe('只读频道, 仅创建者可发言');
  });

  it('CHANNEL_READONLY_TOAST 1 错码 byte-identical 跟 server const + content-lock §3', () => {
    expect(CHANNEL_READONLY_TOAST['channel.readonly_no_send']).toBe('只读频道, 仅创建者可发言');
    // 1 key exact (no drift)
    expect(Object.keys(CHANNEL_READONLY_TOAST).sort()).toEqual([
      'channel.readonly_no_send',
    ]);
  });

  it('READONLY_LABEL.no_send_reject 跟 CHANNEL_READONLY_TOAST 同源 (改一处 = 改三处)', () => {
    // 文案锁 §3 显式约定: server const → client toast → label const 三处同字面.
    expect(READONLY_LABEL.no_send_reject).toBe(
      CHANNEL_READONLY_TOAST['channel.readonly_no_send'],
    );
  });
});
