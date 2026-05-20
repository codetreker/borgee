#!/usr/bin/env bash
# Mirror of scripts/check-openclaw-plugin-version-bump.sh for
# @codetreker/borgee-remote-agent. The npm tarball bundles:
#   - packages/remote-agent/{src,bin,package.json}  (Node CLI + shim)
#   - packages/borgee/                              (Go binary, built per-plat
#                                                   and staged under bin/platforms/)
# So changes under any of those paths require a bump of
# packages/remote-agent/package.json `version`.
#
# Go *_test.go and testdata/ trees never ship in the tarball (only the built
# binary does), so they are excluded — matching the "doesn't ship → doesn't
# need a bump" rule. TS source under packages/remote-agent/src/ is treated as
# a single bucket (mirroring openclaw, whose script likewise treats tests as
# triggering — they ship via `files: ["dist", ...]` if/when bundled).
set -euo pipefail

PKG_VERSION_FILE=${PKG_VERSION_FILE:-packages/remote-agent/package.json}
BASE_SHA=${BASE_SHA:?BASE_SHA is required}
HEAD_SHA=${HEAD_SHA:?HEAD_SHA is required}

MERGE_BASE=$(git merge-base "$BASE_SHA" "$HEAD_SHA")

changed=$(git diff --name-only "$MERGE_BASE" "$HEAD_SHA" -- \
  'packages/remote-agent/src/' \
  'packages/remote-agent/bin/' \
  'packages/remote-agent/package.json' \
  'packages/borgee/' \
  ':(exclude)packages/borgee/**/*_test.go' \
  ':(exclude)packages/borgee/**/testdata/**' \
  || true)

if [ -z "$changed" ]; then
  echo "ok: no remote-agent tarball files changed"
  exit 0
fi

read_package_version() {
  local revision="$1"
  git show "$revision:$PKG_VERSION_FILE" \
    | sed -nE 's/.*"version"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' \
    | head -n 1
}

base_version=$(read_package_version "$MERGE_BASE")
head_version=$(read_package_version "$HEAD_SHA")

if [ -z "$base_version" ] || [ -z "$head_version" ]; then
  echo "::error::Unable to read $PKG_VERSION_FILE version at merge base or head"
  exit 1
fi

if [ "$base_version" = "$head_version" ]; then
  echo "::error::remote-agent tarball files changed but $PKG_VERSION_FILE version stayed at $head_version"
  echo "Changed files:"
  echo "$changed"
  exit 1
fi

echo "ok: remote-agent version bumped $base_version -> $head_version"
