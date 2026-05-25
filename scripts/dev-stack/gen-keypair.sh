#!/usr/bin/env bash
# Generate an ed25519 keypair for local-e2e manifest signing.
# Writes two lines to stdout:
#   BORGEE_MANIFEST_SIGNING_KEY=<base64 32-byte seed>
#   BORGEE_MANIFEST_SIGNING_PUBKEY=<base64 32-byte public key>
#
# Pipe both into .env. The server-go process consumes the SEED line
# (BORGEE_MANIFEST_SIGNING_KEY, see internal/api/manifest_signing.go).
# The PUBKEY line is for the helper-vm side — drop it into a systemd
# drop-in for the borgee.service so the daemon will trust manifests
# signed by this dev-stack server.
set -euo pipefail

if ! command -v go >/dev/null 2>&1; then
  echo "error: go toolchain not found on PATH (needed for ed25519 generation)" >&2
  exit 1
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT
cat >"$tmpdir/main.go" <<'GO'
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func main() {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	seed := priv.Seed()
	fmt.Printf("BORGEE_MANIFEST_SIGNING_KEY=%s\n", base64.StdEncoding.EncodeToString(seed))
	fmt.Printf("BORGEE_MANIFEST_SIGNING_PUBKEY=%s\n", base64.StdEncoding.EncodeToString(pub))
}
GO
go run "$tmpdir/main.go"
