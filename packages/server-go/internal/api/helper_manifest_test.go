package api

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"borgee-server/internal/helpermanifest"
)

// TestBuildCanonicalHelperManifest_Linux locks the shape of the v1
// Linux canonical body — every PathID / ServiceID / ArtifactID that
// store/helper_job_queries.go's binding switch references MUST appear
// here, otherwise the daemon's jobpolicy.Evaluate will reject the
// leased job with ReasonPathDenied / ServiceDenied / ArtifactInvalid.
func TestBuildCanonicalHelperManifest_Linux(t *testing.T) {
	m := helpermanifest.BuildLinux()
	if m.ManifestVersion != 1 {
		t.Fatalf("manifest version = %d, want 1", m.ManifestVersion)
	}
	if m.IssuedAt.IsZero() || m.ExpiresAt.IsZero() {
		t.Fatalf("issued_at/expires_at must be non-zero")
	}
	if !m.ExpiresAt.After(m.IssuedAt) {
		t.Fatalf("expires_at %v must be after issued_at %v", m.ExpiresAt, m.IssuedAt)
	}

	wantPathIDs := []string{
		helpermanifest.PathIDOpenClawInstall,
		helpermanifest.PathIDOpenClawAgentConfig,
		helpermanifest.PathIDBorgeePluginConfig,
		helpermanifest.PathIDBorgeeStateConfig,
		helpermanifest.PathIDHelperState,
		helpermanifest.PathIDHelperRuntime,
	}
	gotPathIDs := map[string]string{}
	for _, p := range m.Paths {
		gotPathIDs[p.ID] = p.Mode
	}
	for _, id := range wantPathIDs {
		if _, ok := gotPathIDs[id]; !ok {
			t.Fatalf("manifest missing required PathID %q (got %v)", id, gotPathIDs)
		}
	}
	for _, p := range m.Paths {
		if !strings.HasPrefix(p.Root, "/") {
			t.Fatalf("path %s root %q must be absolute", p.ID, p.Root)
		}
		if !strings.HasPrefix(p.Mode, "write") {
			t.Fatalf("path %s mode %q must start with write (jobpolicy.pathModeAllowsWrite)", p.ID, p.Mode)
		}
	}

	wantServiceIDs := []string{
		helpermanifest.ServiceIDOpenClawUser,
		helpermanifest.ServiceIDBorgeeHelper,
	}
	gotServiceIDs := map[string]helpermanifest.ServiceDeclaration{}
	for _, s := range m.Services {
		gotServiceIDs[s.ID] = s
	}
	for _, id := range wantServiceIDs {
		s, ok := gotServiceIDs[id]
		if !ok {
			t.Fatalf("manifest missing required ServiceID %q", id)
		}
		if s.Manager != "systemd" || !strings.HasSuffix(s.Unit, ".service") {
			t.Fatalf("service %q must be systemd/.service, got manager=%q unit=%q", id, s.Manager, s.Unit)
		}
		if s.Platform != "linux" {
			t.Fatalf("service %q platform = %q, want linux", id, s.Platform)
		}
	}

	if len(m.Artifacts) == 0 || m.Artifacts[0].ID != helpermanifest.ArtifactIDOpenClawPlugin {
		t.Fatalf("manifest missing openclaw-plugin artifact: %+v", m.Artifacts)
	}
	if len(m.Domains) == 0 || m.Domains[0] != helpermanifest.DomainCDN {
		t.Fatalf("manifest missing CDN domain: %+v", m.Domains)
	}
}

