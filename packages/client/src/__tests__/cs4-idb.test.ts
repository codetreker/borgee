// CS-4.1 — IndexedDB wrapper SSOT 单测 (cs-4-stance-checklist 设计 ① + content-lock §3).
//
// fake-indexeddb 注入: 顶部 import 'fake-indexeddb/auto' 提供 jsdom 缺少的 IDB.
import 'fake-indexeddb/auto';
import { describe, it, expect, beforeEach } from 'vitest';
import {
  openCS4DB,
  cs4Get,
  cs4Put,
  cs4Delete,
  clearStaleEntries,
  STORE_MESSAGES,
  STORE_LAST_READ_AT,
  STORE_AGENT_STATE,
} from '../lib/cs4-idb';

beforeEach(async () => {
  // Reset the fake IndexedDB instance between tests.
  // @ts-expect-error fake-indexeddb internals
  const { default: FDBFactory } = await import('fake-indexeddb/lib/FDBFactory');
  globalThis.indexedDB = new FDBFactory();
});

describe('CS-4.1 — openCS4DB schema (蓝图 §1.4 3 store 拆死)', () => {
  it('TestCS41_DBOpensWithSchema — open succeeds', async () => {
    const db = await openCS4DB();
    expect(db.name).toBe('borgee-cs4');
    expect(db.version).toBe(1);
    db.close();
  });

  it('TestCS41_3StoreCreated — 3 store byte-identical', async () => {
    const db = await openCS4DB();
    const stores = Array.from(db.objectStoreNames).sort();
    expect(stores).toEqual([STORE_AGENT_STATE, STORE_LAST_READ_AT, STORE_MESSAGES].sort());
    db.close();
  });

  it('TestCS41_OnUpgradeNeededFires — second open with same version no upgrade', async () => {
    const db1 = await openCS4DB();
    expect(db1.objectStoreNames.contains(STORE_MESSAGES)).toBe(true);
    db1.close();
    // Re-open: schema should persist
    const db2 = await openCS4DB();
    expect(db2.objectStoreNames.contains(STORE_MESSAGES)).toBe(true);
    db2.close();
  });

  it('TestCS41_GetPutRoundtrip — messages store round-trip', async () => {
    const db = await openCS4DB();
    const msg = {
      id: 'm1',
      channel_id: 'c1',
      body: 'hi',
      sender_id: 'u1',
      cursor: 'c1.cursor1',
      ts_ms: 1000,
    };
    await cs4Put(db, STORE_MESSAGES, msg);
    const got = await cs4Get(db, STORE_MESSAGES, 'm1');
    expect(got).toEqual(msg);
    db.close();
  });

  it('TestCS41_Delete — removes value', async () => {
    const db = await openCS4DB();
    await cs4Put(db, STORE_LAST_READ_AT, { channel_id: 'c1', cursor: 'x' });
    await cs4Delete(db, STORE_LAST_READ_AT, 'c1');
    const got = await cs4Get(db, STORE_LAST_READ_AT, 'c1');
    expect(got).toBeUndefined();
    db.close();
  });

  it('TestCS41_ClearStale — removes entries older than maxAge', async () => {
    const db = await openCS4DB();
    await cs4Put(db, STORE_AGENT_STATE, { agent_id: 'a1', state: 'online', updated_at_ms: 1000 });
    await cs4Put(db, STORE_AGENT_STATE, { agent_id: 'a2', state: 'online', updated_at_ms: 9000 });
    const removed = await clearStaleEntries(db, STORE_AGENT_STATE, 5000, 10000);
    expect(removed).toBe(1);
    expect(await cs4Get(db, STORE_AGENT_STATE, 'a1')).toBeUndefined();
    expect(await cs4Get(db, STORE_AGENT_STATE, 'a2')).toBeDefined();
    db.close();
  });
});
