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
	"errors"
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

// fakeArtifactFetcher returns canned bytes per URL. Tests inject this
// in place of the production httpArtifactFetcher to avoid real HTTP.
type fakeArtifactFetcher struct {
	bytesByURL map[string][]byte
	errByURL   map[string]error
	calls      []string
}

func (f *fakeArtifactFetcher) Fetch(_ context.Context, url string) ([]byte, error) {
	f.calls = append(f.calls, url)
	if err, ok := f.errByURL[url]; ok {
		return nil, err
	}
	if b, ok := f.bytesByURL[url]; ok {
		return b, nil
	}
	return nil, errors.New("fake fetcher: no entry for " + url)
}

// TestNewPolicyEvaluator_PreFetchesArtifactsAndPassesArtifactGate
// (#1050 blocker #4 regression guard) — the install_from_manifest
// policy gate requires EvaluationInput.Artifacts to contain the bound
// artifact's bytes whose sha256 matches the signed manifest's
// ArtifactDeclaration. Before this fix the dispatcher built
// EvaluationInput with Artifacts=nil, so validateArtifacts always
// returned ReasonArtifactInvalid before the executor ran.
//
// This test pins the pre-fetch + cache-construction contract: given a
// real signed manifest with one declared artifact whose Origin returns
// bytes whose sha matches the declared SHA256, the artifact-cache
// gate no longer fires. Downstream gates (validatePaths /
// validateDomains / validateServices) still depend on Sandbox state
// the helper does not yet wire — those are separate scope. The
// regression we are locking is: artifact_invalid never appears when
// pre-fetch is wired correctly.
//
// If anyone removes the pre-fetch wiring this test fails with
// reason=artifact_invalid — the exact symptom that surfaced in
// run_4's live dev-stack.
func TestNewPolicyEvaluator_PreFetchesArtifactsAndPassesArtifactGate(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	now := time.Now()
	artifactBytes := []byte("sentinel-binary-bytes-v1")
	artifactSHA := sha256Hex(artifactBytes)

	manifest := jobpolicy.PolicyManifest{
		ManifestVersion: 1,
		IssuedAt:        now.Add(-time.Hour),
		ExpiresAt:       now.Add(time.Hour),
		Artifacts: []jobpolicy.ArtifactDeclaration{{
			ID:       "openclaw-plugin",
			Platform: "linux-x64",
			Version:  "1.0.0",
			SHA256:   artifactSHA,
			Origin:   "https://cdn.example.test",
		}},
		Paths: []jobpolicy.PathDeclaration{
			{ID: "openclaw_install", Root: "/usr/local/lib/borgee/openclaw", Mode: "write_install"},
			{ID: "openclaw_agent_config", Root: "/var/lib/borgee/openclaw", Mode: "write_config"},
		},
		Domains: []string{"https://cdn.example.test"},
		Services: []jobpolicy.ServiceDeclaration{
			{ID: "openclaw-user", Platform: "linux", Manager: "systemd", Unit: "openclaw.service"},
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
	manifestDigest := sha256Hex(canonical)

	binding := jobpolicy.ManifestBinding{
		ManifestDigest: manifestDigest,
		ArtifactIDs:    []string{"openclaw-plugin"},
		PathIDs:        []string{"openclaw_install", "openclaw_agent_config"},
		Domains:        []string{"https://cdn.example.test"},
	}
	bindingJSON, err := json.Marshal(binding)
	if err != nil {
		t.Fatalf("binding: %v", err)
	}
	payload := json.RawMessage(`{"install_plan_id":"openclaw-plugin-v1"}`)
	payloadHash := sha256Hex(payload)
	leased := &outbound.LeasedJob{
		JobID:               "job-install-1",
		EnrollmentID:        "enroll-1",
		JobType:             jobpolicy.JobTypeOpenClawInstallFromManifest,
		SchemaVersion:       1,
		Payload:             payload,
		OwnerUserID:         "user-1",
		OrgID:               "org-1",
		HelperDeviceID:      "device-1",
		Category:            jobpolicy.CategoryOpenClawLifecycle,
		PayloadHash:         payloadHash,
		ManifestDigest:      manifestDigest,
		ManifestJSON:        signedManifest,
		ManifestBindingJSON: bindingJSON,
		ExpiresAt:           now.Add(time.Hour).UnixMilli(),
		LeaseToken:          "lease-1",
	}

	// Confirm pre-fetch produces the right cache shape FIRST (unit-level
	// regression on the pre-fetch helper).
	cache := resolveArtifactsCache(context.Background(),
		&fakeArtifactFetcher{bytesByURL: map[string][]byte{"https://cdn.example.test": artifactBytes}},
		signedManifest, bindingJSON)
	if got, ok := cache["openclaw-plugin"]; !ok || string(got) != string(artifactBytes) {
		t.Fatalf("resolveArtifactsCache: openclaw-plugin entry missing or wrong: ok=%v len=%d", ok, len(got))
	}

	// End-to-end through the evaluator: artifact_invalid must NOT be the
	// failure reason (the gate we are locking).
	fetcher := &fakeArtifactFetcher{
		bytesByURL: map[string][]byte{"https://cdn.example.test": artifactBytes},
	}
	evaluator := newPolicyEvaluator([]ed25519.PublicKey{pub}, fetcher)
	decision := evaluator(context.Background(), leased)
	if len(fetcher.calls) == 0 {
		t.Fatalf("pre-fetch did not run; fetcher.calls=%v", fetcher.calls)
	}
	if fetcher.calls[0] != "https://cdn.example.test" {
		t.Fatalf("pre-fetch called wrong URL: got %q want %q", fetcher.calls[0], "https://cdn.example.test")
	}
	if decision.Reason == jobpolicy.ReasonArtifactInvalid {
		t.Fatalf("artifact_invalid leaked through even with pre-fetched matching bytes — blocker #4 regressed")
	}
}

// TestNewPolicyEvaluator_PreFetchFailureYieldsArtifactInvalid — when
// the fetcher returns an error (network down, 5xx, oversize, etc) the
// cache entry is absent and the policy gate returns the clean
// ReasonArtifactInvalid rather than silently passing on no bytes.
// Defense-in-depth so a transient fetch failure cannot promote into a
// permissive Allow.
func TestNewPolicyEvaluator_PreFetchFailureYieldsArtifactInvalid(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	now := time.Now()
	artifactBytes := []byte("sentinel-binary-bytes-v1")
	artifactSHA := sha256Hex(artifactBytes)
	manifest := jobpolicy.PolicyManifest{
		ManifestVersion: 1,
		IssuedAt:        now.Add(-time.Hour),
		ExpiresAt:       now.Add(time.Hour),
		Artifacts: []jobpolicy.ArtifactDeclaration{{
			ID: "openclaw-plugin", Platform: "linux-x64", Version: "1.0.0",
			SHA256: artifactSHA, Origin: "https://cdn.example.test",
		}},
		Paths: []jobpolicy.PathDeclaration{
			{ID: "openclaw_install", Root: "/usr/local/lib/borgee/openclaw", Mode: "write_install"},
			{ID: "openclaw_agent_config", Root: "/var/lib/borgee/openclaw", Mode: "write_config"},
		},
		Domains: []string{"https://cdn.example.test"},
		Services: []jobpolicy.ServiceDeclaration{
			{ID: "openclaw-user", Platform: "linux", Manager: "systemd", Unit: "openclaw.service"},
		},
	}
	canonical, _ := jobpolicy.CanonicalManifestBytes(manifest)
	manifest.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, canonical))
	signedManifest, _ := json.Marshal(manifest)
	manifestDigest := sha256Hex(canonical)
	binding := jobpolicy.ManifestBinding{
		ManifestDigest: manifestDigest, ArtifactIDs: []string{"openclaw-plugin"},
		PathIDs: []string{"openclaw_install", "openclaw_agent_config"}, Domains: []string{"https://cdn.example.test"},
	}
	bindingJSON, _ := json.Marshal(binding)
	payload := json.RawMessage(`{"install_plan_id":"openclaw-plugin-v1"}`)
	leased := &outbound.LeasedJob{
		JobID: "job-install-2", EnrollmentID: "enroll-1",
		JobType: jobpolicy.JobTypeOpenClawInstallFromManifest, SchemaVersion: 1,
		Payload: payload, OwnerUserID: "user-1", OrgID: "org-1",
		HelperDeviceID: "device-1", Category: jobpolicy.CategoryOpenClawLifecycle,
		PayloadHash: sha256Hex(payload), ManifestDigest: manifestDigest,
		ManifestJSON: signedManifest, ManifestBindingJSON: bindingJSON,
		ExpiresAt: now.Add(time.Hour).UnixMilli(), LeaseToken: "lease-1",
	}
	fetcher := &fakeArtifactFetcher{errByURL: map[string]error{"https://cdn.example.test": errors.New("synthetic 503")}}
	decision := newPolicyEvaluator([]ed25519.PublicKey{pub}, fetcher)(context.Background(), leased)
	if decision.Allow || decision.Reason != jobpolicy.ReasonArtifactInvalid {
		t.Fatalf("fetch failure must yield artifact_invalid: allow=%v reason=%s", decision.Allow, decision.Reason)
	}
}

// TestHTTPArtifactFetcher_HappyPath — production fetcher against a
// real httptest server. Locks the contract that we GET the URL,
// honor 200 OK, and return the body bytes verbatim.
func TestHTTPArtifactFetcher_HappyPath(t *testing.T) {
	want := []byte("hello-from-test-server")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(want)
	}))
	defer srv.Close()
	f := &httpArtifactFetcher{client: srv.Client(), maxSize: 1 << 20}
	got, err := f.Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("body mismatch: got %q want %q", string(got), string(want))
	}
}

// TestHTTPArtifactFetcher_RejectsNon200 — defense-in-depth: a 404 / 5xx
// must surface as an error so resolveArtifactsCache can leave the
// cache entry absent (-> clean ReasonArtifactInvalid downstream).
func TestHTTPArtifactFetcher_RejectsNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	f := &httpArtifactFetcher{client: srv.Client(), maxSize: 1 << 20}
	if _, err := f.Fetch(context.Background(), srv.URL); err == nil {
		t.Fatal("expected non-2xx error, got nil")
	}
}
