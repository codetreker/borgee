// main-view.test.ts — #682 sidepane 切换语义 + 未保存改动守卫合约.
//
// 这个测试锁两件事:
// 1. mainView 状态机的合法值 (反堆栈 — 同时只能一个 active)
// 2. unsaved-changes 守卫的注册 / 触发 / 取消三种路径
//
// 不渲染 React (不需要 RTL), 直接对 export 的 helper 单元测. 锁的是合约,
// 不是 React 行为.

import { describe, it, expect, beforeEach } from 'vitest';
import {
  registerUnsavedGuard,
  runUnsavedGuards,
  _clearUnsavedGuardsForTest,
} from '../hooks/useUnsavedChangesGuard';
import {
  ALL_MAIN_VIEWS,
  MAIN_VIEW_DEFAULT,
  isSidepane,
  type MainView,
} from '../lib/mainView';

beforeEach(() => {
  _clearUnsavedGuardsForTest();
});

describe('mainView state machine — #682 反堆栈', () => {
  it('默认是 channel — sidepane 不会预先 active', () => {
    expect(MAIN_VIEW_DEFAULT).toBe('channel');
  });

  it('合法值正好 7 个 (6 个 sidepane + 1 个 channel 默认)', () => {
    expect(ALL_MAIN_VIEWS).toHaveLength(7);
    expect(ALL_MAIN_VIEWS).toEqual([
      'channel',
      'agents',
      'invitations',
      'workspaces',
      'remote-nodes',
      'helper-status',
      'settings',
    ]);
  });

  it('isSidepane: channel 不是 sidepane, 其它都是', () => {
    expect(isSidepane('channel')).toBe(false);
    for (const v of ALL_MAIN_VIEWS.filter((x) => x !== 'channel')) {
      expect(isSidepane(v)).toBe(true);
    }
  });

  it('TS 类型锁: MainView 只能取 7 个值之一 (编译期检查, 这里只锚 runtime 数组)', () => {
    // 这个测试主要是给未来加 sidepane 的人提个醒 — 加新值要改 ALL_MAIN_VIEWS
    // + isSidepane 也要顾上.
    const _exhaustive: MainView = 'channel';
    expect(ALL_MAIN_VIEWS).toContain(_exhaustive);
  });
});

describe('useUnsavedChangesGuard — 未保存改动守卫', () => {
  it('没注册任何守卫时 runUnsavedGuards 返 true (放行切换)', () => {
    expect(runUnsavedGuards()).toBe(true);
  });

  it('注册了但都不 dirty 时返 true (放行)', () => {
    registerUnsavedGuard(() => false, '不该看到这条');
    registerUnsavedGuard(() => false, '也不该看到这条');
    expect(runUnsavedGuards()).toBe(true);
  });

  it('一个 dirty 守卫 → 调 confirmFn 拿用户决定', () => {
    registerUnsavedGuard(() => true, '有未保存的改动');
    let confirmedMessage = '';
    const result = runUnsavedGuards((msg) => {
      confirmedMessage = msg;
      return true; // 用户点确认离开
    });
    expect(confirmedMessage).toBe('有未保存的改动');
    expect(result).toBe(true);
  });

  it('用户取消时 runUnsavedGuards 返 false (停在原视图)', () => {
    registerUnsavedGuard(() => true, '会丢的改动');
    const result = runUnsavedGuards(() => false); // 用户点取消
    expect(result).toBe(false);
  });

  it('多个 dirty 守卫合并成一条消息只弹一次', () => {
    registerUnsavedGuard(() => true, 'Form A 没保存');
    registerUnsavedGuard(() => true, 'Form B 没保存');
    let callCount = 0;
    let lastMessage = '';
    runUnsavedGuards((msg) => {
      callCount += 1;
      lastMessage = msg;
      return true;
    });
    expect(callCount).toBe(1);
    expect(lastMessage).toContain('2 处未保存改动');
    expect(lastMessage).toContain('Form A 没保存');
    expect(lastMessage).toContain('Form B 没保存');
  });

  it('多个守卫但只有一个 dirty 时只显这一条消息, 不混列表', () => {
    registerUnsavedGuard(() => true, '只有 A dirty');
    registerUnsavedGuard(() => false, 'B 是干净的');
    let lastMessage = '';
    runUnsavedGuards((msg) => {
      lastMessage = msg;
      return true;
    });
    expect(lastMessage).toBe('只有 A dirty');
  });

  it('反注册函数能从 Set 里拿掉守卫', () => {
    const unregister = registerUnsavedGuard(() => true, '会被反注册');
    unregister();
    expect(runUnsavedGuards()).toBe(true); // 已经反注册, 放行
  });

  it('isDirty 函数动态读 — 同一个守卫不同时刻可以变 dirty / 不 dirty', () => {
    let dirty = false;
    registerUnsavedGuard(() => dirty, '动态 dirty');
    expect(runUnsavedGuards()).toBe(true); // 当前不 dirty, 放行
    dirty = true;
    expect(runUnsavedGuards(() => false)).toBe(false); // 现在 dirty, 用户取消, 拦住
    dirty = false;
    expect(runUnsavedGuards()).toBe(true); // 又不 dirty, 再放行
  });
});

describe('#682 集成场景 (mainView + 守卫合约一起)', () => {
  it('场景 1: 干净视图切换 — 切到 settings 不弹任何提示', () => {
    // 无守卫, 直接切.
    expect(runUnsavedGuards()).toBe(true);
  });

  it('场景 2: 有 dirty 守卫切换 — 用户确认离开, 切换继续', () => {
    registerUnsavedGuard(() => true, 'AgentManager 表单填了一半');
    const result = runUnsavedGuards(() => true); // 用户点离开
    expect(result).toBe(true);
  });

  it('场景 3: 有 dirty 守卫切换 — 用户取消, 不切换', () => {
    registerUnsavedGuard(() => true, 'AgentManager 表单填了一半');
    const result = runUnsavedGuards(() => false); // 用户点不要
    expect(result).toBe(false);
  });

  it('场景 4 (回归 #682): 之前堆栈 bug — 现在合法值只 7 种, 状态机里同时只能一个 active', () => {
    // 这个 case 概念锁: 5 个独立 boolean 时状态空间是 2^5 = 32, 31 种是
    // 'showAgents 跟 showSettings 同时 true' 之类的 bug. 改成 1 个字符串
    // 后状态空间收缩到 7 (合法集), 反堆栈是 by construction.
    expect(ALL_MAIN_VIEWS.length).toBe(7); // 锚: 7 种状态, 不是 32
  });
});
