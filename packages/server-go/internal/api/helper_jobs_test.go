package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

func TestHelperJobsPollAckResultWithHelperCredential(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	enrollment, secret := createHelperEnrollmentViaAPI(t, ts.URL, ownerToken)
	enrollmentID := enrollment["enrollment_id"].(string)
	_, helperCredential := claimHelperEnrollmentViaAPI(t, ts.URL, enrollmentID, secret, "device-1")
	agent := seedHelperJobAgent(t, s, "owner@test.com", "api-poll-agent")
	seedHelperJobAgentConfig(t, s, agent.ID, 2, map[string]any{"name": "OpenClaw", "enabled": true})

	enqueueBody := map[string]any{
		"job_type":        "openclaw.configure_agent",
		"schema_version":  1,
		"payload":         map[string]any{"agent_id": agent.ID},
		"idempotency_key": "poll-ack-result-1",
	}
	resp, data := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, enqueueBody)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("enqueue helper job: status %d body %v", resp.StatusCode, data)
	}

	resp, wrongRail := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/poll", ownerToken, map[string]any{
		"helper_device_id": "device-1",
		"helper_platform":  "linux",
	})
	if resp.StatusCode != http.StatusUnauthorized || wrongRail["code"] != "unauthorized" {
		t.Fatalf("user token must not poll helper rail: status %d body %v", resp.StatusCode, wrongRail)
	}

	resp, poll := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/poll", helperCredential, map[string]any{
		"helper_device_id": "device-1",
		"helper_platform":  "linux",
		"wait_ms":          0,
	})
	if resp.StatusCode != http.StatusOK || poll["status"] != "leased" {
		t.Fatalf("helper poll should lease queued job: status %d body %v", resp.StatusCode, poll)
	}
	job := poll["job"].(map[string]any)
	jobID, _ := job["job_id"].(string)
	leaseToken, _ := job["lease_token"].(string)
	if jobID == "" || leaseToken == "" || job["status"] != "leased" || job["job_type"] != "openclaw.configure_agent" {
		t.Fatalf("leased job missing identity/lease fields: %v", job)
	}
	if job["enrollment_id"] != enrollmentID || job["schema_version"] != float64(1) || job["attempt"] != float64(1) {
		t.Fatalf("leased job missing enrollment/schema/attempt: %v", job)
	}
	if _, ok := job["lease_expires_at"].(float64); !ok {
		t.Fatalf("leased job missing lease_expires_at: %v", job)
	}
	payload := job["payload"].(map[string]any)
	if payload["agent_id"] != agent.ID || payload["config_schema_version"] != float64(2) {
		t.Fatalf("leased job payload should be safe effective payload: %v", payload)
	}
	assertNoHelperLeaseSensitiveFields(t, job)

	resp, secondPoll := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/poll", helperCredential, map[string]any{
		"helper_device_id": "device-1",
		"helper_platform":  "linux",
	})
	if resp.StatusCode != http.StatusOK || secondPoll["status"] != "no_work" || secondPoll["retry_after_ms"] == nil {
		t.Fatalf("second poll should not lease duplicate work: status %d body %v", resp.StatusCode, secondPoll)
	}

	ackBody := map[string]any{"helper_device_id": "device-1", "lease_token": leaseToken, "ack_status": "received"}
	resp, ack := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/"+jobID+"/ack", helperCredential, ackBody)
	if resp.StatusCode != http.StatusOK || ack["job"].(map[string]any)["status"] != "running" {
		t.Fatalf("ack should move leased job to running: status %d body %v", resp.StatusCode, ack)
	}
	resp, ackReplay := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/"+jobID+"/ack", helperCredential, ackBody)
	if resp.StatusCode != http.StatusOK || ackReplay["job"].(map[string]any)["status"] != "running" {
		t.Fatalf("ack replay should be idempotent: status %d body %v", resp.StatusCode, ackReplay)
	}

	resp, rawLogRejected := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/"+jobID+"/result", helperCredential, map[string]any{
		"helper_device_id": "device-1",
		"lease_token":      leaseToken,
		"status":           "failed",
		"failure_code":     "policy_denied",
		"result_summary": map[string]any{
			"raw_logs": "token=secret and private file content",
		},
	})
	if resp.StatusCode != http.StatusBadRequest || rawLogRejected["code"] != "forbidden_field" {
		t.Fatalf("raw logs must be rejected before terminal settlement: status %d body %v", resp.StatusCode, rawLogRejected)
	}

	resultBody := map[string]any{
		"helper_device_id": "device-1",
		"lease_token":      leaseToken,
		"status":           "failed",
		"failure_code":     "policy_denied",
		"failure_message":  "policy handoff denied",
		"result_summary": map[string]any{
			"audit_refs": []string{"audit-1"},
			"log_refs":   []string{},
		},
	}
	resp, result := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/"+jobID+"/result", helperCredential, resultBody)
	if resp.StatusCode != http.StatusOK || result["job"].(map[string]any)["status"] != "failed" || result["job"].(map[string]any)["failure_code"] != "policy_denied" {
		t.Fatalf("result should settle terminal failed state: status %d body %v", resp.StatusCode, result)
	}
	resultJob := result["job"].(map[string]any)
	if resultJob["failure_message"] != "policy handoff denied" {
		t.Fatalf("result response should expose bounded failure_message, got %v", resultJob)
	}
	resultSummary := resultJob["result_summary"].(map[string]any)
	if refs := resultSummary["audit_refs"].([]any); len(refs) != 1 || refs[0] != "audit-1" {
		t.Fatalf("result response should expose bounded audit refs, got %v", resultSummary)
	}
	if _, ok := resultJob["result_summary_json"]; ok {
		t.Fatalf("result response leaked raw result_summary_json: %v", resultJob)
	}
	resp, resultReplay := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/"+jobID+"/result", helperCredential, resultBody)
	if resp.StatusCode != http.StatusOK || resultReplay["job"].(map[string]any)["status"] != "failed" {
		t.Fatalf("same terminal result replay should be idempotent: status %d body %v", resp.StatusCode, resultReplay)
	}
	resultBody["failure_code"] = "execution_failed"
	resp, conflict := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/"+jobID+"/result", helperCredential, resultBody)
	if resp.StatusCode != http.StatusConflict || conflict["code"] != "terminal_conflict" {
		t.Fatalf("conflicting terminal replay should fail: status %d body %v", resp.StatusCode, conflict)
	}
}

