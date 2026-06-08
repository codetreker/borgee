#!/usr/bin/env bash
# Mirror of scripts/check-openclaw-plugin-version-bump.sh for
# @codetreker/borgee-remote-agent. The npm tarball is sourced from
# packages/borgee and bundles:
#   - packages/borgee/{package.json, borgee-remote-agent.cjs}  (manifest +
#                                                   the zero-dep Node dispatcher)
#   - packages/borgee/bin/platforms/<plat>-<arch>/borgee       (the 4 Go
#                                                   binaries, cross-compiled and
#                                                   staged at pack time, NOT
#                                                   committed)
# So a change to any shipped file under packages/borgee/ requires a bump of
# packages/borgee/package.json `version`.
#
# Go *_test.go and testdata/ trees never ship in the tarball (only the built
# binary does), so they are excluded — matching the "doesn't ship → doesn't
# need a bump" rule.
set -euo pipefail

PKG_VERSION_FILE=${PKG_VERSION_FILE:-packages/borgee/package.json}
BASE_SHA=${BASE_SHA:?BASE_SHA is required}
HEAD_SHA=${HEAD_SHA:?HEAD_SHA is required}

MERGE_BASE=$(git merge-base "$BASE_SHA" "$HEAD_SHA")

changed=$(git diff --name-only "$MERGE_BASE" "$HEAD_SHA" -- \
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
  # A file that is NEW in this PR does not exist at the merge base; `git show`
  # would fatal (exit 128) and abort the script under `set -euo pipefail`
  # before the empty-version branch below can run. Suppress that into an empty
  # string so a new file is handled as "no base version to compare".
  git show "$revision:$PKG_VERSION_FILE" 2>/dev/null \
    | sed -nE 's/.*"version"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' \
    | head -n 1 \
    || true
}

base_version=$(read_package_version "$MERGE_BASE")
head_version=$(read_package_version "$HEAD_SHA")

if [ -z "$head_version" ]; then
  # Head must always have a readable version — if tarball files changed but
  # the package.json version is unreadable at head, that is a real error.
  echo "::error::Unable to read $PKG_VERSION_FILE version at head"
  exit 1
fi
if [ -z "$base_version" ]; then
  # The file is NEW in this PR (absent at the merge base). You cannot fail to
  # bump a file you just created — pass the bump check.
  echo "ok: $PKG_VERSION_FILE is new in this PR (version $head_version); bump check satisfied"
  exit 0
fi

if [ "$base_version" = "$head_version" ]; then
  echo "::error::remote-agent tarball files changed but $PKG_VERSION_FILE version stayed at $head_version"
  echo "Changed files:"
  echo "$changed"
  exit 1
fi

echo "ok: remote-agent version bumped $base_version -> $head_version"
