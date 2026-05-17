// SettingsPage — 用户设置页骨架.
//
// 现有 tab: runtime (Remote Nodes / Helper Status 入口) + channels
// (ChannelManagementSurface). 默认进 runtime.
//
// 反约束:
//   - 跟 admin SPA SettingsPage (packages/client/src/admin/pages/) 路径分叉
//     (ADM-0 constraint: admin/user 路径不混用).
//   - 不引入 react-router (跟 App.tsx mainView 同模式 — App-level state 切视图).
import { PermissionsView } from '../PermissionsView';
import ChannelManagementSurface from './ChannelManagementSurface';
import { useState } from 'react';

interface Props {
  onBack: () => void;
  onRemoteNodesOpen?: () => void;
  onHelperStatusOpen?: () => void;
}

export type SettingsTab = 'runtime' | 'channels';

export default function SettingsPage({ onBack, onRemoteNodesOpen, onHelperStatusOpen }: Props) {
  const [activeTab, setActiveTab] = useState<SettingsTab>('runtime');

  return (
    <div className="settings-page" data-page="settings">
      <header className="settings-page-header">
        <button
          type="button"
          className="settings-back-btn"
          onClick={onBack}
          aria-label="返回"
        >
          ←
        </button>
        <h1 className="settings-page-title">设置</h1>
      </header>

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
                  onClick={onRemoteNodesOpen}
                  disabled={!onRemoteNodesOpen}
                >
                  <span className="settings-runtime-entry-title">Remote Nodes</span>
                  <span className="settings-runtime-entry-meta">Remote Agent file proxy</span>
                </button>
                <button
                  type="button"
                  className="settings-runtime-entry"
                  data-runtime-entry="helper-status"
                  data-authority-rail="helper-actuator"
                  onClick={onHelperStatusOpen}
                  disabled={!onHelperStatusOpen}
                >
                  <span className="settings-runtime-entry-title">Helper Status</span>
                  <span className="settings-runtime-entry-meta">Helper actuator enrollment</span>
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
