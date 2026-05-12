// Package idgen is the ID generator single source of truth, implementing the
// blueprint §4.A.1 ULID lock-in.
//
// Spec: docs/implementation/modules/ulid-migration-spec.md §0 position ① +
// §1 UM.1 + blueprint data-layer.md §4.A.1 literal "ID 方案 = ULID 所有业务表
// 主键, 禁 INTEGER PK / Snowflake / KSUID / UUIDv7".
//
// Single-source position: all new IDs come from NewID. Inline `ulid.Make()` /
// `uuid.NewString()` calls must not spread through the codebase (post-ULID-
// MIGRATION reverse grep guard: 0 hits).
//
// Forward-compatibility: NewID returns a 26-char canonical ULID (Crockford
// base32, lexicographically sortable by time). Existing UUID-36 rows stay as
// they are because db columns are TEXT and do not impose a fixed length; new
// rows use ULID-26. Existing UUID lexicographic order differs from ULID order,
// so callers must not depend on primary-key lexical order. RT-1 cursors use an
// independent lex_id ULID (blueprint §4.A.4), decoupled from the primary key.

package idgen

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	mu      sync.Mutex
	entropy = ulid.Monotonic(rand.Reader, 0)
)

// NewID returns a fresh ULID string (26 chars, monotonic within
// millisecond). It is goroutine-safe via a mutex around the shared entropy
// reader, matching the DL-2 #615 events_store newULID single-writer pattern and
// preventing monotonic-order violations across goroutines.
//
// Negative constraint: do not expose ulid.ULID. Callers consume only string IDs
// (db column TEXT + json string fields). This prevents type ID string abstraction
// drift from the blueprint §v0 code-debt audit row line 219 literal "v1 切回
// 永久混用 + type ID string 抽象". This v1 migration changes only the generator;
// type abstraction remains deferred to v2+.
func NewID() string {
	mu.Lock()
	defer mu.Unlock()
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}
