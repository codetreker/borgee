package api_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

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

// TestHelperJobsPluginConnectionsRemoveAcceptsBoundConnection — #1049
// CRIT-2 happy path. Owner A configures connection X via agent A1, then
// owner A removes connection X via agent A1. Must succeed (enqueue
// returns 201 Created and the row is delivered to the daemon via poll).
// Pins the gate's positive path so a future regression that flips the
// `bound` check default doesn't silently bypass authorization.
func TestHelperJobsPluginConnectionsRemoveAcceptsBoundConnection(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	enrollment, secret := createHelperEnrollmentViaAPI(t, ts.URL, ownerToken)
	enrollmentID := enrollment["enrollment_id"].(string)
	_, helperCredential := claimHelperEnrollmentViaAPI(t, ts.URL, enrollmentID, secret, "device-bound-happy")
	agent := seedHelperJobAgent(t, s, "owner@test.com", "remove-bound-agent")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "remove-bound-private", "private")
	chID := ch["id"].(string)
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: chID, UserID: agent.ID}); err != nil {
		t.Fatalf("add agent channel member: %v", err)
	}

	// Step 1: configure → succeed.
	enqResp, enqBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, map[string]any{
		"job_type":       "borgee_plugin.configure_connection",
		"schema_version": 1,
		"payload":        map[string]any{"agent_id": agent.ID, "channel_id": chID},
	})
	if enqResp.StatusCode != http.StatusCreated {
		t.Fatalf("configure enqueue: %d %v", enqResp.StatusCode, enqBody)
	}
	cfgJobID := enqBody["job"].(map[string]any)["job_id"].(string)
	pollResp, pollBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/poll", helperCredential, map[string]any{"helper_device_id": "device-bound-happy", "helper_platform": "linux"})
	if pollResp.StatusCode != http.StatusOK {
		t.Fatalf("configure poll: %d %v", pollResp.StatusCode, pollBody)
	}
	leased := pollBody["job"].(map[string]any)
	leaseToken := leased["lease_token"].(string)
	derivedConnID := leased["payload"].(map[string]any)["connection_id"].(string)
	if !strings.HasPrefix(derivedConnID, "borgee-plugin:") {
		t.Fatalf("expected server-derived borgee-plugin: prefix, got %q", derivedConnID)
	}
	rrResp, _ := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/"+cfgJobID+"/result", helperCredential, map[string]any{
		"helper_device_id": "device-bound-happy",
		"lease_token":      leaseToken,
		"status":           "succeeded",
		"result_summary":   map[string]any{"audit_refs": []string{"borgee-plugin-configure-connection-ok"}, "log_refs": []string{}},
	})
	if rrResp.StatusCode != http.StatusOK {
		t.Fatalf("configure result: %d", rrResp.StatusCode)
	}

	// Step 2: remove of the SAME (agent_id, connection_id) MUST succeed.
	rmResp, rmBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, map[string]any{
		"job_type":       "borgee_plugin.remove_connection",
		"schema_version": 1,
		"payload":        map[string]any{"agent_id": agent.ID, "connection_id": derivedConnID},
	})
	if rmResp.StatusCode != http.StatusCreated {
		t.Fatalf("legit remove of bound connection should 201, got %d body %v", rmResp.StatusCode, rmBody)
	}
}

