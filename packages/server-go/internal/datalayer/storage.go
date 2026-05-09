// DL-1 — Storage interface (蓝图 §4 B 第 1 条).
//
// 设计 ① (cs-spec §0): GetURL / PutBlob / Delete 三方法 byte-identical 跟蓝图.
// v1 实现 LocalDBStorage 走既有 store.Store artifacts blob 路径 (artifacts
// 当前实际入 DB, 不在 fs; 蓝图 "local fs" 字面是 v0 假设, v1 实际 DB blob).
//
// 切换路径 (留 v3+, DL-3 阈值哨触发):
//   - LocalDBStorage  → DB blob (v1 现状)
//   - S3Storage / R2Storage → 真对象存储 (v3+)
//   - LocalFSStorage   → 本地 fs (留作 self-host 单机部署)
package datalayer

import (
	"context"
	"errors"
)

// Storage is the SSOT interface for blob (artifact body) storage.
// v1 read/write 都走 DB blob 列; v3+ 切对象存储仅改 NewStorage factory.
type Storage interface {
	// GetURL returns a (possibly signed, time-bounded) URL or path identifier
	// for the blob keyed by `key`. v1 LocalDBStorage 返 "db://artifact/<id>"
	// 占位 string (caller 不直消费, 走 Repository.GetArtifact 取 body 单源).
	GetURL(ctx context.Context, key string) (string, error)

	// PutBlob writes the blob payload under `key`. v1 走 store.Store.UpdateArtifactBody.
	PutBlob(ctx context.Context, key string, data []byte) error

	// Delete removes the blob. v1 走 store soft-delete (forward-only audit
// 设计: 不 真删 DB row, 跟 ADM-3 audit-forward-only 同精神).
	Delete(ctx context.Context, key string) error
}

// ErrStorageKeyNotFound is returned by Storage methods when the key has
// no associated blob.
var ErrStorageKeyNotFound = errors.New("datalayer: storage key not found")
