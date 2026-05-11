// Package api_test — admin_audit_query_test.go: ADM-3 multi-source audit
// 合并查询 unit tests.
//
// Spec: docs/implementation/modules/adm-3-spec.md §1 ADM3.1.
//
// Pins:
//   - 4 source enum 单一来源 字节级一致 (server/plugin/host_bridge/agent)
//   - UNION ALL across audit_events + channel_events + global_events
//   - admin-rail only (user cookie → 401, ADM-0 §1.3 红线)
//   - source filter / since-until time range / limit
//   - host_bridge placeholder (HB-1 audit table 未落, 0 行)

package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/api"
	"borgee-server/internal/testutil"
)

// TestADM3_AuditSources_ByteIdentical pins 4 enum 字面 + ordering.
func TestADM3_AuditSources_ByteIdentical(t *testing.T) {
	t.Parallel()
	want := []string{"server", "plugin", "host_bridge", "agent"}
	if len(api.AuditSources) != len(want) {
		t.Fatalf("AuditSources len = %d, want %d", len(api.AuditSources), len(want))
	}
	for i, v := range want {
		if api.AuditSources[i] != v {
			t.Errorf("AuditSources[%d] = %q, want %q", i, api.AuditSources[i], v)
		}
	}
	// Const 单一来源 字节级一致.
	if api.AuditSourceServer != "server" || api.AuditSourcePlugin != "plugin" ||
		api.AuditSourceHostBridge != "host_bridge" || api.AuditSourceAgent != "agent" {
		t.Errorf("source const mismatch: %s/%s/%s/%s",
			api.AuditSourceServer, api.AuditSourcePlugin, api.AuditSourceHostBridge, api.AuditSourceAgent)
	}
}

