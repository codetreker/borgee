// Package borgee — single-binary HB stack daemon (separate module from
// server-go to keep server binary slim per HB stack Go spec patch §5.5).
//
// t3a (binary strip) reduced the public subcommand surface to:
//   - borgee install  — operator bootstrap (fail-loud stub; rebuilt by T3b)
//   - borgee daemon   — host-bridge daemon (fail-loud stub; rebuilt by T3b)
//
// The high-privilege host subcommands (rootd / install-plugin /
// uninstall-host) and their backing packages were removed in t3a along with
// the helper enrollment / signed-manifest installer rails. T3b rebuilds the
// install + daemon bodies (reverse-WS client) and re-adds coder/websocket.
module borgee

go 1.25.0

require github.com/coder/websocket v1.8.14
