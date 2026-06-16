// host_manifest_race_test.go — #1117 regression: canonicalJSON must be PURE.
//
// Background (#1117): canonicalJSON did `sort.Strings(payload.Plugins[i].Platforms)`
// in place. handleGet builds `signed[i] = e` (struct copy → copies the Platforms
// SLICE HEADER, not the backing array), so every concurrently-signed payload
// aliases the same backing array originally held by LoadManifestEntries' built-in
// default (PluginManifestEntries, a package-level var shared across requests).
// Two concurrent GET /api/v1/plugin-manifest therefore sort.Strings the SAME
// backing array → DATA RACE (intermittently flaking go-test-race).
//
// This file pins:
//   - TestCanonicalJSON_NoMutateSharedBacking_Race: deterministic concurrent
//     -race regression. Constructs entries whose Platforms slices ALIAS one
//     shared backing array (mirroring signed[i] = e aliasing), then calls
//     canonicalJSON from many parallel goroutines. Under -race this reports a
//     DATA RACE on the in-place code (RED) and is clean after the fix (GREEN).
//   - TestCanonicalJSON_ByteInvariance: golden byte-invariance — unsorted
//     Platforms input still marshals to the platforms-sorted canonical bytes,
//     proving the defensive-copy fix did NOT change the signed bytes
//     (ed25519 signature / install-butler verify contract preserved).
//   - TestCanonicalJSON_DoesNotMutateInput: after canonicalJSON, the caller's
//     own Platforms slice is left untouched (no shared-backing-array write).

package api

import (
	"sync"
	"testing"
)

// TestCanonicalJSON_NoMutateSharedBacking_Race is the #1117 red→green probe.
//
// It builds N entries that ALL share ONE Platforms backing array — exactly the
// aliasing that handleGet's `signed[i] = e` produces over LoadManifestEntries'
// shared default. It then calls canonicalJSON from many goroutines in parallel.
// On the in-place `sort.Strings` code this trips `WARNING: DATA RACE` at
// host_manifest.go; after the defensive-copy fix it is race-clean.
func TestCanonicalJSON_NoMutateSharedBacking_Race(t *testing.T) {
	t.Parallel()

	// One backing array, deliberately UNSORTED, shared across every entry —
	// this is the shared-state condition #1117 exposes.
	shared := []string{"linux-x64", "darwin-x64", "darwin-arm64"}

	// Build several payloads. Each payload's entry's Platforms slice header
	// aliases the SAME `shared` backing array (struct/slice-header copy), so
	// concurrent in-place sort.Strings calls write the same memory.
	const payloads = 4
	ps := make([]PluginManifestPayload, payloads)
	for i := range ps {
		ps[i] = PluginManifestPayload{
			ManifestVersion: 1,
			IssuedAt:        1700000000000,
			ExpiresAt:       1700086400000,
			Plugins: []PluginManifestEntry{
				{
					ID:        "openclaw",
					Version:   "1.0.0",
					BinaryURL: "https://cdn.borgee.io/plugins/openclaw-1.0.0-linux-x64",
					SHA256:    "0000000000000000000000000000000000000000000000000000000000000000",
					Platforms: shared, // aliases the shared backing array
				},
			},
		}
	}

	// 50 iterations × (payloads) goroutines reliably trips the race detector
	// on the unfixed in-place code (a spec reviewer proved 50×2 trips it).
	const iterations = 50
	var wg sync.WaitGroup
	for iter := 0; iter < iterations; iter++ {
		for i := range ps {
			wg.Add(1)
			go func(p PluginManifestPayload) {
				defer wg.Done()
				if _, err := canonicalJSON(p); err != nil {
					t.Errorf("canonicalJSON: %v", err)
				}
			}(ps[i])
		}
	}
	wg.Wait()
}

// TestCanonicalJSON_ByteInvariance pins that the canonical (signed) bytes are
// byte-identical to the platforms-sorted form regardless of input order. This
// is the contract install-butler (HB-1b Rust verifier) recomputes — the
// defensive-copy fix must not change a single output byte.
func TestCanonicalJSON_ByteInvariance(t *testing.T) {
	t.Parallel()

	// UNSORTED platforms on input.
	payload := PluginManifestPayload{
		ManifestVersion: 1,
		IssuedAt:        1700000000000,
		ExpiresAt:       1700086400000,
		Plugins: []PluginManifestEntry{
			{
				ID:        "openclaw",
				Version:   "1.0.0",
				BinaryURL: "https://cdn.borgee.io/plugins/openclaw-1.0.0-linux-x64",
				SHA256:    "0000000000000000000000000000000000000000000000000000000000000000",
				Signature: "",
				Platforms: []string{"linux-x64", "darwin-x64", "darwin-arm64"},
			},
		},
	}

	got, err := canonicalJSON(payload)
	if err != nil {
		t.Fatalf("canonicalJSON: %v", err)
	}

	// Golden canonical bytes: struct fields in declared order, platforms
	// sorted ASCENDING (darwin-arm64 < darwin-x64 < linux-x64). Class is
	// omitempty (absent). This is byte-identical to the pre-fix output.
	const want = `{"manifest_version":1,"issued_at":1700000000000,"expires_at":1700086400000,"signature":"","plugins":[{"id":"openclaw","version":"1.0.0","binary_url":"https://cdn.borgee.io/plugins/openclaw-1.0.0-linux-x64","sha256":"0000000000000000000000000000000000000000000000000000000000000000","signature":"","platforms":["darwin-arm64","darwin-x64","linux-x64"]}]}`

	if string(got) != want {
		t.Fatalf("canonical bytes changed — signature contract broken!\n got: %s\nwant: %s", got, want)
	}
}

// TestCanonicalJSON_DoesNotMutateInput verifies canonicalJSON leaves the
// caller's Platforms slice untouched (no write to the shared backing array).
// On the old in-place code this slice would come back sorted; with the
// defensive copy it stays in its original (unsorted) order.
func TestCanonicalJSON_DoesNotMutateInput(t *testing.T) {
	t.Parallel()

	original := []string{"linux-x64", "darwin-x64", "darwin-arm64"}
	payload := PluginManifestPayload{
		ManifestVersion: 1,
		Plugins: []PluginManifestEntry{
			{ID: "openclaw", Version: "1.0.0", Platforms: original},
		},
	}

	if _, err := canonicalJSON(payload); err != nil {
		t.Fatalf("canonicalJSON: %v", err)
	}

	want := []string{"linux-x64", "darwin-x64", "darwin-arm64"}
	for i := range want {
		if original[i] != want[i] {
			t.Fatalf("canonicalJSON mutated caller's Platforms backing array: got %v, want %v",
				original, want)
		}
	}
}
