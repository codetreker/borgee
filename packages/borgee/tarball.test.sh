#!/usr/bin/env bash
set -euo pipefail
# Tarball integration test for @codetreker/borgee-remote-agent (EV-3 + EV-4).
# Packs the package, asserts the tarball ships the dispatcher + a platform
# binary and NO Go source, then `npm i -g`s it and invokes the bin against a
# FAKE host-arch binary (echoes argv + a marker exit) to prove the dispatcher
# resolves + re-chmods (npm strips the exec bit) + forwards argv + the exit
# code. No real Go daemon / server / systemd needed.
# Run bounded: `timeout 120 bash packages/borgee/tarball.test.sh`.
#
# Clean BOTH the tmp dir AND the bin/ this script writes, so the tree is left
# pristine (bin/ is .gitignored so it never dirties git, but remove it anyway).
WORK=$(mktemp -d); trap 'rm -rf "$WORK" packages/borgee/bin' EXIT
# 1. fake binary at the host's platform key, then pack.
KEY="$(node -p "process.platform")-$(node -p "process.arch")"   # e.g. linux-x64
mkdir -p "packages/borgee/bin/platforms/$KEY"
cat > "packages/borgee/bin/platforms/$KEY/borgee" <<'BIN'
#!/usr/bin/env bash
echo "BORGEE-FAKE argv: $*"
[ "$1" = "install" ] && exit 0
exit 7
BIN
chmod 0755 "packages/borgee/bin/platforms/$KEY/borgee"
( cd packages/borgee && pnpm pack --pack-destination "$WORK" )
TGZ=$(ls "$WORK"/*.tgz)
# 2. assert tarball shape (EV-3): dispatcher + the host binary, NO .go.
tar tzf "$TGZ" | grep -q 'package/borgee-remote-agent.cjs'
tar tzf "$TGZ" | grep -q "package/bin/platforms/$KEY/borgee"
! tar tzf "$TGZ" | grep -q '\.go$'
# 3. install + run (EV-4): the dispatcher's chmod re-asserts the stripped exec bit.
npm install -g --prefix "$WORK/global" "$TGZ" >/dev/null 2>&1
OUT=$("$WORK/global/bin/borgee-remote-agent" install --server wss://x --token y --dirs /z)
echo "$OUT" | grep -q 'BORGEE-FAKE argv: install --server wss://x --token y --dirs /z'
"$WORK/global/bin/borgee-remote-agent" install --server wss://x --token y --dirs /z; test $? -eq 0
set +e; "$WORK/global/bin/borgee-remote-agent" ls; test $? -eq 7; set -e
echo "ok: tarball install+dispatch+exit-forwarding verified"
