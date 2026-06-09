#!/usr/bin/env bash
# Build the borgee-vm-base:latest dev-VM image with the REAL borgee remote-agent
# daemon installed from a locally-packed npm tarball (NOT the public registry).
#
# Steps: cross-compile the 4 platform binaries (scripts/build-borgee-binaries.sh)
# -> `pnpm pack` the @codetreker/borgee-remote-agent tarball straight into this
# build context (scripts/dev-vm/) under the STABLE name borgee-remote-agent.tgz
# (so the Dockerfile COPY line never churns on a version bump) -> docker build
# (the Dockerfile COPYs + `npm install -g` that tarball) -> remove the staged
# tarball so the tree is left pristine.
#
# Run this instead of a bare `docker build` / `docker compose build` for the
# dev-vm image: a bare build would fail at `COPY borgee-remote-agent.tgz`
# because the tarball is a generated artifact, not committed.
#
# Pack-an-artifact-then-let-the-build-consume-it. Wrap the docker build in a
# timeout when invoking, e.g.
#   timeout 600 bash scripts/dev-vm/build-image.sh
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
REPO_ROOT=$(cd "$SCRIPT_DIR/../.." && pwd -P)
STAGED_TGZ="$SCRIPT_DIR/borgee-remote-agent.tgz"
IMAGE="borgee-vm-base:latest"

# Always clean up the staged tarball, even on failure, so the tree stays pristine.
cleanup() { rm -f "$STAGED_TGZ"; }
trap cleanup EXIT

echo "[build-image] cross-compiling 4 platform binaries…"
bash "$REPO_ROOT/scripts/build-borgee-binaries.sh"

echo "[build-image] packing @codetreker/borgee-remote-agent tarball into build context…"
rm -f "$STAGED_TGZ"
( cd "$REPO_ROOT/packages/borgee" && pnpm pack --pack-destination "$SCRIPT_DIR" )
# pnpm pack emits codetreker-borgee-remote-agent-<version>.tgz; rename to the
# stable name the Dockerfile COPYs.
mv "$SCRIPT_DIR"/codetreker-borgee-remote-agent-*.tgz "$STAGED_TGZ"

# Fail loud if the staged tarball is missing before we hand off to docker.
if [ ! -f "$STAGED_TGZ" ]; then
  echo "[build-image] FAIL: expected staged tarball $STAGED_TGZ but it is missing" >&2
  exit 1
fi

echo "[build-image] docker build $IMAGE (context $SCRIPT_DIR)…"
docker build -t "$IMAGE" "$SCRIPT_DIR"

echo "[build-image] ok: built $IMAGE"
