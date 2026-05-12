// Package datalayer — events_threshold.go: DL-3 §1 threshold monitor for 4 metrics.
//
// Spec: docs/implementation/modules/dl-3-spec.md §1 DL3.1.
// Blueprint: data-layer.md §5 threshold monitor (db_size / wal_pending / write_lock / row_count).
//
// Policy (matching the DL-2 #615 retention sweeper pattern):
//   - 4 threshold constants match blueprint §5 exactly.
//   - level has two alert tiers (WARN / CRITICAL), avoiding scattered inline literals.
//   - Start(ctx) is ctx-aware to avoid goroutine leaks (#608 / #614 / #615).
//   - Alerts go through slog Logger.Warn/Error; there is no admin endpoint for
//     this path (ADM-0 §1.3 prohibition).

package datalayer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"gorm.io/gorm"
)

// ThresholdLevel is the severity classification for a metric reading.
type ThresholdLevel int

const (
	// ThresholdLevelOK indicates value below WARN.
	ThresholdLevelOK ThresholdLevel = iota
	// ThresholdLevelWarn indicates value crossed WARN, below CRITICAL.
	ThresholdLevelWarn
	// ThresholdLevelCritical indicates value crossed CRITICAL.
	ThresholdLevelCritical
)

// String returns the canonical level name (lowercase, slog-friendly).
func (l ThresholdLevel) String() string {
	switch l {
	case ThresholdLevelWarn:
		return "warn"
	case ThresholdLevelCritical:
		return "critical"
	default:
		return "ok"
	}
}

// DBThreshold is the canonical 4-metric contract for the v1 single-machine threshold monitor.
//
// v1 estimates (matching blueprint §5 exactly), tunable in a follow-up after rollout:
//   - db_size_mb         WARN=5000  CRITICAL=10000
//   - wal_pending_pages  WARN=1000  CRITICAL=5000
//   - write_lock_wait_ms WARN=100   CRITICAL=1000
//   - events_row_count   WARN=1_000_000 CRITICAL=10_000_000
type DBThreshold struct {
	Name     string
	Warn     int64
	Critical int64
}

// DefaultThresholds returns the 4 canonical metrics with v1 estimates.
// Policy: keep threshold constants in one place to avoid inline literal drift.
func DefaultThresholds() []DBThreshold {
	return []DBThreshold{
		{Name: "db_size_mb", Warn: 5000, Critical: 10000},
		{Name: "wal_pending_pages", Warn: 1000, Critical: 5000},
		{Name: "write_lock_wait_ms", Warn: 100, Critical: 1000},
		{Name: "events_row_count", Warn: 1_000_000, Critical: 10_000_000},
	}
}

// Classify returns the level for a measured value against this threshold.
func (t DBThreshold) Classify(value int64) ThresholdLevel {
	if value >= t.Critical {
		return ThresholdLevelCritical
	}
	if value >= t.Warn {
		return ThresholdLevelWarn
	}
	return ThresholdLevelOK
}

// MetricCollector reads one metric value via SQLite. Test seam — production
// path uses sqliteMetricCollector below.
type MetricCollector interface {
	Collect(ctx context.Context) (int64, error)
}

// ThresholdMonitor periodically reads 4 metrics, classifies vs DBThreshold,
// emits slog Warn/Error on threshold crossings. Start(ctx) performs graceful
// shutdown with the same ctx-aware pattern as DL-2 EventsRetentionSweeper.
type ThresholdMonitor struct {
	collectors map[string]MetricCollector // metric name → collector
	thresholds []DBThreshold
	logger     *slog.Logger
	interval   time.Duration

	mu       sync.Mutex
	stopOnce sync.Once
	stopped  chan struct{}
}

// NewThresholdMonitor constructs a monitor wrapping db. interval=0 disables
// background ticker (caller drives via RunOnce for tests).
func NewThresholdMonitor(db *gorm.DB, logger *slog.Logger, interval time.Duration) *ThresholdMonitor {
	collectors := map[string]MetricCollector{
		"db_size_mb":         &sqliteDBSizeCollector{db: db},
		"wal_pending_pages":  &sqliteWALPagesCollector{db: db},
		"write_lock_wait_ms": &noopCollector{}, // v1 placeholder (SQLite single-writer, no contention metric)
		"events_row_count":   &sqliteRowCountCollector{db: db, table: "channel_events"},
	}
	return &ThresholdMonitor{
		collectors: collectors,
		thresholds: DefaultThresholds(),
		logger:     logger,
		interval:   interval,
		stopped:    make(chan struct{}),
	}
}