func TestHelperJobsResultRedactsSensitiveFailureMessageInAPIResponse(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	enrollment, secret := createHelperEnrollmentViaAPI(t, ts.URL, ownerToken)
	enrollmentID := enrollment["enrollment_id"].(string)
	_, helperCredential := claimHelperEnrollmentViaAPI(t, ts.URL, enrollmentID, secret, "device-1")
	agent := seedHelperJobAgent(t, s, "owner@test.com", "api-redaction-agent")
	seedHelperJobAgentConfig(t, s, agent.ID, 2, map[string]any{"name": "OpenClaw", "enabled": true})

	resp, _ := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, map[string]any{
		"job_type":       "openclaw.configure_agent",
		"schema_version": 1,
		"payload":        map[string]any{"agent_id": agent.ID},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("enqueue helper job: status %d", resp.StatusCode)
	}
	resp, poll := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/poll", helperCredential, map[string]any{"helper_device_id": "device-1", "helper_platform": "linux"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("poll status %d body %v", resp.StatusCode, poll)
	}
	job := poll["job"].(map[string]any)
	jobID := job["job_id"].(string)
	leaseToken := job["lease_token"].(string)
	resp, _ = testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/"+jobID+"/ack", helperCredential, map[string]any{"helper_device_id": "device-1", "lease_token": leaseToken, "ack_status": "received"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ack status %d", resp.StatusCode)
	}

	resp, result := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/"+jobID+"/result", helperCredential, map[string]any{
		"helper_device_id": "device-1",
		"lease_token":      leaseToken,
		"status":           "failed",
		"failure_code":     "execution_failed",
		"failure_message":  "token=secret-token Authorization: Bearer auth-secret private file content /home/alice/.ssh/id_rsa env=OPENAI_API_KEY=sk-test",
		"result_summary":   map[string]any{"audit_refs": []string{"audit-1"}, "log_refs": []string{"log-1"}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("result status %d body %v", resp.StatusCode, result)
	}
	msg := result["job"].(map[string]any)["failure_message"].(string)
	for _, forbidden := range []string{"secret-token", "auth-secret", "private file content", "/home/alice/.ssh/id_rsa", "sk-test"} {
		if strings.Contains(msg, forbidden) {
			t.Fatalf("API failure_message leaked %q: %q", forbidden, msg)
		}
	}
	if !strings.Contains(msg, "[redacted]") {
		t.Fatalf("API failure_message should include redaction marker, got %q", msg)
	}

	resp, bad := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/"+jobID+"/result", helperCredential, map[string]any{
		"helper_device_id": "device-1",
		"lease_token":      leaseToken,
		"status":           "cancelled",
	})
	if resp.StatusCode != http.StatusBadRequest || bad["code"] != "schema_invalid" {
		t.Fatalf("terminal replay without reason should fail schema validation, status %d body %v", resp.StatusCode, bad)
	}
}

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

