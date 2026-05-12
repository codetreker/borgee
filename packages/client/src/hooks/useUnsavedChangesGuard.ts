// useUnsavedChangesGuard — sidepane 切换时给有未保存改动的视图机会先问要不要离开.
//
// 这个 hook 解决 #682 的 product-side 部分: 切换 sidepane 时, 当前视图如果
// 还有未保存的表单改动, 弹一个 window.confirm 让用户选 "继续切换" 还是
// "停在这里". 用户决定的产品方向, 反"切换就静默丢"的 UX bug.
//
// 实现说明:
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

import { useEffect, useRef } from 'react';

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
 * # useRef 包 isDirty 反 React closure staleness (#695 feima review 抓的)
 *
 * 调用方常这么写 (AgentManager.tsx 就是):
 *
 *   useUnsavedChangesGuard(
 *     () => createdKey === null && (displayName.trim() !== '' || agentId.trim() !== ''),
 *     '...'
 *   );
 *
 * 这个箭头函数闭包绑定的是 mount 那一刻的 displayName / agentId / createdKey
 * (都是空 / null). 之前的实现把这个闭包直接存进 module-level Set, 后来用户
 * 填表单 React 重新 render 但 hook 里的 isDirty 没变 — Set 里那条永远拿
 * 旧闭包跑, 永远返 false. 守卫对外看着注册了, 实际不工作.
 *
 * 修法: 用 useRef 装一个 *引用*, 每次 render 把最新的 isDirty 写进
 * `isDirtyRef.current`, useEffect 只在 mount 时把"调 ref" 注册进 Set.
 * 这样 Set 里那条永远调当前 render 的 isDirty, 拿到当前 React state.
 *
 * deps 改成 `[message]` 而不是 `[]` — message 改了重新注册让 Set 里存的
 * message 跟显示的同步; isDirty 的 staleness 由 ref 解决, 不用进 deps.
 */
export function useUnsavedChangesGuard(isDirty: () => boolean, message?: string): void {
  const isDirtyRef = useRef(isDirty);
  isDirtyRef.current = isDirty;
  useEffect(() => {
    const unregister = registerUnsavedGuard(() => isDirtyRef.current(), message);
    return unregister;
  }, [message]);

  // gh#703 PR-2/2 — beforeunload 监听: 浏览器 ctrl+W / 关 tab / refresh 时
  // 如果当前 dirty, 让浏览器弹原生提示 ("您输入的内容可能不会被保存"). 跨
  // 5 处 form (AgentManager / AgentConfigPanel / DescriptionEditor /
  // NodeManager 双 form) 自动获益, 不需要每处自己写. Constraint: 不挂自定义
  // modal (跟 CV-10 ArtifactCommentDraftInput rule ② 一致); 现代浏览器忽略
  // message, 设 returnValue 触发原生提示就行.
  //
  // 空 deps OK — isDirtyRef 是 ref, handler 关到 ref 没 staleness 问题
  // (跟上面的 unregister 同理).
  useEffect(() => {
    const handler = (e: BeforeUnloadEvent) => {
      if (!isDirtyRef.current()) return;
      e.preventDefault();
      e.returnValue = '';
    };
    window.addEventListener('beforeunload', handler);
    return () => window.removeEventListener('beforeunload', handler);
  }, []);
}
