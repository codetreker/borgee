// AP-2 client — capability label SSOT (i18n-ready, currently zh-CN literals).
//
// 设计沿用 (ap-2-spec.md §0.2 + content-lock + capability-dot-spec.md):
//   - 14 capability const byte-identical 跟 server `internal/auth/capabilities.go::ALL`
//     (改 = 改两处: server const + 此 LABEL_MAP)
//   - capabilityLabel(token) 单源 helper, 反 inline 字面散落 (grep 检查
//     `function capabilityLabel|export.*capabilityLabel` ==1 hit)
//   - 反 RBAC 角色名双语 (英 a/e/v/o + 中文 3 词) 0 hit (反 role bleed)
//
// CAPABILITY-DOT (post-rename): tokens are dot-notation `<domain>.<verb>` per
// 蓝图 auth-permissions.md §1 字面.
//
// 改 = 改三处: server `internal/auth/capabilities.go::ALL` + 此 LABEL_MAP +
// content-lock §1+§2 字面锁.

/** 14 capability token byte-identical 跟 server `auth.ALL` 顺序 */
export const CAPABILITY_TOKENS = [
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
  // channel acl
  'channel.manage_members',
  'channel.invite',
  'channel.change_role',
] as const;

export type CapabilityToken = (typeof CAPABILITY_TOKENS)[number];

/** 14 capability token → 中文 label SSOT (反 role 名双语漂). */
const LABEL_MAP: Record<CapabilityToken, string> = {
  'channel.read': '查看频道',
  'channel.write': '在频道发消息',
  'channel.delete': '删除频道',
  'artifact.read': '查看产物',
  'artifact.write': '编辑产物',
  'artifact.commit': '提交产物',
  'artifact.iterate': '迭代产物',
  'artifact.rollback': '回滚产物',
  'user.mention': '提及用户',
  'dm.read': '查看私信',
  'dm.send': '发送私信',
  'channel.manage_members': '管理频道成员',
  'channel.invite': '邀请用户',
  'channel.change_role': '调整成员能力',
};

/**
 * capabilityLabel — single-source helper to render a capability token
 * label. Unknown tokens render the raw token (forward-compat).
 *
 * 反约束: 调用方禁止 inline 字面 (如 `'查看频道'`); 走此 helper SSOT
 * (grep 检查 `function capabilityLabel|export.*capabilityLabel` ==1 hit).
 */
export function capabilityLabel(token: string): string {
  if (token in LABEL_MAP) {
    return LABEL_MAP[token as CapabilityToken];
  }
  return token;
}

/** isKnownCapability — 反向断言 helper (跟 server `auth.IsValidCapability` 同精神). */
export function isKnownCapability(token: string): token is CapabilityToken {
  return token in LABEL_MAP;
}