func TestHelperJobsEnqueueOpenClawInstallLeaseCarriesServerManifestBinding(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	resp, created := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments", ownerToken, map[string]any{
		"host_label":         "Mac Studio",
		"allowed_categories": []string{"openclaw_lifecycle"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create lifecycle enrollment: status %d body %v", resp.StatusCode, created)
	}
	enrollment := created["enrollment"].(map[string]any)
	enrollmentID := enrollment["enrollment_id"].(string)
	secret := created["enrollment_secret"].(string)
	_, helperCredential := claimHelperEnrollmentViaAPI(t, ts.URL, enrollmentID, secret, "device-install")

	resp, data := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, map[string]any{
		"job_type":        "openclaw.install_from_manifest",
		"schema_version":  1,
		"payload":         map[string]any{"runtime": "openclaw"},
		"idempotency_key": "install-openclaw-api-1",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("enqueue install job: status %d body %v", resp.StatusCode, data)
	}
	job := data["job"].(map[string]any)
	if job["category"] != "openclaw_lifecycle" || job["job_type"] != "openclaw.install_from_manifest" {
		t.Fatalf("install enqueue response had wrong category/type: %v", job)
	}
	assertNoHelperJobSensitiveFields(t, job)

	resp, poll := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/poll", helperCredential, map[string]any{
		"helper_device_id": "device-install",
		"helper_platform":  "linux",
	})
	if resp.StatusCode != http.StatusOK || poll["status"] != "leased" {
		t.Fatalf("poll install job: status %d body %v", resp.StatusCode, poll)
	}
	leased := poll["job"].(map[string]any)
	if leased["job_type"] != "openclaw.install_from_manifest" || leased["manifest_digest"] == nil {
		t.Fatalf("leased install job missing type/digest: %v", leased)
	}
	payload := leased["payload"].(map[string]any)
	if payload["install_plan_id"] != "openclaw-plugin-v1" {
		t.Fatalf("leased install payload should be server-owned effective payload: %v", payload)
	}
	binding := leased["manifest_binding"].(map[string]any)
	if binding["manifest_digest"] != leased["manifest_digest"] {
		t.Fatalf("manifest binding digest mismatch: leased=%v binding=%v", leased, binding)
	}
	assertAnyStringSet(t, "artifact_ids", binding["artifact_ids"], []string{"openclaw-plugin"})
	assertAnyStringSet(t, "path_ids", binding["path_ids"], []string{"openclaw_install", "openclaw_agent_config"})
	assertAnyStringSet(t, "domains", binding["domains"], []string{"https://cdn.borgee.io"})
	if serviceIDs, ok := binding["service_ids"]; ok {
		t.Fatalf("Task9 install binding must not grant service ids, got %v", serviceIDs)
	}
	assertNoHelperLeaseSensitiveFields(t, leased)
	if count := countAPIHelperJobs(t, s); count != 1 {
		t.Fatalf("install enqueue inserted %d jobs, want 1", count)
	}
}

