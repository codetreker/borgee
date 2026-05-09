// AP-2 client — capability-bundles content-lock + reverse-grep tests.
//
// 设计沿用 (acceptance §1.1+§2.2 + content-lock):
//   §1.1 CAPABILITY_BUNDLES 内 capability token 必 ∈ AP-1 14 const 跨层锁
//   §2.2 反 hardcode bundle 漂 ('Workspace'|'Reader'|'Mention' 在
//        components/ body 0 hit, 走 const 单源)
//   §2.2 反 RBAC role name in CAPABILITY_BUNDLES const ('admin'|'editor'
//        /'moderator'/'role' 0 hit)
import { describe, it, expect } from 'vitest';
import {
  CAPABILITY_BUNDLES,
  BUNDLE_IDS,
  bundleCapabilities,
  bundlesContaining,
  assertBundlesValid,
} from '../lib/capability-bundles';
import { CAPABILITY_TOKENS } from '../lib/capabilities';

describe('AP-2 ⭐ capability-bundles SSOT — 跨 AP-1 层锁 + 反 RBAC', () => {
  it('§1.1 each bundle capability ∈ AP-1 14 const (跨层锁)', () => {
    const known = new Set<string>(CAPABILITY_TOKENS);
    for (const id of BUNDLE_IDS) {
      for (const tok of CAPABILITY_BUNDLES[id]) {
        expect(known.has(tok)).toBe(true);
      }
    }
  });

  it('§1.1 assertBundlesValid passes (no drift)', () => {
    expect(() => assertBundlesValid()).not.toThrow();
  });

  it('§2 BUNDLE_IDS = workspace/reader/mention (3, 反 role ladder admin/editor/...)', () => {
    expect(BUNDLE_IDS).toEqual(['workspace', 'reader', 'mention']);
  });

  it('§2 CAPABILITY_BUNDLES bundle membership byte-identical', () => {
    expect(CAPABILITY_BUNDLES.workspace).toEqual([
      'channel.write',
      'artifact.write',
      'artifact.commit',
    ]);
    expect(CAPABILITY_BUNDLES.reader).toEqual([
      'channel.read',
      'artifact.read',
      'dm.read',
    ]);
    expect(CAPABILITY_BUNDLES.mention).toEqual(['user.mention', 'dm.send']);
  });

  it('§2.2 bundleCapabilities + bundlesContaining helpers correct', () => {
    expect(bundleCapabilities('workspace')).toEqual(CAPABILITY_BUNDLES.workspace);
    expect(bundlesContaining('channel.write')).toEqual(['workspace']);
    expect(bundlesContaining('dm.read')).toEqual(['reader']);
    // unrelated capability — empty.
    expect(bundlesContaining('channel.manage_members')).toEqual([]);
  });
});
