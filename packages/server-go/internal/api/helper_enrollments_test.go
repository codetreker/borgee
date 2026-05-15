package api_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

func createHelperEnrollmentViaAPI(t *testing.T, baseURL, token string) (map[string]any, string) {
	t.Helper()
	resp, body := testutil.JSON(t, http.MethodPost, baseURL+"/api/v1/helper/enrollments", token, map[string]any{
		"host_label":         "Mac Studio",
		"allowed_categories": []string{"openclaw_config", "status_collect"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create helper enrollment: status %d body %v", resp.StatusCode, body)
	}
	enrollment, ok := body["enrollment"].(map[string]any)
	if !ok {
		t.Fatalf("missing enrollment object: %v", body)
	}
	secret, ok := body["enrollment_secret"].(string)
	if !ok || secret == "" {
		t.Fatalf("missing one-time enrollment secret: %v", body)
	}
	return enrollment, secret
}

func claimHelperEnrollmentViaAPI(t *testing.T, baseURL, enrollmentID, secret, deviceID string) (map[string]any, string) {
	t.Helper()
	resp, body := testutil.JSON(t, http.MethodPost, baseURL+"/api/v1/helper/enrollments/"+enrollmentID+"/claim", "", map[string]any{
		"enrollment_secret": secret,
		"helper_device_id":  deviceID,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("claim helper enrollment: status %d body %v", resp.StatusCode, body)
	}
	enrollment, ok := body["enrollment"].(map[string]any)
	if !ok {
		t.Fatalf("missing enrollment object: %v", body)
	}
	credential, ok := body["helper_credential"].(string)
	if !ok || credential == "" {
		t.Fatalf("missing persistent helper credential: %v", body)
	}
	return enrollment, credential
}

func rotateHelperCredentialViaAPI(t *testing.T, baseURL, enrollmentID, credential, deviceID string) (map[string]any, string) {
	t.Helper()
	resp, body := testutil.JSON(t, http.MethodPost, baseURL+"/api/v1/helper/enrollments/"+enrollmentID+"/rotate-credential", credential, map[string]any{
		"helper_device_id": deviceID,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rotate helper credential: status %d body %v", resp.StatusCode, body)
	}
	enrollment, ok := body["enrollment"].(map[string]any)
	if !ok {
		t.Fatalf("missing enrollment object: %v", body)
	}
	newCredential, ok := body["helper_credential"].(string)
	if !ok || newCredential == "" {
		t.Fatalf("missing rotated helper credential: %v", body)
	}
	if newCredential == credential {
		t.Fatalf("rotated credential matched old credential")
	}
	return enrollment, newCredential
}

func assertNoSensitiveHelperFields(t *testing.T, m map[string]any) {
	t.Helper()
	for _, key := range []string{
		"org_id",
		"owner_user_id",
		"enrollment_secret_digest",
		"persistent_credential_digest",
		"credential_digest",
		"connection_token",
	} {
		if _, ok := m[key]; ok {
			t.Fatalf("sensitive/internal field %q leaked in %v", key, m)
		}
	}
}

func TestHelperEnrollmentsUserRailCRUDRedactionAndCategoryValidation(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	enrollment, secret := createHelperEnrollmentViaAPI(t, ts.URL, ownerToken)
	if enrollment["status"] != "pending" {
		t.Fatalf("status=%v, want pending", enrollment["status"])
	}
	assertNoSensitiveHelperFields(t, enrollment)
	if _, ok := enrollment["enrollment_secret"]; ok {
		t.Fatalf("enrollment_secret must be top-level one-time response only, not inside enrollment: %v", enrollment)
	}
	if secret == "" {
		t.Fatal("secret should not be empty")
	}
	enrollmentID := enrollment["enrollment_id"].(string)

	resp, listBody := testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/helper/enrollments", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list helper enrollments: status %d body %v", resp.StatusCode, listBody)
	}
	items, ok := listBody["enrollments"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("expected non-empty enrollments list: %v", listBody)
	}
	assertNoSensitiveHelperFields(t, items[0].(map[string]any))

	resp, getBody := testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID, ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get helper enrollment: status %d body %v", resp.StatusCode, getBody)
	}
	assertNoSensitiveHelperFields(t, getBody["enrollment"].(map[string]any))

	resp, _ = testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID, memberToken, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong owner GET should be 403, got %d", resp.StatusCode)
	}

	resp, badBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments", ownerToken, map[string]any{
		"host_label":         "Mac Studio",
		"allowed_categories": []string{"shell"},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid category should be 400, got %d body %v", resp.StatusCode, badBody)
	}

	resp, delBody := testutil.JSON(t, http.MethodDelete, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID, ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete/revoke helper enrollment: status %d body %v", resp.StatusCode, delBody)
	}
	if delBody["status"] != "revoked" {
		t.Fatalf("DELETE should revoke, got %v", delBody)
	}
}

