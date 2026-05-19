#!/usr/bin/env bash
# build-pkg.sh — assemble borgee-helper-${VERSION}-darwin-universal.pkg.
#
# Usage:
#   build-pkg.sh <version> <output_dir>
#
# Preconditions:
#   - cmd/borgee-helper/borgee-helper        — universal binary (lipo -create)
#   - cmd/borgee-helper-claim/borgee-helper-claim — universal binary
#   - install/cloud.borgee.host-bridge.plist     — LaunchDaemon plist
#   - install/borgee-helper.sb                   — sandbox-exec profile
#
# Produces:
#   <output_dir>/borgee-helper-<version>-darwin-universal.pkg
#
# Notarization is intentionally out of scope (key handling lives in #997).
set -euo pipefail

if [[ $# -lt 2 ]]; then
    echo "usage: build-pkg.sh <version> <output_dir>" >&2
    exit 64
fi

VERSION="$1"
OUT_DIR="$2"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
HELPER_ROOT="$(cd "$SCRIPT_DIR/../../" && pwd)"

PAYLOAD="$(mktemp -d)"
trap 'rm -rf "$PAYLOAD"' EXIT

mkdir -p "$PAYLOAD/usr/local/bin"
mkdir -p "$PAYLOAD/Library/LaunchDaemons"
mkdir -p "$PAYLOAD/Library/Application Support/Borgee"

cp "$HELPER_ROOT/cmd/borgee-helper/borgee-helper"             "$PAYLOAD/usr/local/bin/borgee-helper"
cp "$HELPER_ROOT/cmd/borgee-helper-claim/borgee-helper-claim" "$PAYLOAD/usr/local/bin/borgee-helper-claim"
cp "$HELPER_ROOT/install/cloud.borgee.host-bridge.plist"      "$PAYLOAD/Library/LaunchDaemons/"
cp "$HELPER_ROOT/install/borgee-helper.sb"                    "$PAYLOAD/Library/Application Support/Borgee/"

chmod 0755 "$PAYLOAD/usr/local/bin/borgee-helper"
chmod 0755 "$PAYLOAD/usr/local/bin/borgee-helper-claim"
chmod 0644 "$PAYLOAD/Library/LaunchDaemons/cloud.borgee.host-bridge.plist"
chmod 0644 "$PAYLOAD/Library/Application Support/Borgee/borgee-helper.sb"

mkdir -p "$OUT_DIR"

OUT_FILE="$OUT_DIR/borgee-helper-${VERSION}-darwin-universal.pkg"

pkgbuild \
    --root "$PAYLOAD" \
    --identifier cloud.borgee.host-bridge \
    --version "$VERSION" \
    --install-location "/" \
    --scripts "$SCRIPT_DIR/scripts" \
    "$OUT_FILE"

echo "built: $OUT_FILE"
