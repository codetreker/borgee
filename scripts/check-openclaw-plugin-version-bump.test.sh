#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
SCRIPT="$ROOT/scripts/check-openclaw-plugin-version-bump.sh"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

setup_repo() {
  local repo="$1"
  mkdir -p "$repo/packages/plugins/openclaw/src"
  cat > "$repo/packages/plugins/openclaw/package.json" <<'JSON'
{"name":"@codetreker/borgee-openclaw-plugin","version":"0.1.1"}
JSON
  printf 'export const value = 1;\n' > "$repo/packages/plugins/openclaw/src/index.ts"
  git -C "$repo" init -q
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name test
  git -C "$repo" add .
  git -C "$repo" commit -qm base
}

run_check() {
  local repo="$1"
  local base="$2"
  local head="$3"
  (cd "$repo" && BASE_SHA="$base" HEAD_SHA="$head" "$SCRIPT")
}

expect_fail_without_version_bump() {
  local repo="$TMP/fail-without-bump"
  setup_repo "$repo"
  local base
  base=$(git -C "$repo" rev-parse HEAD)
  printf 'export const value = 2;\n' > "$repo/packages/plugins/openclaw/src/index.ts"
  git -C "$repo" add .
  git -C "$repo" commit -qm change-plugin-code
  local head
  head=$(git -C "$repo" rev-parse HEAD)

  local output
  if output=$(run_check "$repo" "$base" "$head" 2>&1); then
    echo "expected plugin code change without version bump to fail" >&2
    return 1
  fi

  echo "$output" | grep -F 'package.json version stayed at 0.1.1' >/dev/null
}

expect_pass_with_version_bump() {
  local repo="$TMP/pass-with-bump"
  setup_repo "$repo"
  local base
  base=$(git -C "$repo" rev-parse HEAD)
  printf 'export const value = 2;\n' > "$repo/packages/plugins/openclaw/src/index.ts"
  cat > "$repo/packages/plugins/openclaw/package.json" <<'JSON'
{"name":"@codetreker/borgee-openclaw-plugin","version":"0.1.2"}
JSON
  git -C "$repo" add .
  git -C "$repo" commit -qm change-plugin-code-with-bump
  local head
  head=$(git -C "$repo" rev-parse HEAD)

  run_check "$repo" "$base" "$head"
}

expect_fail_for_test_only_change_without_version_bump() {
  local repo="$TMP/fail-test-only-without-bump"
  setup_repo "$repo"
  local base
  base=$(git -C "$repo" rev-parse HEAD)
  printf 'test("x", () => {});\n' > "$repo/packages/plugins/openclaw/src/index.test.ts"
  git -C "$repo" add .
  git -C "$repo" commit -qm change-plugin-test-only
  local head
  head=$(git -C "$repo" rev-parse HEAD)

  local output
  if output=$(run_check "$repo" "$base" "$head" 2>&1); then
    echo "expected plugin test-only change without version bump to fail" >&2
    return 1
  fi

  echo "$output" | grep -F 'package.json version stayed at 0.1.1' >/dev/null
}

expect_pass_without_plugin_change() {
  local repo="$TMP/pass-without-plugin-change"
  setup_repo "$repo"
  local base
  base=$(git -C "$repo" rev-parse HEAD)
  mkdir -p "$repo/docs"
  printf '# note\n' > "$repo/docs/note.md"
  git -C "$repo" add .
  git -C "$repo" commit -qm change-docs
  local head
  head=$(git -C "$repo" rev-parse HEAD)

  run_check "$repo" "$base" "$head"
}

expect_fail_without_version_bump
expect_pass_with_version_bump
expect_fail_for_test_only_change_without_version_bump
expect_pass_without_plugin_change
