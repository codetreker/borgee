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
go test -timeout=120s -coverprofile=coverage.out -coverpkg=borgee-server/internal/api,borgee-server/internal/auth,borgee-server/internal/config,borgee-server/internal/store,borgee-server/internal/ws,borgee-server/internal/server ./...
go tool cover -func=coverage.out | tail -1
