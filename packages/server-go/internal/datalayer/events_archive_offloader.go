// Package datalayer — events_archive_offloader.go: DL-3 §1 DL3.2 cold archive
// offload.
//
// Spec: docs/implementation/modules/dl-3-spec.md §1 DL3.2.
// Blueprint: data-layer.md §5 阈值哨 + cold archive offload.
//
// 设计 (跟 DL-2 #615 EventStore + retention sweeper 同精神承袭):
//   - 当 channel_events 行数超 offload threshold → 走 SQLite ATTACH 旁库
//     INSERT SELECT 把 created_at < cutoff 的 row 搬到 archive_<yyyy-mm>.db,
//     源表 DELETE 同事务 — v1 单机磁盘, v2+ 切 Storage interface (蓝图 §4.B.8).
//   - audit log "events.archive_offload" 走 DL-2 EventBus.Publish 必落 kind
//     (跟 must_persist_kinds.go admin.force_* 同精神承袭, EventBus 不挂
//     admin god-mode endpoint, ADM-0 §1.3 红线).
//   - ctx-aware RunOnce(ctx), 反 goroutine leak (DL-2 retention sweeper 设计
//     承袭).

package datalayer

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gorm.io/gorm"
)

// EventsArchiveOffloader offloads aged channel_events rows to a per-month
// SQLite sidecar file when row count exceeds a threshold, then DELETEs the
// source rows. Audit trail emits "events.archive_offload" via EventBus.
type EventsArchiveOffloader struct {
	db        *gorm.DB
	bus       EventBus      // DL-2 EventBus for audit "events.archive_offload"
	logger    *slog.Logger
	archiveDir string       // base directory for archive_<yyyy-mm>.db files
	threshold int64         // row count above which offload triggers
	cutoffAge time.Duration // rows with created_at older than now-cutoffAge offload
	interval  time.Duration // background ticker cadence (0 disables; caller drives via RunOnce)
	now       func() time.Time

	mu       sync.Mutex
	stopOnce sync.Once
	stopped  chan struct{}
}

// NewEventsArchiveOffloader constructs an offloader. archiveDir is created
// on first run. threshold=0 → use default 1_000_000 (蓝图 §5 events_row_count
// WARN). cutoffAge=0 → 30 days (channel.* default retention). interval=0
// disables background ticker (caller drives via RunOnce for tests); WIRE-1
// production wires interval=time.Hour 跟 ThresholdMonitor 同精神 ctx-aware.
func NewEventsArchiveOffloader(db *gorm.DB, bus EventBus, logger *slog.Logger, archiveDir string, threshold int64, cutoffAge, interval time.Duration) *EventsArchiveOffloader {
	if threshold <= 0 {
		threshold = 1_000_000
	}
	if cutoffAge <= 0 {
		cutoffAge = 30 * 24 * time.Hour
	}
	if archiveDir == "" {
		archiveDir = "./data"
	}
	return &EventsArchiveOffloader{
		db:         db,
		bus:        bus,
		logger:     logger,
		archiveDir: archiveDir,
		threshold:  threshold,
		cutoffAge:  cutoffAge,
		interval:   interval,
		now:        time.Now,
		stopped:    make(chan struct{}),
	}
}

// Start runs the offloader loop until ctx cancels. Returns immediately;
// caller may `<-offloader.Done()` after ctx cancel to ensure clean shutdown
// (反 goroutine leak, 跟 DL-2 EventsRetentionSweeper / DL-3 ThresholdMonitor
// 同模式承袭).
func (o *EventsArchiveOffloader) Start(ctx context.Context) {
	if o.interval <= 0 {
		o.stopOnce.Do(func() { close(o.stopped) })
		return
	}
	go func() {
		defer o.stopOnce.Do(func() { close(o.stopped) })
		ticker := time.NewTicker(o.interval)
		defer ticker.Stop()
		o.runOnceLog(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				o.runOnceLog(ctx)
			}
		}
	}()
}

// Done returns a chan closed when the background loop has exited.
func (o *EventsArchiveOffloader) Done() <-chan struct{} { return o.stopped }

// runOnceLog wraps RunOnce + logger output for the ticker driver.
func (o *EventsArchiveOffloader) runOnceLog(ctx context.Context) {
	res, err := o.RunOnce(ctx)
	if err != nil {
		if o.logger != nil {
			o.logger.Error("dl3.archive_offload_failed", "error", err)
		}
		return
	}
	if res.Triggered && o.logger != nil {
		o.logger.Info("dl3.archive_offload_tick_done",
			"archive", res.ArchivePath,
			"rows_archived", res.RowsArchived,
			"rows_deleted", res.RowsDeleted)
	}
}

