// Package borgee-helper — HB stack daemon binaries (separate module from
// server-go to keep server binary slim per HB stack Go spec patch §5.5).
//
// Contains:
//   - cmd/borgee-helper — HB-2 host-bridge daemon (常驻无 sudo, IPC server)
//   - cmd/borgee-helper-claim — one-time claim CLI (#968)
//   - cmd/install-butler — HB-1 short-lived signed-manifest installer (#996)
module borgee-helper

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
