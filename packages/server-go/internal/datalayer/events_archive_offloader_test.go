// Package datalayer — events_archive_offloader_test.go: DL-3 §1 DL3.2 unit tests.
//
// Spec: docs/implementation/modules/dl-3-spec.md §1 DL3.2.
//
// Pins:
//   - threshold no-op when row count < threshold
//   - archive file creation + INSERT writes + source DELETE row count
//   - audit "events.archive_offload" through EventBus.Publish
//   - ctx-aware execution through m.db.WithContext to avoid leaks

package datalayer

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// captureBus records Publish calls for audit verification.
type captureBus struct {
	mu       sync.Mutex
	captured []capturedEvent
}

type capturedEvent struct {
	topic   string
	payload []byte
}

func (b *captureBus) Publish(_ context.Context, topic string, payload []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.captured = append(b.captured, capturedEvent{topic: topic, payload: append([]byte(nil), payload...)})
	return nil
}

func (b *captureBus) Subscribe(_ context.Context, _ string) (<-chan Event, error) {
	return make(chan Event), nil
}

// TestEventsArchiveOffloader_BelowThreshold_NoOp pins below-threshold no-op.
func TestEventsArchiveOffloader_BelowThreshold_NoOp(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	bus := &captureBus{}
	dir := t.TempDir()
	o := NewEventsArchiveOffloader(db, bus, nil, dir, 1000, 30*24*time.Hour, 0)

	res, err := o.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if res.Triggered {
		t.Error("Triggered=true below threshold")
	}
	if len(bus.captured) != 0 {
		t.Errorf("EventBus published %d events, want 0", len(bus.captured))
	}
}

// TestEventsArchiveOffloader_OffloadsExpired writes rows older than cutoff,
// triggers offload, verifies archive file + source DELETE + audit publish.
func TestEventsArchiveOffloader_OffloadsExpired(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	bus := &captureBus{}
	dir := t.TempDir()

	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	const day = int64(86400000)
	nowMs := now.UnixMilli()

	must := func(sql string, args ...any) {
		if err := db.Exec(sql, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	// Seed 5 rows: 3 expired (>30d) + 2 fresh.
	for i, ageDays := range []int64{45, 60, 100, 5, 10} {
		must(`INSERT INTO channel_events (lex_id, channel_id, kind, payload, created_at, retention_days) VALUES (?, ?, ?, ?, ?, ?)`,
			"l-"+string(rune('a'+i)), "ch-1", "channel.archived", "{}", nowMs-ageDays*day, 30)
	}

	o := NewEventsArchiveOffloader(db, bus, nil, dir, 3, 30*24*time.Hour, 0)
	o.now = func() time.Time { return now }

	res, err := o.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !res.Triggered {
		t.Fatal("Triggered=false above threshold")
	}
	if res.RowsArchived != 3 {
		t.Errorf("RowsArchived = %d, want 3", res.RowsArchived)
	}
	if res.RowsDeleted != 3 {
		t.Errorf("RowsDeleted = %d, want 3", res.RowsDeleted)
	}

	// Archive file exists at expected path.
	wantPath := filepath.Join(dir, "events_archive_2026-05.db")
	if res.ArchivePath != wantPath {
		t.Errorf("ArchivePath = %q, want %q", res.ArchivePath, wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Errorf("archive file missing: %v", err)
	}

	// Source: 2 fresh rows remain.
	var n int64
	db.Raw(`SELECT COUNT(*) FROM channel_events`).Row().Scan(&n)
	if n != 2 {
		t.Errorf("source row count = %d, want 2", n)
	}

	// Archive: 3 rows present (open archive db separately).
	adb, err := gorm.Open(sqlite.Open(wantPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	var arN int64
	adb.Raw(`SELECT COUNT(*) FROM channel_events`).Row().Scan(&arN)
	if arN != 3 {
		t.Errorf("archive row count = %d, want 3", arN)
	}

	// Audit emitted via EventBus.
	if len(bus.captured) != 1 {
		t.Fatalf("EventBus events = %d, want 1", len(bus.captured))
	}
	if bus.captured[0].topic != "events.archive_offload" {
		t.Errorf("topic = %q, want events.archive_offload", bus.captured[0].topic)
	}
}

// TestEventsArchiveOffloader_NoBus_OK verifies offload works without EventBus.
func TestEventsArchiveOffloader_NoBus_OK(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	dir := t.TempDir()

	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	const day = int64(86400000)
	nowMs := now.UnixMilli()
	for i := 0; i < 4; i++ {
		if err := db.Exec(`INSERT INTO channel_events (lex_id, channel_id, kind, payload, created_at, retention_days) VALUES (?, ?, ?, ?, ?, ?)`,
			"x"+string(rune('0'+i)), "ch-1", "channel.archived", "{}", nowMs-100*day, 30,
		).Error; err != nil {
			t.Fatal(err)
		}
	}
	o := NewEventsArchiveOffloader(db, nil, nil, dir, 2, 30*24*time.Hour, 0)
	o.now = func() time.Time { return now }
	res, err := o.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Triggered || res.RowsArchived != 4 {
		t.Errorf("Triggered=%v RowsArchived=%d", res.Triggered, res.RowsArchived)
	}
}

// TestEventsArchiveOffloader_DefaultsApplied pins constructor defaults.
func TestEventsArchiveOffloader_DefaultsApplied(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	o := NewEventsArchiveOffloader(db, nil, nil, "", 0, 0, 0)
	if o.threshold != 1_000_000 {
		t.Errorf("threshold default = %d, want 1_000_000", o.threshold)
	}
	if o.cutoffAge != 30*24*time.Hour {
		t.Errorf("cutoffAge default = %v, want 30d", o.cutoffAge)
	}
	if o.archiveDir != "./data" {
		t.Errorf("archiveDir default = %q", o.archiveDir)
	}
}
