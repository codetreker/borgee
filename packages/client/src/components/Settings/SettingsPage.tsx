// SettingsPage — 用户设置页骨架.
//
// 现有 tab: runtime (Remote Nodes 入口) + channels
// (ChannelManagementSurface). 默认进 runtime.
//
// AP-2 #970: the runtime-tab permissions section also mounts the
// BundleSelector. On confirm it fans out one PUT /api/v1/permissions per
// selected capability (caller-driven; no bundle endpoint), grants for the
// signed-in user, then nudges the adjacent PermissionsView to re-fetch so
// the new capability rows surface.
//
// 反约束:
//   - 跟 admin SPA SettingsPage (packages/client/src/admin/pages/) 路径分叉
//     (ADM-0 constraint: admin/user 路径不混用).
//   - 不引入 react-router (跟 App.tsx nav-history 同模式 — useNavigation
//     维护 App-level view 栈).
//   - 此 wiring 不写 RBAC role 字面 (capability token 走 BundleSelector const
//     单源; ap-2 reverse-grep 守 BundleSelector.tsx 本体).
import { useState } from 'react';
import PageHeader from '../common/PageHeader';
import { useNavigation } from '../Navigation/NavigationContext';
import { PermissionsView } from '../PermissionsView';
import { BundleSelector } from '../BundleSelector';
import type { CapabilityToken } from '../../lib/capabilities';
import ChannelManagementSurface from './ChannelManagementSurface';

export type SettingsTab = 'runtime' | 'channels';

/** Grant a single capability for the signed-in user via the AP-1 path. */
async function putSelfPermission(capability: CapabilityToken): Promise<void> {
  const res = await fetch('/api/v1/permissions', {
    method: 'PUT',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ permission: capability, scope: '*' }),
  });
  if (!res.ok) {
    const err = new Error(`grant ${capability}: ${res.status}`) as Error & { status: number };
    err.status = res.status;
    throw err;
  }
}

export default function SettingsPage() {
  const [activeTab, setActiveTab] = useState<SettingsTab>('runtime');
  const nav = useNavigation();

  // AP-2 #970 grant fan-out state — mirrors PermissionsView's
  // loading/forbidden/error surfaces.
  const [granting, setGranting] = useState(false);
  const [grantForbidden, setGrantForbidden] = useState(false);
  const [grantError, setGrantError] = useState(false);
  // Bumped after a successful grant to remount PermissionsView → re-fetch.
  const [permissionsEpoch, setPermissionsEpoch] = useState(0);

  async function handleGrantBundle(capabilities: CapabilityToken[]) {
    if (capabilities.length === 0) return;
    setGranting(true);
    setGrantForbidden(false);
    setGrantError(false);
    try {
      // Caller-driven fan-out: one PUT per selected capability (no bundle
      // endpoint — reverse-grep bans it).
      for (const capability of capabilities) {
        await putSelfPermission(capability);
      }
      // Nudge the adjacent PermissionsView to re-fetch the new rows.
      setPermissionsEpoch((n) => n + 1);
    } catch (e) {
      const status = (e as { status?: number }).status;
      if (status === 401 || status === 403) {
        setGrantForbidden(true);
      } else {
        setGrantError(true);
      }
    } finally {
      setGranting(false);
    }
  }

  return (
    <div className="settings-page" data-page="settings">
      <PageHeader title="设置" />

      <nav className="settings-tabs" aria-label="设置分类">
        <button
          type="button"
          className={`settings-tab${activeTab === 'runtime' ? ' active' : ''}`}
          data-tab="runtime"
          aria-current={activeTab === 'runtime' ? 'page' : undefined}
          onClick={() => setActiveTab('runtime')}
        >
          运行时
        </button>
        <button
          type="button"
          className={`settings-tab${activeTab === 'channels' ? ' active' : ''}`}
          data-tab="channels"
          aria-current={activeTab === 'channels' ? 'page' : undefined}
          onClick={() => setActiveTab('channels')}
        >
          频道
        </button>
      </nav>

      <main className="settings-page-content">
        {activeTab === 'runtime' && (
          <>
            <section
              className="settings-runtime-surface"
              data-settings-runtime-surface="true"
              aria-label="Runtime management"
            >
              <div className="settings-runtime-header">
                <h2>Runtime</h2>
              </div>
              <div className="settings-runtime-actions">
                <button
                  type="button"
                  className="settings-runtime-entry"
                  data-runtime-entry="remote-nodes"
                  data-authority-rail="remote-agent"
                  onClick={() => nav.push('remote-nodes')}
                >
                  <span className="settings-runtime-entry-title">Remote Nodes</span>
                  <span className="settings-runtime-entry-meta">Remote Agent file proxy</span>
                </button>
              </div>
            </section>
            <section className="settings-permissions-section" data-settings-permissions-surface="true">
              <div
                className="settings-bundle-grant"
                data-settings-bundle-surface="true"
                aria-busy={granting ? 'true' : undefined}
              >
                <BundleSelector onConfirm={handleGrantBundle} />
                {granting && <div data-ap2-bundle-loading="true">授予中</div>}
                {grantForbidden && (
                  <div data-ap2-bundle-forbidden="true" role="alert">
                    无权授予
                  </div>
                )}
                {grantError && (
                  <div data-ap2-bundle-error="true" role="alert">
                    授予失败
                  </div>
                )}
              </div>
              <PermissionsView key={permissionsEpoch} />
            </section>
          </>
        )}
        {activeTab === 'channels' && <ChannelManagementSurface />}
      </main>
    </div>
  );
}
