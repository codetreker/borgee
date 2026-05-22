//go:build linux || darwin

package daemon

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"borgee/internal/dispatch"
	"borgee/internal/executors/statuscollect"
	"borgee/internal/jobpolicy"
	"borgee/internal/outbound"

	"github.com/coder/websocket"
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

// TestDefaultPolicyEvaluator_AcceptsRealisticLeasedJob (amend gap #1
// regression guard) — projects an outbound.LeasedJob with the SAME
// envelope shape the server now emits via serializeHelperJobLease
// (owner_user_id, org_id, helper_device_id, category, payload_hash,
// expires_at) and asserts defaultPolicyEvaluator's Decision is NOT
// ReasonSchemaInvalid. Without this test a future field addition could
// regress silently — the lease frame and validateJobSchema would drift
// apart again and every pushed job would 5xx the executor before it
// ran.
//
// Uses status.collect: it's the only no-manifest-required JobType in
// the gate, so the manifest branch is skipped and the schema gate is
// the only thing we're guarding. Other JobTypes have separate tests
// (TestDefaultPolicyEvaluator_AcceptsCanonicalManifest covers the
// manifest path).
func TestDefaultPolicyEvaluator_AcceptsRealisticLeasedJob(t *testing.T) {
	t.Setenv("BORGEE_MANIFEST_SIGNING_PUBKEY", "")
	evaluator := defaultPolicyEvaluator()
	payload := json.RawMessage(`{"scope":"helper"}`)
	job := &outbound.LeasedJob{
		JobID:          "job-1",
		EnrollmentID:   "enroll-1",
		JobType:        jobpolicy.JobTypeStatusCollect,
		SchemaVersion:  1,
		Payload:        payload,
		OwnerUserID:    "user-1",
		OrgID:          "org-1",
		HelperDeviceID: "device-1",
		Category:       jobpolicy.CategoryHelperLifecycle,
		PayloadHash:    sha256Hex(payload),
		ExpiresAt:      time.Now().Add(time.Hour).UnixMilli(),
		LeaseToken:     "lease-1",
	}
	decision := evaluator(context.Background(), job)
	if decision.Reason == jobpolicy.ReasonSchemaInvalid {
		t.Fatalf("defaultPolicyEvaluator rejected a realistic LeasedJob with ReasonSchemaInvalid — server lease frame is missing an envelope field again (regression guard for amend gap #1). Job: %+v", job)
	}
	if !decision.Allow {
		t.Fatalf("status.collect must Allow under default evaluator: allow=%v reason=%s", decision.Allow, decision.Reason)
	}
}

// TestDefaultPolicyEvaluator_RejectsMissingEnvelope — companion to the
// above: confirm the schema gate still fires when the lease frame
// omits a required field (zero PayloadHash). If this test ever passes
// "allow", the gate has become permissive and the double-validate
// contract is broken.
func TestDefaultPolicyEvaluator_RejectsMissingEnvelope(t *testing.T) {
	t.Setenv("BORGEE_MANIFEST_SIGNING_PUBKEY", "")
	evaluator := defaultPolicyEvaluator()
	payload := json.RawMessage(`{"scope":"helper"}`)
	job := &outbound.LeasedJob{
		JobID:          "job-1",
		EnrollmentID:   "enroll-1",
		JobType:        jobpolicy.JobTypeStatusCollect,
		SchemaVersion:  1,
		Payload:        payload,
		OwnerUserID:    "user-1",
		OrgID:          "org-1",
		HelperDeviceID: "device-1",
		Category:       jobpolicy.CategoryHelperLifecycle,
		// PayloadHash intentionally empty — must trip validateJobSchema.
		ExpiresAt:  time.Now().Add(time.Hour).UnixMilli(),
		LeaseToken: "lease-1",
	}
	decision := evaluator(context.Background(), job)
	if decision.Allow {
		t.Fatal("evaluator must reject LeasedJob with empty PayloadHash")
	}
}

