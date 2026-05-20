#!/usr/bin/env bash
# Mirror of scripts/check-openclaw-plugin-version-bump.test.sh for the
# remote-agent version-bump rule. Each case sets up a tiny throwaway repo,
# makes a commit, and asserts pass/fail with the right message.
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
SCRIPT="$ROOT/scripts/check-remote-agent-version-bump.sh"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

setup_repo() {
  local repo="$1"
  mkdir -p "$repo/packages/remote-agent/src" \
           "$repo/packages/remote-agent/bin" \
           "$repo/packages/borgee/cmd/borgee" \
           "$repo/packages/borgee/internal/grants/testdata"
  cat > "$repo/packages/remote-agent/package.json" <<'JSON'
{"name":"@codetreker/borgee-remote-agent","version":"0.1.2"}
JSON
  printf 'export const value = 1;\n' > "$repo/packages/remote-agent/src/index.ts"
  printf '#!/usr/bin/env node\n' > "$repo/packages/remote-agent/bin/borgee.js"
  printf 'package main\nfunc main(){}\n' > "$repo/packages/borgee/cmd/borgee/main.go"
  printf 'package grants\nfunc Do(){}\n' > "$repo/packages/borgee/internal/grants/grants.go"
  printf 'package grants\nfunc TestX(){}\n' > "$repo/packages/borgee/internal/grants/grants_test.go"
  printf 'fixture\n' > "$repo/packages/borgee/internal/grants/testdata/x.json"
  git -C "$repo" init -q
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name test
  git -C "$repo" add .
  git -C "$repo" commit -qm base
}

run_check() {
  local repo="$1" base="$2" head="$3"
  (cd "$repo" && BASE_SHA="$base" HEAD_SHA="$head" "$SCRIPT")
}

expect_fail_node_src_without_bump() {
  local repo="$TMP/fail-node-src"
  setup_repo "$repo"
  local base; base=$(git -C "$repo" rev-parse HEAD)
  printf 'export const value = 2;\n' > "$repo/packages/remote-agent/src/index.ts"
  git -C "$repo" add . && git -C "$repo" commit -qm change-src
  local head; head=$(git -C "$repo" rev-parse HEAD)
  local output
  if output=$(run_check "$repo" "$base" "$head" 2>&1); then
    echo "expected fail on src change without bump" >&2; return 1
  fi
  echo "$output" | grep -F 'version stayed at 0.1.2' >/dev/null
}

expect_fail_bin_without_bump() {
  local repo="$TMP/fail-bin"
  setup_repo "$repo"
  local base; base=$(git -C "$repo" rev-parse HEAD)
  printf '#!/usr/bin/env node\nconsole.log(1);\n' > "$repo/packages/remote-agent/bin/borgee.js"
  git -C "$repo" add . && git -C "$repo" commit -qm change-bin
  local head; head=$(git -C "$repo" rev-parse HEAD)
  ! run_check "$repo" "$base" "$head" >/dev/null 2>&1
}

expect_fail_go_src_without_bump() {
  local repo="$TMP/fail-go-src"
  setup_repo "$repo"
  local base; base=$(git -C "$repo" rev-parse HEAD)
  printf 'package grants\nfunc Do2(){}\n' > "$repo/packages/borgee/internal/grants/grants.go"
  git -C "$repo" add . && git -C "$repo" commit -qm change-go
  local head; head=$(git -C "$repo" rev-parse HEAD)
  ! run_check "$repo" "$base" "$head" >/dev/null 2>&1
}

expect_fail_package_json_non_version_without_bump() {
  local repo="$TMP/fail-pkg-meta"
  setup_repo "$repo"
  local base; base=$(git -C "$repo" rev-parse HEAD)
  cat > "$repo/packages/remote-agent/package.json" <<'JSON'
{"name":"@codetreker/borgee-remote-agent","version":"0.1.2","description":"x"}
JSON
  git -C "$repo" add . && git -C "$repo" commit -qm change-pkg-meta
  local head; head=$(git -C "$repo" rev-parse HEAD)
  ! run_check "$repo" "$base" "$head" >/dev/null 2>&1
}

expect_pass_with_bump() {
  local repo="$TMP/pass-bump"
  setup_repo "$repo"
  local base; base=$(git -C "$repo" rev-parse HEAD)
  printf 'export const value = 2;\n' > "$repo/packages/remote-agent/src/index.ts"
  cat > "$repo/packages/remote-agent/package.json" <<'JSON'
{"name":"@codetreker/borgee-remote-agent","version":"0.1.3"}
JSON
  git -C "$repo" add . && git -C "$repo" commit -qm bumped
  local head; head=$(git -C "$repo" rev-parse HEAD)
  run_check "$repo" "$base" "$head"
}

expect_pass_go_test_only_no_bump() {
  local repo="$TMP/pass-go-test"
  setup_repo "$repo"
  local base; base=$(git -C "$repo" rev-parse HEAD)
  printf 'package grants\nfunc TestY(){}\n' > "$repo/packages/borgee/internal/grants/grants_test.go"
  git -C "$repo" add . && git -C "$repo" commit -qm go-test-only
  local head; head=$(git -C "$repo" rev-parse HEAD)
  run_check "$repo" "$base" "$head"
}

expect_pass_go_testdata_only_no_bump() {
  local repo="$TMP/pass-testdata"
  setup_repo "$repo"
  local base; base=$(git -C "$repo" rev-parse HEAD)
  printf 'fixture v2\n' > "$repo/packages/borgee/internal/grants/testdata/x.json"
  git -C "$repo" add . && git -C "$repo" commit -qm testdata-only
  local head; head=$(git -C "$repo" rev-parse HEAD)
  run_check "$repo" "$base" "$head"
}

expect_pass_unrelated_change_no_bump() {
  local repo="$TMP/pass-unrelated"
  setup_repo "$repo"
  local base; base=$(git -C "$repo" rev-parse HEAD)
  mkdir -p "$repo/docs"
  printf '# note\n' > "$repo/docs/note.md"
  git -C "$repo" add . && git -C "$repo" commit -qm docs
  local head; head=$(git -C "$repo" rev-parse HEAD)
  run_check "$repo" "$base" "$head"
}

expect_pass_version_only_change() {
  local repo="$TMP/pass-version-only"
  setup_repo "$repo"
  local base; base=$(git -C "$repo" rev-parse HEAD)
  cat > "$repo/packages/remote-agent/package.json" <<'JSON'
{"name":"@codetreker/borgee-remote-agent","version":"0.1.3"}
JSON
  git -C "$repo" add . && git -C "$repo" commit -qm version-only
  local head; head=$(git -C "$repo" rev-parse HEAD)
  run_check "$repo" "$base" "$head"
}

expect_fail_node_src_without_bump
expect_fail_bin_without_bump
expect_fail_go_src_without_bump
expect_fail_package_json_non_version_without_bump
expect_pass_with_bump
expect_pass_go_test_only_no_bump
expect_pass_go_testdata_only_no_bump
expect_pass_unrelated_change_no_bump
expect_pass_version_only_change
echo "all remote-agent version-bump cases pass"
