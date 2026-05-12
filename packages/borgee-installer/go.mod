// Package borgee-installer contains the HB-1B-INSTALLER cross-platform Go
// installer binaries. It stays separate from server-go and borgee-helper so the
// installer binaries remain small, as required by HB stack Go spec patch §5.5.
//
// Contains:
//   - cmd/borgee-installer-linux  — Linux .deb installer (sudo apt + systemd)
//   - cmd/borgee-installer-darwin — macOS .pkg installer (sudo installer + launchd)
//   - cmd/borgee-installer-windows — Windows .msi installer (PowerShell + Windows
//     Service; planned for v2)
//
// Shared internal/ packages: manifest (HB-1 #589 endpoint fetch + ed25519
// verify), dialog (4 grant_type permission dialog), and deploy (per-platform
// service unit deployment wrapping borgee-helper.{service,plist}).
//
// Boundaries (hb-1b-installer-spec §0):
//   - HB-1 #589 server endpoint and HB-2 v0(D) #617 daemon remain canonical sources
//   - installer implementation stays in this module and uses helper install assets
//   - no installer admin API path (ADM-0 §1.3 red line)
module borgee-installer

go 1.25.0