func TestHelperEnrollmentsConfigureOpenClawProjectionTruthfulTerminalStates(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, err := s.GetUserByEmail("owner@test.com")
	if err != nil {
		t.Fatalf("GetUserByEmail owner: %v", err)
	}

	enrollmentID, helperCredential := createClaimedHelperEnrollmentWithCategories(t, ts.URL, ownerToken, []string{"openclaw_config", "openclaw_lifecycle"})
	now := time.UnixMilli(1778840000000)
	seedConfigureOpenClawJob(t, s, owner, enrollmentID, "job-install", "openclaw.install_from_manifest", "openclaw_lifecycle", "succeeded", "", "", nil, now)
	seedConfigureOpenClawJob(t, s, owner, enrollmentID, "job-config", "openclaw.configure_agent", "openclaw_config", "succeeded", "", "", nil, now.Add(time.Second))
	seedConfigureOpenClawJob(t, s, owner, enrollmentID, "job-plugin", "borgee_plugin.configure_connection", "openclaw_config", "succeeded", "", "", nil, now.Add(2*time.Second))

	projection := fetchEnrollmentConfigureOpenClawProjection(t, ts.URL, ownerToken, enrollmentID)
	if projection["state"] == "succeeded" {
		t.Fatalf("Configure OpenClaw must not succeed before service.lifecycle closure: %v", projection)
	}
	if projection["state"] != "manual_debug" {
		t.Fatalf("incomplete terminal chain state=%v, want manual_debug: %v", projection["state"], projection)
	}

	seedConfigureOpenClawJob(t, s, owner, enrollmentID, "job-service", "service.lifecycle", "openclaw_lifecycle", "succeeded", "", "", nil, now.Add(3*time.Second))
	projection = fetchEnrollmentConfigureOpenClawProjection(t, ts.URL, ownerToken, enrollmentID)
	if projection["state"] != "succeeded" {
		t.Fatalf("all closure jobs succeeded state=%v, want succeeded: %v", projection["state"], projection)
	}
	detailProjection := fetchEnrollmentConfigureOpenClawDetailProjection(t, ts.URL, ownerToken, enrollmentID)
	if detailProjection["state"] != "succeeded" {
		t.Fatalf("detail route closure state=%v, want succeeded: %v", detailProjection["state"], detailProjection)
	}
	if projection["label"] != "Configure OpenClaw complete" {
		t.Fatalf("success label should be explicit terminal Configure OpenClaw completion: %v", projection)
	}
	if _, ok := projection["payload_hash"]; ok {
		t.Fatalf("projection leaked payload_hash: %v", projection)
	}
	if _, ok := projection["manifest_digest"]; ok {
		t.Fatalf("projection leaked manifest_digest: %v", projection)
	}
	if helperCredential == "" {
		t.Fatal("helper credential fixture should not be empty")
	}
}

