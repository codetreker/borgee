// ws-envelope-flatten.test.ts — #678 / #680 group creation blank-screen fix.
//
// This test locks the contract between useWebSocket's ws.onmessage path and
// the handler switch in handleMessage: the server can send two frame shapes,
// while the handler expects the flattened shape. onmessage must flatten every
// frame before passing it to the handler.
//
// Why this helper has a focused test: the earlier group_created blank screen
// came from a contract mismatch. The server used BroadcastEventToAll and put
// group under data.group, while the client handler read the already-flattened
// `data.group` value and received undefined before dispatching to the reducer.
// Keeping flattening in a pure helper gives the regression test a stable target.

import { describe, it, expect } from 'vitest';
import { flattenWsFrame } from '../hooks/useWebSocket';

describe('flattenWsFrame — WebSocket envelope compatibility', () => {
  it('flattens BroadcastEventToAll shape `{type, data: {group}}` to `{type, group}`', () => {
    // Matches server hub.go:284-289 + channels.go:854:
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
    // data should not remain nested in the result; the handler does not expect it.
    expect(flat.data).toBeUndefined();
  });

  it('keeps direct flattened frame `{type, cursor, artifact_id, ...}` unchanged', () => {
    // Matches server cursor_test.go:175; the Push* series sends flattened frames directly:
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

  it('keeps both shapes when top-level cursor and nested data are present', () => {
    // Hypothetical case: BroadcastEventToAll wraps data while cursor stays at the
    // top level. The current server does not send this shape, but the helper
    // behavior should stay stable: top-level fields first, nested data overrides.
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

  it('prefers nested payload fields when data conflicts with top-level fields', () => {
    // Design choice: server BroadcastEventToAll puts semantic fields in data,
    // while the top level should only hold envelope metadata like type / cursor.
    // If names collide, the data value wins.
    const wire = {
      type: 'group_updated',
      group_id: 'OLD-LEGACY-FIELD',
      data: { group_id: 'g-new' },
    };
    const flat = flattenWsFrame(wire);
    expect(flat.group_id).toBe('g-new');
  });

  it('returns only top-level fields when data is null, undefined, or not an object', () => {
    expect(flattenWsFrame({ type: 'pong' })).toEqual({ type: 'pong' });
    expect(flattenWsFrame({ type: 'X', data: null })).toEqual({ type: 'X' });
    expect(flattenWsFrame({ type: 'X', data: 'string-not-obj' })).toEqual({ type: 'X' });
  });

  it('returns a safe empty frame for invalid input without throwing', () => {
    expect(flattenWsFrame(null)).toEqual({ type: '' });
    expect(flattenWsFrame(undefined)).toEqual({ type: '' });
    expect(flattenWsFrame('not-an-object')).toEqual({ type: '' });
    expect(flattenWsFrame(42)).toEqual({ type: '' });
  });

  it('#678 group creation blank-screen regression: flattened group_created gives handler a group', () => {
    // This case covers the issue #678 regression: before onmessage flattened
    // frames, the handler's case 'group_created' path read const group =
    // data.group as undefined. Dispatching ADD_GROUP then made the reducer read
    // group.id, causing a TypeError and a blank SPA.
    const wire = {
      type: 'group_created',
      data: { group: { id: 'g1', name: 'Test', position: '000001', created_by: 'u1', created_at: 1 } },
    };
    const flat = flattenWsFrame(wire);
    // This is the value the handler needs; it should no longer be undefined.
    expect(flat.group).toBeDefined();
    expect((flat.group as { id: string }).id).toBe('g1');
  });
});
