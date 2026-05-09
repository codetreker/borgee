// AP-2 client — capability label SSOT + 反 role 名 + grep 检查.
import { describe, it, expect } from 'vitest';
import {
  CAPABILITY_TOKENS,
  capabilityLabel,
  isKnownCapability,
} from '../lib/capabilities';

describe('AP-2 ⭐ capability label SSOT — 14 const + 反 role bleed', () => {
  it('§1 14 capability tokens byte-identical 跟 server `auth.ALL` 顺序', () => {
    expect(CAPABILITY_TOKENS).toEqual([
      // channel scope
      'channel.read',
      'channel.write',
      'channel.delete',
      // artifact scope
      'artifact.read',
      'artifact.write',
      'artifact.commit',
      'artifact.iterate',
      'artifact.rollback',
      // messaging
      'user.mention',
      'dm.read',
      'dm.send',
      // channel admin
      'channel.manage_members',
      'channel.invite',
      'channel.change_role',
    ]);
  });

  it('§2 capabilityLabel — 14 token 各 byte-identical 中文 label', () => {
    expect(capabilityLabel('channel.read')).toBe('查看频道');
    expect(capabilityLabel('channel.write')).toBe('在频道发消息');
    expect(capabilityLabel('channel.delete')).toBe('删除频道');
    expect(capabilityLabel('artifact.read')).toBe('查看产物');
    expect(capabilityLabel('artifact.write')).toBe('编辑产物');
    expect(capabilityLabel('artifact.commit')).toBe('提交产物');
    expect(capabilityLabel('artifact.iterate')).toBe('迭代产物');
    expect(capabilityLabel('artifact.rollback')).toBe('回滚产物');
    expect(capabilityLabel('user.mention')).toBe('提及用户');
    expect(capabilityLabel('dm.read')).toBe('查看私信');
    expect(capabilityLabel('dm.send')).toBe('发送私信');
    expect(capabilityLabel('channel.manage_members')).toBe('管理频道成员');
    expect(capabilityLabel('channel.invite')).toBe('邀请用户');
    expect(capabilityLabel('channel.change_role')).toBe('调整成员能力');
  });

  it('§3 capabilityLabel — unknown token forward-compat 渲染原 token (反 silent drop)', () => {
    expect(capabilityLabel('future_capability_v3')).toBe('future_capability_v3');
    expect(capabilityLabel('')).toBe('');
  });

  it('§4 isKnownCapability — 14 known + 反向断言 unknown', () => {
    for (const t of CAPABILITY_TOKENS) {
      expect(isKnownCapability(t)).toBe(true);
    }
    expect(isKnownCapability('admin')).toBe(false); // RBAC role 名反 reject
    expect(isKnownCapability('editor')).toBe(false);
    expect(isKnownCapability('viewer')).toBe(false);
    expect(isKnownCapability('owner')).toBe(false);
    expect(isKnownCapability('未知')).toBe(false);
  });

  it('§5 反 role 名双语 — capabilityLabel 输出 0 hit RBAC role 字面', () => {
    // 14 token 输出 label 反向断言不含 admin/editor/viewer/owner /
    // 管理员/编辑者/查看者 字面 (反 role bleed).
    const forbidden = [
      /admin/i,
      /editor/i,
      /viewer/i,
      /owner/i,
      /管理员/,
      /编辑者/,
      /查看者/,
    ];
    for (const tok of CAPABILITY_TOKENS) {
      const label = capabilityLabel(tok);
      for (const re of forbidden) {
        expect(re.test(label)).toBe(false);
      }
    }
  });
});