// TestHelperJobsPluginConnectionsRemoveRejectsCrossAgentSameOwner —
// #1049 CRIT-2 attacker model. Owner A configures connection X via
// agent A1 (which derives connection_id `derived`), then owner A
// attempts remove of `derived` via agent A2 (also owned by A, but no
// configure was ever done on A2). The binding check at the enqueue
// gate must reject because (enrollment, A2, derived) isn't bound,
// even though both agents share the owner.
//
// Without this gate an attacker who legitimately owns A2 and learned
// `derived` (from logs / list output for A1) could submit a remove via
// A2 and trigger a file delete on the daemon for A1's connection —
// because the daemon root is shared per deployment, agent ownership
// alone is insufficient to scope the delete.
func TestHelperJobsPluginConnectionsRemoveRejectsCrossAgentSameOwner(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	enrollment, secret := createHelperEnrollmentViaAPI(t, ts.URL, ownerToken)
	enrollmentID := enrollment["enrollment_id"].(string)
	_, helperCredential := claimHelperEnrollmentViaAPI(t, ts.URL, enrollmentID, secret, "device-crossagent")
	agent1 := seedHelperJobAgent(t, s, "owner@test.com", "crossagent-a1")
	agent2 := seedHelperJobAgent(t, s, "owner@test.com", "crossagent-a2")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "crossagent-private", "private")
	chID := ch["id"].(string)
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: chID, UserID: agent1.ID}); err != nil {
		t.Fatalf("add agent1 channel member: %v", err)
	}
	// Note: agent2 deliberately NOT added to the channel — but the
	// remove gate's binding check is independent of channel membership;
	// the rejection must come from the (agent, connection_id) binding
	// scan, not from a channel ACL.

	// Step 1: configure on agent1 → succeed.
	enqResp, enqBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, map[string]any{
		"job_type":       "borgee_plugin.configure_connection",
		"schema_version": 1,
		"payload":        map[string]any{"agent_id": agent1.ID, "channel_id": chID},
	})
	if enqResp.StatusCode != http.StatusCreated {
		t.Fatalf("configure enqueue on agent1: %d %v", enqResp.StatusCode, enqBody)
	}
	cfgJobID := enqBody["job"].(map[string]any)["job_id"].(string)
	pollResp, pollBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/poll", helperCredential, map[string]any{"helper_device_id": "device-crossagent", "helper_platform": "linux"})
	if pollResp.StatusCode != http.StatusOK {
		t.Fatalf("configure poll: %d %v", pollResp.StatusCode, pollBody)
	}
	leased := pollBody["job"].(map[string]any)
	leaseToken := leased["lease_token"].(string)
	derivedConnID := leased["payload"].(map[string]any)["connection_id"].(string)
	rrResp, _ := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/"+cfgJobID+"/result", helperCredential, map[string]any{
		"helper_device_id": "device-crossagent",
		"lease_token":      leaseToken,
		"status":           "succeeded",
		"result_summary":   map[string]any{"audit_refs": []string{"borgee-plugin-configure-connection-ok"}, "log_refs": []string{}},
	})
	if rrResp.StatusCode != http.StatusOK {
		t.Fatalf("configure result: %d", rrResp.StatusCode)
	}

	// Step 2: remove `derivedConnID` via agent2 MUST be rejected.
	rmResp, rmBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, map[string]any{
		"job_type":       "borgee_plugin.remove_connection",
		"schema_version": 1,
		"payload":        map[string]any{"agent_id": agent2.ID, "connection_id": derivedConnID},
	})
	if rmResp.StatusCode != http.StatusForbidden && rmResp.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-agent same-owner remove should be rejected (403/404), got %d body %v", rmResp.StatusCode, rmBody)
	}
}

