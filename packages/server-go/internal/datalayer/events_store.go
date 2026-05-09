// Package datalayer — events_store.go: DL-2 cold-stream SQLite consumer.
//
// Spec: docs/implementation/modules/dl-2-spec.md §1 DL2.2.
//
// 设计 ① + ② (dl-2-spec.md §0):
//   - hot stream byte-identical 不破 (InProcessEventBus.Publish/Subscribe
//     既有 in-process map + buffered chan, live fanout 路径不动).
//   - cold stream 异步 INSERT 到 channel_events / global_events 表; 失败
//     logging-only 不阻塞 hot stream (hot 永远先返回 success).
//
// SQLite consumer 路由规则:
//   - kind 含 "channel." prefix 或 explicit channelID payload → channel_events
//   - 其他 (perm.* / impersonate.* / agent.state / admin.force_*) → global_events
//
// ULID lex_id 跟 RT-1.3 cursor replay 同精神, monotonic 单调递增.

package datalayer

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

// EventStore persists events to SQLite (cold stream). Implements the
// "consumer" side that InProcessEventBus.Publish forks asynchronously.
type EventStore interface {
	// PersistChannel writes one channel-scoped event row.
	PersistChannel(ctx context.Context, channelID, kind string, payload []byte) error
	// PersistGlobal writes one global-scoped event row.
	PersistGlobal(ctx context.Context, kind string, payload []byte) error
}

// sqliteEventStore is the v1 SQLite-backed EventStore.
type sqliteEventStore struct {
	db     *gorm.DB
	logger *slog.Logger
	mu     sync.Mutex // serialize writes (SQLite single-writer)
	now    func() time.Time
}

// NewSQLiteEventStore returns an EventStore wrapping db. Logger is optional
// (nil → no logging). Time injection (now) is optional (nil → time.Now).
func NewSQLiteEventStore(db *gorm.DB, logger *slog.Logger) EventStore {
	return &sqliteEventStore{db: db, logger: logger, now: time.Now}
}

// PersistChannel inserts into channel_events. retention_days NULL = sweeper
// default per kind (RetentionDaysForKind).
func (s *sqliteEventStore) PersistChannel(ctx context.Context, channelID, kind string, payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	lexID := newULID(s.now())
	createdAt := s.now().UnixMilli()
	err := s.db.WithContext(ctx).Exec(
		`INSERT INTO channel_events (lex_id, channel_id, kind, payload, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		lexID, channelID, kind, string(payload), createdAt,
	).Error
	if err != nil && s.logger != nil {
		s.logger.Error("dl2.persist_channel_failed", "kind", kind, "error", err)
	}
	return err
}

// PersistGlobal inserts into global_events.
func (s *sqliteEventStore) PersistGlobal(ctx context.Context, kind string, payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	lexID := newULID(s.now())
	createdAt := s.now().UnixMilli()
	err := s.db.WithContext(ctx).Exec(
		`INSERT INTO global_events (lex_id, kind, payload, created_at)
		 VALUES (?, ?, ?, ?)`,
		lexID, kind, string(payload), createdAt,
	).Error
	if err != nil && s.logger != nil {
		s.logger.Error("dl2.persist_global_failed", "kind", kind, "error", err)
	}
	return err
}

// IsChannelScopedKind reports whether a kind belongs to channel_events.
// Channel-scoped: prefix "channel." or "message." (per-channel events).
// All else (perm, impersonate, agent.state, admin.force_*) → global_events.
func IsChannelScopedKind(kind string) bool {
	return strings.HasPrefix(kind, "channel.") || strings.HasPrefix(kind, "message.")
}

// newULID returns a 26-char monotonic-ish ULID. v1 simple impl:
// 10 hex chars from millis + 16 hex chars from crypto/rand.
// Lexicographically sortable by time.
func newULID(t time.Time) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%013x%016x", t.UnixMilli(), b)
}