// SetCollector overrides a metric collector (test seam).
func (m *ThresholdMonitor) SetCollector(name string, c MetricCollector) {
	m.collectors[name] = c
}

// Start runs the monitor loop until ctx cancels. Returns immediately.
func (m *ThresholdMonitor) Start(ctx context.Context) {
	if m.interval <= 0 {
		m.stopOnce.Do(func() { close(m.stopped) })
		return
	}
	go func() {
		defer m.stopOnce.Do(func() { close(m.stopped) })
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()
		m.RunOnce(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.RunOnce(ctx)
			}
		}
	}()
}

// Done returns a chan closed on graceful shutdown.
func (m *ThresholdMonitor) Done() <-chan struct{} { return m.stopped }

// RunOnce reads each metric, classifies, and emits log. Returns the per-metric
// readings + level for callers / tests.
func (m *ThresholdMonitor) RunOnce(ctx context.Context) map[string]ThresholdReading {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]ThresholdReading, len(m.thresholds))
	for _, t := range m.thresholds {
		c, ok := m.collectors[t.Name]
		if !ok {
			continue
		}
		v, err := c.Collect(ctx)
		if err != nil {
			if m.logger != nil {
				m.logger.Error("dl3.threshold_collect_failed", "metric", t.Name, "error", err)
			}
			continue
		}
		level := t.Classify(v)
		out[t.Name] = ThresholdReading{Value: v, Level: level}
		switch level {
		case ThresholdLevelCritical:
			if m.logger != nil {
				m.logger.Error("dl3.threshold_crossed",
					"metric", t.Name, "value", v, "critical", t.Critical, "level", level.String())
			}
		case ThresholdLevelWarn:
			if m.logger != nil {
				m.logger.Warn("dl3.threshold_crossed",
					"metric", t.Name, "value", v, "warn", t.Warn, "level", level.String())
			}
		}
	}
	return out
}

// ThresholdReading captures one metric snapshot.
type ThresholdReading struct {
	Value int64
	Level ThresholdLevel
}

// ----- v1 SQLite-backed collectors -----

type sqliteDBSizeCollector struct{ db *gorm.DB }

func (c *sqliteDBSizeCollector) Collect(ctx context.Context) (int64, error) {
	var pageCount, pageSize int64
	if err := c.db.WithContext(ctx).Raw(`PRAGMA page_count`).Row().Scan(&pageCount); err != nil {
		return 0, err
	}
	if err := c.db.WithContext(ctx).Raw(`PRAGMA page_size`).Row().Scan(&pageSize); err != nil {
		return 0, err
	}
	bytes := pageCount * pageSize
	return bytes / (1024 * 1024), nil // MB
}

type sqliteWALPagesCollector struct{ db *gorm.DB }

func (c *sqliteWALPagesCollector) Collect(ctx context.Context) (int64, error) {
	// PRAGMA wal_checkpoint(PASSIVE) returns (busy, log_size, checkpointed).
	// We read log_size as proxy for pending pages without forcing checkpoint.
	var busy, logSize, ckpt int64
	row := c.db.WithContext(ctx).Raw(`PRAGMA wal_checkpoint(PASSIVE)`).Row()
	if err := row.Scan(&busy, &logSize, &ckpt); err != nil {
		// SQLite without WAL returns NULL — treat as 0 pending.
		return 0, nil
	}
	if logSize < 0 {
		return 0, nil
	}
	return logSize, nil
}

type sqliteRowCountCollector struct {
	db    *gorm.DB
	table string
}

func (c *sqliteRowCountCollector) Collect(ctx context.Context) (int64, error) {
	var n int64
	if err := c.db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM ` + c.table).Row().Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// noopCollector returns 0 (placeholder for write_lock_wait_ms — v1 SQLite
// single-writer doesn't expose contention; v2+ instruments via Begin/Commit
// timing wrapper).
type noopCollector struct{}

func (noopCollector) Collect(_ context.Context) (int64, error) { return 0, nil }
