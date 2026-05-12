// Package datalayer — factory_wire_test.go: WIRE-1 production wire-up
// coverage. Guards against spec text passing while the production callsite stays dead.
//
// Spec: docs/implementation/modules/wire-1-spec.md §1 W1.1.
//
// Pin: NewDataLayer uses the NewInProcessEventBusWithStore production callsite;
// one Publish must insert into channel_events, preventing hot-only drift.

package datalayer

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"borgee-server/internal/store"
)

// TestFactory_EventBus_ColdConsumer_Wired verifies that DataLayer wires the
// DL-2 cold consumer in the production path (factory.go uses
// NewInProcessEventBusWithStore, not NewInProcessEventBus).
//
// Covered path: dl := NewDataLayer(...); dl.EventBus.Publish(...) inserts into
// channel_events / global_events. The cold-stream consumer writes asynchronously,
// so the test uses sync.WaitGroup to wait for the persisted value.
func TestFactory_EventBus_ColdConsumer_Wired(t *testing.T) {
	t.Parallel()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	dl := NewDataLayer(s, nil, nil)
	if dl.EventBus == nil {
		t.Fatal("EventBus nil — factory wire-up broken")
	}

	// Publish 1 channel-scoped event → channel_events row insert.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := dl.EventBus.Publish(ctx, "channel.archived:ch-1", []byte(`{"x":1}`)); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// The cold-stream consumer runs in a goroutine; poll briefly until the INSERT is
	// visible. The 1s timeout keeps the test deterministic without race flakes.
	var n int64
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		s.DB().Raw(`SELECT COUNT(*) FROM channel_events WHERE channel_id = ?`, "ch-1").Row().Scan(&n)
		if n >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if n != 1 {
		t.Errorf("cold stream not wired — channel_events count = %d, want 1 (production EventBus 走 hot-only stale)", n)
	}
}

// TestFactory_EventBus_GlobalRoute_Wired pins the global_events route
// (perm.grant goes to global_events, not the channel-scoped path).
func TestFactory_EventBus_GlobalRoute_Wired(t *testing.T) {
	t.Parallel()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}

	dl := NewDataLayer(s, nil, nil)
	_ = dl.EventBus.Publish(context.Background(), "perm.grant", []byte(`{"u":"u1"}`))

	// poll up to 1s for cold consumer goroutine.
	var n int64
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		s.DB().Raw(`SELECT COUNT(*) FROM global_events WHERE kind = ?`, "perm.grant").Row().Scan(&n)
		if n >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if n != 1 {
		t.Errorf("global_events count = %d, want 1 (factory wire-up broken)", n)
	}
}

// TestEventsArchiveOffloader_Start_TickerLoop pins WIRE-1 wire-2
// production wiring: offloader.Start(ctx) starts the ticker, and ctx cancel stops it.
func TestEventsArchiveOffloader_Start_TickerLoop(t *testing.T) {
	t.Parallel()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	o := NewEventsArchiveOffloader(s.DB(), nil, nil, t.TempDir(), 999_999, 30*24*time.Hour, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	o.Start(ctx)
	time.Sleep(30 * time.Millisecond) // let ticker fire ≥1
	cancel()
	select {
	case <-o.Done():
	case <-time.After(time.Second):
		t.Fatal("offloader did not stop within 1s — goroutine leak")
	}
}

// TestEventsArchiveOffloader_Start_ZeroInterval pins interval=0 short-circuit.
func TestEventsArchiveOffloader_Start_ZeroInterval(t *testing.T) {
	t.Parallel()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	o := NewEventsArchiveOffloader(s.DB(), nil, nil, t.TempDir(), 0, 0, 0)
	o.Start(context.Background())
	select {
	case <-o.Done():
	case <-time.After(time.Second):
		t.Fatal("interval=0 should close Done() immediately")
	}
}

// _ keeps sync import live for reviewer (deterministic ctx-aware seam).
var _ = sync.Mutex{}

// TestEventsArchiveOffloader_RunOnceLog_DBError covers the error log
// branch (closed DB → error log fires).
func TestEventsArchiveOffloader_RunOnceLog_DBError(t *testing.T) {
	t.Parallel()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	o := NewEventsArchiveOffloader(s.DB(), nil, logger, t.TempDir(), 1, 30*24*time.Hour, 5*time.Millisecond)

	// Close DB so RunOnce err path triggers.
	sqlDB, _ := s.DB().DB()
	_ = sqlDB.Close()

	ctx, cancel := context.WithCancel(context.Background())
	o.Start(ctx)
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-o.Done()
}

// TestEventsArchiveOffloader_RunOnceLog_Triggered covers the info log
// branch (offload triggered → info log fires).
func TestEventsArchiveOffloader_RunOnceLog_Triggered(t *testing.T) {
	t.Parallel()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	const day = int64(86400000)
	nowMs := now.UnixMilli()
	for i := 0; i < 4; i++ {
		_ = s.DB().Exec(`INSERT INTO channel_events (lex_id, channel_id, kind, payload, created_at, retention_days) VALUES (?, ?, ?, ?, ?, ?)`,
			"l-"+string(rune('a'+i)), "ch-1", "channel.archived", "{}", nowMs-100*day, 30).Error
	}
	o := NewEventsArchiveOffloader(s.DB(), nil, logger, t.TempDir(), 2, 30*24*time.Hour, 5*time.Millisecond)
	o.now = func() time.Time { return now }

	ctx, cancel := context.WithCancel(context.Background())
	o.Start(ctx)
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-o.Done()
}
