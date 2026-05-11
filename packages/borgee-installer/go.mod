// Package borgee-installer — HB-1B-INSTALLER cross-platform installer Go
// binaries (separate module from server-go + borgee-helper to keep binaries
// slim per HB stack Go spec patch §5.5).
//
// Contains:
//   - cmd/borgee-installer-linux  — Linux .deb installer (sudo apt + systemd)
//   - cmd/borgee-installer-darwin — macOS .pkg installer (sudo installer + launchd)
//   - (留 v2) cmd/borgee-installer-windows — Windows .msi (PowerShell + Service)
//
// Shared internal/ packages: manifest (HB-1 #589 endpoint fetch + ed25519
// verify) + dialog (4 grant_type permission popup) + deploy (per-platform
// service unit 部署 wrapping borgee-helper.{service,plist}).
//
// 立场 (hb-1b-installer-spec §0):
//   - HB-1 #589 server endpoint and HB-2 v0(D) #617 daemon remain source-of-truth
//   - installer implementation stays in this module and uses helper install assets
//   - no installer admin API path (ADM-0 §1.3 red line)
module borgee-installer

go 1.25.0
