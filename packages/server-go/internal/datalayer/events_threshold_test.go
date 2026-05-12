// Package datalayer — events_threshold_test.go: DL-3 §1 DL3.1 ThresholdMonitor
// unit tests.
//
// Spec: docs/implementation/modules/dl-3-spec.md §1 DL3.1.
//
// 立场承袭:
//   - 4 metric × OK/WARN/CRITICAL classification matches blueprint §5 exactly
//   - ctx-aware Start(ctx) deterministic shutdown (sync.WaitGroup + Done() chan)
//   - SQLite collector roundtrip (db_size_mb / wal_pending_pages / row_count)

package datalayer

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

// TestDefaultThresholds_ByteIdentical pins the exact 4 metric thresholds from
// blueprint §5 / spec §0 principle ② (db_size 5000/10000 / wal 1000/5000 / lock 100/1000
// / rows 1M/10M).
func TestDefaultThresholds_ByteIdentical(t *testing.T) {
	t.Parallel()
	got := DefaultThresholds()
	want := []DBThreshold{
		{Name: "db_size_mb", Warn: 5000, Critical: 10000},
		{Name: "wal_pending_pages", Warn: 1000, Critical: 5000},
		{Name: "write_lock_wait_ms", Warn: 100, Critical: 1000},
		{Name: "events_row_count", Warn: 1_000_000, Critical: 10_000_000},
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("DefaultThresholds[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestDBThreshold_Classify covers OK / WARN / CRITICAL boundary semantics.
func TestDBThreshold_Classify(t *testing.T) {
	t.Parallel()
	thr := DBThreshold{Name: "x", Warn: 100, Critical: 1000}
	cases := []struct {
		v    int64
		want ThresholdLevel
	}{
		{0, ThresholdLevelOK},
		{99, ThresholdLevelOK},
		{100, ThresholdLevelWarn},
		{500, ThresholdLevelWarn},
		{999, ThresholdLevelWarn},
		{1000, ThresholdLevelCritical},
		{99999, ThresholdLevelCritical},
	}
	for _, c := range cases {
		if got := thr.Classify(c.v); got != c.want {
			t.Errorf("Classify(%d) = %v, want %v", c.v, got, c.want)
		}
	}
}

// TestThresholdLevel_String verifies slog-friendly lowercase names.
func TestThresholdLevel_String(t *testing.T) {
	t.Parallel()
	if ThresholdLevelOK.String() != "ok" {
		t.Errorf("OK = %q, want ok", ThresholdLevelOK.String())
	}
	if ThresholdLevelWarn.String() != "warn" {
		t.Errorf("Warn = %q", ThresholdLevelWarn.String())
	}
	if ThresholdLevelCritical.String() != "critical" {
		t.Errorf("Critical = %q", ThresholdLevelCritical.String())
	}
}

// stubCollector returns a fixed value or error.
type stubCollector struct {
	val int64
	err error
}

func (s *stubCollector) Collect(_ context.Context) (int64, error) { return s.val, s.err }

// TestThresholdMonitor_RunOnce_AllLevels exercises each metric at OK/Warn/Critical.
func TestThresholdMonitor_RunOnce_AllLevels(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := NewThresholdMonitor(db, logger, 0)

	// Inject stub collectors covering OK / Warn / Critical / Warn for the 4 metrics.
	m.SetCollector("db_size_mb", &stubCollector{val: 0})              // OK
	m.SetCollector("wal_pending_pages", &stubCollector{val: 1500})    // Warn (>=1000)
	m.SetCollector("write_lock_wait_ms", &stubCollector{val: 5000})   // Critical (>=1000)
	m.SetCollector("events_row_count", &stubCollector{val: 200_000})  // OK

	out := m.RunOnce(context.Background())
	if out["db_size_mb"].Level != ThresholdLevelOK {
		t.Errorf("db_size_mb level = %v, want OK", out["db_size_mb"].Level)
	}
	if out["wal_pending_pages"].Level != ThresholdLevelWarn {
		t.Errorf("wal level = %v, want Warn", out["wal_pending_pages"].Level)
	}
	if out["write_lock_wait_ms"].Level != ThresholdLevelCritical {
		t.Errorf("lock level = %v, want Critical", out["write_lock_wait_ms"].Level)
	}
	if out["events_row_count"].Level != ThresholdLevelOK {
		t.Errorf("rows level = %v, want OK", out["events_row_count"].Level)
	}
	if out["events_row_count"].Value != 200_000 {
		t.Errorf("rows value = %d", out["events_row_count"].Value)
	}
}

// TestThresholdMonitor_RunOnce_CollectError continues past collector errors.
func TestThresholdMonitor_RunOnce_CollectError(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := NewThresholdMonitor(db, logger, 0)

	m.SetCollector("db_size_mb", &stubCollector{err: errors.New("boom")})
	m.SetCollector("wal_pending_pages", &stubCollector{val: 5000})    // Critical
	m.SetCollector("write_lock_wait_ms", &stubCollector{val: 0})      // OK
	m.SetCollector("events_row_count", &stubCollector{val: 0})        // OK

	out := m.RunOnce(context.Background())
	if _, ok := out["db_size_mb"]; ok {
		t.Error("db_size_mb should be skipped on Collect err")
	}
	if out["wal_pending_pages"].Level != ThresholdLevelCritical {
		t.Errorf("wal level = %v", out["wal_pending_pages"].Level)
	}
}

// TestThresholdMonitor_StartStop covers ctx-aware lifecycle (反 goroutine leak).
func TestThresholdMonitor_StartStop(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	m := NewThresholdMonitor(db, nil, 10*time.Millisecond)
	// Replace SQLite collectors with stubs to avoid PRAGMA flakiness.
	m.SetCollector("db_size_mb", &stubCollector{val: 0})
	m.SetCollector("wal_pending_pages", &stubCollector{val: 0})
	m.SetCollector("write_lock_wait_ms", &stubCollector{val: 0})
	m.SetCollector("events_row_count", &stubCollector{val: 0})

	ctx, cancel := context.WithCancel(context.Background())
	m.Start(ctx)
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case <-m.Done():
	case <-time.After(time.Second):
		t.Fatal("monitor did not shutdown within 1s — goroutine leak")
	}
}

// TestThresholdMonitor_StartZeroInterval — interval=0 short-circuits.
func TestThresholdMonitor_StartZeroInterval(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	m := NewThresholdMonitor(db, nil, 0)
	m.Start(context.Background())
	select {
	case <-m.Done():
	case <-time.After(time.Second):
		t.Fatal("interval=0 should close Done() immediately")
	}
}

// TestSQLiteRowCountCollector_Roundtrip writes via DL-2 EventStore + reads.
func TestSQLiteRowCountCollector_Roundtrip(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	es := NewSQLiteEventStore(db, nil)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := es.PersistChannel(ctx, "ch-1", "channel.archived", []byte("{}")); err != nil {
			t.Fatal(err)
		}
	}
	c := &sqliteRowCountCollector{db: db, table: "channel_events"}
	v, err := c.Collect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if v != 3 {
		t.Errorf("row count = %d, want 3", v)
	}
}

// TestNoopCollector_ReturnsZero pins write_lock_wait_ms v1 placeholder.
func TestNoopCollector_ReturnsZero(t *testing.T) {
	t.Parallel()
	v, err := noopCollector{}.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v != 0 {
		t.Errorf("noop = %d, want 0", v)
	}
}

// TestSQLiteWALPagesCollector_NonNegative covers PRAGMA wal_checkpoint path.
// Non-WAL SQLite returns 0 (NULL Scan err swallowed).
func TestSQLiteWALPagesCollector_NonNegative(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	c := &sqliteWALPagesCollector{db: db}
	v, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v < 0 {
		t.Errorf("wal pages = %d, want >=0", v)
	}
}

// TestSQLiteDBSizeCollector_NonNegative ensures PRAGMA roundtrip returns >=0.
func TestSQLiteDBSizeCollector_NonNegative(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	c := &sqliteDBSizeCollector{db: db}
	v, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v < 0 {
		t.Errorf("db_size_mb = %d, want >=0", v)
	}
}
