# Borgee Helper packaging

How the `.deb` / `.pkg` get built and where they end up. Tracks issue #1003.

## What gets built

A release run produces three artifacts plus a `SHA256SUMS` file, attached to a
GitHub Release tagged `borgee-helper-vX.Y.Z`:

| Artifact | Target |
|---|---|
| `borgee-helper_X.Y.Z_amd64.deb` | Debian/Ubuntu x86_64 hosts |
| `borgee-helper_X.Y.Z_arm64.deb` | Debian/Ubuntu aarch64 hosts |
| `borgee-helper-X.Y.Z-darwin-universal.pkg` | macOS (amd64 + arm64 in one binary) |
| `SHA256SUMS` | Plain `sha256sum -c` format |

All artifacts install:

- `/usr/local/bin/borgee-helper` — daemon (Linux: CGO + SQLite, macOS: same)
- `/usr/local/bin/borgee-helper-claim` — one-time enrollment CLI
- systemd unit (`/etc/systemd/system/borgee-helper.service`) on Linux
- LaunchDaemon plist (`/Library/LaunchDaemons/cloud.borgee.host-bridge.plist`)
  + sandbox profile on macOS

Install scripts create the `borgee-helper` (Linux) / `_borgee-helper` (macOS)
system user, lay down state directories under `/var/lib/borgee-helper`
(Linux) or `/Library/Application Support/Borgee/Helper` (macOS), and enable —
but do NOT start — the service.

## How releases run in CI

Workflow: `.github/workflows/release-helper.yml`.

Triggers:

- Tag push matching `borgee-helper-v*`. Creates a real GitHub Release.
- `workflow_dispatch` with a `version` input. Builds artifacts and uploads
  them as workflow artifacts but does NOT create a Release (use this for
  dry-runs / staging).

Jobs:

1. `resolve-version` — pulls the version from either the tag or the dispatch
   input.
2. `lint-nfpm-yaml` — runs `nfpm package` in dry-run mode to catch yaml
   schema errors before the matrix kicks in.
3. `build-linux` (matrix amd64 / arm64) — `go build` (CGO=1 for the daemon,
   CGO=0 for the claim CLI), then `nfpm package --packager deb`.
4. `build-darwin` — `go build` amd64 + arm64, `lipo -create` into a universal
   binary, then `pkgbuild` via `install/pkg/build-pkg.sh`.
5. `release` (tag-only) — downloads all artifacts, builds `SHA256SUMS`,
   creates the GitHub Release with attachments.

## How to build locally

Linux .deb (must be run on Linux, or in a Linux container):

```bash
cd packages/borgee-helper
go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest

CGO_ENABLED=1 go build -trimpath -ldflags='-s -w' \
    -o ./cmd/borgee-helper/borgee-helper ./cmd/borgee-helper
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' \
    -o ./cmd/borgee-helper-claim/borgee-helper-claim ./cmd/borgee-helper-claim

VERSION=0.0.0-dev NFPM_ARCH=amd64 \
    nfpm package --packager deb --target ./borgee-helper_0.0.0-dev_amd64.deb -f nfpm.yaml
```

macOS .pkg (must be run on macOS, requires `lipo` + `pkgbuild`):

```bash
cd packages/borgee-helper

for arch in amd64 arm64; do
  GOOS=darwin GOARCH=$arch CGO_ENABLED=1 go build -trimpath -ldflags='-s -w' \
      -o ./cmd/borgee-helper/borgee-helper-$arch ./cmd/borgee-helper
  GOOS=darwin GOARCH=$arch CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' \
      -o ./cmd/borgee-helper-claim/borgee-helper-claim-$arch ./cmd/borgee-helper-claim
done

lipo -create -output ./cmd/borgee-helper/borgee-helper \
    ./cmd/borgee-helper/borgee-helper-amd64 ./cmd/borgee-helper/borgee-helper-arm64
lipo -create -output ./cmd/borgee-helper-claim/borgee-helper-claim \
    ./cmd/borgee-helper-claim/borgee-helper-claim-amd64 ./cmd/borgee-helper-claim/borgee-helper-claim-arm64

mkdir -p dist
bash install/pkg/build-pkg.sh 0.0.0-dev "$(pwd)/dist"
```

## Why no auto-start

The Linux postinstall script enables but does not start
`borgee-helper.service`; the macOS one bootstraps the LaunchDaemon but does
not kickstart it. The daemon's heartbeat producer needs three files inside
the state directory (enrollment id / device id / credential) that only the
`borgee-helper-claim` CLI can populate. Starting the daemon before claim
would just spin a stub that logs `no enrollment configured, skipping
heartbeat`. Letting the operator drive the order (install -> claim -> start)
makes the bring-up sequence obvious in the postinstall output.

## Not yet done in this layer

- Apple notarization + Developer ID signing. Tracked in #997 along with the
  manifest signer.
- `host_manifest.go` auto-update with real SHA256 + signature. Also #997 — it
  will consume the GitHub Release URL + SHA produced by this workflow.
- Windows release. v2 follow-up.
- One-key uninstall purge of `/var/lib/borgee-helper`. Tracked in #998 (the
  preremove hook intentionally leaves state alone so `apt upgrade` doesn't
  drop credentials).