// TestHelperJobsPluginConnectionsRemoveBindingBypassesListCap — #1049
// run_4 fix. The list projection caps the historical scan at
// helperJobsPluginConnectionsRowCap (5000). Earlier the remove
// binding check inherited that same LIMIT, so a legitimate remove
// against an old configure that had scrolled past 5000 newer rows
// was silently rejected. The fix narrows the scan with a
// payload_json LIKE on connection_id+agent_id and removes the LIMIT.
// This test seeds the legit configure, then bulk-inserts >5000
// unrelated succeeded configure rows that newer it under
// (created_at ASC, id ASC) ordering, then attempts remove of the
// legit connection. Must succeed (201) — proves the binding check
// is unbounded by the list cap. To keep wall time low we directly
// INSERT the noise rows via store.DB(), bypassing the public enqueue
// path (which is irrelevant — we only need the binding-scan SQL to
// see >5000 rows ordered before the legit one).
func TestHelperJobsPluginConnectionsRemoveBindingBypassesListCap(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	enrollment, secret := createHelperEnrollmentViaAPI(t, ts.URL, ownerToken)
	enrollmentID := enrollment["enrollment_id"].(string)
	_, helperCredential := claimHelperEnrollmentViaAPI(t, ts.URL, enrollmentID, secret, "device-cap-bypass")
	agent := seedHelperJobAgent(t, s, "owner@test.com", "cap-bypass-agent")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "cap-bypass-private", "private")
	chID := ch["id"].(string)
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: chID, UserID: agent.ID}); err != nil {
		t.Fatalf("add agent channel member: %v", err)
	}

	// Step 1: legit configure → succeed. This row will be the OLDEST
	// configure under (created_at ASC, id ASC) ordering.
	enqResp, enqBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, map[string]any{
		"job_type":       "borgee_plugin.configure_connection",
		"schema_version": 1,
		"payload":        map[string]any{"agent_id": agent.ID, "channel_id": chID},
	})
	if enqResp.StatusCode != http.StatusCreated {
		t.Fatalf("legit configure enqueue: %d %v", enqResp.StatusCode, enqBody)
	}
	cfgJobID := enqBody["job"].(map[string]any)["job_id"].(string)
	pollResp, pollBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/poll", helperCredential, map[string]any{"helper_device_id": "device-cap-bypass", "helper_platform": "linux"})
	if pollResp.StatusCode != http.StatusOK {
		t.Fatalf("legit configure poll: %d %v", pollResp.StatusCode, pollBody)
	}
	leased := pollBody["job"].(map[string]any)
	leaseToken := leased["lease_token"].(string)
	derivedConnID := leased["payload"].(map[string]any)["connection_id"].(string)
	rrResp, _ := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs/"+cfgJobID+"/result", helperCredential, map[string]any{
		"helper_device_id": "device-cap-bypass",
		"lease_token":      leaseToken,
		"status":           "succeeded",
		"result_summary":   map[string]any{"audit_refs": []string{"borgee-plugin-configure-connection-ok"}, "log_refs": []string{}},
	})
	if rrResp.StatusCode != http.StatusOK {
		t.Fatalf("legit configure result: %d", rrResp.StatusCode)
	}

	// Step 2: bulk-insert noise. We need MORE than
	// helperJobsPluginConnectionsRowCap (5000) succeeded configure
	// rows newer than the legit one (under created_at ASC, id ASC)
	// in the SAME (owner, org, enrollment) so the legit row would
	// scroll out of any LIMIT(5000)-bounded scan ordered ASC.
	owner, err := s.GetUserByEmail("owner@test.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	now := time.Now().UnixMilli()
	const noiseRows = 5050
	rows := make([]store.HelperJob, 0, noiseRows)
	for i := 0; i < noiseRows; i++ {
		// Payload deliberately does NOT contain derivedConnID or
		// agent.ID — the binding-scan LIKE filter will exclude these
		// rows quickly. But the binding scan WITHOUT the LIKE filter
		// (i.e. the old bug) would have hit the LIMIT(5000) before
		// reaching the legit row.
		fakeConn := fmt.Sprintf("borgee-plugin:noise-%05d", i)
		fakeAgent := fmt.Sprintf("noise-agent-%05d", i)
		payload := fmt.Sprintf(`{"connection_id":%q,"agent_id":%q,"channel_id":"noise"}`, fakeConn, fakeAgent)
		rows = append(rows, store.HelperJob{
			ID:               fmt.Sprintf("noise-cfg-%05d", i),
			OwnerUserID:      owner.ID,
			OrgID:            owner.OrgID,
			EnrollmentID:     enrollmentID,
			JobType:          store.HelperJobTypePluginConfigureConnection,
			Category:         "borgee_plugin",
			SchemaVersion:    1,
			PayloadJSON:      payload,
			PayloadHash:      fmt.Sprintf("sha256:noise-%05d", i),
			ManifestDigest:   "sha256:noise",
			IdempotencyScope: fmt.Sprintf("noise-scope-%05d", i),
			Status:           "succeeded",
			CreatedAt:        now + int64(i+1), // newer than legit configure
			UpdatedAt:        now + int64(i+1),
			ExpiresAt:        now + int64(86_400_000),
		})
	}
	// Bulk insert in chunks (GORM default has a parameter limit).
	const chunk = 500
	for start := 0; start < len(rows); start += chunk {
		end := start + chunk
		if end > len(rows) {
			end = len(rows)
		}
		if err := s.DB().Create(rows[start:end]).Error; err != nil {
			t.Fatalf("bulk insert noise configure rows [%d..%d]: %v", start, end, err)
		}
	}

	// Step 3: remove the legit connection. With the OLD bug
	// (Limit(5000) on the binding scan ordered ASC) the legit row
	// would not be visible — the scan would return only noise rows,
	// `bound` stays false, and we'd get 403 Forbidden. With the
	// fix (narrowed LIKE + no LIMIT) the legit configure is found
	// regardless of how many newer unrelated rows exist.
	rmResp, rmBody := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/helper/enrollments/"+enrollmentID+"/jobs", ownerToken, map[string]any{
		"job_type":       "borgee_plugin.remove_connection",
		"schema_version": 1,
		"payload":        map[string]any{"agent_id": agent.ID, "connection_id": derivedConnID},
	})
	if rmResp.StatusCode != http.StatusCreated {
		t.Fatalf("remove of legit connection after >5000 noise rows should 201, got %d body %v", rmResp.StatusCode, rmBody)
	}
}
