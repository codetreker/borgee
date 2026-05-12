// Package push — gateway.go: DL-4.3 Web Push gateway (server → browser
// push via VAPID).
//
// Blueprint anchor: docs/blueprint/current/client-shape.md L46 wording:
// "manifest.json + push subscription endpoint + VAPID key generation + one
// server-go push channel wired to data-layer §3.4 global_events fan-out".
// Spec brief: docs/implementation/modules/dl-4-spec.md §1 DL-4.3.
//
// What this gateway does:
//   1. Read VAPID private/public/subject from server env at construction
//      (Bootstrap fails loudly if missing, matching the admin Bootstrap env pattern).
//   2. Send(userID, payload) — query web_push_subscriptions WHERE
//      user_id=? → for each row, web-push library encrypt + POST endpoint.
//   3. 410 Gone response → DELETE subscription row (browser unsubscribed,
//      table cleanup). Other errors → log warn, continue.
//   4. Best-effort: caller (mention dispatch / agent_task_state_changed
//      derive) doesn't await; failures don't propagate, matching DM-2.2 #372
//      mention dispatch.
//
// Negative constraints (blueprint L22 + spec §0 principles ①②③):
//   - VAPID private key is read only from server env. It is not stored in a
//     table, request body, or log.
//   - Push 不走 hub.cursors sequence (fire-and-forget).
//   - There is no admin route for actively pushing to a specific user. The
//     reverse-grep check is `admin.*push\.Gateway|admin.*PushSubscribe` count==0.
//   - subscription 410 Gone → DELETE row. Unsubscribe state has one source: the
//     row is deleted instead of keeping an enabled=false flag.
package push

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/SherClockHolmes/webpush-go"

	"borgee-server/internal/store"
)

// Gateway is the server→browser Web Push fan-out seam. Implements the
// minimum invariant: caller passes (userID, payload) → gateway looks up
// all subscriptions + emits encrypted notifications. Best-effort.
//
// Constructed once at server boot via NewGateway(env-driven). Nil-safe
// callers may inject a no-op gateway via NewNoopGateway() in tests / when
// VAPID env not set in dev.
type Gateway interface {
	// Send fires a push notification to every subscription owned by
	// userID. Payload must be a JSON-serializable map (gateway encodes
	// to JSON before encryption). Returns the count of attempts (sent +
	// failed) — pure observability, not error semantics.
	Send(ctx context.Context, userID string, payload map[string]any) int
}

// vapidGateway is the production Gateway backed by SherClockHolmes/webpush-go
// + the web_push_subscriptions store table.
type vapidGateway struct {
	store      *store.Store
	logger     *slog.Logger
	publicKey  string // VAPID public key (base64 url-safe)
	privateKey string // VAPID private key (base64 url-safe)
	subject    string // mailto: or https:// URL identifying the application
	httpClient *http.Client
	now        func() time.Time
}

// noopGateway is the dev/test Gateway that records call count but does
// not emit. Used when VAPID env missing OR caller wants test isolation.
type noopGateway struct {
	logger *slog.Logger
}

// NewNoopGateway returns a Gateway that logs each call and returns 0
// without emitting. Used in tests + dev when VAPID env unset.
func NewNoopGateway(logger *slog.Logger) Gateway {
	return &noopGateway{logger: logger}
}

func (g *noopGateway) Send(ctx context.Context, userID string, payload map[string]any) int {
	if g.logger != nil {
		g.logger.Debug("push.noopGateway.Send", "user_id", userID, "payload_keys", keysOf(payload))
	}
	return 0
}

// NewGateway constructs a production Gateway from server env.
//
// Required env vars:
//   - BORGEE_VAPID_PUBLIC_KEY  (base64 url-safe)
//   - BORGEE_VAPID_PRIVATE_KEY (base64 url-safe)
//   - BORGEE_VAPID_SUBJECT     (mailto:admin@example.com OR https://...)
//
// Returns (Gateway, nil) on success; (nil, error) on missing env. Caller
// MAY fall back to NewNoopGateway in dev. This differs from admin Bootstrap,
// which must panic on missing env; push is a user-experience add-on and should
// not block server startup.
func NewGateway(s *store.Store, logger *slog.Logger) (Gateway, error) {
	pub := os.Getenv("BORGEE_VAPID_PUBLIC_KEY")
	priv := os.Getenv("BORGEE_VAPID_PRIVATE_KEY")
	sub := os.Getenv("BORGEE_VAPID_SUBJECT")
	if pub == "" || priv == "" || sub == "" {
		return nil, fmt.Errorf("push: VAPID env missing (BORGEE_VAPID_PUBLIC_KEY=%t / PRIVATE=%t / SUBJECT=%t)",
			pub != "", priv != "", sub != "")
	}
	return &vapidGateway{
		store:      s,
		logger:     logger,
		publicKey:  pub,
		privateKey: priv,
		subject:    sub,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		now:        time.Now,
	}, nil
}