// TestDaemonEndToEnd_StatusCollectRunsAcrossWS (amend gap #8) — the
// regression test that should have caught amend gap #1. Spins up a
// real httptest WS server speaking the helper subprotocol, runs the
// daemon-side dispatcher with defaultPolicyEvaluator + the real
// status.collect executor, pushes a full-envelope leased job, and
// asserts the daemon sends back a `result` frame with status=succeeded.
// If the schema gate ever rejects again the result.status will be
// "failed" with failure_code="schema_invalid" — which makes the
// failure mode visible at unit-test time instead of only on the
// testing-borgee container.
func TestDaemonEndToEnd_StatusCollectRunsAcrossWS(t *testing.T) {
	t.Setenv("BORGEE_MANIFEST_SIGNING_PUBKEY", "")
	type frameSink struct {
		results chan map[string]any
		acks    atomic.Int32
	}
	sink := &frameSink{results: make(chan map[string]any, 4)}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/helper/{enrollmentId}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer helper-token" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols:       []string{outbound.HelperWSSubprotocol},
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}
		ctx := r.Context()
		// Reader pump: capture ack + result frames as the dispatcher
		// emits them so the test can assert on the terminal state.
		go func() {
			for {
				_, data, err := conn.Read(ctx)
				if err != nil {
					return
				}
				var m map[string]any
				if err := json.Unmarshal(data, &m); err != nil {
					continue
				}
				switch m["type"] {
				case "ack":
					sink.acks.Add(1)
				case "result":
					select {
					case sink.results <- m:
					default:
					}
				}
			}
		}()
		// Push one well-formed status.collect leased job.
		payload := json.RawMessage(`{"scope":"helper"}`)
		hash := sha256Hex(payload)
		frame := map[string]any{
			"type": "job",
			"job": map[string]any{
				"job_id":           "job-status-1",
				"enrollment_id":    "enroll-1",
				"job_type":         jobpolicy.JobTypeStatusCollect,
				"schema_version":   1,
				"payload":          map[string]any{"scope": "helper"},
				"manifest_digest":  "",
				"lease_token":      "lease-1",
				"lease_expires_at": time.Now().Add(time.Minute).UnixMilli(),
				"attempt":          1,
				// The six envelope fields the server now projects per
				// amend gap #1. Daemon-side validateJobSchema fails
				// closed if any are missing.
				"owner_user_id":    "user-1",
				"org_id":           "org-1",
				"helper_device_id": "device-1",
				"category":         jobpolicy.CategoryHelperLifecycle,
				"payload_hash":     hash,
				"expires_at":       time.Now().Add(time.Hour).UnixMilli(),
			},
		}
		data, _ := json.Marshal(frame)
		_ = conn.Write(ctx, websocket.MessageText, data)
		time.Sleep(300 * time.Millisecond)
		_ = conn.Close(websocket.StatusNormalClosure, "")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client, err := outbound.NewClient(
		outbound.PreparedConfig{Enabled: true, ServerOrigin: srv.URL},
		outbound.StaticCredentialSource{Credential: "helper-token", HelperDeviceID: "device-1"},
		outbound.WithHTTPClient(srv.Client()),
		outbound.WithReconnectBackoff(10*time.Millisecond, 50*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	client.SetEnrollmentID("enroll-1")

	d := &dispatch.Dispatcher{
		Client:          client,
		EnrollmentID:    "enroll-1",
		PolicyEvaluator: defaultPolicyEvaluator(),
		Executors: map[string]dispatch.Executor{
			jobpolicy.JobTypeStatusCollect: &statuscollect.Executor{},
		},
		LeaseRenewEvery: 5 * time.Second, // dampen ack noise in the test window
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go d.Run(ctx)

	select {
	case res := <-sink.results:
		// Regression guard for amend gap #1: if schema_invalid surfaces
		// here, the envelope projection broke again. Fail loud with the
		// full result frame so the operator sees the actual code.
		status, _ := res["status"].(string)
		failureCode, _ := res["failure_code"].(string)
		if status != "succeeded" {
			t.Fatalf("status.collect end-to-end must succeed; got status=%q failure_code=%q result=%v", status, failureCode, res)
		}
		if failureCode == string(jobpolicy.ReasonSchemaInvalid) {
			t.Fatalf("schema_invalid leaked through end-to-end; amend gap #1 regressed: %v", res)
		}
	case <-ctx.Done():
		t.Fatalf("end-to-end status.collect timed out before result frame arrived")
	}
}
