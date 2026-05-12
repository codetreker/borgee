// fetcher_test.go verifies REG-HB1B-001 and REG-HB1B-002.
package manifest

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// signEnvelope signs test envelopes using the same payload bytes as server-side
// SignManifestPayload.
func signEnvelope(t *testing.T, env *Envelope, priv ed25519.PrivateKey) {
	t.Helper()
	signed, err := CanonicalSignedBytes(env)
	if err != nil {
		t.Fatalf("CanonicalSignedBytes: %v", err)
	}
	sig := ed25519.Sign(priv, signed)
	env.Signature = base64.StdEncoding.EncodeToString(sig)
}

func TestHB1B_FetchManifest_Happy(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	env := &Envelope{
		Entries: []PluginEntry{
			{ID: "borgee-helper", Version: "0.1.0", BinaryURL: "https://example.test/bh.tgz",
				SHA256: "abc", Platforms: []string{"linux", "darwin"}},
		},
		SignedAt: 1700000000,
	}
	signEnvelope(t, env, priv)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(env)
	}))
	defer srv.Close()

	got, err := Fetch(context.Background(), nil, srv.URL+"/api/v1/plugin-manifest", "tok")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if err := Verify(got, pub); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if len(got.Entries) != 1 || got.Entries[0].ID != "borgee-helper" {
		t.Errorf("unexpected entries: %+v", got.Entries)
	}
}

func TestHB1B_FetchManifest_NetworkErr_FetchFailed(t *testing.T) {
	_, err := Fetch(context.Background(), &http.Client{}, "http://127.0.0.1:1/nope", "")
	if err == nil {
		t.Fatal("expected error")
	}
	fe, ok := err.(*FetchError)
	if !ok || fe.Reason != ReasonManifestFetchFailed {
		t.Errorf("expected ReasonManifestFetchFailed, got %v", err)
	}
}

func TestHB1B_VerifyManifest_BadSig_Reject(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	env := &Envelope{Entries: []PluginEntry{{ID: "x"}}, SignedAt: 123}
	signEnvelope(t, env, priv)
	// Change SignedAt after signing so the signature no longer matches. This guards
	// against silently skipping verification.
	env.SignedAt = 999
	if err := Verify(env, pub); err == nil {
		t.Fatal("expected verify failure")
	} else {
		fe, ok := err.(*FetchError)
		if !ok || fe.Reason != ReasonManifestSignatureInvalid {
			t.Errorf("expected ReasonManifestSignatureInvalid, got %v", err)
		}
	}
}

func TestHB1B_VerifyManifest_EmptySig_Reject(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	env := &Envelope{Entries: []PluginEntry{{ID: "x"}}, SignedAt: 1, Signature: ""}
	if err := Verify(env, pub); err == nil {
		t.Fatal("expected verify failure for empty sig")
	}
}

func TestHB1B_Reasons7DictByteIdentical(t *testing.T) {
	// Regression check: these seven literals must exactly match spec §3.2
	// and server-side HB1AllReasons.
	want := []string{
		"ok",
		"manifest_signature_invalid",
		"binary_sha256_mismatch",
		"binary_gpg_invalid",
		"manifest_fetch_failed",
		"disk_write_failed",
		"unknown_plugin",
	}
	if len(AllReasons) != len(want) {
		t.Fatalf("AllReasons length mismatch: %d vs %d", len(AllReasons), len(want))
	}
	for i, w := range want {
		if AllReasons[i] != w {
			t.Errorf("AllReasons[%d] = %q, want %q", i, AllReasons[i], w)
		}
	}
}

func TestHB1B_FetchManifest_Non200_FetchFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	_, err := Fetch(context.Background(), nil, srv.URL+"/api/v1/plugin-manifest", "")
	if err == nil {
		t.Fatal("expected error on 503")
	}
	fe, ok := err.(*FetchError)
	if !ok || fe.Reason != ReasonManifestFetchFailed {
		t.Errorf("expected ReasonManifestFetchFailed, got %v", err)
	}
}

func TestHB1B_FetchManifest_BadJSON_FetchFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()
	_, err := Fetch(context.Background(), nil, srv.URL+"/api/v1/plugin-manifest", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), ReasonManifestFetchFailed) {
		t.Errorf("expected manifest_fetch_failed reason in error: %v", err)
	}
}
