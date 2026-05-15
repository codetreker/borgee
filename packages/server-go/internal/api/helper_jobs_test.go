package api_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

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

func TestHelperJobsEnqueueChannelBindingRequiresTargetAgentAccess(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	enrollment, secret := createHelperEnrollmentViaAPI(t, ts.URL, ownerToken)
	enrollmentID := enrollment["enrollment_id"].(string)
	claimHelperEnrollmentViaAPI(t, ts.URL, enrollmentID, secret, "device-1")
	agent := seedHelperJobAgent(t, s, "owner@test.com", "api-channel-agent")
	seedHelperJobAgentConfig(t, s, agent.ID, 1, map[string]any{"name": "Channel Agent"})
	privateChannel := testutil.CreateChannel(t, ts.URL, ownerToken, "helper-job-private", "private")
	privateChannelID := privateChannel["id"].(string)

	validWithChannel := map[string]any{
		"job_type":       "openclaw.configure_agent",
		"schema_version": 1,
		"payload":        map[string]any{"agent_id": agent.ID, "channel_id": privateChannelID},
	}
	resp, denied := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, validWithChannel)
	if resp.StatusCode != http.StatusForbidden || denied["code"] != "forbidden" {
		t.Fatalf("private channel without target agent access: status %d body %v", resp.StatusCode, denied)
	}
	if count := countAPIHelperJobs(t, s); count != 0 {
		t.Fatalf("denied channel binding inserted %d jobs, want 0", count)
	}

	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: privateChannelID, UserID: agent.ID}); err != nil {
		t.Fatalf("add agent channel member: %v", err)
	}
	resp, allowed := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, validWithChannel)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("private channel with target agent access: status %d body %v", resp.StatusCode, allowed)
	}
	job := allowed["job"].(map[string]any)
	if _, ok := job["payload_hash"]; ok {
		t.Fatalf("helper job response leaked internal payload_hash: %v", job)
	}
	if _, ok := job["manifest_digest"]; ok {
		t.Fatalf("helper job response leaked internal manifest_digest: %v", job)
	}
}

