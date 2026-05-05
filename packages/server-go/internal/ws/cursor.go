// Package ws — cursor.go: RT-1.1 (#269) source-of-truth for the
// monotonic event cursor + per-(artifact_id, version) dedup gate that
// fronts the `artifact_updated` push frame defined in
// docs/blueprint/current/realtime.md §1.4 + §2.1.
//
// Behaviour contract (飞马 review §0 hardline + RT-1 spec §1.1):
//
//   1. Cursor is strictly monotonically increasing within a single
//      origin server. Two concurrent commits never see the same value.
//   2. Cursor never rolls back, even across server restart. The store's
//      `events` table (cursor INTEGER PRIMARY KEY AUTOINCREMENT) is the
//      durable backing; we seed the in-memory atomic from
//      `Store.GetLatestCursor()` at construction.
//   3. Same (artifact_id, version) tuple resolves to the SAME cursor on
//      a re-emit. The hub does not double-send and the frame's `cursor`
//      stays stable so client dedup (RT-1.2) is fail-closed.
//   4. The `ArtifactUpdated` envelope field order is byte-identical with
//      #237 invitation envelope (cursor first, semantic IDs next, then
//      timestamp/kind tail). See AgentInvitationPendingFrame for the
//      template; the reverse-grep anchor in the RT-1 spec §3 enforces
//      that we never sort frames by `timestamp`.
//
// Phase 4 BPP cutover: this file's symbols are wire-layer-agnostic and
// `bpp/frame_schemas.go` will type-alias `ArtifactUpdatedFrame` so the
// schema is owned in exactly one place.

package ws

import (
	"sync"
	"sync/atomic"

	"borgee-server/internal/store"
)

// FrameTypeArtifactUpdated is the `type` discriminator emitted on the
// `/ws` envelope's outer frame; the matching client switch lives in
// packages/client/src/realtime/wsClient.ts (RT-1.2).
const FrameTypeArtifactUpdated = "artifact_updated"

// ArtifactUpdatedFrame — server → client push that fires after an
// artifact commit lands in the store. Field order is the RT-1.1 review
// hardline; do NOT reorder without updating
// packages/client/src/types/ws-frames.ts in the same PR.
//
// `Cursor` is the monotonic sequence number; clients persist it to
// localStorage as `last_seen_cursor` and pass it back on reconnect via
// `?since=N` so the server can backfill any gap.
type ArtifactUpdatedFrame struct {
	Type       string `json:"type"`
	Cursor     int64  `json:"cursor"`
	ArtifactID string `json:"artifact_id"`
	Version    int64  `json:"version"`
	ChannelID  string `json:"channel_id"`
	UpdatedAt  int64  `json:"updated_at"` // Unix ms; semantic only — clients MUST NOT sort by this field
	Kind       string `json:"kind"`
}

// CursorAllocator hands out monotonic cursors and dedups re-emits of
// the same (artifact_id, version) tuple. Constructed once per Hub.
//
// Concurrency: NextCursor + AllocateForArtifact are safe under heavy
// parallel commit (race detector + 100-goroutine test pinned in
// cursor_test.go). The atomic head guarantees uniqueness even when the
// dedup map's mutex is contended.
type CursorAllocator struct {
	store *store.Store

	// head is the highest cursor already handed out. Seeded from
	// Store.GetLatestCursor() at NewCursorAllocator and bumped via
	// CompareAndSwap so two callers cannot collide.
	head atomic.Int64

	// dedup keys (artifact_id|version) → assigned cursor. A re-emit of
	// the same tuple resolves to the same cursor and is reported via
	// the bool return so callers can suppress double-broadcast.
	mu    sync.Mutex
	dedup map[string]int64
}

// NewCursorAllocator builds the allocator and primes the in-memory head
// from the durable `events.cursor` MAX so a server restart never rolls
// the sequence back.
func NewCursorAllocator(s *store.Store) *CursorAllocator {
	a := &CursorAllocator{
		store: s,
		dedup: make(map[string]int64),
	}
	if s != nil {
		a.head.Store(s.GetLatestCursor())
	}
	return a
}

// NextCursor reserves and returns the next monotonic cursor without
// touching the dedup map. Callers that don't need (artifact_id,
// version) idempotency (e.g. one-off control frames) use this directly.
func (a *CursorAllocator) NextCursor() int64 {
	for {
		cur := a.head.Load()
		next := cur + 1
		if a.head.CompareAndSwap(cur, next) {
			return next
		}
	}
}

// PeekCursor returns the highest cursor handed out so far without
// advancing it. Used by tests + by the long-poll backfill path so it
// can report the server's current high-water mark to reconnecting
// clients.
func (a *CursorAllocator) PeekCursor() int64 {
	return a.head.Load()
}

// AllocateForArtifact resolves the cursor for a (artifact_id, version)
// tuple. Returns the cursor + a `fresh` bool: true on first sight of
// the tuple (caller SHOULD broadcast); false on a re-emit (caller MUST
// suppress the broadcast — the client already has the original frame
// indexed by this same cursor and a duplicate would break RT-1.2's
// already-rendered set dedup).
func (a *CursorAllocator) AllocateForArtifact(artifactID string, version int64) (cursor int64, fresh bool) {
	key := dedupKey(artifactID, version)

	a.mu.Lock()
	if existing, ok := a.dedup[key]; ok {
		a.mu.Unlock()
		return existing, false
	}
	// Reserve cursor while holding the dedup mutex so two concurrent
	// callers with the same (artifact_id, version) cannot both see a
	// miss and both allocate; the second one will block on the mutex,
	// then find the entry on retry.
	next := a.NextCursor()
	a.dedup[key] = next
	a.mu.Unlock()
	return next, true
}

// dedupKey is split out so tests can pin the byte form. Format is
// `<artifact_id>|<version>` — '|' is not legal in our UUID artifact IDs
// so the separator is collision-free.
func dedupKey(artifactID string, version int64) string {
	return artifactID + "|" + itoa64(version)
}

// itoa64 — strconv-free formatter to keep the hot path allocation-light
// on heavy commit bursts. Only handles non-negative ints (versions are
// monotonic positive per CV-1.1).
func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		// fallback: negative versions are illegal but don't panic.
		return "-" + itoa64(-n)
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
