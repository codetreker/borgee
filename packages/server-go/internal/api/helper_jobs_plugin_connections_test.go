package api_test

import (
	"net/http"
	"strings"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// TestHelperJobsPluginConnectionsListAddRemoveFlow — #1049. Exercises
// the configure → list → remove → list cycle end-to-end via the public
// REST surface. Also asserts cross-owner access returns one of
// [401, 403, 404] for both list and remove.
func TestHelperJobsPluginConnectionsListAddRemoveFlow(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	enrollment, secret := createHelperEnrollmentViaAPI(t, ts.URL, ownerToken)
	enrollmentID := enrollment["enrollment_id"].(string)
	_, helperCredential := claimHelperEnrollmentViaAPI(t, ts.URL, enrollmentID, secret, "device-plugin-list")
	agent := seedHelperJobAgent(t, s, "owner@test.com", "api-plugin-list-agent")
	privateChannel := testutil.CreateChannel(t, ts.URL, ownerToken, "plugin-list-private", "private")
	privateChannelID := privateChannel["id"].(string)
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: privateChannelID, UserID: agent.ID}); err != nil {
		t.Fatalf("add agent channel member: %v", err)
	}

	// Initial list: empty.
	resp, listBody := testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/plugin-connections", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initial list: status %d body %v", resp.StatusCode, listBody)
	}
	conns, ok := listBody["plugin_connections"].([]any)
	if !ok || len(conns) != 0 {
		t.Fatalf("initial list expected empty plugin_connections, got %v", listBody)
	}

	// Enqueue + complete a configure_connection.
	enqResp, enqBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, map[string]any{
		"job_type":       "borgee_plugin.configure_connection",
		"schema_version": 1,
		"payload":        map[string]any{"agent_id": agent.ID, "channel_id": privateChannelID},
	})
	if enqResp.StatusCode != http.StatusCreated {
		t.Fatalf("configure enqueue: status %d body %v", enqResp.StatusCode, enqBody)
	}
	configureJob := enqBody["job"].(map[string]any)
	configureJobID := configureJob["job_id"].(string)

	// Poll → lease → complete succeeded.
	pollResp, pollBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/poll", helperCredential, map[string]any{"helper_device_id": "device-plugin-list", "helper_platform": "linux"})
	if pollResp.StatusCode != http.StatusOK {
		t.Fatalf("configure poll: %d %v", pollResp.StatusCode, pollBody)
	}
	leased := pollBody["job"].(map[string]any)
	leaseToken := leased["lease_token"].(string)
	connectionID := leased["payload"].(map[string]any)["connection_id"].(string)
	if !strings.HasPrefix(connectionID, "borgee-plugin:") {
		t.Fatalf("unexpected server-derived connection_id %q", connectionID)
	}
	resultResp, resultBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/"+configureJobID+"/result", helperCredential, map[string]any{
		"helper_device_id": "device-plugin-list",
		"lease_token":      leaseToken,
		"status":           "succeeded",
		"result_summary":   map[string]any{"audit_refs": []string{"borgee-plugin-configure-connection-ok"}, "log_refs": []string{}},
	})
	if resultResp.StatusCode != http.StatusOK {
		t.Fatalf("configure result: %d %v", resultResp.StatusCode, resultBody)
	}

	// List should now show one entry.
	resp, listBody = testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/plugin-connections", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("post-configure list: %d %v", resp.StatusCode, listBody)
	}
	conns = listBody["plugin_connections"].([]any)
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection after configure, got %d: %v", len(conns), conns)
	}
	first := conns[0].(map[string]any)
	if first["connection_id"] != connectionID {
		t.Fatalf("list returned wrong connection_id: got %v want %v", first["connection_id"], connectionID)
	}
	if first["agent_id"] != agent.ID || first["channel_id"] != privateChannelID {
		t.Fatalf("list row missing agent/channel: %v", first)
	}

	// Reject invalid connection_id on remove (server-side prefix check).
	resp, bad := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, map[string]any{
		"job_type":       "borgee_plugin.remove_connection",
		"schema_version": 1,
		"payload":        map[string]any{"agent_id": agent.ID, "connection_id": "not-borgee-plugin-prefix"},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid connection_id should 400, got %d %v", resp.StatusCode, bad)
	}

	// Enqueue + complete a remove_connection.
	rmResp, rmBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, map[string]any{
		"job_type":       "borgee_plugin.remove_connection",
		"schema_version": 1,
		"payload":        map[string]any{"agent_id": agent.ID, "connection_id": connectionID},
	})
	if rmResp.StatusCode != http.StatusCreated {
		t.Fatalf("remove enqueue: %d %v", rmResp.StatusCode, rmBody)
	}
	rmJobID := rmBody["job"].(map[string]any)["job_id"].(string)
	pollResp, pollBody = testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/poll", helperCredential, map[string]any{"helper_device_id": "device-plugin-list", "helper_platform": "linux"})
	if pollResp.StatusCode != http.StatusOK {
		t.Fatalf("remove poll: %d %v", pollResp.StatusCode, pollBody)
	}
	rmLeased := pollBody["job"].(map[string]any)
	rmLeaseToken := rmLeased["lease_token"].(string)
	rmPayload := rmLeased["payload"].(map[string]any)
	if rmPayload["connection_id"] != connectionID || rmPayload["agent_id"] != agent.ID {
		t.Fatalf("remove leased payload mismatch: %v", rmPayload)
	}
	rmBinding := rmLeased["manifest_binding"].(map[string]any)
	assertAnyStringSet(t, "path_ids", rmBinding["path_ids"], []string{"borgee_plugin_config"})
	resultResp, _ = testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/"+rmJobID+"/result", helperCredential, map[string]any{
		"helper_device_id": "device-plugin-list",
		"lease_token":      rmLeaseToken,
		"status":           "succeeded",
		"result_summary":   map[string]any{"audit_refs": []string{"borgee-plugin-remove-connection-ok"}, "log_refs": []string{}},
	})
	if resultResp.StatusCode != http.StatusOK {
		t.Fatalf("remove result not ok: %d", resultResp.StatusCode)
	}

	// List should be empty again.
	resp, listBody = testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/plugin-connections", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("post-remove list: %d %v", resp.StatusCode, listBody)
	}
	conns = listBody["plugin_connections"].([]any)
	if len(conns) != 0 {
		t.Fatalf("expected 0 connections after remove, got %d: %v", len(conns), conns)
	}
}

