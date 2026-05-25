// Package borgee — single-binary HB stack daemon + signed-manifest
// installer + one-shot install bootstrap (separate module from server-go
// to keep server binary slim per HB stack Go spec patch §5.5).
//
// Folded from the 3 prior binaries (borgee-helper / borgee-helper-claim /
// install-butler) by the chore/npm-bundle-rework PR. After issue #1055
// the public subcommand surface is:
//   - borgee install         — one-shot operator bootstrap (chains internal setup + claim + start + heartbeat-wait)
//   - borgee uninstall-host  — operator-driven local cleanup (mirror of install)
//   - borgee daemon          — HB-2 host-bridge daemon (常驻无 sudo, IPC server)
//   - borgee rootd           — root companion daemon (narrow IPC whitelist)
//   - borgee install-plugin  — HB-1 signed-manifest installer (was install-butler)
//
// The prior top-level `claim` and `setup` subcommands were dropped (issue
// #1055); their packages live on as internal helpers under
// internal/cli/claim and internal/cli/setup, invoked by `borgee install`.
module borgee

go 1.25.0

require (
	github.com/mattn/go-sqlite3 v1.14.22
	golang.org/x/sys v0.43.0
)

require github.com/coder/websocket v1.8.14 // indirect
