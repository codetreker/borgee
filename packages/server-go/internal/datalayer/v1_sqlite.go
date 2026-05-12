// DL-1 — concrete v1 implementations wrapping existing store.Store.
//
// Principle ② (DL-1 spec §0): factory pattern + dependency-injection wiring as
// the central source, matching the BPP-3 PluginFrameDispatcher / reasons.IsValid canonical
// pattern.
//
// v1 wrapper preserves behavior: handlers use the Repository interface, and
// the implementation forwards to existing store.Store methods. Errors pass
// through, except gorm.ErrRecordNotFound maps to the single
// ErrRepositoryNotFound sentinel.

package datalayer

import (
	"context"
	"errors"
	"fmt"

	"borgee-server/internal/presence"
	"borgee-server/internal/store"

	"gorm.io/gorm"
)

// ----- UserRepository v1 (sqlite wrap) -----

type sqliteUserRepo struct{ s *store.Store }

func NewSQLiteUserRepository(s *store.Store) UserRepository { return &sqliteUserRepo{s: s} }

func (r *sqliteUserRepo) GetByID(_ context.Context, id string) (*store.User, error) {
	u, err := r.s.GetUserByID(id)
	return u, mapGormErr(err)
}
func (r *sqliteUserRepo) GetByEmail(_ context.Context, email string) (*store.User, error) {
	u, err := r.s.GetUserByEmail(email)
	return u, mapGormErr(err)
}
func (r *sqliteUserRepo) GetByAPIKey(_ context.Context, apiKey string) (*store.User, error) {
	u, err := r.s.GetUserByAPIKey(apiKey)
	return u, mapGormErr(err)
}
func (r *sqliteUserRepo) GetByDisplayName(_ context.Context, displayName string) (*store.User, error) {
	u, err := r.s.GetUserByDisplayName(displayName)
	return u, mapGormErr(err)
}
func (r *sqliteUserRepo) Create(_ context.Context, user *store.User) error {
	return r.s.CreateUser(user)
}

// ----- ChannelRepository v1 -----

type sqliteChannelRepo struct{ s *store.Store }

func NewSQLiteChannelRepository(s *store.Store) ChannelRepository { return &sqliteChannelRepo{s: s} }

func (r *sqliteChannelRepo) GetByID(_ context.Context, id string) (*store.Channel, error) {
	c, err := r.s.GetChannelByID(id)
	return c, mapGormErr(err)
}
func (r *sqliteChannelRepo) GetByName(_ context.Context, name string) (*store.Channel, error) {
	c, err := r.s.GetChannelByName(name)
	return c, mapGormErr(err)
}
func (r *sqliteChannelRepo) GetByNameInOrg(_ context.Context, orgID, name string) (*store.Channel, error) {
	c, err := r.s.GetChannelByNameInOrg(orgID, name)
	return c, mapGormErr(err)
}
func (r *sqliteChannelRepo) Create(_ context.Context, ch *store.Channel) error {
	return r.s.CreateChannel(ch)
}

// ----- MessageRepository v1 -----

type sqliteMessageRepo struct{ s *store.Store }

func NewSQLiteMessageRepository(s *store.Store) MessageRepository {
	return &sqliteMessageRepo{s: s}
}

func (r *sqliteMessageRepo) GetByID(_ context.Context, id string) (*store.Message, error) {
	m, err := r.s.GetMessageByID(id)
	return m, mapGormErr(err)
}
func (r *sqliteMessageRepo) Create(_ context.Context, msg *store.Message) error {
	return r.s.CreateMessage(msg)
}

// ----- PresenceStore v1 (wrap presence.PresenceTracker) -----

type inMemoryPresence struct{ pt presence.PresenceTracker }

// NewInMemoryPresence wraps an existing presence.PresenceTracker (e.g.
// presence.NewSessionsTracker). Returns ErrRepositoryNotFound never
// (presence.PresenceTracker can't fail; method signature accepts ctx for
// future-proofing v3+ Redis path).
func NewInMemoryPresence(pt presence.PresenceTracker) PresenceStore {
	return &inMemoryPresence{pt: pt}
}

