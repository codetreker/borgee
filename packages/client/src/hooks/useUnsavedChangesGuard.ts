// useUnsavedChangesGuard — sidepane 切换时给有未保存改动的视图机会先问要不要离开.
//
// 这个 hook 解决 #682 的 product-side 部分: 切换 sidepane 时, 当前视图如果
// 还有未保存的表单改动, 弹一个 window.confirm 让用户选 "继续切换" 还是
// "停在这里". 用户决定的产品方向, 反"切换就静默丢"的 UX bug.
//
// 设计:
// - 注册表是 module-level 的一个 Set, 不走 React Context — 这样 App.tsx
//   的 requestMainView 不需要 useContext, 任何视图都能注册退出守卫.
// - 一个视图可以注册多个守卫 (例如同时有两个 form), 全部 dirty 才弹一次
//   提示 (合并消息).
// - 守卫返回 true = 这个视图当前 dirty / 不要静默切换; false = 干净的, 可
//   以直接切.
// - hook 只在 mount 期间注册, unmount 自动反注册 (React useEffect cleanup).
// - 故意不内置 "保存" 按钮 — Save 在每个视图的具体 form 里, 这个 hook
//   只负责挡退路 + 拿用户决定. 守卫被触发后用户选 "确认离开", 视图来不及
//   保存就走 — 由产品方向决定 (用户已经看到弹窗自己点了离开).
//
// 用法 (任意视图组件内):
//   useUnsavedChangesGuard(() => isDirty, '有未保存的改动, 确认离开吗?');
//
// 测试:
// - 见 packages/client/src/__tests__/main-view.test.tsx 锁切换语义 + 守卫
//   合约.
// - registerUnsavedGuard / runUnsavedGuards 都 export 让单测可以直接验,
//   不用搞 React Testing Library 的全 App 渲染.

import { useEffect } from 'react';

type Guard = {
  isDirty: () => boolean;
  message: string;
};

const guards = new Set<Guard>();

const DEFAULT_MESSAGE = '有未保存的改动, 确认离开当前页面吗?';

/**
 * 注册一个未保存改动的守卫. 返回反注册函数.
 * 直接 export 给 hook 用; 测试也可以直接调.
 */
export function registerUnsavedGuard(isDirty: () => boolean, message: string = DEFAULT_MESSAGE): () => void {
  const guard: Guard = { isDirty, message };
  guards.add(guard);
  return () => {
    guards.delete(guard);
  };
}

/**
 * 跑一遍所有注册的守卫. 任何一个 isDirty() === true 就触发 confirmation.
 * 返回 true 表示 "可以继续切换" (没 dirty / 用户确认离开),
 * 返回 false 表示 "停在原视图" (用户取消).
 *
 * 默认用 window.confirm; 测试可以替换 confirmFn 注入.
 */
export function runUnsavedGuards(
  confirmFn: (message: string) => boolean = (msg) => window.confirm(msg),
): boolean {
  const dirtyMessages: string[] = [];
  for (const g of guards) {
    if (g.isDirty()) dirtyMessages.push(g.message);
  }
  if (dirtyMessages.length === 0) return true;
  // 多个 dirty 守卫合并成一条消息, 只弹一次.
  const merged = dirtyMessages.length === 1
    ? dirtyMessages[0]
    : `${dirtyMessages.length} 处未保存改动:\n` + dirtyMessages.map((m, i) => `${i + 1}. ${m}`).join('\n') + '\n\n确认离开吗?';
  return confirmFn(merged);
}

/**
 * 测试用 — 清空所有注册的守卫. 不用于产品代码.
 */
export function _clearUnsavedGuardsForTest(): void {
  guards.clear();
}

/**
 * React hook: 在组件 mount 期间注册一个 dirty 守卫, unmount 自动清理.
 *
 * isDirty 是 stable 引用 (用 useCallback) 还是每次 render 新建都行 — guard
 * 注册一次后存的是当前的 isDirty 函数引用; 如果想运行时更新条件, 请通过
 * isDirty 内部读 ref 或 state.
 *
 * 用 useEffect 只在 mount/unmount 触发; 我们不重新注册以避免抖动.
 */
export function useUnsavedChangesGuard(isDirty: () => boolean, message?: string): void {
  useEffect(() => {
    const unregister = registerUnsavedGuard(isDirty, message);
    return unregister;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
}
