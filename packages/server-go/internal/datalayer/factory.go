// DL-1 — DataLayer factory (blueprint §4 B wiring point).
//
// Principle ② (DL-1 spec §0): factory pattern + dependency-injection wiring as
// the central source. Handlers / server.go receive *DataLayer instead of importing store
// directly, matching the BPP-3 PluginFrameDispatcher / reasons.IsValid canonical
// pattern.
//
// v1: NewDataLayer wires SQLite store + in-memory presence + in-process bus
// + DB blob storage without changing behavior. v3+ implementation swaps should
// change this factory only, leaving handlers untouched through the interfaces.

package datalayer

import (
	"log/slog"

	"borgee-server/internal/presence"
	"borgee-server/internal/store"
)

// DataLayer is the canonical bundle of blueprint §4 B interfaces plus narrow
// task-specific repositories. It is wired once at server boot and passed to
// handlers through dependency injection instead of direct store fields.
type DataLayer struct {
	Storage              Storage
	Presence             PresenceStore
	EventBus             EventBus
	UserRepo             UserRepository
	ChannelRepo          ChannelRepository
	MessageRepo          MessageRepository
	HelperEnrollmentRepo HelperEnrollmentRepository
	HelperJobRepo        HelperJobRepository
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
		Storage:              NewLocalDBStorage(s),
		Presence:             NewInMemoryPresence(pt),
		EventBus:             NewInProcessEventBusWithStore(eventStore),
		UserRepo:             NewSQLiteUserRepository(s),
		ChannelRepo:          NewSQLiteChannelRepository(s),
		MessageRepo:          NewSQLiteMessageRepository(s),
		HelperEnrollmentRepo: NewSQLiteHelperEnrollmentRepository(s),
		HelperJobRepo:        NewSQLiteHelperJobRepository(s),
	}
}
