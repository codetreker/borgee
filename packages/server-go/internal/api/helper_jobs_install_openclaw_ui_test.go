package api_test

// Tests for issue #1050 — Install OpenClaw UI trigger.
//
// The owner-facing UI POSTs `openclaw.install_from_manifest` through the
// existing helper-jobs enqueue endpoint (`handleEnqueue` in helper_jobs.go).
// These tests pin the contract the client relies on (acceptance OUT-4 / OUT-5
// / OUT-6):
//
//   - happy path: 201 + persisted row with `category=openclaw_lifecycle`,
//     `job_type=openclaw.install_from_manifest`, server-owned canonical
//     payload (install_plan_id), signed manifest binding present, and
//     the signed `manifest_json` body served on lease declaring the
//     install path root + plugin artifact origin (acceptance OUT-4
//     spec-literal intent: the daemon's executor must see the
//     `manifest_url` / `pubkey_base64` / `target_path` data, which the
//     canonical manifest body carries via its Artifacts + Paths +
//     Signature fields);
//   - non-owner 403, no row inserted;
//   - idempotency (deterministic `install-openclaw-<enrollmentId>` key):
//     repeat POST while the job is in-flight returns the same `job_id`
//     with 200 and does not insert a second row.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"borgee-server/internal/testutil"
)

// installOpenClawEnvelope mirrors the client's `installOpenClawOnHelper`
// request body so the server-side contract stays in lock-step with the
// real UI POST.
func installOpenClawEnvelope(enrollmentID string) map[string]any {
	return map[string]any{
		"job_type":        "openclaw.install_from_manifest",
		"schema_version":  1,
		"payload":         map[string]any{"runtime": "openclaw"},
		"idempotency_key": "install-openclaw-" + enrollmentID,
	}
}

