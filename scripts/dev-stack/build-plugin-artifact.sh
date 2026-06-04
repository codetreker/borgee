#!/usr/bin/env bash
# Build the REAL @codetreker/borgee-openclaw-plugin tarball and stage it
# under scripts/dev-stack/artifacts/openclaw-plugin/<platform> so the
# server-go dev-stack can serve the bytes that openclaw itself accepts
# via `openclaw plugins install`.
#
# Run before `docker compose up -d`. Output: prints the sha256 of the
# tarball and the suggested .env override line.
#
# Why bytes-not-checked-in: the .tgz is generated from packages/plugins/
# openclaw/src + dist; pinning bytes to git would invalidate the moment
# anyone edits the plugin. The dir is committed; the bytes are ignored.
set -euo pipefail

repo_root() {
  git -C "$(dirname "${BASH_SOURCE[0]}")" rev-parse --show-toplevel
}

ROOT="$(repo_root)"
PLUGIN_DIR="${ROOT}/packages/plugins/openclaw"
ARTIFACTS_DIR="${ROOT}/scripts/dev-stack/artifacts/openclaw-plugin"

echo "[build-plugin-artifact] root=${ROOT}"
echo "[build-plugin-artifact] plugin=${PLUGIN_DIR}"
echo "[build-plugin-artifact] artifacts=${ARTIFACTS_DIR}"

cd "${PLUGIN_DIR}"

if [[ ! -d node_modules ]]; then
  echo "[build-plugin-artifact] installing deps (pnpm install --filter)…"
  ( cd "${ROOT}" && pnpm install --filter @codetreker/borgee-openclaw-plugin --frozen-lockfile=false )
fi

echo "[build-plugin-artifact] tsc build…"
pnpm build

echo "[build-plugin-artifact] npm pack…"
# `npm pack --json` emits a JSON array; the .tgz lands in cwd with the
# canonical name `codetreker-borgee-openclaw-plugin-<version>.tgz`.
PACK_NAME="$(npm pack --silent)"
TARBALL="${PLUGIN_DIR}/${PACK_NAME}"
if [[ ! -f "${TARBALL}" ]]; then
  echo "[build-plugin-artifact] ERROR: expected ${TARBALL} but it is missing"
  exit 1
fi

# Stage as both linux-x64 and darwin-arm64 (the plugin is platform-
# independent JS — same tarball serves both). install-butler writes
# whichever the helper requested; openclaw's plugins-install path-watcher
# unit (dev-vm) then renames to .tgz and feeds it to `openclaw plugins
# install --force`.
mkdir -p "${ARTIFACTS_DIR}"
cp -f "${TARBALL}" "${ARTIFACTS_DIR}/linux-x64"
cp -f "${TARBALL}" "${ARTIFACTS_DIR}/darwin-arm64"

SHA="$(sha256sum "${TARBALL}" | awk '{print $1}')"
SIZE="$(wc -c < "${TARBALL}")"

echo
echo "[build-plugin-artifact] OK"
echo "  tarball         : ${TARBALL}"
echo "  size (bytes)    : ${SIZE}"
echo "  sha256          : ${SHA}"
echo
echo "Put this line in scripts/dev-stack/.env (overwriting the previous"
echo "BORGEE_DEV_MANIFEST_SHA256_OVERRIDE line):"
echo
echo "BORGEE_DEV_MANIFEST_SHA256_OVERRIDE={\"openclaw-plugin\":\"${SHA}\"}"
echo