func TestHelperJobsEnqueueServiceLifecycleLeaseCarriesDeclaredServiceID(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	resp, created := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments", ownerToken, map[string]any{
		"host_label":         "Mac Studio",
		"allowed_categories": []string{"openclaw_lifecycle"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create lifecycle enrollment: status %d body %v", resp.StatusCode, created)
	}
	enrollment := created["enrollment"].(map[string]any)
	enrollmentID := enrollment["enrollment_id"].(string)
	secret := created["enrollment_secret"].(string)
	_, helperCredential := claimHelperEnrollmentViaAPI(t, ts.URL, enrollmentID, secret, "device-service")

	resp, data := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, map[string]any{
		"job_type":        "service.lifecycle",
		"schema_version":  1,
		"payload":         map[string]any{"target": "openclaw", "operation": "restart"},
		"idempotency_key": "restart-openclaw-api-1",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("enqueue service lifecycle job: status %d body %v", resp.StatusCode, data)
	}
	job := data["job"].(map[string]any)
	if job["category"] != "openclaw_lifecycle" || job["job_type"] != "service.lifecycle" {
		t.Fatalf("service lifecycle enqueue response had wrong category/type: %v", job)
	}
	assertNoHelperJobSensitiveFields(t, job)

	resp, poll := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/poll", helperCredential, map[string]any{
		"helper_device_id": "device-service",
		"helper_platform":  "linux",
	})
	if resp.StatusCode != http.StatusOK || poll["status"] != "leased" {
		t.Fatalf("poll service lifecycle job: status %d body %v", resp.StatusCode, poll)
	}
	leased := poll["job"].(map[string]any)
	if leased["job_type"] != "service.lifecycle" || leased["manifest_digest"] == nil {
		t.Fatalf("leased service lifecycle job missing type/digest: %v", leased)
	}
	payload := leased["payload"].(map[string]any)
	if payload["operation"] != "restart" {
		t.Fatalf("leased service lifecycle payload should be server-owned restart operation: %v", payload)
	}
	for _, forbidden := range []string{"target", "service_id", "service_ids", "service_unit", "command", "shell", "argv"} {
		if _, ok := payload[forbidden]; ok {
			t.Fatalf("leased service lifecycle payload leaked %q: %v", forbidden, payload)
		}
	}
	binding := leased["manifest_binding"].(map[string]any)
	if binding["manifest_digest"] != leased["manifest_digest"] {
		t.Fatalf("manifest binding digest mismatch: leased=%v binding=%v", leased, binding)
	}
	assertAnyStringSet(t, "service_ids", binding["service_ids"], []string{"openclaw-user"})
	if _, ok := binding["path_ids"]; ok {
		t.Fatalf("service lifecycle binding must not grant path ids: %v", binding)
	}
	if count := countAPIHelperJobs(t, s); count != 1 {
		t.Fatalf("service lifecycle enqueue inserted %d jobs, want 1", count)
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

func TestHelperJobsEnqueuePluginConfigureConnectionRequiresChannelAuthority(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	enrollment, secret := createHelperEnrollmentViaAPI(t, ts.URL, ownerToken)
	enrollmentID := enrollment["enrollment_id"].(string)
	_, helperCredential := claimHelperEnrollmentViaAPI(t, ts.URL, enrollmentID, secret, "device-plugin")
	agent := seedHelperJobAgent(t, s, "owner@test.com", "api-plugin-bound-agent")
	privateChannel := testutil.CreateChannel(t, ts.URL, ownerToken, "helper-job-plugin-private", "private")
	privateChannelID := privateChannel["id"].(string)

	valid := map[string]any{
		"job_type":       "borgee_plugin.configure_connection",
		"schema_version": 1,
		"payload":        map[string]any{"agent_id": agent.ID, "channel_id": privateChannelID},
	}
	resp, denied := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, valid)
	if resp.StatusCode != http.StatusForbidden || denied["code"] != "forbidden" {
		t.Fatalf("plugin binding without target agent channel access: status %d body %v", resp.StatusCode, denied)
	}
	if count := countAPIHelperJobs(t, s); count != 0 {
		t.Fatalf("denied plugin binding inserted %d jobs, want 0", count)
	}

	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: privateChannelID, UserID: agent.ID}); err != nil {
		t.Fatalf("add agent channel member: %v", err)
	}
	resp, allowed := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, valid)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("plugin binding with target agent channel access: status %d body %v", resp.StatusCode, allowed)
	}
	job := allowed["job"].(map[string]any)
	if job["job_type"] != "borgee_plugin.configure_connection" || job["category"] != "openclaw_config" {
		t.Fatalf("unexpected plugin binding job response: %v", job)
	}
	assertNoHelperJobSensitiveFields(t, job)

	leaseResp, leaseBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/poll", helperCredential, map[string]any{"helper_device_id": "device-plugin", "helper_platform": "linux"})
	if leaseResp.StatusCode != http.StatusOK {
		t.Fatalf("poll plugin binding job: status %d body %v", leaseResp.StatusCode, leaseBody)
	}
	leased := leaseBody["job"].(map[string]any)
	payload := leased["payload"].(map[string]any)
	if payload["agent_id"] != agent.ID || payload["channel_id"] != privateChannelID {
		t.Fatalf("leased plugin payload lost channel binding: %v", payload)
	}
	connectionID, _ := payload["connection_id"].(string)
	if !strings.HasPrefix(connectionID, "borgee-plugin:") {
		t.Fatalf("leased plugin payload missing server-owned connection id: %v", payload)
	}
	binding := leased["manifest_binding"].(map[string]any)
	assertAnyStringSet(t, "path_ids", binding["path_ids"], []string{"borgee_plugin_config"})
	assertNoHelperLeaseSensitiveFields(t, leased)
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
		{"install client manifest authority", map[string]any{"job_type": "openclaw.install_from_manifest", "schema_version": 1, "payload": map[string]any{"runtime": "openclaw", "manifest_id": "client"}}, http.StatusBadRequest, "forbidden_field"},
		{"plugin connection client authority", map[string]any{"job_type": "borgee_plugin.configure_connection", "schema_version": 1, "payload": map[string]any{"connection_id": "server-owned"}}, http.StatusBadRequest, "forbidden_field"},
		{"service lifecycle requires lifecycle delegation", map[string]any{"job_type": "service.lifecycle", "schema_version": 1, "payload": map[string]any{"target": "openclaw"}}, http.StatusForbidden, "delegation_denied"},
		{"state write payload schema invalid", map[string]any{"job_type": "state.write", "schema_version": 1, "payload": map[string]any{"state_id": "server-owned"}}, http.StatusBadRequest, "schema_invalid"},
		{"status collect requires scope", map[string]any{"job_type": "status.collect", "schema_version": 1, "payload": map[string]any{"scope": ""}}, http.StatusBadRequest, "schema_invalid"},
		{"delegation revoke requires helper-lifecycle delegation", map[string]any{"job_type": "delegation.revoke", "schema_version": 1, "payload": map[string]any{"target_category": "openclaw_config"}}, http.StatusForbidden, "delegation_denied"},
		{"helper uninstall rejects wrong scope", map[string]any{"job_type": "helper.uninstall", "schema_version": 1, "payload": map[string]any{"scope": "agent"}}, http.StatusForbidden, "delegation_denied"},
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

	resp, lifecycleCreated := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments", ownerToken, map[string]any{
		"host_label":         "Lifecycle reject host",
		"allowed_categories": []string{"openclaw_lifecycle"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create lifecycle enrollment: status %d body %v", resp.StatusCode, lifecycleCreated)
	}
	lifecycleEnrollment := lifecycleCreated["enrollment"].(map[string]any)
	lifecycleEnrollmentID := lifecycleEnrollment["enrollment_id"].(string)
	lifecycleSecret := lifecycleCreated["enrollment_secret"].(string)
	claimHelperEnrollmentViaAPI(t, ts.URL, lifecycleEnrollmentID, lifecycleSecret, "device-lifecycle-reject")

	lifecycleInvalidCases := []struct {
		name string
		body map[string]any
		want int
		code string
	}{
		{"service lifecycle requires closed payload", map[string]any{"job_type": "service.lifecycle", "schema_version": 1, "payload": map[string]any{"target": "openclaw"}}, http.StatusBadRequest, "schema_invalid"},
		{"service lifecycle rejects client service id", map[string]any{"job_type": "service.lifecycle", "schema_version": 1, "payload": map[string]any{"target": "openclaw", "operation": "restart", "service_id": "evil"}}, http.StatusBadRequest, "forbidden_field"},
		{"service lifecycle rejects client service unit", map[string]any{"job_type": "service.lifecycle", "schema_version": 1, "payload": map[string]any{"target": "openclaw", "operation": "restart", "service_unit": "evil.service"}}, http.StatusBadRequest, "forbidden_field"},
	}
	for _, tc := range lifecycleInvalidCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, data := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+lifecycleEnrollmentID+"/jobs", ownerToken, tc.body)
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
	resp, missingEnrollmentBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/missing-helper-enrollment/jobs", ownerToken, valid)
	if resp.StatusCode != http.StatusNotFound || missingEnrollmentBody["code"] != "not_found" {
		t.Fatalf("missing enrollment enqueue: status %d body %v", resp.StatusCode, missingEnrollmentBody)
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

	uninstalled, uninstallSecret := createHelperEnrollmentViaAPI(t, ts.URL, ownerToken)
	uninstalledID := uninstalled["enrollment_id"].(string)
	_, uninstallCredential := claimHelperEnrollmentViaAPI(t, ts.URL, uninstalledID, uninstallSecret, "device-uninstalled")
	resp, uninstallBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+uninstalledID+"/uninstall", uninstallCredential, map[string]any{
		"helper_device_id": "device-uninstalled",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("uninstall fixture: status %d body %v", resp.StatusCode, uninstallBody)
	}
	resp, uninstalledBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+uninstalledID+"/jobs", ownerToken, valid)
	if resp.StatusCode != http.StatusForbidden || uninstalledBody["code"] != "uninstalled" {
		t.Fatalf("uninstalled enrollment enqueue: status %d body %v", resp.StatusCode, uninstalledBody)
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

func assertNoHelperLeaseSensitiveFields(t *testing.T, job map[string]any) {
	t.Helper()
	for _, key := range []string{
		"payload_json",
		// manifest_binding_json (raw) is intentionally emitted on the lease
		// per PR-3 #1041 so the daemon's no-root executors get byte-stable
		// bytes for manifestpath.Resolve. The binding has no secrets — same
		// PathIDs/ArtifactIDs/Domains/ServiceIDs as the structured
		// `manifest_binding` field already exposed.
		//
		// PR-4 amend gap #1: owner_user_id / org_id / helper_device_id /
		// payload_hash / expires_at are now intentionally emitted on the
		// lease so the daemon's jobpolicy.validateJobSchema receives a
		// complete envelope. The helper's WS credential already authenticates
		// it for (owner, org, enrollment, device) — echoing those IDs back
		// to the same authenticated peer is not a leak; the gate the test
		// originally guarded was the human owner-token poll path (HTTP 401
		// is asserted separately above before the helper-credential poll
		// runs).
		"credential", "credentials", "credential_digest", "persistent_credential_digest", "token",
		"result_summary_json", "idempotency_key",
	} {
		if _, ok := job[key]; ok {
			t.Fatalf("helper lease response leaked field %q: %v", key, job)
		}
	}
	payload, _ := job["payload"].(map[string]any)
	for _, key := range []string{"owner_user_id", "org_id", "credential", "credentials", "token", "shell", "argv", "command", "script", "service_unit", "path", "domain", "url"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("helper lease payload leaked field %q: %v", key, payload)
		}
	}
}

