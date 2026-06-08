#!/usr/bin/env bash
# Cross-compile the borgee daemon for all 4 supported npm-package platforms in
# one shot from a single host. The binary is pure Go (only coder/websocket),
# so CGO_ENABLED=0 cross-compiles every target without a C toolchain or a
# native per-platform runner. Output lands under
# packages/borgee/bin/platforms/<plat>-<arch>/borgee, which the npm `files`
# whitelist ships and the dispatcher resolves at runtime.
#
# VERSION defaults to packages/borgee/package.json's version (so a local build
# and the CI publish stamp the same value). Override via `VERSION=x.y.z`.
set -euo pipefail

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
PKG_DIR="$REPO_ROOT/packages/borgee"
OUT_ROOT="$PKG_DIR/bin/platforms"

VERSION="${VERSION:-$(node -p "require('$PKG_DIR/package.json').version")}"
if [ -z "$VERSION" ]; then
  echo "FAIL: could not determine VERSION" >&2
  exit 1
fi

# (GOOS, GOARCH, plat-key) triples. plat-key matches the dispatcher's SUPPORTED
# set + npm arch naming (x64 not amd64, arm64 not arm64-the-goarch alias).
TARGETS=(
  "linux  amd64 linux-x64"
  "linux  arm64 linux-arm64"
  "darwin amd64 darwin-x64"
  "darwin arm64 darwin-arm64"
)

rm -rf "$OUT_ROOT"
for t in "${TARGETS[@]}"; do
  # shellcheck disable=SC2086
  set -- $t
  goos="$1"; goarch="$2"; key="$3"
  out="$OUT_ROOT/$key/borgee"
  mkdir -p "$OUT_ROOT/$key"
  echo "building $key (GOOS=$goos GOARCH=$goarch) -> $out"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go -C "$PKG_DIR" build \
      -trimpath \
      -ldflags="-s -w -X main.version=$VERSION" \
      -o "$out" \
      ./cmd/borgee
  chmod 0755 "$out"
done

# fail-loud: exactly 4 binaries built.
count=$(find "$OUT_ROOT" -type f -name borgee | wc -l | tr -d ' ')
if [ "$count" != "4" ]; then
  echo "FAIL: expected 4 platform binaries, got $count" >&2
  find "$OUT_ROOT" -type f
  exit 1
fi
echo "ok: built 4 borgee binaries (version $VERSION) under $OUT_ROOT"
