// IteratePanel.test.tsx — CV-4.3 acceptance §3 vitest 锁.
//
// 锚: docs/qa/cv-4-content-lock.md §1 ②③⑦ + acceptance §3 + spec §0 设计 ②④⑤⑥⑦.
//
// state 4 态 byte-identical (改 = 改三处 #249 + AL-3 #305 + 此组件) +
// failed reason 走 REASON_LABELS 单测锁 + 反向断言无 "重试" / "Retry"
// 按钮 (失败状态机锁死, 设计 ⑦ 永久锁).
import { describe, it, expect } from 'vitest';
import { stateLabel } from '../components/IteratePanel';
import { REASON_LABELS } from '../lib/agent-state';

describe('IteratePanel.stateLabel — 4 态文案 byte-identical', () => {
  it('pending → "等待 agent 开始…"', () => {
    expect(stateLabel('pending', {})).toBe('等待 agent 开始…');
  });

  it('running → "agent 正在迭代…"', () => {
    expect(stateLabel('running', {})).toBe('agent 正在迭代…');
  });

  it('completed → "已生成 v{N}"', () => {
    expect(stateLabel('completed', { newVersionId: 7 })).toBe('已生成 v7');
  });

  it('completed 缺 newVersionId → "已生成 v?" (优雅降级)', () => {
    expect(stateLabel('completed', {})).toBe('已生成 v?');
    expect(stateLabel('completed', { newVersionId: null })).toBe('已生成 v?');
  });

  it.each(Object.entries(REASON_LABELS))(
    'failed + reason=%s → "失败: {label}" byte-identical 跟 #249 + AL-3 #305 三处单测锁',
    (reason, expectedLabel) => {
      const out = stateLabel('failed', { reason: reason as keyof typeof REASON_LABELS });
      expect(out).toBe(`失败: ${expectedLabel}`);
    },
  );

  it('failed 缺 reason → "失败: 未知错误" 优雅降级', () => {
    expect(stateLabel('failed', {})).toBe('失败: 未知错误');
    expect(stateLabel('failed', { reason: null })).toBe('失败: 未知错误');
  });

  // 反向断言 — 同义词漂移防御 (content-lock §2 黑名单 grep 同精神).
  it.each([
    'Pending',
    'Running',
    'Completed',
    'Failed',
    '处理中',
    '进行中',
    '出错: ',
    '成功',
  ])('rejects 同义词漂移: %s', (synonym) => {
    const allLabels = [
      stateLabel('pending', {}),
      stateLabel('running', {}),
      stateLabel('completed', { newVersionId: 1 }),
      stateLabel('failed', { reason: 'unknown' }),
    ];
    for (const lbl of allLabels) {
      expect(lbl).not.toContain(synonym);
    }
  });
});

describe('IteratePanel — 设计 ⑦ failed 不渲染 重试 / Retry 按钮 (源码层锁)', () => {
  // 跟 #338 cross-grep 反模式遵守 — 源码字面 grep, 不能在源码出现 button.
  // 用 vite ?raw import 加载源码字符串 (跟 ArtifactPanel-kind-switch.test.tsx
  // 同模式).
  // @ts-ignore vite ?raw import lacks built-in TS module decl
  let src: string;
  // 先用 import 语法 (同模块顶层)
  // 注: vitest 直接支持 vite 转换器 — ?raw 走 vite 内置 raw plugin.
  //
  // 为避免顶层 await 仍占该 describe scope, 用 fetch via vite import.

  it.each(['重试', 'Retry', '重新尝试', '再试一次'])(
    '源码不含 %s 按钮文案 (失败状态机锁死, 设计 ⑦)',
    async (synonym) => {
      if (!src) {
        // @ts-ignore vite raw import
        const mod = await import('../components/IteratePanel.tsx?raw');
        src = (mod as { default: string }).default;
      }
      const buttonPattern = new RegExp(`<button[^>]*>[^<]*${synonym}|onClick=.*${synonym}`);
      expect(src).not.toMatch(buttonPattern);
    },
  );
});
