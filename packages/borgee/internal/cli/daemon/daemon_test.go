//go:build linux || darwin

package daemon

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"borgee/internal/jobpolicy"
)

func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// TestLoadHelperManifestTrustRoots_EmptyEnv — daemon launched without
// BORGEE_MANIFEST_SIGNING_PUBKEY produces an empty TrustRoots slice;
// jobpolicy.verifyManifestAuthority then rejects manifest-required jobs
// with ReasonManifestInvalid (the safe production default).
func TestLoadHelperManifestTrustRoots_EmptyEnv(t *testing.T) {
	t.Setenv("BORGEE_MANIFEST_SIGNING_PUBKEY", "")
	roots := loadHelperManifestTrustRoots()
	if len(roots) != 0 {
		t.Fatalf("empty env produced %d roots, want 0", len(roots))
	}
}

// TestLoadHelperManifestTrustRoots_MultiCSV — comma-separated pubkeys
// support a key-rotation grace window. Each must base64-decode to 32
// bytes; malformed entries log + drop without poisoning the slice.
func TestLoadHelperManifestTrustRoots_MultiCSV(t *testing.T) {
	pub1, _, _ := ed25519.GenerateKey(rand.Reader)
	pub2, _, _ := ed25519.GenerateKey(rand.Reader)
	pubB64 := base64.StdEncoding.EncodeToString(pub1) + "," + base64.StdEncoding.EncodeToString(pub2) + ",not-base64==="
	t.Setenv("BORGEE_MANIFEST_SIGNING_PUBKEY", pubB64)
	roots := loadHelperManifestTrustRoots()
	if len(roots) != 2 {
		t.Fatalf("loaded %d roots, want 2 (third entry malformed must drop)", len(roots))
	}
}

// TestDefaultPolicyEvaluator_AcceptsCanonicalManifest — end-to-end
// inside the daemon process: a leased job carrying a real signed
// PolicyManifest body + matching binding evaluates to Allow when
// TrustRoots is populated from the env. This locks the PR-4 amend
// promise that 5/8 manifest-required job types stop landing in
// ReasonManifestInvalid once the server emits signed manifest bodies.
func TestDefaultPolicyEvaluator_AcceptsCanonicalManifest(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	t.Setenv("BORGEE_MANIFEST_SIGNING_PUBKEY", base64.StdEncoding.EncodeToString(pub))
	roots := loadHelperManifestTrustRoots()
	if len(roots) != 1 {
		t.Fatalf("expected 1 trust root, got %d", len(roots))
	}

	now := time.Now()
	manifest := jobpolicy.PolicyManifest{
		ManifestVersion: 1,
		IssuedAt:        now.Add(-time.Hour),
		ExpiresAt:       now.Add(time.Hour),
		Paths: []jobpolicy.PathDeclaration{
			{ID: "borgee_state_config", Root: "/var/lib/borgee/state", Mode: "write_config"},
		},
	}
	canonical, err := jobpolicy.CanonicalManifestBytes(manifest)
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	manifest.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, canonical))
	signedManifest, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	digest := sha256Hex(canonical)

	binding := jobpolicy.ManifestBinding{ManifestDigest: digest, PathIDs: []string{"borgee_state_config"}}
	bindingJSON, err := json.Marshal(binding)
	if err != nil {
		t.Fatalf("binding: %v", err)
	}
	payload := json.RawMessage(`{"state_key":"foo"}`)
	payloadHashSum := sha256Hex(payload)

	// Direct call to Evaluate via the input shape the defaultPolicyEvaluator
	// builds (envelope fields populated as Evaluate expects — owner / org /
	// enrollment matches; allowed category; non-zero expiry).
	decision := jobpolicy.Evaluate(jobpolicy.EvaluationInput{
		Now:        now,
		TrustRoots: roots,
		Job: jobpolicy.Job{
			JobID:                "job-1",
			OwnerUserID:          "user-1",
			OrgID:                "org-1",
			EnrollmentID:         "enroll-1",
			HelperDeviceID:       "device-1",
			CredentialGeneration: 1,
			JobType:              jobpolicy.JobTypeStateWrite,
			Category:             jobpolicy.CategoryOpenClaw,
			SchemaVersion:        1,
			PayloadJSON:          payload,
			PayloadHash:          payloadHashSum,
			ManifestDigest:       digest,
			ManifestJSON:         signedManifest,
			ManifestBindingJSON:  bindingJSON,
			ExpiresAt:            now.Add(time.Hour),
		},
		Enrollment: jobpolicy.EnrollmentState{
			OwnerUserID:          "user-1",
			OrgID:                "org-1",
			EnrollmentID:         "enroll-1",
			HelperDeviceID:       "device-1",
			CredentialGeneration: 1,
			Status:               "active",
			AllowedCategories:    []string{jobpolicy.CategoryOpenClaw},
		},
		Sandbox: jobpolicy.SandboxProfile{WriteRoots: []string{"/var/lib/borgee/state"}},
	})
	if !decision.Allow || decision.Reason != jobpolicy.ReasonOK {
		t.Fatalf("Evaluate denied: allow=%v reason=%s", decision.Allow, decision.Reason)
	}

	// Without the trust root, the same input lands in ReasonManifestInvalid —
	// confirming TrustRoots populated from env is what flips the gate.
	t.Setenv("BORGEE_MANIFEST_SIGNING_PUBKEY", "")
	noRoots := loadHelperManifestTrustRoots()
	noRootDecision := jobpolicy.Evaluate(jobpolicy.EvaluationInput{
		Now:        now,
		TrustRoots: noRoots,
		Job: jobpolicy.Job{
			JobID:                "job-1",
			OwnerUserID:          "user-1",
			OrgID:                "org-1",
			EnrollmentID:         "enroll-1",
			HelperDeviceID:       "device-1",
			CredentialGeneration: 1,
			JobType:              jobpolicy.JobTypeStateWrite,
			Category:             jobpolicy.CategoryOpenClaw,
			SchemaVersion:        1,
			PayloadJSON:          payload,
			PayloadHash:          payloadHashSum,
			ManifestDigest:       digest,
			ManifestJSON:         signedManifest,
			ManifestBindingJSON:  bindingJSON,
			ExpiresAt:            now.Add(time.Hour),
		},
		Enrollment: jobpolicy.EnrollmentState{
			OwnerUserID:          "user-1",
			OrgID:                "org-1",
			EnrollmentID:         "enroll-1",
			HelperDeviceID:       "device-1",
			CredentialGeneration: 1,
			Status:               "active",
			AllowedCategories:    []string{jobpolicy.CategoryOpenClaw},
		},
	})
	if noRootDecision.Allow || !strings.Contains(string(noRootDecision.Reason), "manifest_invalid") {
		t.Fatalf("no-roots branch unexpected: allow=%v reason=%s", noRootDecision.Allow, noRootDecision.Reason)
	}
}
