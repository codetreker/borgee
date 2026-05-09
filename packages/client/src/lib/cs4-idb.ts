// CS-4 — IndexedDB wrapper SSOT (蓝图 client-shape.md §1.4 乐观缓存 B 路径).
//
// 设计 ① (cs-4-stance-checklist):
//   - 3 store 拆死: messages / last_read_at / agent_state
//   - typing/presence-realtime 必从 server 实时拉, 不入 IDB (蓝图 §1.4 字面)
//   - artifact 内容 / DM body / 草稿 走 CV-10 localStorage 既有, 不漂
//
// DB version=1; schema 改 = bump version + onupgradeneeded migration
// (跟 server schema_migrations 同精神).

const DB_NAME = 'borgee-cs4';
const DB_VERSION = 1;

export const STORE_MESSAGES = 'messages';
export const STORE_LAST_READ_AT = 'last_read_at';
export const STORE_AGENT_STATE = 'agent_state';

/** Open the CS-4 IndexedDB instance. Idempotent (browser dedup by name). */
export function openCS4DB(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    if (typeof indexedDB === 'undefined') {
      reject(new Error('IndexedDB not supported'));
      return;
    }
    const req = indexedDB.open(DB_NAME, DB_VERSION);
    req.onerror = () => reject(req.error);
    req.onsuccess = () => resolve(req.result);
    req.onupgradeneeded = (event) => {
      const db = (event.target as IDBOpenDBRequest).result;
      if (!db.objectStoreNames.contains(STORE_MESSAGES)) {
        const s = db.createObjectStore(STORE_MESSAGES, { keyPath: 'id' });
        s.createIndex('channel_id', 'channel_id', { unique: false });
      }
      if (!db.objectStoreNames.contains(STORE_LAST_READ_AT)) {
        db.createObjectStore(STORE_LAST_READ_AT, { keyPath: 'channel_id' });
      }
      if (!db.objectStoreNames.contains(STORE_AGENT_STATE)) {
        db.createObjectStore(STORE_AGENT_STATE, { keyPath: 'agent_id' });
      }
    };
  });
}

/** Typed wrapper — put a value into a store. */
export function cs4Put(db: IDBDatabase, store: string, value: unknown): Promise<void> {
  return new Promise((resolve, reject) => {
    const tx = db.transaction(store, 'readwrite');
    const req = tx.objectStore(store).put(value);
    req.onsuccess = () => resolve();
    req.onerror = () => reject(req.error);
  });
}

/** Typed wrapper — get a value by key. */
export function cs4Get<T = unknown>(db: IDBDatabase, store: string, key: IDBValidKey): Promise<T | undefined> {
  return new Promise((resolve, reject) => {
    const tx = db.transaction(store, 'readonly');
    const req = tx.objectStore(store).get(key);
    req.onsuccess = () => resolve(req.result as T | undefined);
    req.onerror = () => reject(req.error);
  });
}

/** Typed wrapper — delete a value by key. */
export function cs4Delete(db: IDBDatabase, store: string, key: IDBValidKey): Promise<void> {
  return new Promise((resolve, reject) => {
    const tx = db.transaction(store, 'readwrite');
    const req = tx.objectStore(store).delete(key);
    req.onsuccess = () => resolve();
    req.onerror = () => reject(req.error);
  });
}

/**
 * clearStaleEntries — remove store entries older than maxAgeMs. v1 best-effort
 * cleanup; not a goroutine / scheduled job (留 v1 用户 logout 时调).
 */
export async function clearStaleEntries(
  db: IDBDatabase,
  store: string,
  maxAgeMs: number,
  now: number = Date.now(),
): Promise<number> {
  return new Promise((resolve, reject) => {
    const tx = db.transaction(store, 'readwrite');
    const objStore = tx.objectStore(store);
    const req = objStore.openCursor();
    let removed = 0;
    req.onerror = () => reject(req.error);
    req.onsuccess = (event) => {
      const cursor = (event.target as IDBRequest<IDBCursorWithValue | null>).result;
      if (cursor) {
        const v = cursor.value as { updated_at_ms?: number };
        if (typeof v.updated_at_ms === 'number' && now - v.updated_at_ms > maxAgeMs) {
          cursor.delete();
          removed++;
        }
        cursor.continue();
      } else {
        resolve(removed);
      }
    };
  });
}
