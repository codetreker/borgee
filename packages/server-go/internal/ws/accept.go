package ws

import (
	"github.com/coder/websocket"

	"borgee-server/internal/config"
)

// wsAcceptOptions builds the coder/websocket AcceptOptions shared by all
// three WS rails (/ws, /ws/plugin, /ws/remote).
//
// CSWSH (Cross-Site WebSocket Hijacking) defense: in production the upgrade
// is authorized only for
//   - requests with NO Origin header (the Go / Node / openclaw PROCESS
//     clients dial without one — coder/websocket auto-authorizes these);
//   - a same-origin browser request (Origin host == request Host — the SPA
//     served from the same deploy host);
//   - any origin listed in OriginPatterns, sourced from the per-env
//     CORS_ORIGIN config (e.g. https://testing-borgee.codetrek.cn). Because
//     CORS_ORIGIN carries a scheme, coder/websocket matches it against
//     "scheme://host", so a cross-origin browser is 403'd.
//
// In development we keep the old permissive behavior (InsecureSkipVerify)
// so the e2e Playwright browser (Origin http://127.0.0.1:5174 → server
// 127.0.0.1:4901, a cross-origin pair) and unit-test dials are not rejected.
// CORS_ORIGIN is required-non-empty in non-dev (config.Validate), so the
// production branch always has a concrete allow-list.
func wsAcceptOptions(cfg *config.Config) *websocket.AcceptOptions {
	if cfg.IsDevelopment() {
		return &websocket.AcceptOptions{InsecureSkipVerify: true}
	}
	return &websocket.AcceptOptions{OriginPatterns: []string{cfg.CORSOrigin}}
}
