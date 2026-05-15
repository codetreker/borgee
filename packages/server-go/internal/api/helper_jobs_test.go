package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

func TestHelperJobsEnqueueHappyPathIdempotencyAndSerializer(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	enrollment, secret := createHelperEnrollmentViaAPI(t, ts.URL, ownerToken)
	enrollmentID := enrollment["enrollment_id"].(string)
	claimHelperEnrollmentViaAPI(t, ts.URL, enrollmentID, secret, "device-1")
	agent := seedHelperJobAgent(t, s, "owner@test.com", "api-openclaw-agent")
	seedHelperJobAgentConfig(t, s, agent.ID, 4, map[string]any{"name": "OpenClaw", "enabled": true})

	body := map[string]any{
		"job_type":        "openclaw.configure_agent",
		"schema_version":  1,
		"payload":         map[string]any{"agent_id": agent.ID},
		"idempotency_key": "retry-api-1",
	}
	resp, data := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("enqueue helper job: status %d body %v", resp.StatusCode, data)
	}
	job := data["job"].(map[string]any)
	jobID, _ := job["job_id"].(string)
	if jobID == "" || job["status"] != "queued" || job["job_type"] != "openclaw.configure_agent" || job["category"] != "openclaw_config" {
		t.Fatalf("bad job response: %v", job)
	}
	if job["enrollment_id"] != enrollmentID || job["schema_version"] != float64(1) {
		t.Fatalf("job response missing enrollment/schema binding: %v", job)
	}
	if _, ok := job["created_at"].(float64); !ok {
		t.Fatalf("job response missing created_at: %v", job)
	}
	if expiresAt, ok := job["expires_at"].(float64); !ok || expiresAt <= job["created_at"].(float64) {
		t.Fatalf("job response missing server expires_at after created_at: %v", job)
	}
	if hash, _ := job["payload_hash"].(string); !strings.HasPrefix(hash, "sha256:") {
		t.Fatalf("job response missing safe payload hash: %v", job)
	}
	if digest, _ := job["manifest_digest"].(string); !strings.HasPrefix(digest, "sha256:") {
		t.Fatalf("job response missing safe manifest digest: %v", job)
	}
	assertNoHelperJobSensitiveFields(t, job)

	resp, retry := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("idempotent retry: status %d body %v", resp.StatusCode, retry)
	}
	if retryJob := retry["job"].(map[string]any); retryJob["job_id"] != jobID {
		t.Fatalf("idempotent retry returned different job: first=%s retry=%v", jobID, retryJob)
	}
	if count := countAPIHelperJobs(t, s); count != 1 {
		t.Fatalf("idempotent retry inserted %d jobs, want 1", count)
	}

	otherAgent := seedHelperJobAgent(t, s, "owner@test.com", "api-openclaw-agent-2")
	seedHelperJobAgentConfig(t, s, otherAgent.ID, 1, map[string]any{"name": "Other"})
	conflictBody := map[string]any{
		"job_type":        "openclaw.configure_agent",
		"schema_version":  1,
		"payload":         map[string]any{"agent_id": otherAgent.ID},
		"idempotency_key": "retry-api-1",
	}
	resp, conflict := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, conflictBody)
	if resp.StatusCode != http.StatusConflict || conflict["code"] != "idempotency_conflict" {
		t.Fatalf("idempotency conflict: status %d body %v", resp.StatusCode, conflict)
	}
}

func TestHelperJobsEnqueueRejectsUnauthorizedRailsAndInvalidEnvelopes(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	enrollment, secret := createHelperEnrollmentViaAPI(t, ts.URL, ownerToken)
	enrollmentID := enrollment["enrollment_id"].(string)
	_, helperCredential := claimHelperEnrollmentViaAPI(t, ts.URL, enrollmentID, secret, "device-1")
	agent := seedHelperJobAgent(t, s, "owner@test.com", "api-reject-agent")
	seedHelperJobAgentConfig(t, s, agent.ID, 1, map[string]any{"name": "Reject Agent"})

	valid := map[string]any{"job_type": "openclaw.configure_agent", "schema_version": 1, "payload": map[string]any{"agent_id": agent.ID}}
	resp, _ := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", "", valid)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous enqueue should be 401, got %d", resp.StatusCode)
	}
	resp, _ = testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", helperCredential, valid)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("helper credential must not authenticate user-rail enqueue, got %d", resp.StatusCode)
	}
	resp, wrongOwner := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", memberToken, valid)
	if resp.StatusCode != http.StatusForbidden || wrongOwner["code"] != "wrong_owner" {
		t.Fatalf("wrong owner enqueue: status %d body %v", resp.StatusCode, wrongOwner)
	}

	cases := []struct {
		name string
		body map[string]any
		want int
		code string
	}{
		{"unknown job type", map[string]any{"job_type": "command.run", "schema_version": 1, "payload": map[string]any{"agent_id": agent.ID}}, http.StatusBadRequest, "unknown_job_type"},
		{"recognized disabled type", map[string]any{"job_type": "service.lifecycle", "schema_version": 1, "payload": map[string]any{"target": "openclaw"}}, http.StatusBadRequest, "job_type_not_enabled"},
		{"extra top field owner", map[string]any{"job_type": "openclaw.configure_agent", "schema_version": 1, "payload": map[string]any{"agent_id": agent.ID}, "owner_user_id": "client"}, http.StatusBadRequest, "extra_field"},
		{"client ttl", map[string]any{"job_type": "openclaw.configure_agent", "schema_version": 1, "payload": map[string]any{"agent_id": agent.ID}, "ttl": 999999}, http.StatusBadRequest, "ttl_invalid"},
		{"payload shell", map[string]any{"job_type": "openclaw.configure_agent", "schema_version": 1, "payload": map[string]any{"agent_id": agent.ID, "shell": "whoami"}}, http.StatusBadRequest, "forbidden_field"},
		{"payload url", map[string]any{"job_type": "openclaw.configure_agent", "schema_version": 1, "payload": map[string]any{"agent_id": agent.ID, "url": "https://example.com"}}, http.StatusBadRequest, "forbidden_field"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, data := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, tc.body)
			if resp.StatusCode != tc.want || data["code"] != tc.code {
				t.Fatalf("status/body = %d %v, want %d code %s", resp.StatusCode, data, tc.want, tc.code)
			}
		})
	}
	if count := countAPIHelperJobs(t, s); count != 0 {
		t.Fatalf("rejected enqueue attempts inserted %d jobs, want 0", count)
	}
}

