// DL-1 — DataLayer factory (蓝图 §4 B SSOT seam).
//
// 设计 ② (DL-1 spec §0): factory pattern + DI seam 单源. handler / server.go
// 拿 *DataLayer 不直 import store, 跟 BPP-3 PluginFrameDispatcher /
// reasons.IsValid SSOT 同精神.
//
// v1: NewDataLayer wires SQLite store + in-memory presence + in-process bus
// + DB blob storage byte-identical 不破. v3+ swap underlying impls 仅改本
// factory, handler 0 改 (interface seam 锁).

package datalayer

import (
	"log/slog"

	"borgee-server/internal/presence"
	"borgee-server/internal/store"
)

// DataLayer is the SSOT bundle of the 4 蓝图 §4 B interfaces. Wired once at
// server boot, passed to handlers via DI (替换 server.go 直 store 字段).
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
// WIRE-1 (post-Phase 4 closure 后续): EventBus 真接 DL-2 cold consumer
// 通过 NewInProcessEventBusWithStore — production Publish 真落 channel_events
// / global_events 表 (反 hot-only stale, 跟 spec wire-1 设计 ① 字面). logger
// 可 nil (NewSQLiteEventStore nil-safe).
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
