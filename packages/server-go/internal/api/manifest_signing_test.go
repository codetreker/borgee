// Package api_test — manifest_signing_test.go: HB-1 #997 ed25519 per-entry
// signing helper unit tests.
//
// Pins:
//
//	TS-1 TestManifestSigning_SignEntry_NonEmptyBase64
//	TS-2 TestManifestSigning_VerifyEntry_RoundtripOK
//	TS-3 TestManifestSigning_VerifyEntry_TamperFails
//	TS-4 TestManifestSigning_LoadSigningKey_EnvUnsetReturnsNilNoPanic
//	TS-5 TestManifestSigning_LoadSigningKey_EnvMalformedReturnsErr
//
// Supplement:
//
//	TestManifestSigning_LoadManifestEntries_EnvJSONOverride
//	TestManifestSigning_LoadManifestEntries_FallbackToDefault
//	TestManifestSigning_EntryCanonicalBytes_Stable
package api_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"borgee-server/internal/api"
)

func testEntry() api.PluginManifestEntry {
	return api.PluginManifestEntry{
		ID:        "openclaw",
		Version:   "1.0.0",
		BinaryURL: "https://example.com/openclaw-1.0.0-linux-x64",
		SHA256:    "deadbeef0000000000000000000000000000000000000000000000000000beef",
		Signature: "",
		Platforms: []string{"linux-x64"},
	}
}

func genKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey: %v", err)
	}
	return pub, priv
}

// TS-1 — SignEntry with a generated key produces non-empty base64 signature.
func TestManifestSigning_SignEntry_NonEmptyBase64(t *testing.T) {
	t.Parallel()
	_, priv := genKey(t)
	sig := api.SignEntry(priv, testEntry())
	if sig == "" {
		t.Fatal("SignEntry returned empty signature with valid key")
	}
	raw, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		t.Fatalf("signature not valid base64: %v", err)
	}
	if len(raw) != ed25519.SignatureSize {
		t.Errorf("signature byte length: got %d, want %d", len(raw), ed25519.SignatureSize)
	}
}

// TS-2 — VerifyEntry returns true for a freshly-signed entry.
func TestManifestSigning_VerifyEntry_RoundtripOK(t *testing.T) {
	t.Parallel()
	pub, priv := genKey(t)
	e := testEntry()
	sig := api.SignEntry(priv, e)
	if !api.VerifyEntry(pub, e, sig) {
		t.Fatal("VerifyEntry(roundtrip) returned false; sign/verify byte mismatch")
	}
}

// TS-3 — Tamper any field → VerifyEntry returns false.
func TestManifestSigning_VerifyEntry_TamperFails(t *testing.T) {
	t.Parallel()
	pub, priv := genKey(t)
	orig := testEntry()
	sig := api.SignEntry(priv, orig)

	cases := []struct {
		name  string
		mutFn func(*api.PluginManifestEntry)
	}{
		{"sha256", func(e *api.PluginManifestEntry) { e.SHA256 = strings.Repeat("a", 64) }},
		{"binary_url", func(e *api.PluginManifestEntry) { e.BinaryURL = "https://evil.example.com/x" }},
		{"version", func(e *api.PluginManifestEntry) { e.Version = "9.9.9" }},
		{"id", func(e *api.PluginManifestEntry) { e.ID = "evilclaw" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tampered := orig
			tc.mutFn(&tampered)
			if api.VerifyEntry(pub, tampered, sig) {
				t.Fatalf("VerifyEntry returned true after tampering %s — security regression", tc.name)
			}
		})
	}
}

// TS-4 — env unset → LoadSigningKey returns (nil, nil), no panic.
func TestManifestSigning_LoadSigningKey_EnvUnsetReturnsNilNoPanic(t *testing.T) {
	// 不并行 — 改 env.
	t.Setenv(api.EnvManifestSigningKey, "")
	key, err := api.LoadSigningKey(slog.Default())
	if err != nil {
		t.Fatalf("env unset must return nil error, got %v", err)
	}
	if key != nil {
		t.Errorf("env unset must return nil key, got len=%d", len(key))
	}
}

