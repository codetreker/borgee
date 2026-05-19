// NavigationContext — App 主区域视图导航栈.
//
// 原 App.tsx 用单个 mainView 字符串切视图, 任何"返回"都硬编码跳 'channel'
// (例如 SettingsPage 左上 ← 调 onBack → setMainView('channel')). 用户看到的
// 是"← 把整个堆栈关了, 不是真正后退一步" — 从 settings 点进 remote-nodes
// 再点 ← 应该回到 settings, 不是直接回 channel.
//
// 修法: 把 mainView 升成栈 (stack). push 入栈 / back 出栈一层 / close 清栈
// 回 'channel'. 'channel' 是隐含主页, 栈空 fallback 它. 不引 react-router
// 不持久化 URL — 跟 SettingsPage 反约束一致 (App-level state 切视图).
import React, {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
} from 'react';
import type { MainView } from '../../lib/mainView';
import { MAIN_VIEW_DEFAULT } from '../../lib/mainView';

export interface NavigationApi {
  current: MainView;
  /** 入栈; 栈顶已是 view 时不重复 push (防爆栈). */
  push: (view: MainView) => void;
  /** 出栈一层. 栈只剩 1 层时替成 'channel' (等价 close). */
  back: () => void;
  /** 清栈直接回 'channel' (用户拍: close 永远到 channel). */
  close: () => void;
  /** 栈 >1 层时为 true; 用来给 PageHeader 决定是否显 ←. */
  canGoBack: boolean;
}

const NavigationContext = createContext<NavigationApi | null>(null);

interface NavigationProviderProps {
  children: React.ReactNode;
  initial?: MainView;
}

export function NavigationProvider({
  children,
  initial = MAIN_VIEW_DEFAULT,
}: NavigationProviderProps) {
  const [stack, setStack] = useState<MainView[]>([initial]);

  const push = useCallback((view: MainView) => {
    setStack(prev => {
      if (prev[prev.length - 1] === view) return prev;
      return [...prev, view];
    });
  }, []);

  const back = useCallback(() => {
    setStack(prev => {
      if (prev.length > 1) return prev.slice(0, -1);
      return [MAIN_VIEW_DEFAULT];
    });
  }, []);

  const close = useCallback(() => {
    setStack([MAIN_VIEW_DEFAULT]);
  }, []);

  const value = useMemo<NavigationApi>(() => ({
    current: stack[stack.length - 1],
    push,
    back,
    close,
    canGoBack: stack.length > 1,
  }), [stack, push, back, close]);

  return (
    <NavigationContext.Provider value={value}>
      {children}
    </NavigationContext.Provider>
  );
}

export function useNavigation(): NavigationApi {
  const ctx = useContext(NavigationContext);
  if (!ctx) {
    throw new Error('useNavigation must be used within NavigationProvider');
  }
  return ctx;
}