// OffloadResult reports counts from one offload pass.
type OffloadResult struct {
	Triggered    bool   // false when row count below threshold (no-op)
	ArchivePath  string // resolved archive_<yyyy-mm>.db path on disk
	RowsArchived int64
	RowsDeleted  int64
}

// RunOnce inspects channel_events row count; if above threshold, offloads
// rows older than cutoffAge to archive_<yyyy-mm>.db and DELETEs from source.
// Emits "events.archive_offload" audit via EventBus on success.
func (o *EventsArchiveOffloader) RunOnce(ctx context.Context) (OffloadResult, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	var res OffloadResult

	var rowCount int64
	if err := o.db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM channel_events`).Row().Scan(&rowCount); err != nil {
		return res, fmt.Errorf("count channel_events: %w", err)
	}
	if rowCount < o.threshold {
		return res, nil // below threshold — no-op
	}

	now := o.now()
	cutoffMs := now.Add(-o.cutoffAge).UnixMilli()
	if err := os.MkdirAll(o.archiveDir, 0o755); err != nil {
		return res, fmt.Errorf("mkdir archive: %w", err)
	}
	archivePath := filepath.Join(o.archiveDir, fmt.Sprintf("events_archive_%s.db", now.Format("2006-01")))
	res.ArchivePath = archivePath
	res.Triggered = true

	// SQLite forbids ATTACH/DETACH inside a transaction, so we ATTACH first,
	// run INSERT SELECT + DELETE in a single transaction (savepoint), then
	// DETACH after commit. Failure path detaches before returning.
	conn := o.db.WithContext(ctx)
	if err := conn.Exec(fmt.Sprintf(`ATTACH DATABASE '%s' AS arch`, archivePath)).Error; err != nil {
		return res, fmt.Errorf("attach archive: %w", err)
	}
	detach := func() {
		_ = conn.Exec(`DETACH DATABASE arch`).Error
	}
	if err := conn.Exec(`CREATE TABLE IF NOT EXISTS arch.channel_events (
		lex_id TEXT PRIMARY KEY,
		channel_id TEXT NOT NULL,
		kind TEXT NOT NULL,
		payload TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		retention_days INTEGER
	)`).Error; err != nil {
		detach()
		return res, fmt.Errorf("create arch table: %w", err)
	}

	txErr := conn.Transaction(func(tx *gorm.DB) error {
		insRes := tx.Exec(`INSERT OR IGNORE INTO arch.channel_events
			(lex_id, channel_id, kind, payload, created_at, retention_days)
			SELECT lex_id, channel_id, kind, payload, created_at, retention_days
			FROM main.channel_events
			WHERE created_at < ?`, cutoffMs)
		if insRes.Error != nil {
			return fmt.Errorf("insert arch: %w", insRes.Error)
		}
		res.RowsArchived = insRes.RowsAffected

		delRes := tx.Exec(`DELETE FROM main.channel_events WHERE created_at < ?`, cutoffMs)
		if delRes.Error != nil {
			return fmt.Errorf("delete source: %w", delRes.Error)
		}
		res.RowsDeleted = delRes.RowsAffected
		return nil
	})
	detach()
	if txErr != nil {
		return res, txErr
	}

	// Audit via DL-2 EventBus (must-persist kind admin.force_* 邻域;
	// kind = "events.archive_offload" 跟 spec §0 设计 ② 字面 byte-identical).
	if o.bus != nil {
		payload := []byte(fmt.Sprintf(
			`{"archive_path":%q,"rows_archived":%d,"rows_deleted":%d,"cutoff_ms":%d}`,
			archivePath, res.RowsArchived, res.RowsDeleted, cutoffMs))
		if err := o.bus.Publish(ctx, "events.archive_offload", payload); err != nil && o.logger != nil {
			o.logger.Error("dl3.archive_offload_audit_failed", "error", err)
		}
	}
	if o.logger != nil {
		o.logger.Info("dl3.archive_offload_done",
			"archive", archivePath,
			"rows_archived", res.RowsArchived,
			"rows_deleted", res.RowsDeleted)
	}
	return res, nil
}