// TS-5 — env set but malformed → LoadSigningKey returns nil key + error.
func TestManifestSigning_LoadSigningKey_EnvMalformedReturnsErr(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{"not_base64", "@@@not-base64!!!"},
		{"wrong_length_short", base64.StdEncoding.EncodeToString([]byte{0x01, 0x02})},
		{"wrong_length_long", base64.StdEncoding.EncodeToString(make([]byte, 64))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(api.EnvManifestSigningKey, tc.value)
			key, err := api.LoadSigningKey(slog.Default())
			if err == nil {
				t.Fatalf("malformed env (%s) must return error", tc.name)
			}
			if key != nil {
				t.Errorf("malformed env (%s) must return nil key, got non-nil", tc.name)
			}
		})
	}
}

// TS-5b — env set + well-formed → LoadSigningKey returns key, no error.
func TestManifestSigning_LoadSigningKey_EnvValidReturnsKey(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i)
	}
	t.Setenv(api.EnvManifestSigningKey, base64.StdEncoding.EncodeToString(seed))
	key, err := api.LoadSigningKey(slog.Default())
	if err != nil {
		t.Fatalf("valid env must return nil error, got %v", err)
	}
	if key == nil {
		t.Fatal("valid env must return non-nil key")
	}
	// Round-trip: sign + verify with derived pub key.
	pub := key.Public().(ed25519.PublicKey)
	e := testEntry()
	sig := api.SignEntry(key, e)
	if !api.VerifyEntry(pub, e, sig) {
		t.Fatal("loaded key sign/verify roundtrip failed")
	}
}

// Supplement — env JSON override entries.
func TestManifestSigning_LoadManifestEntries_EnvJSONOverride(t *testing.T) {
	override := `[{"id":"x","version":"2.0.0","binary_url":"https://e.example/x","sha256":"abc","signature":"","platforms":["linux-x64"]}]`
	t.Setenv(api.EnvManifestEntriesJSON, override)
	t.Setenv(api.EnvManifestEntriesFile, "")
	entries := api.LoadManifestEntries(slog.Default())
	if len(entries) != 1 || entries[0].ID != "x" || entries[0].Version != "2.0.0" {
		t.Fatalf("env JSON override not applied: %+v", entries)
	}
}

// Supplement — env file override entries.
func TestManifestSigning_LoadManifestEntries_EnvFileOverride(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "entries.json")
	body := `[{"id":"y","version":"3.0.0","binary_url":"https://e.example/y","sha256":"xyz","signature":"","platforms":["darwin-arm64"]}]`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv(api.EnvManifestEntriesJSON, "")
	t.Setenv(api.EnvManifestEntriesFile, p)
	entries := api.LoadManifestEntries(slog.Default())
	if len(entries) != 1 || entries[0].ID != "y" {
		t.Fatalf("env file override not applied: %+v", entries)
	}
}

// Supplement — malformed env falls back to default (fail-soft).
func TestManifestSigning_LoadManifestEntries_MalformedEnvFallback(t *testing.T) {
	t.Setenv(api.EnvManifestEntriesJSON, "not-json-{{{")
	t.Setenv(api.EnvManifestEntriesFile, "")
	entries := api.LoadManifestEntries(slog.Default())
	if len(entries) == 0 {
		t.Fatal("malformed env JSON must fall back to default (non-empty)")
	}
	// Must equal default slice byte-for-byte.
	if entries[0].ID != api.PluginManifestEntries[0].ID {
		t.Errorf("fallback default mismatch: got %q, want %q", entries[0].ID, api.PluginManifestEntries[0].ID)
	}
}

// Supplement — env unset returns default slice.
func TestManifestSigning_LoadManifestEntries_FallbackToDefault(t *testing.T) {
	t.Setenv(api.EnvManifestEntriesJSON, "")
	t.Setenv(api.EnvManifestEntriesFile, "")
	entries := api.LoadManifestEntries(slog.Default())
	if len(entries) == 0 {
		t.Fatal("default fallback must return non-empty PluginManifestEntries")
	}
}

// Supplement — EntryCanonicalBytes is stable + uses documented separator.
func TestManifestSigning_EntryCanonicalBytes_Stable(t *testing.T) {
	t.Parallel()
	e := testEntry()
	got := string(api.EntryCanonicalBytes(e))
	want := "openclaw|1.0.0|https://example.com/openclaw-1.0.0-linux-x64|deadbeef0000000000000000000000000000000000000000000000000000beef"
	if got != want {
		t.Errorf("canonical form drift — install-butler verify would break:\n  got:  %q\n  want: %q", got, want)
	}
}