func TestHelperJobsEnqueueRejectsStaleAndRevokedEnrollmentsAndKeepsLaterRoutesUnmounted(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	enrollment, secret := createHelperEnrollmentViaAPI(t, ts.URL, ownerToken)
	enrollmentID := enrollment["enrollment_id"].(string)
	claimHelperEnrollmentViaAPI(t, ts.URL, enrollmentID, secret, "device-1")
	agent := seedHelperJobAgent(t, s, "owner@test.com", "api-stale-agent")
	seedHelperJobAgentConfig(t, s, agent.ID, 1, map[string]any{"name": "Stale Agent"})
	old := int64(1)
	if err := s.DB().Model(&store.HelperEnrollment{}).Where("id = ?", enrollmentID).Update("last_seen_at", old).Error; err != nil {
		t.Fatalf("seed stale last_seen_at: %v", err)
	}
	valid := map[string]any{"job_type": "openclaw.configure_agent", "schema_version": 1, "payload": map[string]any{"agent_id": agent.ID}}
	resp, stale := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, valid)
	if resp.StatusCode != http.StatusForbidden || stale["code"] != "stale_enrollment" {
		t.Fatalf("stale enrollment enqueue: status %d body %v", resp.StatusCode, stale)
	}

	fresh, freshSecret := createHelperEnrollmentViaAPI(t, ts.URL, ownerToken)
	freshID := fresh["enrollment_id"].(string)
	claimHelperEnrollmentViaAPI(t, ts.URL, freshID, freshSecret, "device-2")
	resp, revoked := testutil.JSON(t, http.MethodDelete, ts.URL+"/api/v1/helper/enrollments/"+freshID, ownerToken, nil)
	if resp.StatusCode != http.StatusOK || revoked["status"] != "revoked" {
		t.Fatalf("revoke fixture: status %d body %v", resp.StatusCode, revoked)
	}
	resp, revokedBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+freshID+"/jobs", ownerToken, valid)
	if resp.StatusCode != http.StatusForbidden || revokedBody["code"] != "revoked" {
		t.Fatalf("revoked enrollment enqueue: status %d body %v", resp.StatusCode, revokedBody)
	}

	for _, path := range []string{
		"/api/v1/helper/jobs/poll",
		"/api/v1/helper/enrollments/" + freshID + "/jobs/lease",
		"/api/v1/helper/enrollments/" + freshID + "/jobs/result",
		"/api/v1/helper/enrollments/" + freshID + "/jobs/ack",
		"/api/v1/helper/enrollments/" + freshID + "/jobs/logs",
		"/api/v1/helper/enrollments/" + freshID + "/service-lifecycle",
	} {
		resp, _ := testutil.JSON(t, http.MethodPost, ts.URL+path, ownerToken, map[string]any{})
		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("later-scope route %s should remain unmounted, got %d", path, resp.StatusCode)
		}
	}
}

func seedHelperJobAgent(t *testing.T, s *store.Store, ownerEmail, name string) *store.User {
	t.Helper()
	owner, err := s.GetUserByEmail(ownerEmail)
	if err != nil {
		t.Fatalf("GetUserByEmail(%s): %v", ownerEmail, err)
	}
	apiKey := name + "-key"
	agent := &store.User{DisplayName: name, Role: "agent", OwnerID: &owner.ID, APIKey: &apiKey, OrgID: owner.OrgID, PasswordHash: "hash"}
	if err := s.CreateUser(agent); err != nil {
		t.Fatalf("CreateUser agent: %v", err)
	}
	return agent
}

func seedHelperJobAgentConfig(t *testing.T, s *store.Store, agentID string, version int64, blob map[string]any) {
	t.Helper()
	b, err := json.Marshal(blob)
	if err != nil {
		t.Fatalf("marshal agent config: %v", err)
	}
	if err := s.DB().Exec(`INSERT INTO agent_configs (agent_id, schema_version, blob, created_at, updated_at) VALUES (?, ?, ?, 1, 1)`, agentID, version, string(b)).Error; err != nil {
		t.Fatalf("seed agent config: %v", err)
	}
}

func assertNoHelperJobSensitiveFields(t *testing.T, job map[string]any) {
	t.Helper()
	for _, key := range []string{
		"owner_user_id", "org_id", "helper_device_id", "payload", "payload_json",
		"manifest_binding_json", "credential", "credential_digest", "token", "result_summary_json",
	} {
		if _, ok := job[key]; ok {
			t.Fatalf("helper job response leaked field %q: %v", key, job)
		}
	}
}

func countAPIHelperJobs(t *testing.T, s *store.Store) int64 {
	t.Helper()
	var count int64
	if err := s.DB().Table("helper_jobs").Count(&count).Error; err != nil {
		t.Fatalf("count helper_jobs: %v", err)
	}
	return count
}