func TestHelperJobsEnqueueRejectsAgentAPIKeyAuthority(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	pluginAgent := seedHelperJobAgent(t, s, "owner@test.com", "api-plugin-authority")
	childAgent := seedHelperJobAgentForOwner(t, s, pluginAgent, "api-plugin-child")
	seedHelperJobAgentConfig(t, s, childAgent.ID, 1, map[string]any{"name": "Plugin Child"})
	legacyEnrollmentID := seedLegacyAgentOwnedHelperEnrollment(t, s, pluginAgent)

	resp, createBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments", *pluginAgent.APIKey, map[string]any{
		"host_label":         "Plugin-created host",
		"allowed_categories": []string{"openclaw_config"},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("agent API key must not create helper enrollments, got %d body %v", resp.StatusCode, createBody)
	}

	valid := map[string]any{"job_type": "openclaw.configure_agent", "schema_version": 1, "payload": map[string]any{"agent_id": childAgent.ID}}
	resp, enqueueBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+legacyEnrollmentID+"/jobs", *pluginAgent.APIKey, valid)
	if resp.StatusCode != http.StatusForbidden || enqueueBody["code"] != "forbidden" {
		t.Fatalf("agent API key must not enqueue helper jobs, got %d body %v", resp.StatusCode, enqueueBody)
	}
	if count := countAPIHelperJobs(t, s); count != 0 {
		t.Fatalf("agent-key enqueue inserted %d jobs, want 0", count)
	}

	ownerEnrollment, ownerSecret := createHelperEnrollmentViaAPI(t, ts.URL, ownerToken)
	ownerEnrollmentID := ownerEnrollment["enrollment_id"].(string)
	claimHelperEnrollmentViaAPI(t, ts.URL, ownerEnrollmentID, ownerSecret, "device-owner")
	resp, invalidBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+ownerEnrollmentID+"/jobs", *pluginAgent.APIKey, map[string]any{"owner_user_id": "client"})
	if resp.StatusCode != http.StatusForbidden || invalidBody["code"] != "forbidden" {
		t.Fatalf("agent API key should be rejected before envelope decode, got %d body %v", resp.StatusCode, invalidBody)
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
	owner, err := s.GetUserByEmail("owner@test.com")
	if err != nil {
		t.Fatalf("GetUserByEmail owner: %v", err)
	}
	remoteNode, err := s.CreateRemoteNode(owner.ID, "helper-job-remote-node")
	if err != nil {
		t.Fatalf("CreateRemoteNode: %v", err)
	}
	resp, grantBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/host-grants", ownerToken, map[string]any{
		"grant_type": "filesystem",
		"scope":      "/tmp",
		"ttl_kind":   "always",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("seed host grant: status %d body %v", resp.StatusCode, grantBody)
	}
	hostGrantID := grantBody["id"].(string)

	valid := map[string]any{"job_type": "openclaw.configure_agent", "schema_version": 1, "payload": map[string]any{"agent_id": agent.ID}}
	resp, _ = testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", "", valid)
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
	for name, token := range map[string]string{
		"remote_node_token": remoteNode.ConnectionToken,
		"host_grant_id":     hostGrantID,
	} {
		resp, _ := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", token, valid)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s must not authenticate helper job enqueue, got %d", name, resp.StatusCode)
		}
	}

	cases := []struct {
		name string
		body map[string]any
		want int
		code string
	}{
		{"unknown job type", map[string]any{"job_type": "command.run", "schema_version": 1, "payload": map[string]any{"agent_id": agent.ID}}, http.StatusBadRequest, "unknown_job_type"},
		{"recognized install type", map[string]any{"job_type": "openclaw.install_from_manifest", "schema_version": 1, "payload": map[string]any{"manifest_id": "server-owned"}}, http.StatusBadRequest, "manifest_required"},
		{"recognized plugin connection type", map[string]any{"job_type": "borgee_plugin.configure_connection", "schema_version": 1, "payload": map[string]any{"connection_id": "server-owned"}}, http.StatusBadRequest, "job_type_not_enabled"},
		{"recognized service lifecycle type", map[string]any{"job_type": "service.lifecycle", "schema_version": 1, "payload": map[string]any{"target": "openclaw"}}, http.StatusBadRequest, "job_type_not_enabled"},
		{"recognized state write type", map[string]any{"job_type": "state.write", "schema_version": 1, "payload": map[string]any{"state_id": "server-owned"}}, http.StatusBadRequest, "job_type_not_enabled"},
		{"recognized status collect type", map[string]any{"job_type": "status.collect", "schema_version": 1, "payload": map[string]any{"scope": "helper"}}, http.StatusBadRequest, "job_type_not_enabled"},
		{"recognized delegation revoke type", map[string]any{"job_type": "delegation.revoke", "schema_version": 1, "payload": map[string]any{"delegation_id": "server-owned"}}, http.StatusBadRequest, "job_type_not_enabled"},
		{"recognized helper uninstall type", map[string]any{"job_type": "helper.uninstall", "schema_version": 1, "payload": map[string]any{"scope": "helper"}}, http.StatusBadRequest, "job_type_not_enabled"},
		{"extra top field owner", map[string]any{"job_type": "openclaw.configure_agent", "schema_version": 1, "payload": map[string]any{"agent_id": agent.ID}, "owner_user_id": "client"}, http.StatusBadRequest, "extra_field"},
		{"client ttl", map[string]any{"job_type": "openclaw.configure_agent", "schema_version": 1, "payload": map[string]any{"agent_id": agent.ID}, "ttl": 999999}, http.StatusBadRequest, "ttl_invalid"},
		{"payload expires_at", map[string]any{"job_type": "openclaw.configure_agent", "schema_version": 1, "payload": map[string]any{"agent_id": agent.ID, "expires_at": 999999}}, http.StatusBadRequest, "ttl_invalid"},
		{"payload deadline", map[string]any{"job_type": "openclaw.configure_agent", "schema_version": 1, "payload": map[string]any{"agent_id": agent.ID, "deadline": 999999}}, http.StatusBadRequest, "ttl_invalid"},
		{"payload lease_expires_at", map[string]any{"job_type": "openclaw.configure_agent", "schema_version": 1, "payload": map[string]any{"agent_id": agent.ID, "lease_expires_at": 999999}}, http.StatusBadRequest, "ttl_invalid"},
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
	missingSeen, missingSecret := createHelperEnrollmentViaAPI(t, ts.URL, ownerToken)
	missingSeenID := missingSeen["enrollment_id"].(string)
	claimHelperEnrollmentViaAPI(t, ts.URL, missingSeenID, missingSecret, "device-missing-seen")
	if err := s.DB().Exec(`UPDATE helper_enrollments SET last_seen_at = NULL WHERE id = ?`, missingSeenID).Error; err != nil {
		t.Fatalf("seed missing last_seen_at: %v", err)
	}
	resp, missingSeenBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+missingSeenID+"/jobs", ownerToken, valid)
	if resp.StatusCode != http.StatusForbidden || missingSeenBody["code"] != "stale_enrollment" {
		t.Fatalf("missing last_seen_at enqueue: status %d body %v", resp.StatusCode, missingSeenBody)
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
		"/api/v1/helper/jobs/any-job/lease",
		"/api/v1/helper/jobs/any-job/result",
		"/api/v1/helper/jobs/any-job/ack",
		"/api/v1/helper/jobs/any-job/logs",
		"/api/v1/helper/enrollments/" + freshID + "/jobs/lease",
		"/api/v1/helper/enrollments/" + freshID + "/jobs/result",
		"/api/v1/helper/enrollments/" + freshID + "/jobs/ack",
		"/api/v1/helper/enrollments/" + freshID + "/jobs/logs",
		"/api/v1/helper/enrollments/" + freshID + "/jobs/install",
		"/api/v1/helper/enrollments/" + freshID + "/jobs/uninstall",
		"/api/v1/helper/enrollments/" + freshID + "/jobs/local-policy",
		"/api/v1/helper/enrollments/" + freshID + "/service-lifecycle",
	} {
		resp, _ := testutil.JSON(t, http.MethodPost, ts.URL+path, ownerToken, map[string]any{})
		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("later-scope route %s should remain unmounted, got %d", path, resp.StatusCode)
		}
	}
	resp, _ = testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/helper/enrollments/"+freshID+"/jobs", ownerToken, nil)
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("helper enqueue route should reject GET, got %d", resp.StatusCode)
	}
}