func assertAnyStringSet(t *testing.T, label string, raw any, want []string) {
	t.Helper()
	gotRaw, ok := raw.([]any)
	if !ok {
		t.Fatalf("%s was not an array: %v", label, raw)
	}
	if len(gotRaw) != len(want) {
		t.Fatalf("%s got %v, want %v", label, gotRaw, want)
	}
	seen := map[string]bool{}
	for _, item := range gotRaw {
		value, ok := item.(string)
		if !ok {
			t.Fatalf("%s item was not string: %v", label, gotRaw)
		}
		seen[value] = true
	}
	for _, value := range want {
		if !seen[value] {
			t.Fatalf("%s got %v, missing %q", label, gotRaw, value)
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

// TestHelperJobsEnqueueHelperUninstallAcceptsAndCarriesManifestBinding —
// THUJ-1 (#998): with the `helper.uninstall` taxonomy row flipped to
// Enabled=true, an owner-rail enqueue against a `helper_lifecycle`-allowed
// enrollment with a well-formed `{"scope":"helper"}` payload must be
// accepted, persist as `queued` with category `helper_lifecycle`, and the
// helper-side lease must include a manifest binding declaring the helper's
// own state-path / runtime-path / service-id ids so the helper's
// jobpolicy.Evaluate gate accepts the leased uninstall job.
func TestHelperJobsEnqueueHelperUninstallAcceptsAndCarriesManifestBinding(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	resp, created := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments", ownerToken, map[string]any{
		"host_label":         "Uninstall fixture host",
		"allowed_categories": []string{"helper_lifecycle"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create lifecycle enrollment: status %d body %v", resp.StatusCode, created)
	}
	enrollment := created["enrollment"].(map[string]any)
	enrollmentID := enrollment["enrollment_id"].(string)
	secret := created["enrollment_secret"].(string)
	_, helperCredential := claimHelperEnrollmentViaAPI(t, ts.URL, enrollmentID, secret, "device-uninstall-1")

	// THUJ-1a: well-formed payload accepted, persisted queued + helper_lifecycle.
	resp, accepted := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, map[string]any{
		"job_type":       "helper.uninstall",
		"schema_version": 1,
		"payload":        map[string]any{"scope": "helper"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("accepted uninstall enqueue: status %d body %v", resp.StatusCode, accepted)
	}
	job := accepted["job"].(map[string]any)
	if job["job_type"] != "helper.uninstall" || job["category"] != "helper_lifecycle" || job["status"] != "queued" {
		t.Fatalf("unexpected accepted uninstall job: %v", job)
	}

	// THUJ-1b: helper lease carries manifest binding with path + service ids.
	resp, leased := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/poll", helperCredential, map[string]any{
		"helper_device_id": "device-uninstall-1",
		"helper_platform":  "linux",
		"wait_ms":          0,
	})
	if resp.StatusCode != http.StatusOK || leased["status"] != "leased" {
		t.Fatalf("helper poll lease: status %d body %v", resp.StatusCode, leased)
	}
	leasedJob := leased["job"].(map[string]any)
	binding, ok := leasedJob["manifest_binding"].(map[string]any)
	if !ok {
		t.Fatalf("uninstall lease missing manifest_binding: %v", leasedJob)
	}
	if digest, _ := binding["manifest_digest"].(string); digest == "" {
		t.Fatalf("uninstall lease manifest_binding missing digest: %v", binding)
	}
	assertAnyStringSet(t, "path_ids", binding["path_ids"], []string{"helper_state", "helper_runtime"})
	assertAnyStringSet(t, "service_ids", binding["service_ids"], []string{"borgee-helper-service"})
	payload := leasedJob["payload"].(map[string]any)
	if payload["scope"] != "helper" {
		t.Fatalf("uninstall lease payload missing scope=helper: %v", payload)
	}
	if count := countAPIHelperJobs(t, s); count != 1 {
		t.Fatalf("expected 1 enqueued uninstall job, got %d", count)
	}
}

// TestHelperJobsEnqueueHelperUninstallRejectsInvalidPayload — THUJ-2 (#998):
// payload schema check must reject malformed uninstall requests with
// schema_invalid (wrong scope, extra unknown field) or forbidden_field
// (e.g. operator tries to pass a `path` override). Reject-path must NOT
// persist any rows.
func TestHelperJobsEnqueueHelperUninstallRejectsInvalidPayload(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	resp, created := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments", ownerToken, map[string]any{
		"host_label":         "Uninstall reject fixture",
		"allowed_categories": []string{"helper_lifecycle"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create lifecycle enrollment: status %d body %v", resp.StatusCode, created)
	}
	enrollment := created["enrollment"].(map[string]any)
	enrollmentID := enrollment["enrollment_id"].(string)
	secret := created["enrollment_secret"].(string)
	claimHelperEnrollmentViaAPI(t, ts.URL, enrollmentID, secret, "device-uninstall-reject")

	cases := []struct {
		name string
		body map[string]any
		want int
		code string
	}{
		{"missing scope", map[string]any{"job_type": "helper.uninstall", "schema_version": 1, "payload": map[string]any{}}, http.StatusBadRequest, "schema_invalid"},
		{"wrong scope", map[string]any{"job_type": "helper.uninstall", "schema_version": 1, "payload": map[string]any{"scope": "runtime"}}, http.StatusBadRequest, "schema_invalid"},
		{"unknown payload field", map[string]any{"job_type": "helper.uninstall", "schema_version": 1, "payload": map[string]any{"scope": "helper", "extra": true}}, http.StatusBadRequest, "schema_invalid"},
		{"forbidden path override", map[string]any{"job_type": "helper.uninstall", "schema_version": 1, "payload": map[string]any{"scope": "helper", "path": "/etc/passwd"}}, http.StatusBadRequest, "forbidden_field"},
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
		t.Fatalf("rejected uninstall enqueue persisted %d rows, want 0", count)
	}
}

// TestHelperJobsHelperUninstallTerminalSucceededMarksEnrollmentUninstalled —
// THUJ-3 (#998): when the helper posts terminal `succeeded` for a
// `helper.uninstall` job, the same transaction flips the enrollment status
// to `uninstalled` so subsequent enqueues / polls / status reads see the
// correct server-recorded lifecycle state. Non-succeeded terminals (failed)
// must NOT flip the enrollment so an operator can retry.
func TestHelperJobsHelperUninstallTerminalSucceededMarksEnrollmentUninstalled(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	// Failure-first fixture: failed terminal must leave enrollment alone.
	failEnrollment, failSecret := uninstallFixtureEnrollment(t, ts.URL, ownerToken, "Failure host")
	failEnrollmentID := failEnrollment["enrollment_id"].(string)
	_, failCredential := claimHelperEnrollmentViaAPI(t, ts.URL, failEnrollmentID, failSecret, "device-fail")
	_, failLeaseToken, failJobID := enqueueAndLeaseUninstall(t, ts.URL, ownerToken, failCredential, failEnrollmentID, "device-fail")
	failResp, failBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+failEnrollmentID+"/jobs/"+failJobID+"/result", failCredential, map[string]any{
		"helper_device_id": "device-fail",
		"lease_token":      failLeaseToken,
		"status":           "failed",
		"failure_code":     "execution_failed",
		"failure_message":  "simulated executor crash",
	})
	if failResp.StatusCode != http.StatusOK {
		t.Fatalf("failed terminal post: status %d body %v", failResp.StatusCode, failBody)
	}
	if got := loadEnrollmentStatus(t, s, failEnrollmentID); got == "uninstalled" {
		t.Fatalf("failed terminal must NOT mark enrollment uninstalled, got status=%s", got)
	}

	// Success path: terminal succeeded flips enrollment.uninstalled +
	// subsequent enqueue is rejected with uninstalled.
	okEnrollment, okSecret := uninstallFixtureEnrollment(t, ts.URL, ownerToken, "Success host")
	okEnrollmentID := okEnrollment["enrollment_id"].(string)
	_, okCredential := claimHelperEnrollmentViaAPI(t, ts.URL, okEnrollmentID, okSecret, "device-ok")
	_, okLeaseToken, okJobID := enqueueAndLeaseUninstall(t, ts.URL, ownerToken, okCredential, okEnrollmentID, "device-ok")
	okResp, okBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+okEnrollmentID+"/jobs/"+okJobID+"/result", okCredential, map[string]any{
		"helper_device_id": "device-ok",
		"lease_token":      okLeaseToken,
		"status":           "succeeded",
	})
	if okResp.StatusCode != http.StatusOK {
		t.Fatalf("succeeded terminal post: status %d body %v", okResp.StatusCode, okBody)
	}
	if got := loadEnrollmentStatus(t, s, okEnrollmentID); got != "uninstalled" {
		t.Fatalf("succeeded uninstall must mark enrollment uninstalled, got status=%s", got)
	}
	// Re-enqueue against an uninstalled enrollment is rejected.
	resp, body := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+okEnrollmentID+"/jobs", ownerToken, map[string]any{
		"job_type":       "helper.uninstall",
		"schema_version": 1,
		"payload":        map[string]any{"scope": "helper"},
	})
	if resp.StatusCode != http.StatusForbidden || body["code"] != "uninstalled" {
		t.Fatalf("post-uninstall enqueue must be rejected with uninstalled: status %d body %v", resp.StatusCode, body)
	}
}