// TestHelperJobsPluginConnectionsCrossOwnerIDOR — OUT-5 #1049. A foreign
// (different-org) owner cannot list / add / remove on owner A's
// enrollment. Every endpoint returns one of [401, 403, 404] AND after
// the cross-owner attempt the original owner's list view continues to
// show zero new rows (no silent enqueue-then-deny).
func TestHelperJobsPluginConnectionsCrossOwnerIDOR(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerAToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	enrollment, _ := createHelperEnrollmentViaAPI(t, ts.URL, ownerAToken)
	enrollmentID := enrollment["enrollment_id"].(string)
	agent := seedHelperJobAgent(t, s, "owner@test.com", "idor-plugin-agent")

	// Owner B in a DIFFERENT org via the SeedForeignOrgUser helper —
	// `member@test.com` is in the same org as owner@test.com and would
	// not exercise the cross-org SQL filter. The foreign-org owner
	// flow proves the `org_id = ?` filter + the
	// `enrollment.OwnerUserID != input.OwnerUserID` gate together.
	_ = testutil.SeedForeignOrgUser(t, s, "PluginConn Foreign Owner", "plugin-conn-foreign@test.com")
	ownerBToken := testutil.LoginAs(t, ts.URL, "plugin-conn-foreign@test.com", "password123")

	cases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"list", http.MethodGet, "/api/v1/helper/enrollments/" + enrollmentID + "/plugin-connections", nil},
		{"configure", http.MethodPost, "/api/v1/helper/enrollments/" + enrollmentID + "/jobs", map[string]any{
			"job_type":       "borgee_plugin.configure_connection",
			"schema_version": 1,
			"payload":        map[string]any{"agent_id": agent.ID, "channel_id": "any"},
		}},
		{"remove", http.MethodPost, "/api/v1/helper/enrollments/" + enrollmentID + "/jobs", map[string]any{
			"job_type":       "borgee_plugin.remove_connection",
			"schema_version": 1,
			"payload":        map[string]any{"agent_id": agent.ID, "connection_id": "borgee-plugin:abc123"},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := testutil.JSON(t, tc.method, ts.URL+tc.path, ownerBToken, tc.body)
			if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
				t.Fatalf("cross-owner %s expected 401/403/404, got %d body %v", tc.name, resp.StatusCode, body)
			}
		})
	}

	// After cross-owner attempts, owner A's list MUST still be empty —
	// confirms no silent enqueue slipped through (defense in depth, also
	// pins the regression if a future change reorders the gate).
	resp, listBody := testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/plugin-connections", ownerAToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("post-IDOR ownerA list: status %d body %v", resp.StatusCode, listBody)
	}
	if conns, ok := listBody["plugin_connections"].([]any); !ok || len(conns) != 0 {
		t.Fatalf("post-IDOR ownerA list expected empty, got %v", listBody)
	}
}

// TestHelperJobsPluginConnectionsRemoveRejectsUnboundConnection — #1049
// CRIT-2. Removing a `connection_id` that was never configured for the
// given (enrollment_id, agent_id) must be rejected at the enqueue path,
// not silently enqueued + delivered to the daemon. Prevents the
// cross-agent file-delete DoS where an attacker who learns ANY
// borgee-plugin connection_id can submit a remove via their own owned
// agent_id and trigger a file delete on the daemon.
func TestHelperJobsPluginConnectionsRemoveRejectsUnboundConnection(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	enrollment, _ := createHelperEnrollmentViaAPI(t, ts.URL, ownerToken)
	enrollmentID := enrollment["enrollment_id"].(string)
	agent := seedHelperJobAgent(t, s, "owner@test.com", "remove-unbound-agent")

	// Attempt to remove a well-formed but never-configured connection_id.
	rmResp, rmBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, map[string]any{
		"job_type":       "borgee_plugin.remove_connection",
		"schema_version": 1,
		"payload":        map[string]any{"agent_id": agent.ID, "connection_id": "borgee-plugin:never-configured-12345"},
	})
	if rmResp.StatusCode != http.StatusForbidden && rmResp.StatusCode != http.StatusNotFound {
		t.Fatalf("remove of unbound connection_id should be rejected (403/404), got %d body %v", rmResp.StatusCode, rmBody)
	}
}
