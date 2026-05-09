// AP-2 client — BundleSelector component (acceptance §3.1).
//
// 设计沿用 (ap-2-spec.md + acceptance §3.1 + content-lock):
//   - bundle 展开 capability checkbox (用户主权, 反偷默认勾全部)
//   - 用户必显式 confirm (反自动 submit; 跟 DM-9 user 主权设计沿用)
//   - DOM `data-bundle-name` 锚 + capabilityLabel SSOT 复用
//   - 复用 AP-1 既有 PUT /api/v1/permissions endpoint (反开旁路 endpoint)
//   - 反 RBAC role name (grep 检查)
import { useState } from 'react';
import {
  BUNDLE_IDS,
  BUNDLE_LABELS,
  CAPABILITY_BUNDLES,
  type BundleID,
} from '../lib/capability-bundles';
import { capabilityLabel } from '../lib/capabilities';
import type { CapabilityToken } from '../lib/capabilities';

export interface BundleSelectorProps {
  /** Called on confirm with the user-curated capability list. */
  onConfirm: (capabilities: CapabilityToken[]) => void | Promise<void>;
  /** Optional override for grantee user id (reserved for future per-user UI). */
  granteeID?: string;
}

/**
 * BundleSelector — bundle UI 反 role 名.
 *
 * Flow:
 *   1. user clicks a bundle → expand its capability checkboxes (default all
 *      checked, but user can uncheck — 反偷默认全勾的设计)
 *   2. confirm button → onConfirm(selected list); caller dispatches N x
 *      AP-1 PUT /api/v1/permissions (无 bundle endpoint, 蓝图 §1.1 字面)
 */
export function BundleSelector({ onConfirm }: BundleSelectorProps) {
  const [openBundle, setOpenBundle] = useState<BundleID | null>(null);
  const [selected, setSelected] = useState<Set<CapabilityToken>>(new Set());

  function expandBundle(id: BundleID) {
    setOpenBundle(id);
    // Default all bundle members checked — user can uncheck (主权).
    setSelected(new Set<CapabilityToken>(CAPABILITY_BUNDLES[id]));
  }

  function toggleCap(token: CapabilityToken) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(token)) {
        next.delete(token);
      } else {
        next.add(token);
      }
      return next;
    });
  }

  async function handleConfirm() {
    if (selected.size === 0) return;
    await onConfirm(Array.from(selected));
    // Reset after confirm — caller is responsible for closing UI.
    setSelected(new Set());
    setOpenBundle(null);
  }

  return (
    <div data-ap2-bundle-selector="true">
      <ul data-ap2-bundle-list="true">
        {BUNDLE_IDS.map((id) => (
          <li
            key={id}
            data-ap2-bundle-row="true"
            data-bundle-name={id}
            data-ap2-expanded={openBundle === id ? 'true' : 'false'}
          >
            <button
              type="button"
              data-ap2-bundle-expand="true"
              data-bundle-name={id}
              onClick={() => expandBundle(id)}
            >
              {BUNDLE_LABELS[id]}
            </button>
            {openBundle === id && (
              <ul data-ap2-bundle-capabilities="true">
                {CAPABILITY_BUNDLES[id].map((token) => (
                  <li key={token} data-ap2-bundle-capability-row={token}>
                    <label>
                      <input
                        type="checkbox"
                        data-ap2-bundle-checkbox={token}
                        checked={selected.has(token)}
                        onChange={() => toggleCap(token)}
                      />
                      <span data-ap2-bundle-capability-label>
                        {capabilityLabel(token)}
                      </span>
                    </label>
                  </li>
                ))}
              </ul>
            )}
          </li>
        ))}
      </ul>
      {openBundle !== null && (
        <button
          type="button"
          data-ap2-bundle-confirm="true"
          onClick={handleConfirm}
          disabled={selected.size === 0}
        >
          确认授予 {selected.size} 项能力
        </button>
      )}
    </div>
  );
}
