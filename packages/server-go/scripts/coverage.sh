#!/bin/bash
# CI-SPLIT-RACE-COV: coverage.sh runs no-race deterministic coverage —
# matches CI's go-test-cov job (race lives in go-test-race separately).
# Race detector affects goroutine scheduling, which makes some defer/panic
# branches hit non-deterministically (e.g. ws/hub.go::StartHeartbeat
# 33.3% no-race vs 58.3% with-race), causing ±0.1% coverage variation.
set -e
export TMPDIR="${TMPDIR:-/tmp/go-test}"
mkdir -p "$TMPDIR"
cd "$(dirname "$0")/.."
COVERPROFILE="${COVERPROFILE:-coverage.out}" go run github.com/codetreker/go-cov/cmd/go-cov@v0.1.0
