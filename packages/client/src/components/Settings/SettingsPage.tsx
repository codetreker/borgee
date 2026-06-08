// SettingsPage — 用户设置页骨架.
//
// 现有 tab: runtime (Remote Nodes 入口) + channels
// (ChannelManagementSurface). 默认进 runtime.
//
// 反约束:
//   - 跟 admin SPA SettingsPage (packages/client/src/admin/pages/) 路径分叉
//     (ADM-0 constraint: admin/user 路径不混用).
//   - 不引入 react-router (跟 App.tsx nav-history 同模式 — useNavigation
//     维护 App-level view 栈).
import { useState } from 'react';
import PageHeader from '../common/PageHeader';
import { useNavigation } from '../Navigation/NavigationContext';
import { PermissionsView } from '../PermissionsView';
import ChannelManagementSurface from './ChannelManagementSurface';

export type SettingsTab = 'runtime' | 'channels';

export default function SettingsPage() {
  const [activeTab, setActiveTab] = useState<SettingsTab>('runtime');
  const nav = useNavigation();

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
              <PermissionsView />
            </section>
          </>
        )}
        {activeTab === 'channels' && <ChannelManagementSurface />}
      </main>
    </div>
  );
}