func seedHelperJobAgent(t *testing.T, s *store.Store, ownerEmail, name string) *store.User {
	t.Helper()
	owner, err := s.GetUserByEmail(ownerEmail)
	if err != nil {
		t.Fatalf("GetUserByEmail(%s): %v", ownerEmail, err)
	}
	return seedHelperJobAgentForOwner(t, s, owner, name)
}

func seedHelperJobAgentForOwner(t *testing.T, s *store.Store, owner *store.User, name string) *store.User {
	t.Helper()
	apiKey := name + "-key"
	agent := &store.User{DisplayName: name, Role: "agent", OwnerID: &owner.ID, APIKey: &apiKey, OrgID: owner.OrgID, PasswordHash: "hash"}
	if err := s.CreateUser(agent); err != nil {
		t.Fatalf("CreateUser agent: %v", err)
	}
	return agent
}

func seedLegacyAgentOwnedHelperEnrollment(t *testing.T, s *store.Store, owner *store.User) string {
	t.Helper()
	now := time.Now().UnixMilli()
	deviceID := "legacy-device-" + owner.ID
	digest := "sha256:legacy-digest"
	row := &store.HelperEnrollment{
		ID:                         "legacy-agent-enrollment-" + owner.ID,
		OwnerUserID:                owner.ID,
		OrgID:                      owner.OrgID,
		HostLabel:                  "Legacy agent-owned helper",
		HelperDeviceID:             &deviceID,
		AllowedCategories:          `["openclaw_config"]`,
		Status:                     "connected",
		LastSeenAt:                 &now,
		CreatedAt:                  now,
		UpdatedAt:                  now,
		ClaimedAt:                  &now,
		PersistentCredentialDigest: &digest,
		CredentialCreatedAt:        &now,
		CredentialGeneration:       1,
	}
	if err := s.DB().Create(row).Error; err != nil {
		t.Fatalf("seed legacy agent-owned helper enrollment: %v", err)
	}
	return row.ID
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
		"payload_hash", "manifest_digest",
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
