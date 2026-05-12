// DL-1 — DataLayer factory (blueprint §4 B SSOT seam).
//
// Principle ② (DL-1 spec §0): factory pattern + DI seam as the single wiring
// source. Handlers / server.go receive *DataLayer instead of importing store
// directly, matching the BPP-3 PluginFrameDispatcher / reasons.IsValid SSOT
// pattern.
//
// v1: NewDataLayer wires SQLite store + in-memory presence + in-process bus
// + DB blob storage without changing behavior. v3+ implementation swaps should
// change this factory only, leaving handlers untouched through the interface seam.

package datalayer

import (
	"log/slog"

	"borgee-server/internal/presence"
	"borgee-server/internal/store"
)

// DataLayer is the SSOT bundle of the 4 blueprint §4 B interfaces. It is wired
// once at server boot and passed to handlers via DI instead of direct store fields.
type DataLayer struct {
	Storage     Storage
	Presence    PresenceStore
	EventBus    EventBus
	UserRepo    UserRepository
	ChannelRepo ChannelRepository
	MessageRepo MessageRepository
}

// NewDataLayer assembles the v1 (SQLite + in-memory) bundle. Caller owns
// store.Store + presence.PresenceTracker lifecycles (close on shutdown).
//
// WIRE-1 (post-Phase 4 closure follow-up): EventBus is wired to the DL-2 cold
// consumer through NewInProcessEventBusWithStore. Production Publish writes to
// channel_events / global_events, preventing the stale hot-only path described
// by spec wire-1 principle ①. logger may be nil (NewSQLiteEventStore is nil-safe).
func NewDataLayer(s *store.Store, pt presence.PresenceTracker, logger *slog.Logger) *DataLayer {
	eventStore := NewSQLiteEventStore(s.DB(), logger)
	return &DataLayer{
		Storage:     NewLocalDBStorage(s),
		Presence:    NewInMemoryPresence(pt),
		EventBus:    NewInProcessEventBusWithStore(eventStore),
		UserRepo:    NewSQLiteUserRepository(s),
		ChannelRepo: NewSQLiteChannelRepository(s),
		MessageRepo: NewSQLiteMessageRepository(s),
	}
}
