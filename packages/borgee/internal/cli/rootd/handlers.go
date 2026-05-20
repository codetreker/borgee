//go:build linux || darwin

package rootd

import (
	"context"
	"encoding/json"
	"time"
)

// DefaultHandlers returns the production whitelist. PR-1 only exposes
// `ping` to validate the IPC + auth + audit pattern. PR-4 will add three
// real commands by extending this map:
//
//   - install_plugin      — invokes installbutler to fetch + verify + place
//                           a signed runtime plugin binary
//   - service_lifecycle   — start/stop/restart a declared systemd unit
//                           (units come from the signed manifest, not from
//                           client-supplied strings)
//   - delegation_revoke   — disable a delegation locally, drain in-flight
//                           leased jobs
//
// Every future addition MUST:
//
//  1. Type-check its params with a fixed schema (no map[string]any pass-through).
//  2. Reject unknown / extra fields.
//  3. Be safe to log the cmd name + ok status (no secrets in audit line).
//  4. Document the threat model addition in the package doc comment.
func DefaultHandlers() map[string]HandlerFunc {
	return map[string]HandlerFunc{
		"ping": pingHandler,
	}
}

// pingHandler is the smoke command. Echoes a small pong envelope so the
// main daemon can prove IPC connectivity + audit-trail wiring before
// PR-4 ships the real root commands.
func pingHandler(_ context.Context, _ json.RawMessage) (any, error) {
	return map[string]any{
		"pong": true,
		"time": time.Now().UnixMilli(),
	}, nil
}
