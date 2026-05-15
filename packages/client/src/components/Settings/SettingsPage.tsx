// SettingsPage — ADM-1 用户设置页骨架 (Phase 4 启动 milestone) + ADM-2 扩展.
//
// Blueprint: docs/blueprint/current/admin-model.md §4.1 + §1.4 (ADM-2 audit + impersonate)
// Spec: docs/qa/adm-1-implementation-spec.md §1 + docs/implementation/modules/adm-2-spec.md §2
// Acceptance: docs/qa/acceptance-templates/adm-1.md §2 + adm-2.md §4.1.c+§4.2.a
//
// 设计反查:
//   - v1 仅一个 tab "隐私" 默认展开 (反 details-element 包裹, acceptance §2.3)
//   - 后续 tab (账号 / 通知) 留 placeholder 但不入 v1
//   - 跟 admin SPA SettingsPage (packages/client/src/admin/pages/) 路径分叉
//     (ADM-0 constraint: admin/user 路径不混用, spec §1 第 ② 项注释字面)
//   - ADM-2 子段: PrivacyPromise (ADM-1) + ImpersonateGrantSection (ADM-2 业主授权
//     §4.2.a) + AdminActionsList (ADM-2 影响记录 §4.1.c) 同 tab 三段
//
// 反约束:
//   - URL `?tab=privacy` deep-link (默认 'privacy', 唯一支持的 tab)
//   - 不引入 react-router (跟 App.tsx showAgents/showInvitations 同模式 — App-level
//     state 切视图)
import PrivacyPromise from './PrivacyPromise';
import AdminActionsList from './AdminActionsList';
import ImpersonateGrantSection from './ImpersonateGrantSection';
import { PermissionsView } from '../PermissionsView';
import ChannelManagementSurface from './ChannelManagementSurface';
import { useState } from 'react';
import {
  getMyAdminActions,
  getMyImpersonateGrant,
  createMyImpersonateGrant,
  revokeMyImpersonateGrant,
} from '../../lib/api';

interface Props {
  onBack: () => void;
  onRemoteNodesOpen?: () => void;
  onHelperStatusOpen?: () => void;
}

export type SettingsTab = 'privacy' | 'runtime' | 'channels';

export default function SettingsPage({ onBack, onRemoteNodesOpen, onHelperStatusOpen }: Props) {
  const [activeTab, setActiveTab] = useState<SettingsTab>('privacy');

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
          className={`settings-tab${activeTab === 'privacy' ? ' active' : ''}`}
          data-tab="privacy"
          aria-current={activeTab === 'privacy' ? 'page' : undefined}
          onClick={() => setActiveTab('privacy')}
        >
          隐私
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
        <button
          type="button"
          className={`settings-tab${activeTab === 'runtime' ? ' active' : ''}`}
          data-tab="runtime"
          aria-current={activeTab === 'runtime' ? 'page' : undefined}
          onClick={() => setActiveTab('runtime')}
        >
          运行时
        </button>
      </nav>

      <main className="settings-page-content">
        {activeTab === 'privacy' && (
          <>
            <PrivacyPromise />
            {/* ADM-2.2 业主授权 24h impersonate (acceptance §4.2.a; 设计 ⑦ +
                content-lock §3) — 跟 PrivacyPromise 同 tab. */}
            <ImpersonateGrantSection
              fetchGrant={() => getMyImpersonateGrant().then((r) => r.grant)}
              createGrant={() => createMyImpersonateGrant().then((r) => r.grant)}
              revokeGrant={() => revokeMyImpersonateGrant()}
            />
            {/* ADM-2.2 影响记录 (acceptance §4.1.c; 设计 ④ 只见自己 +
                content-lock §4 字面). */}
            <AdminActionsList
              fetchActions={() => getMyAdminActions().then((r) => r.actions)}
            />
            <section className="settings-permissions-section" data-settings-permissions-surface="true">
              <PermissionsView />
            </section>
          </>
        )}
        {activeTab === 'runtime' && (
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
        )}
        {activeTab === 'channels' && <ChannelManagementSurface />}
      </main>
    </div>
  );
}