func TestHelperEnrollmentsConfigureOpenClawProjectionDenialLogsRevokedAndManualDebug(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, err := s.GetUserByEmail("owner@test.com")
	if err != nil {
		t.Fatalf("GetUserByEmail owner: %v", err)
	}

	deniedEnrollmentID, _ := createClaimedHelperEnrollmentWithCategories(t, ts.URL, ownerToken, []string{"openclaw_config", "openclaw_lifecycle"})
	now := time.UnixMilli(1778845000000)
	seedConfigureOpenClawJob(t, s, owner, deniedEnrollmentID, "job-denied", "openclaw.configure_agent", "openclaw_config", "failed", "policy_denied", "policy handoff denied", map[string][]string{"audit_refs": []string{"audit-1"}, "log_refs": []string{"log-1"}}, now)
	projection := fetchEnrollmentConfigureOpenClawProjection(t, ts.URL, ownerToken, deniedEnrollmentID)
	if projection["state"] != "denied" || projection["failure_code"] != "policy_denied" || projection["failure_message"] != "policy handoff denied" {
		t.Fatalf("denied projection missing reason/message: %v", projection)
	}
	if refs := projection["log_refs"].([]any); len(refs) != 1 || refs[0] != "log-1" {
		t.Fatalf("denied projection should expose bounded log refs: %v", projection)
	}
	if _, ok := projection["result_summary_json"]; ok {
		t.Fatalf("projection leaked raw result summary: %v", projection)
	}

	manualEnrollmentID, _ := createClaimedHelperEnrollmentWithCategories(t, ts.URL, ownerToken, []string{"openclaw_config", "openclaw_lifecycle"})
	seedConfigureOpenClawJob(t, s, owner, manualEnrollmentID, "job-expired", "service.lifecycle", "openclaw_lifecycle", "expired", "ttl_expired", "", nil, now.Add(time.Second))
	projection = fetchEnrollmentConfigureOpenClawProjection(t, ts.URL, ownerToken, manualEnrollmentID)
	if projection["state"] != "manual_debug" || projection["failure_code"] != "ttl_expired" {
		t.Fatalf("expired/cancelled closure should require manual debug: %v", projection)
	}

	resp, revokeBody := testutil.JSON(t, http.MethodDelete, ts.URL+"/api/v1/helper/enrollments/"+manualEnrollmentID, ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("revoke enrollment: status %d body %v", resp.StatusCode, revokeBody)
	}
	projection = fetchEnrollmentConfigureOpenClawProjection(t, ts.URL, ownerToken, manualEnrollmentID)
	if projection["state"] != "revoked" || projection["failure_code"] != "revoked" {
		t.Fatalf("revoked enrollment should override job state: %v", projection)
	}
}

