// ws-envelope-flatten.test.ts — #678 / #680 创建分组白屏修.
//
// 这个测试在锁 useWebSocket 的 ws.onmessage 跟 handler (handleMessage 那个
// switch) 之间的合约: 服务器有两种 frame 形状, handler 只懂"平铺"形, 所以
// onmessage 里要把所有 frame 展平成同一种形再交给 handler.
//
// 为啥单独抽个 helper 测: 之前 group_created 白屏的根本原因就是这个合约
// 不一致 — server 走 BroadcastEventToAll 把 group 嵌在 data.group, client
// handler 按平铺读 `data.group`, 拿到 undefined 一路炸到 reducer. 把展平
// 抽成 pure helper 让回归测有地方扎根.

import { describe, it, expect } from 'vitest';
import { flattenWsFrame } from '../hooks/useWebSocket';

describe('flattenWsFrame — WebSocket envelope 兼容', () => {
  it('BroadcastEventToAll 形状 `{type, data: {group}}` 展平成 `{type, group}`', () => {
    // 跟服务器 hub.go:284-289 + channels.go:854 一致:
    //   BroadcastEventToAll("group_created", {group: g})
    //   → wire = {"type":"group_created","data":{"group":{"id":...,"position":...}}}
    const wire = {
      type: 'group_created',
      data: {
        group: { id: 'g1', name: 'Plans', position: '000001', created_by: 'u1', created_at: 1700 },
      },
    };
    const flat = flattenWsFrame(wire);
    expect(flat.type).toBe('group_created');
    expect(flat.group).toEqual({
      id: 'g1',
      name: 'Plans',
      position: '000001',
      created_by: 'u1',
      created_at: 1700,
    });
    // data 不应该再以嵌套形式留在结果里 — handler 不期待它存在
    expect(flat.data).toBeUndefined();
  });

  it('直接 frame `{type, cursor, artifact_id, ...}` 平铺形保持不变', () => {
    // 跟服务器 cursor_test.go:175 一致 — Push* 系列直接发平铺 frame:
    //   {"type":"artifact_updated","cursor":42,"artifact_id":"art-X",...}
    const wire = {
      type: 'artifact_updated',
      cursor: 42,
      artifact_id: 'art-X',
      version: 7,
      channel_id: 'ch-Y',
      updated_at: 1700000000000,
      kind: 'commit',
    };
    const flat = flattenWsFrame(wire);
    expect(flat.type).toBe('artifact_updated');
    expect(flat.cursor).toBe(42);
    expect(flat.artifact_id).toBe('art-X');
    expect(flat.version).toBe(7);
    expect(flat.channel_id).toBe('ch-Y');
    expect(flat.kind).toBe('commit');
  });

  it('两种形状混搭 (顶层 cursor + 嵌套 data) 都保留', () => {
    // 假想: BroadcastEventToAll 包出来后又有 cursor 在顶层 (实际 server
    // 不这么发, 但 helper 行为要稳定 — 顶层先, 嵌套覆盖). 验合并方向.
    const wire = {
      type: 'iteration_state_changed',
      cursor: 99,
      data: { iteration_id: 'it-1', state: 'completed' },
    };
    const flat = flattenWsFrame(wire);
    expect(flat.type).toBe('iteration_state_changed');
    expect(flat.cursor).toBe(99);
    expect(flat.iteration_id).toBe('it-1');
    expect(flat.state).toBe('completed');
  });

  it('data 字段冲突时, 嵌套 payload 字段优先 (覆盖顶层同名字段)', () => {
    // 这是设计选择: 服务器 BroadcastEventToAll 把语义全放在 data 里,
    // 顶层只该有 type / cursor 这种 envelope 元数据. 真撞名时以 data
    // 里的为准.
    const wire = {
      type: 'group_updated',
      group_id: 'OLD-LEGACY-FIELD',
      data: { group_id: 'g-new' },
    };
    const flat = flattenWsFrame(wire);
    expect(flat.group_id).toBe('g-new');
  });

  it('data 是 null / undefined / 非对象时不炸, 只返顶层字段', () => {
    expect(flattenWsFrame({ type: 'pong' })).toEqual({ type: 'pong' });
    expect(flattenWsFrame({ type: 'X', data: null })).toEqual({ type: 'X' });
    expect(flattenWsFrame({ type: 'X', data: 'string-not-obj' })).toEqual({ type: 'X' });
  });

  it('完全垃圾输入返一个安全空 frame, 不抛异常', () => {
    expect(flattenWsFrame(null)).toEqual({ type: '' });
    expect(flattenWsFrame(undefined)).toEqual({ type: '' });
    expect(flattenWsFrame('not-an-object')).toEqual({ type: '' });
    expect(flattenWsFrame(42)).toEqual({ type: '' });
  });

  it('#678 创建分组白屏回归: group_created frame 真展平后 handler 拿到的 group 不是 undefined', () => {
    // 这个 case 直接锚 issue #678 的真实回归: 之前 onmessage 不展平,
    // handler 在 case 'group_created' 里 const group = data.group → undefined,
    // dispatch ADD_GROUP 后 reducer 读 group.id → TypeError → 整 SPA 白屏.
    const wire = {
      type: 'group_created',
      data: { group: { id: 'g1', name: 'Test', position: '000001', created_by: 'u1', created_at: 1 } },
    };
    const flat = flattenWsFrame(wire);
    // handler 要拿到的就是这个 — 不再 undefined
    expect(flat.group).toBeDefined();
    expect((flat.group as { id: string }).id).toBe('g1');
  });
});