// TestBuildCanonicalHelperManifest_DeterministicDigest — same input
// produces same canonical bytes + digest across repeated builds. This
// is the property that lets us pin helper_jobs.manifest_digest at
// enqueue time and still verify against the body at lease-serialize
// time even after a server reboot.
func TestBuildCanonicalHelperManifest_DeterministicDigest(t *testing.T) {
	d1 := helpermanifest.LinuxDigest
	d2, err := helpermanifest.Digest(helpermanifest.BuildLinux())
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	if d1 != d2 || !strings.HasPrefix(d1, "sha256:") {
		t.Fatalf("non-deterministic digest: %q vs %q", d1, d2)
	}
	// Build again from scratch and compare canonical bytes.
	c1, err := helpermanifest.CanonicalBytes(helpermanifest.BuildLinux())
	if err != nil {
		t.Fatalf("canonical bytes: %v", err)
	}
	c2, err := helpermanifest.CanonicalBytes(helpermanifest.BuildLinux())
	if err != nil {
		t.Fatalf("canonical bytes: %v", err)
	}
	if string(c1) != string(c2) {
		t.Fatalf("canonical bytes differ across builds")
	}
	sum := sha256.Sum256(c1)
	want := "sha256:" + hex.EncodeToString(sum[:])
	if want != d1 {
		t.Fatalf("digest mismatch: helper says %q, recomputed %q", d1, want)
	}
}

// TestSignCanonicalHelperManifest_RoundTrip — signing a manifest +
// verifying the embedded signature against the matching public key
// succeeds; using the wrong key fails. This locks the contract that the
// daemon's jobpolicy.verifyManifestAuthority uses (ed25519.Verify over
// canonical bytes with Signature stripped).
func TestSignCanonicalHelperManifest_RoundTrip(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	manifest := helpermanifest.BuildLinux()
	signed, digest, err := SignCanonicalHelperManifest(manifest, priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if !strings.HasPrefix(digest, "sha256:") {
		t.Fatalf("digest must be sha256-prefixed, got %q", digest)
	}

	var roundTrip helpermanifest.PolicyManifest
	if err := json.Unmarshal(signed, &roundTrip); err != nil {
		t.Fatalf("unmarshal signed: %v", err)
	}
	if roundTrip.Signature == "" {
		t.Fatalf("signed manifest missing signature")
	}

	sigBytes, err := base64.StdEncoding.DecodeString(roundTrip.Signature)
	if err != nil {
		t.Fatalf("signature base64 decode: %v", err)
	}
	canonical, err := helpermanifest.CanonicalBytes(roundTrip)
	if err != nil {
		t.Fatalf("canonical bytes: %v", err)
	}
	if !ed25519.Verify(pub, canonical, sigBytes) {
		t.Fatalf("signature did not verify against the signing pubkey")
	}

	// Wrong key must fail.
	wrongPub, _, _ := ed25519.GenerateKey(rand.Reader)
	if ed25519.Verify(wrongPub, canonical, sigBytes) {
		t.Fatalf("signature verified under wrong pubkey — must not")
	}
}

// TestSignCanonicalHelperManifest_DevFallSoft — nil signing key
// produces an empty signature field (matches LoadSigningKey dev path).
// The daemon will reject such manifests with ReasonManifestInvalid;
// this is the safe production default.
func TestSignCanonicalHelperManifest_DevFallSoft(t *testing.T) {
	signed, digest, err := SignCanonicalHelperManifest(helpermanifest.BuildLinux(), nil)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if digest == "" {
		t.Fatalf("digest must be returned even when key is nil")
	}
	var roundTrip helpermanifest.PolicyManifest
	if err := json.Unmarshal(signed, &roundTrip); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if roundTrip.Signature != "" {
		t.Fatalf("dev fall-soft must leave signature empty, got %q", roundTrip.Signature)
	}
}

// TestHelperManifestProvider_Cached — provider hands out byte-identical
// signed bytes across repeated calls for the same platform (cached).
func TestHelperManifestProvider_Cached(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	p := NewHelperManifestProvider(priv)
	sig1, dig1, err := p.SignedManifestForPlatform("linux")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	sig2, dig2, err := p.SignedManifestForPlatform("")
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if string(sig1) != string(sig2) || dig1 != dig2 {
		t.Fatalf("cache miss for platform=linux/'': sigs differ or digests differ")
	}
}