func (p *inMemoryPresence) IsOnline(_ context.Context, userID string) (bool, error) {
	return p.pt.IsOnline(userID), nil
}
func (p *inMemoryPresence) Sessions(_ context.Context, userID string) ([]string, error) {
	return p.pt.Sessions(userID), nil
}

// ----- Storage v1 (DB-backed placeholder) -----
//
// v1: artifacts go thru store.Store directly via gorm; this Storage interface
// is wired but its concrete impl is an opaque-key placeholder pending
// follow-up DL-1.5 (when artifact body extraction is needed).

type localDBStorage struct{ s *store.Store }

func NewLocalDBStorage(s *store.Store) Storage { return &localDBStorage{s: s} }

func (l *localDBStorage) GetURL(_ context.Context, key string) (string, error) {
	if key == "" {
		return "", ErrStorageKeyNotFound
	}
	// v1 placeholder: artifact body access remains in Repository until the DL-1.5
	// follow-up. Current callers do not consume Storage.GetURL directly; this
	// locks the interface shape, not a storage implementation.
	return fmt.Sprintf("db://artifact/%s", key), nil
}
func (l *localDBStorage) PutBlob(_ context.Context, key string, _ []byte) error {
	if key == "" {
		return ErrStorageKeyNotFound
	}
	// v1 placeholder: artifact body writes still use store.Store.UpdateArtifact*.
	// Current callers do not consume PutBlob directly; this reserves the DL-1.5
	// wire path.
	return nil
}
func (l *localDBStorage) Delete(_ context.Context, key string) error {
	if key == "" {
		return ErrStorageKeyNotFound
	}
	// v1: forward-only audit; do not physically delete the DB row. This matches
	// the ADM-3 audit-forward-only pattern.
	return nil
}

// ----- EventBus v1 (in-process map + buffered chan) -----

type inProcessEventBus struct {
	subs  map[string][]chan Event
	store EventStore // optional cold-stream consumer (DL-2)
}

// WIRE-1 #1: removed NewInProcessEventBus (hot-only constructor) —
// production factory.go always wires NewInProcessEventBusWithStore (DL-2
// cold consumer wired to channel_events / global_events tables). The hot-only
// path was a v0 spec stub and has no callsite after WIRE-1.

// NewInProcessEventBusWithStore wires a cold-stream EventStore consumer.
// Publish forks an async INSERT to channel_events / global_events; failures
// are logging-only and do NOT block the hot stream (blueprint §4 policy).
//
// DL-2 spec §0 principle ② — hot stream behavior is unchanged, cold stream is additive.
func NewInProcessEventBusWithStore(store EventStore) EventBus {
	return &inProcessEventBus{
		subs:  make(map[string][]chan Event),
		store: store,
	}
}

func (b *inProcessEventBus) Publish(_ context.Context, topic string, payload []byte) error {
	// hot stream: live fanout to subscribers (unchanged from pre-DL-2 behavior).
	for _, ch := range b.subs[topic] {
		select {
		case ch <- Event{Topic: topic, Payload: payload}:
		default:
			// best-effort: drop when the subscriber buffer is full. This follows
			// the BPP-4 dead_letter policy, with RT-1.3 cursor replay as fallback.
		}
	}
	// cold stream: async persist (DL-2). Failures logging-only, no-op when
	// store is nil (DL-1 backward compat).
	if b.store != nil {
		kind, chID := splitTopicKind(topic)
		go func() {
			persistCtx := context.Background()
			if IsChannelScopedKind(kind) && chID != "" {
				_ = b.store.PersistChannel(persistCtx, chID, kind, payload)
			} else {
				_ = b.store.PersistGlobal(persistCtx, kind, payload)
			}
		}()
	}
	return nil
}
func (b *inProcessEventBus) Subscribe(ctx context.Context, topic string) (<-chan Event, error) {
	ch := make(chan Event, 16)
	b.subs[topic] = append(b.subs[topic], ch)
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
}

// splitTopicKind parses a topic in form "<kind>:<channelID>" — channelID
// after first ':' if present. Returns (kind, channelID).
func splitTopicKind(topic string) (kind, channelID string) {
	for i := 0; i < len(topic); i++ {
		if topic[i] == ':' {
			return topic[:i], topic[i+1:]
		}
	}
	return topic, ""
}

// ----- helpers -----

func mapGormErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrRepositoryNotFound
	}
	return err
}
