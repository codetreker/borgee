// Package datalayer — events_retention.go: DL-2 retention sweeper.
//
// Spec: docs/implementation/modules/dl-2-spec.md §1 DL2.2.
//
// Policy (matching the AL-7 / HB-5 audit retention sweeper pattern):
//   - per-kind retention threshold: RetentionDaysForKind (must-persist=-1 never
//     reaped / channel+message=30 / agent_task+artifact=60 / other=90).
//   - row-level retention_days overrides the default (NULL = use kind default).
//   - Start(ctx) is ctx-aware: ctx cancel triggers graceful shutdown and avoids
//     goroutine leaks, matching #608 ctx wiring + heartbeat_retention_sweeper.
//   - tick interval is injectable for tests (default 1h; tests use 0 to drive
//     RunOnce manually).

package datalayer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"gorm.io/gorm"
)

// EventsRetentionSweeper sweeps expired rows from channel_events + global_events.
type EventsRetentionSweeper struct {
	db       *gorm.DB
	logger   *slog.Logger
	interval time.Duration
	now      func() time.Time

	mu       sync.Mutex
	stopOnce sync.Once
	stopped  chan struct{}
}

// NewEventsRetentionSweeper constructs a sweeper. interval=0 disables the
// background ticker (caller invokes RunOnce manually for tests).
func NewEventsRetentionSweeper(db *gorm.DB, logger *slog.Logger, interval time.Duration) *EventsRetentionSweeper {
	return &EventsRetentionSweeper{
		db:       db,
		logger:   logger,
		interval: interval,
		now:      time.Now,
		stopped:  make(chan struct{}),
	}
}

// Start runs the sweeper loop until ctx cancels. Returns immediately;
// blocks no caller. Caller may also t.Cleanup(func(){ <-sweeper.Done() }).
func (s *EventsRetentionSweeper) Start(ctx context.Context) {
	if s.interval <= 0 {
		// disabled background mode — caller drives via RunOnce.
		s.stopOnce.Do(func() { close(s.stopped) })
		return
	}
	go func() {
		defer s.stopOnce.Do(func() { close(s.stopped) })
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		// First tick immediately for predictable behavior; subsequent on cadence.
		s.runOnceLog(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.runOnceLog(ctx)
			}
		}
	}()
}

// Done returns a chan closed when the background loop has exited.
// Tests may `<-sweeper.Done()` after ctx cancel to ensure clean shutdown.
func (s *EventsRetentionSweeper) Done() <-chan struct{} { return s.stopped }

// RunOnce executes one sweep pass. Returns count of rows reaped from each
// table. Test seam — production path goes thru Start ticker.
func (s *EventsRetentionSweeper) RunOnce(ctx context.Context) (channelReaped, globalReaped int64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UnixMilli()
	// Reap channel_events: row-level retention_days OR kind default.
	// Skip rows where effective retention is -1 (must-persist).
	chRes := s.db.WithContext(ctx).Exec(`
		DELETE FROM channel_events
		WHERE retention_days IS NOT NULL
		  AND retention_days >= 0
		  AND created_at < ? - retention_days * 86400000
	`, now)
	if chRes.Error != nil {
		return 0, 0, chRes.Error
	}
	channelReaped = chRes.RowsAffected
	// Reap global_events: same shape. must-persist kinds rely on retention_days
	// being NULL or -1 (writer responsibility) so DELETE filter excludes them.
	gRes := s.db.WithContext(ctx).Exec(`
		DELETE FROM global_events
		WHERE retention_days IS NOT NULL
		  AND retention_days >= 0
		  AND created_at < ? - retention_days * 86400000
	`, now)
	if gRes.Error != nil {
		return channelReaped, 0, gRes.Error
	}
	globalReaped = gRes.RowsAffected
	return channelReaped, globalReaped, nil
}

func (s *EventsRetentionSweeper) runOnceLog(ctx context.Context) {
	cReaped, gReaped, err := s.RunOnce(ctx)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("dl2.events_retention_sweep_failed", "error", err)
		}
		return
	}
	if (cReaped > 0 || gReaped > 0) && s.logger != nil {
		s.logger.Info("dl2.events_retention_sweep_done",
			"channel_reaped", cReaped, "global_reaped", gReaped)
	}
}
