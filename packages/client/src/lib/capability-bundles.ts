// AP-2 client — capability bundle SSOT (acceptance §1.1).
//
// 设计沿用 (ap-2-spec.md + acceptance §1.1+§1.3 + content-lock):
//   - 3 bundle 内 capability 字面 byte-identical 跟 AP-1 #493
//     `internal/auth/capabilities.go::ALL` 14-const 跨层锁
//     (改 = 改两处: server const + 此 BUNDLES)
//   - bundle 仅 client-side const map; server 不识别 bundle (蓝图 §1.1
//     字面 — grep 检查 `bundle_name|capability_bundle|preset_bundle`
//     在 internal/ 0 hit)
//   - 复用 AP-1 既有 grant endpoint (不开旁路 endpoint);
//     一次 grant 多 capability = client SPA 解开 list → N 次调 AP-1 PUT
//   - 反 RBAC ladder 字面 in bundle const (grep 检查 守, 蓝图 §1.3 A. 设计)
import { CAPABILITY_TOKENS, type CapabilityToken } from './capabilities';

/** Bundle id — content-lock §1 byte-identical (改 = 改两处). */
export const BUNDLE_IDS = ['workspace', 'reader', 'mention'] as const;
export type BundleID = (typeof BUNDLE_IDS)[number];

/** Bundle display label byte-identical (中文, content-lock §1). */
export const BUNDLE_LABELS: Record<BundleID, string> = {
  workspace: '工作能力',
  reader: '阅读能力',
  mention: '提及能力',
};

/**
 * CAPABILITY_BUNDLES — 3 bundle SSOT, 内 capability token 全部来自
 * AP-1 14-const (CAPABILITY_TOKENS) byte-identical 跨层锁.
 *
 * - workspace — 工作场景: 写频道 + 写产物 + 提交产物 (3)
 * - reader    — 阅读场景: 看频道 + 看产物 + 看私信 (3)
 * - mention   — 互动场景: 提及用户 + 发私信 (2)
 *
 * 反约束:
 *   - bundle 内 token 必 ∈ CAPABILITY_TOKENS (反 forward-compat leak)
 *   - bundle id 走 scenario 语义 (workspace/reader/mention), 不走 RBAC
 *     ladder (蓝图 §1.3 A')
 */
export const CAPABILITY_BUNDLES: Record<BundleID, CapabilityToken[]> = {
  workspace: ['channel.write', 'artifact.write', 'artifact.commit'],
  reader: ['channel.read', 'artifact.read', 'dm.read'],
  mention: ['user.mention', 'dm.send'],
};

/** Resolve a bundle to its capability list (反 inline 散落). */
export function bundleCapabilities(id: BundleID): CapabilityToken[] {
  return CAPABILITY_BUNDLES[id];
}

/** Reverse map — capability → bundle ids it belongs to (helper for UI hint). */
export function bundlesContaining(token: CapabilityToken): BundleID[] {
  const out: BundleID[] = [];
  for (const id of BUNDLE_IDS) {
    if (CAPABILITY_BUNDLES[id].includes(token)) out.push(id);
  }
  return out;
}

/** Self-check: 反 byte-identical drift — bundle token 必 ∈ AP-1 14 const. */
export function assertBundlesValid(): void {
  const known = new Set<string>(CAPABILITY_TOKENS);
  for (const id of BUNDLE_IDS) {
    for (const tok of CAPABILITY_BUNDLES[id]) {
      if (!known.has(tok)) {
        throw new Error(
          `AP-2 bundle "${id}" contains unknown capability "${tok}" (反 AP-1 14-const 跨层锁)`,
        );
      }
    }
  }
}
