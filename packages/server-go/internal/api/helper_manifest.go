// Package api — helper_manifest.go: server-side signer + per-platform
// cache around helpermanifest.BuildLinux. The canonical body lives in
// internal/helpermanifest (used by both store and api). This file owns
// the ed25519 signing layer, env-key loading, and the request-time
// provider that serializeHelperJobLease injects into the leased-job
// payload.
//
// Why this exists (PR-4 amend, issue #1033):
//
//	Helper-side jobpolicy.Evaluate (packages/borgee/internal/jobpolicy)
//	gates every manifest-required leased job against a signed manifest
//	body. PR-4 wired the helper executors but the server emitted only
//	manifest_digest + manifest_binding_json (no body). Daemon's
//	verifyManifestAuthority requires ManifestJSON to be non-empty + a
//	valid ed25519 signature under a configured trust root, so 5/8
//	manifest-required job types (state.write, openclaw.configure_agent,
//	borgee_plugin.configure_connection, openclaw.install_from_manifest,
//	service.lifecycle) were rejected with manifest_invalid.
//
//	This file adds:
//
//	- SignCanonicalHelperManifest — ed25519-signs the canonical bytes
//	  produced by helpermanifest.CanonicalBytes. Same canonical-form
//	  contract the daemon recomputes for verification.
//
//	- HelperManifestProvider — per-platform cache of (signedBytes,
//	  digest) so the hot path (every helper-job lease) pays signing
//	  cost once per platform per signing-key generation.
//
//	- LoadHelperManifestSigningKey — reads the same env var the plugin
//	  manifest signer uses (BORGEE_MANIFEST_SIGNING_KEY). Single key
//	  covers both manifests so the daemon's BORGEE_MANIFEST_SIGNING_PUBKEY
//	  trust root is shared.
//
// Trust-root distribution: the daemon reads
// BORGEE_MANIFEST_SIGNING_PUBKEY at startup (already wired by the
// installplugin executor in daemon.go); PR-4 amend extends that wiring
// to populate jobpolicy.EvaluationInput.TrustRoots so every Evaluate call
// verifies signatures against the same key the server signed with.
// Rotation (key v2 with grace window) is a follow-up — issue to be
// opened post-merge.
package api

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"borgee-server/internal/helpermanifest"
)

// SignCanonicalHelperManifest signs the manifest with the given ed25519
// key + returns the signed wire bytes plus the canonical digest. When
// key is nil (dev fall-soft path matching LoadSigningKey), Signature
// stays empty — the daemon's jobpolicy.verifyManifestAuthority rejects
// such a body with ReasonManifestInvalid, which is the safe production
// default; operators must set BORGEE_MANIFEST_SIGNING_KEY to lift dev
// fall-soft.
func SignCanonicalHelperManifest(m helpermanifest.PolicyManifest, key ed25519.PrivateKey) (signedBytes []byte, manifestDigest string, err error) {
	canonical, err := helpermanifest.CanonicalBytes(m)
	if err != nil {
		return nil, "", fmt.Errorf("helper manifest canonical bytes: %w", err)
	}
	digest, err := helpermanifest.Digest(m)
	if err != nil {
		return nil, "", fmt.Errorf("helper manifest digest: %w", err)
	}
	if key != nil {
		m.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, canonical))
	}
	out, err := json.Marshal(m)
	if err != nil {
		return nil, "", fmt.Errorf("helper manifest marshal signed: %w", err)
	}
	return out, digest, nil
}

// HelperManifestProvider caches the signed manifest body + digest per
// platform. Built once at server boot (server.go wiring) and shared by
// every helper-job lease serialization path. Signing happens lazily on
// first lookup so the boot order can stay parallel to other handlers.
//
// Thread-safe under sync.Mutex; cache key is platform.
type HelperManifestProvider struct {
	// SigningKey may be nil — dev fall-soft. When nil, the signed body
	// returned has empty Signature and the daemon will reject it; in dev
	// no helper actually runs jobpolicy.Evaluate so this is acceptable.
	SigningKey ed25519.PrivateKey

	mu    sync.Mutex
	cache map[string]helperManifestCacheEntry
}

type helperManifestCacheEntry struct {
	SignedBytes []byte
	Digest      string
}

// NewHelperManifestProvider constructs a provider bound to the given
// signing key. Nil key → unsigned manifests (dev fall-soft).
func NewHelperManifestProvider(key ed25519.PrivateKey) *HelperManifestProvider {
	return &HelperManifestProvider{SigningKey: key, cache: map[string]helperManifestCacheEntry{}}
}

// SignedManifestForPlatform returns the cached signed manifest bytes +
// canonical digest for the given platform. Empty platform → linux for
// backward compat with pre-PR-4-final-amend callers that did not pass
// a platform; production WS push wires the daemon's X-Helper-Platform
// upgrade header into this function so the platform is always set.
// Unknown platforms return an error — the daemon's WS upgrade handler
// already gates on ParsePlatform so we should never reach this branch
// with garbage.
func (p *HelperManifestProvider) SignedManifestForPlatform(platform string) ([]byte, string, error) {
	if p == nil {
		return nil, "", fmt.Errorf("helper manifest provider not configured")
	}
	token := strings.TrimSpace(platform)
	if token == "" {
		token = string(helpermanifest.PlatformLinux)
	}
	parsed, ok := helpermanifest.ParsePlatform(token)
	if !ok {
		return nil, "", fmt.Errorf("helper manifest: unsupported platform %q", token)
	}
	key := string(parsed)
	p.mu.Lock()
	defer p.mu.Unlock()
	if entry, ok := p.cache[key]; ok {
		return entry.SignedBytes, entry.Digest, nil
	}
	manifest, err := helpermanifest.CanonicalManifest(parsed)
	if err != nil {
		return nil, "", err
	}
	signed, digest, err := SignCanonicalHelperManifest(manifest, p.SigningKey)
	if err != nil {
		return nil, "", err
	}
	p.cache[key] = helperManifestCacheEntry{SignedBytes: signed, Digest: digest}
	return signed, digest, nil
}