func createClaimedHelperEnrollmentWithCategories(t *testing.T, baseURL, token string, categories []string) (string, string) {
	t.Helper()
	resp, body := testutil.JSON(t, http.MethodPost, baseURL+"/api/v1/helper/enrollments", token, map[string]any{
		"host_label":         "OpenClaw Host",
		"allowed_categories": categories,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create helper enrollment: status %d body %v", resp.StatusCode, body)
	}
	enrollment := body["enrollment"].(map[string]any)
	secret := body["enrollment_secret"].(string)
	enrollmentID := enrollment["enrollment_id"].(string)
	_, credential := claimHelperEnrollmentViaAPI(t, baseURL, enrollmentID, secret, "device-"+enrollmentID)
	resp, statusBody := testutil.JSON(t, http.MethodPost, baseURL+"/api/v1/helper/enrollments/"+enrollmentID+"/status", credential, map[string]any{
		"helper_device_id": "device-" + enrollmentID,
		"state":            "connected",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("heartbeat helper enrollment: status %d body %v", resp.StatusCode, statusBody)
	}
	return enrollmentID, credential
}

func seedConfigureOpenClawJob(t *testing.T, s *store.Store, owner *store.User, enrollmentID, id, jobType, category, status, failureCode, failureMessage string, refs map[string][]string, createdAt time.Time) {
	t.Helper()
	createdMS := createdAt.UnixMilli()
	completedMS := createdAt.Add(time.Second).UnixMilli()
	job := &store.HelperJob{
		ID:               id,
		OwnerUserID:      owner.ID,
		OrgID:            owner.OrgID,
		EnrollmentID:     enrollmentID,
		JobType:          jobType,
		Category:         category,
		SchemaVersion:    1,
		PayloadJSON:      `{}`,
		PayloadHash:      id + "-hash",
		IdempotencyScope: id + "-scope",
		Status:           status,
		CreatedAt:        createdMS,
		UpdatedAt:        createdMS,
		ExpiresAt:        createdAt.Add(5 * time.Minute).UnixMilli(),
	}
	if status == "succeeded" || status == "failed" || status == "cancelled" || status == "expired" {
		job.CompletedAt = &completedMS
	}
	if failureCode != "" {
		job.FailureCode = &failureCode
	}
	if failureMessage != "" {
		job.FailureMessage = &failureMessage
	}
	if refs != nil {
		b, err := json.Marshal(refs)
		if err != nil {
			t.Fatalf("marshal refs: %v", err)
		}
		summary := string(b)
		job.ResultSummaryJSON = &summary
	}
	if err := s.DB().Create(job).Error; err != nil {
		t.Fatalf("seed helper job %s: %v", id, err)
	}
}

func fetchEnrollmentConfigureOpenClawProjection(t *testing.T, baseURL, token, enrollmentID string) map[string]any {
	t.Helper()
	resp, body := testutil.JSON(t, http.MethodGet, baseURL+"/api/v1/helper/enrollments", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list helper enrollments: status %d body %v", resp.StatusCode, body)
	}
	for _, raw := range body["enrollments"].([]any) {
		row := raw.(map[string]any)
		if row["enrollment_id"] == enrollmentID {
			projection, ok := row["configure_openclaw"].(map[string]any)
			if !ok {
				t.Fatalf("missing configure_openclaw projection on row: %v", row)
			}
			return projection
		}
	}
	t.Fatalf("enrollment %s not found in list: %v", enrollmentID, body)
	return nil
}

func fetchEnrollmentConfigureOpenClawDetailProjection(t *testing.T, baseURL, token, enrollmentID string) map[string]any {
	t.Helper()
	resp, body := testutil.JSON(t, http.MethodGet, baseURL+"/api/v1/helper/enrollments/"+enrollmentID, token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get helper enrollment: status %d body %v", resp.StatusCode, body)
	}
	row := body["enrollment"].(map[string]any)
	projection, ok := row["configure_openclaw"].(map[string]any)
	if !ok {
		t.Fatalf("missing configure_openclaw projection on detail row: %v", row)
	}
	return projection
}

func TestHelperEnrollmentHelperRailClaimStatusAndUninstall(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	enrollment, secret := createHelperEnrollmentViaAPI(t, ts.URL, ownerToken)
	enrollmentID := enrollment["enrollment_id"].(string)
	claimed, credential := claimHelperEnrollmentViaAPI(t, ts.URL, enrollmentID, secret, "device-1")
	if claimed["status"] != "connected" {
		t.Fatalf("claim status=%v, want connected", claimed["status"])
	}
	assertNoSensitiveHelperFields(t, claimed)

	resp, body := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/claim", "", map[string]any{
		"enrollment_secret": secret,
		"helper_device_id":  "device-2",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("second claim should be 409, got %d body %v", resp.StatusCode, body)
	}

	resp, _ = testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/status", ownerToken, map[string]any{
		"helper_device_id": "device-1",
		"state":            "connected",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("user token must not authenticate helper status, got %d", resp.StatusCode)
	}

	resp, _ = testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/status", credential, map[string]any{
		"helper_device_id": "device-2",
		"state":            "connected",
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong helper_device_id should be 403, got %d", resp.StatusCode)
	}

	resp, body = testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/status", credential, map[string]any{
		"helper_device_id": "device-1",
		"state":            "connected",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("valid helper status should be 200, got %d body %v", resp.StatusCode, body)
	}
	statusEnrollment := body["enrollment"].(map[string]any)
	if statusEnrollment["status"] != "connected" || statusEnrollment["last_seen_at"] == nil {
		t.Fatalf("valid status should return connected with last_seen_at: %v", statusEnrollment)
	}
	assertNoSensitiveHelperFields(t, statusEnrollment)

	rotated, rotatedCredential := rotateHelperCredentialViaAPI(t, ts.URL, enrollmentID, credential, "device-1")
	assertNoSensitiveHelperFields(t, rotated)

	resp, _ = testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/status", credential, map[string]any{
		"helper_device_id": "device-1",
		"state":            "connected",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old credential must be stale after rotation, got %d", resp.StatusCode)
	}

	resp, body = testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/status", rotatedCredential, map[string]any{
		"helper_device_id": "device-1",
		"state":            "connected",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rotated credential status should be 200, got %d body %v", resp.StatusCode, body)
	}
	rotatedStatus := body["enrollment"].(map[string]any)
	if rotatedStatus["status"] != "connected" || rotatedStatus["last_seen_at"] == nil {
		t.Fatalf("rotated credential status should return connected with last_seen_at: %v", rotatedStatus)
	}
	assertNoSensitiveHelperFields(t, rotatedStatus)

	resp, _ = testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/uninstall", ownerToken, map[string]any{
		"helper_device_id": "device-1",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("user token must not authenticate helper uninstall, got %d", resp.StatusCode)
	}

	resp, body = testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/uninstall", rotatedCredential, map[string]any{
		"helper_device_id": "device-1",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("valid helper uninstall should be 200, got %d body %v", resp.StatusCode, body)
	}
	uninstalled := body["enrollment"].(map[string]any)
	if uninstalled["status"] != "uninstalled" {
		t.Fatalf("uninstall status=%v, want uninstalled", uninstalled["status"])
	}

	resp, _ = testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/status", rotatedCredential, map[string]any{
		"helper_device_id": "device-1",
		"state":            "connected",
	})
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusConflict {
		t.Fatalf("heartbeat after uninstall should fail closed, got %d", resp.StatusCode)
	}
}