func uninstallFixtureEnrollment(t *testing.T, baseURL, ownerToken, label string) (map[string]any, string) {
	t.Helper()
	resp, body := testutil.JSON(t, http.MethodPost, baseURL+"/api/v1/helper/enrollments", ownerToken, map[string]any{
		"host_label":         label,
		"allowed_categories": []string{"helper_lifecycle"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create uninstall fixture %q: status %d body %v", label, resp.StatusCode, body)
	}
	return body["enrollment"].(map[string]any), body["enrollment_secret"].(string)
}

func enqueueAndLeaseUninstall(t *testing.T, baseURL, ownerToken, helperCredential, enrollmentID, deviceID string) (map[string]any, string, string) {
	t.Helper()
	resp, accepted := testutil.JSON(t, http.MethodPost, baseURL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, map[string]any{
		"job_type":       "helper.uninstall",
		"schema_version": 1,
		"payload":        map[string]any{"scope": "helper"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("enqueue uninstall: status %d body %v", resp.StatusCode, accepted)
	}
	resp, leased := testutil.JSON(t, http.MethodPost, baseURL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/poll", helperCredential, map[string]any{
		"helper_device_id": deviceID,
		"helper_platform":  "linux",
		"wait_ms":          0,
	})
	if resp.StatusCode != http.StatusOK || leased["status"] != "leased" {
		t.Fatalf("poll uninstall: status %d body %v", resp.StatusCode, leased)
	}
	job := leased["job"].(map[string]any)
	leaseToken := job["lease_token"].(string)
	jobID := job["job_id"].(string)
	// Ack so the job transitions leased -> running before /result.
	resp, ackBody := testutil.JSON(t, http.MethodPost, baseURL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/"+jobID+"/ack", helperCredential, map[string]any{
		"helper_device_id": deviceID,
		"lease_token":      leaseToken,
		"ack_status":       "received",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ack uninstall: status %d body %v", resp.StatusCode, ackBody)
	}
	return job, leaseToken, jobID
}

func loadEnrollmentStatus(t *testing.T, s *store.Store, enrollmentID string) string {
	t.Helper()
	var row store.HelperEnrollment
	if err := s.DB().Where("id = ?", enrollmentID).First(&row).Error; err != nil {
		t.Fatalf("load enrollment %s: %v", enrollmentID, err)
	}
	return row.Status
}
