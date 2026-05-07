import React, { useEffect, useState, useCallback } from 'react';
import { AppProvider, useAppContext } from './context/AppContext';
import { ThemeProvider } from './context/ThemeContext';
import { ToastProvider } from './components/Toast';
import Sidebar from './components/Sidebar';
import ChannelView from './components/ChannelView';
import LoginPage from './components/LoginPage';
import RegisterPage from './components/RegisterPage';
import AgentManager from './components/AgentManager';
import InvitationsInbox from './components/InvitationsInbox';
import WorkspaceManager from './components/WorkspaceManager';
import NodeManager from './components/NodeManager';
import SettingsPage from './components/Settings/SettingsPage';
import BannerImpersonate from './components/Settings/BannerImpersonate';
import { getMyImpersonateGrant, revokeMyImpersonateGrant } from './lib/api';
import { useWebSocket } from './hooks/useWebSocket';
import { fetchMe, ApiError } from './lib/api';
import { runUnsavedGuards } from './hooks/useUnsavedChangesGuard';
import type { MainView } from './lib/mainView';
import { MAIN_VIEW_DEFAULT } from './lib/mainView';
import './index.css';
import 'highlight.js/styles/github.css';

const AUTH_READY_TIMEOUT_MS = 500;
const AUTH_READY_POLL_MS = 50;

async function waitForAuthReady(): Promise<void> {
  const startedAt = Date.now();

  while (Date.now() - startedAt < AUTH_READY_TIMEOUT_MS) {
    try {
      await fetchMe();
      return;
    } catch (err) {
      if (err instanceof ApiError && err.status !== 401) return;
    }

    await new Promise(resolve => setTimeout(resolve, AUTH_READY_POLL_MS));
  }
}

