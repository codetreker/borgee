// Package borgee — single-binary HB stack daemon + claim + signed-manifest
// installer + setup subcommand (separate module from server-go to keep server
// binary slim per HB stack Go spec patch §5.5).
//
// Folded from the 3 prior binaries (borgee-helper / borgee-helper-claim /
// install-butler) by the chore/npm-bundle-rework PR. Subcommands:
//   - borgee daemon   — HB-2 host-bridge daemon (常驻无 sudo, IPC server)
//   - borgee claim    — one-time enrollment claim CLI (was borgee-helper-claim)
//   - borgee install  — HB-1 signed-manifest installer (was install-butler)
//   - borgee setup    — systemd/launchd unit + state-dir bootstrap (was .deb postinstall)
module borgee

go 1.25.0

require (
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mattn/go-sqlite3 v1.14.22 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.20.0 // indirect
	gorm.io/driver/sqlite v1.6.0 // indirect
	gorm.io/gorm v1.31.1 // indirect
)
