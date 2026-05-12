package migrations

import (
	"gorm.io/gorm"
)

// webPushSubscriptions is migration v=26 — Phase 4 / DL-4 must-fix.
//
// Blueprint reference: docs/blueprint/current/client-shape.md L22 ("**Mobile
// PWA** team-awareness channel after leaving the desktop + Web Push (VAPID)")
// + L37 ("without push, the AI team feels like a background script rather than
// colleagues") + L42 ("manifest + install prompt + Web Push + standalone").
// data-layer §3.4 global_events fan-out is the upstream hook.
// Spec brief: docs/implementation/modules/dl-4-spec.md (本 PR 同期).
//
// What this migration does:
//  1. CREATE TABLE web_push_subscriptions:
//     - id          TEXT    NOT NULL PRIMARY KEY  (UUID row ID; paired with
//     endpoint UNIQUE as the
//     server-internal
//     route)
//     - user_id     TEXT    NOT NULL              (FK users.id 逻辑; subscription
//     belongs to a user. One user
//     can have multiple devices.)
//     - endpoint    TEXT    NOT NULL UNIQUE       (browser-issued push
//     endpoint URL — VAPID
//     contract; UNIQUE prevents
//     duplicate rows for the same device)
//     - p256dh_key  TEXT    NOT NULL              (subscription public key,
//     base64 url-safe; web-push
//     library requires it to encrypt payloads)
//     - auth_key    TEXT    NOT NULL              (subscription auth secret,
//     base64 url-safe; same requirement)
//     - user_agent  TEXT    NOT NULL DEFAULT ”   (UA hint for admin diagnostics,
//     opaque; do not add
//     device_id / device_kind
//     columns. This matches AL-3.1
//     presence: UA is an audit hint,
//     not a routing key)
//     - created_at  INTEGER NOT NULL              (Unix ms)
//     - last_used_at INTEGER NULL                 (NULL until first push;
//     non-NULL after 410 Gone
//     reaping or successful
//     emit)
//  2. CREATE INDEX idx_web_push_subscriptions_user_id
//     ON web_push_subscriptions(user_id) — fan-out hot path. When the server
//     receives mention/agent_task_state_changed derivatives, it loads all
//     device rows for the user.
//
// Constraints (blueprint §1.4 privacy + DL-4 spec §2):
//   - Do not add an `org_id` column: subscription belongs to user, and org scope
//     is derived through users.org_id. This avoids duplicating the single source
//     used by al_2a_1 / chn_3_1 / al_1b_1.
//   - Do not add `device_id` / `device_kind` columns: UA is an audit hint, not a
//     routing key. This follows AL-3.1 presence_sessions multi-session design;
//     last-wins multi-session behavior does not need a device dimension.
//   - Do not add a `cursor` column: push is fire-and-forget and does not use the
//     hub.cursors sequence. Only RT-1/CV-2/DM-2/CV-4/AL-2b/RT-3 frame classes
//     share that sequence.
//   - Do not add `enabled` / `paused` / `muted` columns: row exists = subscribed,
//     row absent = unsubscribed. DELETE is the unsubscribe path, avoiding a
//     second source such as PATCH enable=false.
//   - Keep endpoint UNIQUE strict: same-device re-registration uses UPSERT to
//     revive p256dh / auth instead of inserting another row, which avoids
//     duplicate encryption work for the same endpoint.
//   - Do not add secret columns (api_key / token / vapid_secret): the VAPID
//     private key stays in server env. Subscription p256dh + auth are
//     client-side public-key material, not server secrets in the web-push protocol.
//
// v=26 sequencing: ADM-2.2 v=23 (impersonation_grants, #484 merged) +
// ?? v=24 + AL-1.4 v=25 (agent_state_log, #492 merged) + **DL-4 v=26**
// (this migration) + later milestones continue at v=27+.
// registry.go pins the literal version.
//
// v0 stance: forward-only, no Down(). The table is new in v0; IF NOT EXISTS
// provides idempotency.
var webPushSubscriptions = Migration{
	Version: 26,
	Name:    "dl_4_1_web_push_subscriptions",
	Up: func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS web_push_subscriptions (
  id            TEXT    NOT NULL PRIMARY KEY,
  user_id       TEXT    NOT NULL,
  endpoint      TEXT    NOT NULL UNIQUE,
  p256dh_key    TEXT    NOT NULL,
  auth_key      TEXT    NOT NULL,
  user_agent    TEXT    NOT NULL DEFAULT '',
  created_at    INTEGER NOT NULL,
  last_used_at  INTEGER NULL
)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_web_push_subscriptions_user_id
			ON web_push_subscriptions(user_id)`).Error; err != nil {
			return err
		}
		return nil
	},
}