function AppInner() {
  const { state, actions, dispatch, setSendWsMessage, setRegisterAckTimer } = useAppContext();
  const { subscribe, sendWsMessage, registerAckTimer } = useWebSocket();
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [isMobile, setIsMobile] = useState(window.innerWidth < 768);
  const [authChecked, setAuthChecked] = useState(false);
  const [authenticated, setAuthenticated] = useState(false);
  const [showRegister, setShowRegister] = useState(false);
  // #682 修: 5 个独立 boolean (showAgents/showInvitations/showWorkspaces/
  // showRemoteNodes/showSettings) 合并成 1 个 mainView 字符串. 只有一个视
  // 图能 active, 切换 = 替换值, 自动关掉前一个. 反堆栈 bug.
  const [mainView, setMainView] = useState<MainView>(MAIN_VIEW_DEFAULT);

  // Wire sendWsMessage into context
  useEffect(() => {
    setSendWsMessage(sendWsMessage);
    setRegisterAckTimer(registerAckTimer);
  }, [sendWsMessage, setSendWsMessage, registerAckTimer, setRegisterAckTimer]);

  // Responsive check
  useEffect(() => {
    const handler = () => {
      const mobile = window.innerWidth < 768;
      setIsMobile(mobile);
      if (!mobile) setSidebarOpen(false);
    };
    window.addEventListener('resize', handler);
    return () => window.removeEventListener('resize', handler);
  }, []);

  // Auth check
  useEffect(() => {
    fetchMe()
      .then(() => {
        setAuthenticated(true);
        setAuthChecked(true);
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) {
          setAuthenticated(false);
        } else {
          setAuthenticated(false);
        }
        setAuthChecked(true);
      });
  }, []);

  // Initialize app data after auth
  useEffect(() => {
    if (!authenticated) return;
    let cancelled = false;

    const init = async () => {
      try {
        await actions.loadCurrentUser();
        await actions.loadPermissions();
        await actions.loadChannels();
        await actions.loadOnlineUsers();
      } finally {
        if (!cancelled) dispatch({ type: 'SET_INITIALIZED' });
      }
    };
    init();

    const interval = setInterval(() => {
      actions.loadOnlineUsers();
    }, 30_000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [authenticated, actions, dispatch]);

  // Auto-select welcome (type='system') first, otherwise first channel —
  // CM-onboarding §1.4: first eye must not land on a blank screen.
  useEffect(() => {
    if (!state.initialized || state.currentChannelId) return;
    const welcome = state.channels.find(c => c.type === 'system');
    const target = welcome ?? state.channels[0];
    if (target) actions.selectChannel(target.id);
  }, [state.initialized, state.currentChannelId, state.channels, actions]);

  // CM-onboarding: welcome system message carries a quick_action button.
  // MessageItem dispatches a window event so this component can flip the
  // AgentManager without a prop chain.
  useEffect(() => {
    const handler = (e: Event) => {
      const detail = (e as CustomEvent<{ action?: string }>).detail;
      if (detail?.action === 'open_agent_manager') {
        setMainView('agents');
      }
    };
    window.addEventListener('borgee:quick-action', handler);
    return () => window.removeEventListener('borgee:quick-action', handler);
  }, []);

  // Auto-subscribe to all joined channels via WebSocket
  useEffect(() => {
    if (!state.initialized) return;
    for (const ch of state.channels) {
      if (ch.is_member) {
        subscribe(ch.id);
      }
    }
  }, [state.initialized, state.channels, subscribe]);

  const toggleSidebar = useCallback(() => {
    setSidebarOpen(o => !o);
  }, []);

  const closeSidebar = useCallback(() => {
    setSidebarOpen(false);
  }, []);

  const closeAllViews = useCallback(() => {
    setMainView('channel');
  }, []);

  // #682 修: requestMainView 是 sidebar 按钮唯一调用的入口. 切换前跑一遍未
  // 保存改动守卫 (useUnsavedChangesGuard 注册的), 用户取消就不切, 不丢改动.
  const requestMainView = useCallback((target: MainView) => {
    if (target === mainView) return;
    if (!runUnsavedGuards()) return;
    setMainView(target);
  }, [mainView]);

  const handleLogin = useCallback(async () => {
    await waitForAuthReady();
    setAuthenticated(true);
  }, []);

  const handleLogout = useCallback(() => {
    setAuthenticated(false);
    dispatch({ type: 'SET_CURRENT_USER', user: null });
  }, [dispatch]);

  if (!authChecked) {
    return (
      <div className="app-loading">
        <div className="loading-spinner-large" />
      </div>
    );
  }

  if (!authenticated) {
    if (showRegister) {
      return <RegisterPage onLogin={handleLogin} onBack={() => setShowRegister(false)} />;
    }
    return <LoginPage onLogin={handleLogin} onRegister={() => setShowRegister(true)} />;
  }

  if (!state.initialized) {
    return (
      <div className="app-loading">
        <div className="loading-spinner-large" />
        <p>加载中...</p>
      </div>
    );
  }

  return (
    <div className="app">
      {/* ADM-2.2 红横幅 (蓝图 §1.4 红线 2 + ADM-1 §4.1 R3 第 2 条 兑现) —
          顶部常驻显示 active impersonate grant; 立场 ⑦ + content-lock §2. */}
      <BannerImpersonate
        fetchGrant={() => getMyImpersonateGrant().then((r) => r.grant)}
        revokeGrant={() => revokeMyImpersonateGrant()}
      />
      {isMobile && (
        <button className="hamburger-btn" onClick={toggleSidebar}>
          ☰
        </button>
      )}

      {isMobile && sidebarOpen && (
        <div className="sidebar-overlay" onClick={closeSidebar} />
      )}
      <div className={`sidebar-wrapper ${isMobile ? (sidebarOpen ? 'sidebar-open' : 'sidebar-closed') : ''}`}>
        <Sidebar onClose={isMobile ? closeSidebar : undefined} onChannelSelect={closeAllViews} onLogout={handleLogout} onAgentsOpen={() => requestMainView('agents')} onInvitationsOpen={() => requestMainView('invitations')} onWorkspacesOpen={() => requestMainView('workspaces')} onRemoteNodesOpen={() => requestMainView('remote-nodes')} onSettingsOpen={() => requestMainView('settings')} />
      </div>

      <div className="main-content">
        {mainView === 'agents' ? (
          <AgentManager onBack={() => setMainView('channel')} />
        ) : mainView === 'invitations' ? (
          <InvitationsInbox
            onBack={() => setMainView('channel')}
            onJumpToChannel={(channelId) => {
              actions.selectChannel(channelId);
              setMainView('channel');
            }}
          />
        ) : mainView === 'workspaces' ? (
          <WorkspaceManager onBack={() => setMainView('channel')} />
        ) : mainView === 'remote-nodes' ? (
          <NodeManager onBack={() => setMainView('channel')} />
        ) : mainView === 'settings' ? (
          <SettingsPage onBack={() => setMainView('channel')} />
        ) : state.currentChannelId ? (
          <ChannelView channelId={state.currentChannelId} />
        ) : (
          <div className="no-channel">
            {/* CM-onboarding §3 step 1 reduced state — onboarding-journey.md
                locks this copy. Channels load failed or registration's
                welcome-channel insert errored: don't go silent. */}
            <p>正在准备你的工作区, 稍候刷新…</p>
            <button
              type="button"
              className="no-channel-retry"
              onClick={() => actions.loadChannels()}
            >
              重试
            </button>
          </div>
        )}
      </div>

    </div>
  );
}

export default function App() {
  return (
    <ThemeProvider>
      <AppProvider>
        <ToastProvider>
          <AppInner />
        </ToastProvider>
      </AppProvider>
    </ThemeProvider>
  );
}
