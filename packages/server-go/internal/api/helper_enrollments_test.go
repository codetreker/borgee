package api_test

import (
	"net/http"
	"testing"

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