// subscriptionRow mirrors the migration v=24 schema columns we need at
// emit time. last_used_at is bumped on successful send (audit hint).
type subscriptionRow struct {
	ID         string `gorm:"column:id"`
	Endpoint   string `gorm:"column:endpoint"`
	P256DHKey  string `gorm:"column:p256dh_key"`
	AuthKey    string `gorm:"column:auth_key"`
	UserAgent  string `gorm:"column:user_agent"`
	LastUsedAt *int64 `gorm:"column:last_used_at"`
}

// Send fires a push notification to every subscription owned by userID.
// Returns the count of attempts (sent + failed). Failures are logged but
// do not propagate — caller is fire-and-forget (跟 DM-2.2 #372 同模式).
//
// Per-row error handling:
//   - 410 Gone: subscription expired/unsubscribed → DELETE row as the single
//     cleanup source.
//   - Other 4xx/5xx: log warn, do not delete.
//   - Transport error: log warn, do not delete (transient).
func (g *vapidGateway) Send(ctx context.Context, userID string, payload map[string]any) int {
	var rows []subscriptionRow
	if err := g.store.DB().Raw(`SELECT id, endpoint, p256dh_key, auth_key, user_agent, last_used_at
		FROM web_push_subscriptions WHERE user_id = ?`, userID).Scan(&rows).Error; err != nil {
		if g.logger != nil {
			g.logger.Warn("push.vapidGateway.Send: scan failed", "user_id", userID, "err", err)
		}
		return 0
	}

	if len(rows) == 0 {
		return 0
	}

	body, err := json.Marshal(payload)
	if err != nil {
		if g.logger != nil {
			g.logger.Warn("push.vapidGateway.Send: payload marshal failed", "user_id", userID, "err", err)
		}
		return 0
	}

	attempts := 0
	for _, row := range rows {
		attempts++
		if err := g.sendOne(ctx, body, row); err != nil {
			g.logger.Warn("push.vapidGateway.Send: emit failed",
				"user_id", userID, "endpoint", row.Endpoint, "err", err)
		}
	}
	return attempts
}

// sendOne emits one push and handles the 410 Gone GC path.
func (g *vapidGateway) sendOne(ctx context.Context, body []byte, row subscriptionRow) error {
	sub := &webpush.Subscription{
		Endpoint: row.Endpoint,
		Keys: webpush.Keys{
			Auth:   row.AuthKey,
			P256dh: row.P256DHKey,
		},
	}
	resp, err := webpush.SendNotificationWithContext(ctx, body, sub, &webpush.Options{
		HTTPClient:      g.httpClient,
		Subscriber:      g.subject,
		VAPIDPublicKey:  g.publicKey,
		VAPIDPrivateKey: g.privateKey,
		TTL:             30, // seconds — short-lived so AI-team status feels current
	})
	if err != nil {
		return fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound {
		// Subscription dead — DELETE row; blueprint L22 makes row deletion the
		// single unsubscribe state.
		if err := g.store.DB().Exec(`DELETE FROM web_push_subscriptions WHERE id = ?`, row.ID).Error; err != nil {
			return fmt.Errorf("410 GC delete failed: %w", err)
		}
		return errors.New("410 Gone — subscription deleted")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("status=%d", resp.StatusCode)
	}

	// Success — bump last_used_at (audit hint).
	now := g.now().UnixMilli()
	if err := g.store.DB().Exec(`UPDATE web_push_subscriptions SET last_used_at = ? WHERE id = ?`,
		now, row.ID).Error; err != nil && g.logger != nil {
		// Non-fatal — push succeeded, audit hint stale only.
		g.logger.Debug("push.vapidGateway.Send: last_used_at update failed",
			"id", row.ID, "err", err)
	}
	return nil
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