func TestInstallOpenClawUIFlowHappyPath(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	resp, created := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments", ownerToken, map[string]any{
		"host_label":         "Owner's Mac",
		"allowed_categories": []string{"openclaw_lifecycle"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create lifecycle enrollment: status %d body %v", resp.StatusCode, created)
	}
	enrollment := created["enrollment"].(map[string]any)
	enrollmentID := enrollment["enrollment_id"].(string)
	secret := created["enrollment_secret"].(string)
	_, helperCredential := claimHelperEnrollmentViaAPI(t, ts.URL, enrollmentID, secret, "device-ui-install")

	resp, data := testutil.JSON(
		t,
		http.MethodPost,
		ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs",
		ownerToken,
		installOpenClawEnvelope(enrollmentID),
	)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("install_from_manifest enqueue: status %d body %v", resp.StatusCode, data)
	}

	job := data["job"].(map[string]any)
	jobID, _ := job["job_id"].(string)
	if jobID == "" {
		t.Fatalf("UI install enqueue returned no job_id: %v", job)
	}
	if job["job_type"] != "openclaw.install_from_manifest" {
		t.Fatalf("UI install enqueue stored wrong job_type: %v", job)
	}
	if job["category"] != "openclaw_lifecycle" {
		t.Fatalf("UI install enqueue stored wrong category (acceptance OUT-4): %v", job)
	}
	if job["status"] != "queued" {
		t.Fatalf("UI install enqueue stored wrong initial status: %v", job)
	}
	assertNoHelperJobSensitiveFields(t, job)

	// Poll the helper rail to verify the lease emits the server-owned
	// canonical payload + signed manifest binding (OUT-4). The lease frame
	// is what the daemon actually executes, so checking it here guarantees
	// the UI button results in the same payload shape that the executor
	// reads.
	resp, poll := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/poll", helperCredential, map[string]any{
		"helper_device_id": "device-ui-install",
		"helper_platform":  "linux",
	})
	if resp.StatusCode != http.StatusOK || poll["status"] != "leased" {
		t.Fatalf("poll install job: status %d body %v", resp.StatusCode, poll)
	}
	leased := poll["job"].(map[string]any)
	if leased["job_type"] != "openclaw.install_from_manifest" {
		t.Fatalf("leased job_type mismatch: %v", leased)
	}
	if leased["category"] != "openclaw_lifecycle" {
		t.Fatalf("leased category mismatch (acceptance OUT-4): %v", leased)
	}
	leasedPayload, ok := leased["payload"].(map[string]any)
	if !ok {
		t.Fatalf("leased payload missing: %v", leased)
	}
	if leasedPayload["install_plan_id"] != "openclaw-plugin-v1" {
		t.Fatalf("leased payload should be server-owned plan (acceptance OUT-4): %v", leasedPayload)
	}
	if leased["manifest_digest"] == nil || leased["manifest_digest"] == "" {
		t.Fatalf("leased install must carry a signed manifest binding (acceptance OUT-4): %v", leased)
	}
	binding, ok := leased["manifest_binding"].(map[string]any)
	if !ok {
		t.Fatalf("leased install missing manifest_binding (acceptance OUT-4): %v", leased)
	}
	if binding["manifest_digest"] != leased["manifest_digest"] {
		t.Fatalf("manifest binding digest mismatch: %v vs %v", binding["manifest_digest"], leased["manifest_digest"])
	}

	// Acceptance OUT-4 (spec-literal): the row's payload "contains
	// `manifest_url`, `pubkey_base64`, `plugin_id="openclaw"`,
	// `target_path="/usr/local/lib/borgee/openclaw"`". The canonical
	// design puts these in the signed manifest body (`manifest_json`),
	// not the loose payload — daemon-side `installplugin` reads the
	// manifest via `install_plan_id`. Verify the manifest_json the
	// helper actually receives declares the two pieces of data the
	// daemon needs to act:
	//   - a Path declaration with id="openclaw_install" rooted at
	//     /usr/local/lib/borgee/openclaw  (≡ acceptance `target_path`)
	//   - an Artifact declaration with id="openclaw-plugin" whose Origin
	//     resolves to the CDN URL (≡ acceptance `manifest_url`)
	// (Signature presence ≡ acceptance `pubkey_base64` trust anchor is
	// already pinned by the `manifest_digest` + `manifest_binding`
	// checks above — those carry the digest the daemon recomputes from
	// canonical bytes to verify against the trusted pubkey. The test
	// server runs in dev fall-soft mode (no signing key), so the
	// canonical body's `signature` field is empty by design here; what
	// matters at this layer is that the canonical bytes the daemon
	// receives carry the path + artifact intent acceptance OUT-4 names.)
	rawManifest, ok := leased["manifest_json"]
	if !ok || rawManifest == nil {
		t.Fatalf("leased install must carry canonical manifest_json (acceptance OUT-4): %v", leased)
	}
	manifestBytes, err := json.Marshal(rawManifest)
	if err != nil {
		t.Fatalf("manifest_json marshal: %v", err)
	}
	var manifest struct {
		Artifacts []struct {
			ID     string `json:"id"`
			Origin string `json:"origin"`
		} `json:"artifacts"`
		Paths []struct {
			ID   string `json:"id"`
			Root string `json:"root"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("manifest_json unmarshal: %v", err)
	}
	foundInstallPath := false
	for _, p := range manifest.Paths {
		if p.ID == "openclaw_install" {
			foundInstallPath = true
			if p.Root != "/usr/local/lib/borgee/openclaw" {
				t.Fatalf("openclaw_install path root mismatch (acceptance OUT-4 target_path): got %q want %q", p.Root, "/usr/local/lib/borgee/openclaw")
			}
		}
	}
	if !foundInstallPath {
		t.Fatalf("manifest_json paths missing openclaw_install entry (acceptance OUT-4 target_path): %s", string(manifestBytes))
	}
	foundArtifact := false
	for _, a := range manifest.Artifacts {
		if a.ID == "openclaw-plugin" {
			foundArtifact = true
			if !strings.HasPrefix(a.Origin, "https://") {
				t.Fatalf("openclaw-plugin artifact origin must be an https URL (acceptance OUT-4 manifest_url): got %q", a.Origin)
			}
		}
	}
	if !foundArtifact {
		t.Fatalf("manifest_json artifacts missing openclaw-plugin entry (acceptance OUT-4 manifest_url): %s", string(manifestBytes))
	}

	if count := countAPIHelperJobs(t, s); count != 1 {
		t.Fatalf("UI install enqueue inserted %d rows, want 1", count)
	}
}

func TestInstallOpenClawUIFlowRejectsNonOwner(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	resp, created := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments", ownerToken, map[string]any{
		"host_label":         "Owner's Mac",
		"allowed_categories": []string{"openclaw_lifecycle"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create lifecycle enrollment: status %d body %v", resp.StatusCode, created)
	}
	enrollmentID := created["enrollment"].(map[string]any)["enrollment_id"].(string)

	// A different human user POSTs against the owner's enrollment. The
	// server must reject without writing a row (acceptance OUT-5).
	intruderToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	resp, denied := testutil.JSON(
		t,
		http.MethodPost,
		ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs",
		intruderToken,
		installOpenClawEnvelope(enrollmentID),
	)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-owner install POST should be 403 (acceptance OUT-5): status %d body %v", resp.StatusCode, denied)
	}
	if code, _ := denied["code"].(string); code != "wrong_owner" && code != "forbidden" {
		t.Fatalf("non-owner install POST should fail with forbidden/wrong_owner: %v", denied)
	}
	if count := countAPIHelperJobs(t, s); count != 0 {
		t.Fatalf("non-owner install POST inserted %d rows, want 0 (acceptance OUT-5)", count)
	}
}

func TestInstallOpenClawUIFlowIdempotencyInFlight(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	resp, created := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments", ownerToken, map[string]any{
		"host_label":         "Owner's Mac",
		"allowed_categories": []string{"openclaw_lifecycle"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create lifecycle enrollment: status %d body %v", resp.StatusCode, created)
	}
	enrollmentID := created["enrollment"].(map[string]any)["enrollment_id"].(string)
	secret := created["enrollment_secret"].(string)
	claimHelperEnrollmentViaAPI(t, ts.URL, enrollmentID, secret, "device-idem")

	envelope := installOpenClawEnvelope(enrollmentID)

	resp, first := testutil.JSON(
		t,
		http.MethodPost,
		ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs",
		ownerToken,
		envelope,
	)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first install POST: status %d body %v", resp.StatusCode, first)
	}
	firstJob := first["job"].(map[string]any)
	firstJobID, _ := firstJob["job_id"].(string)
	if firstJobID == "" {
		t.Fatalf("first install POST missing job_id: %v", firstJob)
	}

	// A second click while the job is still queued must return the same
	// job_id with 200 (acceptance OUT-6). No second row may be inserted.
	resp, second := testutil.JSON(
		t,
		http.MethodPost,
		ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs",
		ownerToken,
		envelope,
	)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("idempotent repeat install POST: status %d body %v (want 200)", resp.StatusCode, second)
	}
	secondJob := second["job"].(map[string]any)
	if secondJob["job_id"] != firstJobID {
		t.Fatalf("idempotent repeat returned different job_id: first=%s second=%v (acceptance OUT-6)", firstJobID, secondJob["job_id"])
	}
	if count := countAPIHelperJobs(t, s); count != 1 {
		t.Fatalf("idempotent repeat inserted %d rows, want 1 (acceptance OUT-6)", count)
	}
}