// TestADM3_MultiSource_AllSources covers UNION ALL across audit_events
// (server + plugin) + channel_events + global_events (agent).
func TestADM3_MultiSource_AllSources(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)

	owner, _ := s.GetUserByEmail("owner@test.com")
	// server source: classic admin action
	if _, err := s.InsertAdminAction("admin-A", owner.ID, "delete_channel", ""); err != nil {
		t.Fatal(err)
	}
	// plugin source: same audit_events table, action prefix 'plugin_*' (BPP-8 enum).
	if _, err := s.InsertAdminAction("admin-B", owner.ID, "plugin_connect", ""); err != nil {
		t.Fatal(err)
	}
	// agent source: channel_events + global_events
	must := func(sql string, args ...any) {
		if err := s.DB().Exec(sql, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	must(`INSERT INTO channel_events (lex_id, channel_id, kind, payload, created_at) VALUES (?, ?, ?, ?, ?)`,
		"l1", "ch-1", "channel.archived", "{}", int64(1700000000000))
	must(`INSERT INTO global_events (lex_id, kind, payload, created_at) VALUES (?, ?, ?, ?)`,
		"g1", "agent.state", "{}", int64(1700000000001))

	resp, body := testutil.JSON(t, "GET", ts.URL+"/admin-api/v1/audit/multi-source", adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	rowsAny, ok := body["rows"].([]any)
	if !ok {
		t.Fatalf("rows type %T", body["rows"])
	}
	if len(rowsAny) < 4 {
		t.Errorf("expected ≥4 rows across 4 sources, got %d", len(rowsAny))
	}
	gotSources := map[string]int{}
	for _, r := range rowsAny {
		row := r.(map[string]any)
		gotSources[row["source"].(string)]++
	}
	if gotSources["server"] < 1 {
		t.Errorf("missing server source, got %v", gotSources)
	}
	if gotSources["plugin"] < 1 {
		t.Errorf("missing plugin source")
	}
	if gotSources["agent"] < 2 {
		t.Errorf("agent expected 2 (channel+global), got %d", gotSources["agent"])
	}
	// sources field 字节级一致 (4 enum 顺序).
	if srcs, ok := body["sources"].([]any); ok {
		if len(srcs) != 4 || srcs[0] != "server" || srcs[3] != "agent" {
			t.Errorf("sources field mismatch: %v", srcs)
		}
	}
}

// TestADM3_MultiSource_SourceFilter narrows to single source via ?source=.
func TestADM3_MultiSource_SourceFilter(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	owner, _ := s.GetUserByEmail("owner@test.com")
	if _, err := s.InsertAdminAction("admin-A", owner.ID, "suspend_user", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.DB().Exec(
		`INSERT INTO global_events (lex_id, kind, payload, created_at) VALUES (?, ?, ?, ?)`,
		"g2", "agent.state", "{}", int64(1700000000000),
	).Error; err != nil {
		t.Fatal(err)
	}

	// ?source=agent → only agent rows
	resp, body := testutil.JSON(t, "GET",
		ts.URL+"/admin-api/v1/audit/multi-source?source=agent", adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d", resp.StatusCode)
	}
	rows := body["rows"].([]any)
	for _, r := range rows {
		if r.(map[string]any)["source"] != "agent" {
			t.Errorf("non-agent row leaked: %v", r)
		}
	}
	if len(rows) < 1 {
		t.Error("expected ≥1 agent row")
	}
}

// TestADM3_MultiSource_InvalidSource rejects bad ?source=.
func TestADM3_MultiSource_InvalidSource(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	resp, body := testutil.JSON(t, "GET",
		ts.URL+"/admin-api/v1/audit/multi-source?source=hybrid", adminToken, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 on invalid source, got %d", resp.StatusCode)
	}
	if body["error"] != "audit.source_invalid" {
		t.Errorf("error code = %v, want audit.source_invalid", body["error"])
	}
}

// TestADM3_MultiSource_TimeRange filters by ?since/?until ms epoch.
func TestADM3_MultiSource_TimeRange(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)

	must := func(sql string, args ...any) {
		if err := s.DB().Exec(sql, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	must(`INSERT INTO global_events (lex_id, kind, payload, created_at) VALUES (?, ?, ?, ?)`,
		"old", "agent.state", "{}", int64(1000))
	must(`INSERT INTO global_events (lex_id, kind, payload, created_at) VALUES (?, ?, ?, ?)`,
		"mid", "agent.state", "{}", int64(2000))
	must(`INSERT INTO global_events (lex_id, kind, payload, created_at) VALUES (?, ?, ?, ?)`,
		"new", "agent.state", "{}", int64(3000))

	resp, body := testutil.JSON(t, "GET",
		ts.URL+"/admin-api/v1/audit/multi-source?source=agent&since=1500&until=2500",
		adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d", resp.StatusCode)
	}
	rows := body["rows"].([]any)
	if len(rows) != 1 {
		t.Errorf("expected 1 row in [1500,2500], got %d", len(rows))
	}
	if len(rows) == 1 {
		ts2 := int64(rows[0].(map[string]any)["ts"].(float64))
		if ts2 != 2000 {
			t.Errorf("ts = %d, want 2000", ts2)
		}
	}
}

// TestADM3_MultiSource_InvalidTimeRange rejects negative/non-int since.
func TestADM3_MultiSource_InvalidTimeRange(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	resp, _ := testutil.JSON(t, "GET",
		ts.URL+"/admin-api/v1/audit/multi-source?since=-1", adminToken, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 on negative since, got %d", resp.StatusCode)
	}
}

// TestADM3_MultiSource_OrderTSDesc pins newest-first ordering across sources.
func TestADM3_MultiSource_OrderTSDesc(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	owner, _ := s.GetUserByEmail("owner@test.com")

	// Mix ts across 2 sources to exercise post-merge sort.
	if err := s.DB().Exec(
		`INSERT INTO audit_events (id, actor_id, target_user_id, action, metadata, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		"a-old", "admin-A", owner.ID, "delete_channel", "", int64(1000),
	).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.DB().Exec(
		`INSERT INTO global_events (lex_id, kind, payload, created_at) VALUES (?, ?, ?, ?)`,
		"g-new", "agent.state", "{}", int64(5000),
	).Error; err != nil {
		t.Fatal(err)
	}

	resp, body := testutil.JSON(t, "GET", ts.URL+"/admin-api/v1/audit/multi-source", adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatal(resp.StatusCode)
	}
	rows := body["rows"].([]any)
	if len(rows) < 2 {
		t.Fatalf("len = %d", len(rows))
	}
	prev := int64(1<<62)
	for _, r := range rows {
		ts2 := int64(r.(map[string]any)["ts"].(float64))
		if ts2 > prev {
			t.Errorf("ordering not DESC: %d > %d", ts2, prev)
		}
		prev = ts2
	}
}

// TestADM3_MultiSource_UserCookieRejected pins ADM-0 §1.3 red line — user
// cookie 调 /admin-api/v1/audit/multi-source → 401/403.
func TestADM3_MultiSource_UserCookieRejected(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	userToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	resp, _ := testutil.JSON(t, "GET",
		ts.URL+"/admin-api/v1/audit/multi-source", userToken, nil)
	if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden {
		t.Errorf("user cookie should be rejected on /admin-api/v1/audit/multi-source, got %d", resp.StatusCode)
	}
}

// TestADM3_MultiSource_UnauthRejected pins no-auth → 401.
func TestADM3_MultiSource_UnauthRejected(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	resp, _ := testutil.JSON(t, "GET",
		ts.URL+"/admin-api/v1/audit/multi-source", "", nil)
	if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden {
		t.Errorf("unauth → expected 401/403, got %d", resp.StatusCode)
	}
}

// TestADM3_MultiSource_LimitClamp pins ?limit= clamping (default 100, max 500).
func TestADM3_MultiSource_LimitClamp(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	resp, _ := testutil.JSON(t, "GET",
		ts.URL+"/admin-api/v1/audit/multi-source?limit=99999", adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("limit clamp expected 200, got %d", resp.StatusCode)
	}
}

// TestADM3_MultiSource_HostBridgePlaceholder pins HB-1 audit table 未落 v1
// — host_bridge source 不报错 (placeholder 0 行).
func TestADM3_MultiSource_HostBridgePlaceholder(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	resp, body := testutil.JSON(t, "GET",
		ts.URL+"/admin-api/v1/audit/multi-source?source=host_bridge", adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d", resp.StatusCode)
	}
	rows := body["rows"].([]any)
	if len(rows) != 0 {
		t.Errorf("host_bridge placeholder should be 0 rows v1 (HB-1 audit table 未落), got %d", len(rows))
	}
}
