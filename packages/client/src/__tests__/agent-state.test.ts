// agent-state.test.ts — AL-1a (#R3 Phase 2) 文案锁单测.
// 改 REASON_LABELS / describeAgentState 字面 = 改这里 + server 同步.
import { describe, it, expect } from 'vitest';
import { describeAgentState, REASON_LABELS } from '../lib/agent-state';

describe('describeAgentState — 蓝图 §2.3 文案锁', () => {
  it('online → 在线', () => {
    expect(describeAgentState('online', undefined)).toEqual({ text: '在线', tone: 'ok' });
  });

  it('offline → 已离线 (野马 §11: 不准 idle 灰糊弄)', () => {
    expect(describeAgentState('offline', undefined)).toEqual({ text: '已离线', tone: 'muted' });
  });

  it('undefined state defaults to 已离线 (server 没回 state 时不糊弄)', () => {
    expect(describeAgentState(undefined, undefined)).toEqual({ text: '已离线', tone: 'muted' });
  });

  it('error api_key_invalid → 故障 (API key 失效)', () => {
    expect(describeAgentState('error', 'api_key_invalid')).toEqual({
      text: '故障 (API key 失效)',
      tone: 'error',
    });
  });

  it('error quota_exceeded → 故障 (已超出配额)', () => {
    expect(describeAgentState('error', 'quota_exceeded').text).toBe('故障 (已超出配额)');
  });

  it('error runtime_crashed → 故障 (Runtime 崩溃)', () => {
    expect(describeAgentState('error', 'runtime_crashed').text).toBe('故障 (Runtime 崩溃)');
  });

  it('error 没 reason 也得说"未知错误", 不准空括号糊弄', () => {
    expect(describeAgentState('error', undefined).text).toBe('故障 (未知错误)');
  });

  it('reason 表覆盖蓝图 §2.3 全部原因码 (lock — 加 reason 必须 + 中文 label)', () => {
    expect(Object.keys(REASON_LABELS).sort()).toEqual([
      'api_key_invalid',
      'network_unreachable',
      'quota_exceeded',
      'runtime_crashed',
      'runtime_timeout',
      'unknown',
    ]);
  });
});

describe('describeAgentState — AL-1b (#R3 Phase 4) busy/idle 文案锁', () => {
  // acceptance al-1b.md §3.1 — busy 文案锁 byte-identical "在工作" tone='ok'.
  // 反约束 §3.4 — 不准 "活跃" / "running" 等模糊 (烈马 grep 反查闸).
  it('busy → 在工作 tone=ok (acceptance §3.1, 反 "活跃"/"running")', () => {
    expect(describeAgentState('busy', undefined)).toEqual({ text: '在工作', tone: 'ok' });
  });

  // acceptance §3.2 — idle 文案锁 byte-identical "空闲" tone='muted'.
  // 反约束 §3.4 — 不准 "等待中" / "Standing by".
  it('idle → 空闲 tone=muted (acceptance §3.2, 反 "等待中"/"Standing by")', () => {
    expect(describeAgentState('idle', undefined)).toEqual({ text: '空闲', tone: 'muted' });
  });

  // acceptance §3.3 — AL-1a 三态文案不变, REG-AL1A-005 不破回归.
  // 改 busy/idle case 不许影响 online/offline/error 既有 case.
  it('AL-1a 三态文案不变 (REG-AL1A-005 回归不破)', () => {
    expect(describeAgentState('online', undefined).text).toBe('在线');
    expect(describeAgentState('offline', undefined).text).toBe('已离线');
    expect(describeAgentState('error', 'api_key_invalid').text).toBe('故障 (API key 失效)');
  });

  // 设计 ① 拆三路径 — busy/idle 跟 error 互斥 (server 端 5-state 合并优先级
  // error > busy > idle, 此 client 函数仅处理单 state 不重做合并).
  it('busy/idle 不带 reason — REASON_LABELS 不应被查 (跟 error 拆死)', () => {
    // 不传 reason 也不抛异常 + 文案稳定; 即使误传 reason 也不染 busy/idle 文案.
    expect(describeAgentState('busy', 'api_key_invalid').text).toBe('在工作');
    expect(describeAgentState('idle', 'runtime_timeout').text).toBe('空闲');
  });
});
