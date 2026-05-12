// DL-1 — Storage interface (blueprint §4 B item 1).
//
// Principle ① (cs-spec §0): GetURL / PutBlob / Delete stay byte-identical with
// the blueprint. v1 LocalDBStorage uses the existing store.Store artifact blob
// path. Artifacts currently live in DB, not fs; the blueprint's "local fs"
// wording was a v0 assumption, while v1 uses DB blobs.
//
// Implementation swap path (v3+, triggered by DL-3 threshold monitor):
//   - LocalDBStorage  → DB blob (v1 current behavior)
//   - S3Storage / R2Storage → object storage (v3+)
//   - LocalFSStorage   → local fs (reserved for single-machine self-host deployments)
package datalayer

import (
	"context"
	"errors"
)

// Storage is the SSOT interface for blob (artifact body) storage.
// v1 read/write uses DB blob columns; v3+ object storage should only change
// the NewStorage factory.
type Storage interface {
	// GetURL returns a (possibly signed, time-bounded) URL or path identifier
	// for the blob keyed by `key`. v1 LocalDBStorage returns the placeholder
	// string "db://artifact/<id>"; callers do not consume it directly and should
	// read artifact bodies through Repository.GetArtifact.
	GetURL(ctx context.Context, key string) (string, error)

	// PutBlob writes the blob payload under `key`. v1 uses store.Store.UpdateArtifactBody.
	PutBlob(ctx context.Context, key string, data []byte) error

	// Delete removes the blob. v1 uses store soft-delete for forward-only audit:
	// it does not physically delete the DB row, matching ADM-3 audit-forward-only.
	Delete(ctx context.Context, key string) error
}

// ErrStorageKeyNotFound is returned by Storage methods when the key has
// no associated blob.
var ErrStorageKeyNotFound = errors.New("datalayer: storage key not found")