func TestHelperEnrollmentRejectsRemoteHostGrantAndUserPermissionAuthority(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	enrollment, secret := createHelperEnrollmentViaAPI(t, ts.URL, ownerToken)
	enrollmentID := enrollment["enrollment_id"].(string)
	_, credential := claimHelperEnrollmentViaAPI(t, ts.URL, enrollmentID, secret, "device-1")

	owner, err := s.GetUserByEmail("owner@test.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	remoteNode, err := s.CreateRemoteNode(owner.ID, "helper-separation-node")
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

	for name, token := range map[string]string{
		"remote_node_token": remoteNode.ConnectionToken,
		"host_grant_id":     hostGrantID,
		"user_token":        ownerToken,
	} {
		resp, _ := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/status", token, map[string]any{
			"helper_device_id": "device-1",
			"state":            "connected",
		})
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s must not authenticate helper status, got %d", name, resp.StatusCode)
		}

		resp, _ = testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/rotate-credential", token, map[string]any{
			"helper_device_id": "device-1",
		})
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s must not authenticate helper credential rotation, got %d", name, resp.StatusCode)
		}
	}

	resp, _ = testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/rotate-credential", credential, map[string]any{
		"helper_device_id": "device-2",
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong helper_device_id must not rotate helper credential, got %d", resp.StatusCode)
	}

	resp, _ = testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/status", credential, map[string]any{
		"helper_device_id": "device-1",
		"state":            "connected",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("actual helper credential should still authenticate after rejected rails, got %d", resp.StatusCode)
	}
}
