// Package manifest fetches HB-1B-INSTALLER manifests and verifies ed25519
// signatures.
//
// It calls the HB-1 server endpoint `/api/v1/plugin-manifest`, whose
// server-side PluginManifestEntries const slice in hb_1_plugin_manifest.go is
// the source of truth, then verifies the ed25519 detached signature.
//
// The seven-reason dictionary must stay byte-identical with server-side
// HB1AllReasons. Changes must update server hb_1_plugin_manifest.go,
// fetcher.go, and installer/cmd/* together. manifest_test.go contains the
// source-text drift check.
//
// Signature verification invariant: ed25519.Verify must run. Bad signatures return
// ReasonManifestSignatureInvalid instead of being skipped.
package manifest

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// The 7-reason dictionary must match server-side HB1AllReasons byte-for-byte
// (hb_1_plugin_manifest.go). REG-HB1B-002 source-text tests cover these seven
// literals and the server-side 7 literals.
const (
	ReasonOK                       = "ok"
	ReasonManifestSignatureInvalid = "manifest_signature_invalid"
	ReasonBinarySHA256Mismatch     = "binary_sha256_mismatch"
	ReasonBinaryGPGInvalid         = "binary_gpg_invalid"
	ReasonManifestFetchFailed      = "manifest_fetch_failed"
	ReasonDiskWriteFailed          = "disk_write_failed"
	ReasonUnknownPlugin            = "unknown_plugin"
)

// AllReasons lists the seven values used by the source-text drift check.
var AllReasons = []string{
	ReasonOK,
	ReasonManifestSignatureInvalid,
	ReasonBinarySHA256Mismatch,
	ReasonBinaryGPGInvalid,
	ReasonManifestFetchFailed,
	ReasonDiskWriteFailed,
	ReasonUnknownPlugin,
}

// PluginEntry mirrors the server-side PluginManifestEntry shape byte-for-byte
// (hb_1_plugin_manifest.go §3.1 content-lock §1).
type PluginEntry struct {
	ID        string   `json:"id"`
	Version   string   `json:"version"`
	BinaryURL string   `json:"binary_url"`
	SHA256    string   `json:"sha256"`
	Signature string   `json:"signature"`
	Platforms []string `json:"platforms"`
}

// Envelope mirrors the server-side PluginManifestResponse shape byte-for-byte:
// {"entries":[...], "signed_at": <unix-ms>, "signature": "<base64>"}.
type Envelope struct {
	Entries   []PluginEntry `json:"entries"`
	SignedAt  int64         `json:"signed_at"`
	Signature string        `json:"signature"`
}

// FetchError carries a seven-reason dictionary value and an underlying error.
type FetchError struct {
	Reason string
	Err    error
}

func (e *FetchError) Error() string {
	if e.Err == nil {
		return e.Reason
	}
	return fmt.Sprintf("%s: %v", e.Reason, e.Err)
}

func (e *FetchError) Unwrap() error { return e.Err }

// Fetch performs HTTP GET against the HB-1 server endpoint and decodes the
// envelope. It returns ReasonManifestFetchFailed on transport or decode errors.
func Fetch(ctx context.Context, client *http.Client, endpoint, bearerToken string) (*Envelope, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &FetchError{Reason: ReasonManifestFetchFailed, Err: err}
	}
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &FetchError{Reason: ReasonManifestFetchFailed, Err: err}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, &FetchError{
			Reason: ReasonManifestFetchFailed,
			Err:    fmt.Errorf("HTTP %d", resp.StatusCode),
		}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, &FetchError{Reason: ReasonManifestFetchFailed, Err: err}
	}
	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, &FetchError{Reason: ReasonManifestFetchFailed, Err: err}
	}
	return &env, nil
}

// CanonicalSignedBytes returns the byte sequence that the server signs:
// JSON of {entries, signed_at}, matching server canonicalization byte-for-byte
// (entries sorted by ID and stable json.Marshal output). Keep this in sync with
// server hb_1_plugin_manifest.go SignManifestPayload.
func CanonicalSignedBytes(env *Envelope) ([]byte, error) {
	type signedShape struct {
		Entries  []PluginEntry `json:"entries"`
		SignedAt int64         `json:"signed_at"`
	}
	return json.Marshal(signedShape{Entries: env.Entries, SignedAt: env.SignedAt})
}

// Verify checks the ed25519 detached signature against the public key. Bad
// signatures return FetchError{Reason: ReasonManifestSignatureInvalid}.
func Verify(env *Envelope, pubKey ed25519.PublicKey) error {
	if env == nil {
		return &FetchError{Reason: ReasonManifestSignatureInvalid, Err: errors.New("nil envelope")}
	}
	if len(pubKey) != ed25519.PublicKeySize {
		return &FetchError{Reason: ReasonManifestSignatureInvalid, Err: errors.New("bad pub key size")}
	}
	if env.Signature == "" {
		return &FetchError{Reason: ReasonManifestSignatureInvalid, Err: errors.New("empty signature")}
	}
	sig, err := base64.StdEncoding.DecodeString(env.Signature)
	if err != nil {
		return &FetchError{Reason: ReasonManifestSignatureInvalid, Err: err}
	}
	signed, err := CanonicalSignedBytes(env)
	if err != nil {
		return &FetchError{Reason: ReasonManifestSignatureInvalid, Err: err}
	}
	if !ed25519.Verify(pubKey, signed, sig) {
		return &FetchError{Reason: ReasonManifestSignatureInvalid, Err: errors.New("ed25519 verify failed")}
	}
	return nil
}
