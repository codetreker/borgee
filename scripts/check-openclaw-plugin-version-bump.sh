#!/usr/bin/env bash
set -euo pipefail

PLUGIN_DIR=${PLUGIN_DIR:-packages/plugins/openclaw}
BASE_SHA=${BASE_SHA:?BASE_SHA is required}
HEAD_SHA=${HEAD_SHA:?HEAD_SHA is required}

MERGE_BASE=$(git merge-base "$BASE_SHA" "$HEAD_SHA")

changed=$(git diff --name-only "$MERGE_BASE" "$HEAD_SHA" -- "$PLUGIN_DIR" || true)

if [ -z "$changed" ]; then
  echo "ok: no OpenClaw plugin files changed"
  exit 0
fi

read_package_version() {
  local revision="$1"
  git show "$revision:$PLUGIN_DIR/package.json" \
    | sed -nE 's/.*"version"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' \
    | head -n 1
}

base_version=$(read_package_version "$MERGE_BASE")
head_version=$(read_package_version "$HEAD_SHA")

if [ -z "$base_version" ] || [ -z "$head_version" ]; then
  echo "::error::Unable to read $PLUGIN_DIR/package.json version at merge base or head"
  exit 1
fi

if [ "$base_version" = "$head_version" ]; then
  echo "::error::$PLUGIN_DIR files changed but package.json version stayed at $head_version"
  echo "Changed plugin files:"
  echo "$changed"
  exit 1
fi

echo "ok: OpenClaw plugin version bumped $base_version -> $head_version"
