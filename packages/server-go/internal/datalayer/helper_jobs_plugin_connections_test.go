// helper_jobs_plugin_connections_test.go — #1049 datalayer-level
// coverage for sqliteHelperJobRepo.ListPluginConnections.
//
// The API-package integration test exercises the same code path but
// coverage is measured per-package so the datalayer threshold needs
// in-package tests. These tests seed rows directly via store.DB() and
// assert the projection's active-iff-latest-configure-newer logic, the
// owner/org scoping, the malformed-payload skip, and the non-succeeded
// skip branch.

package datalayer

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"borgee-server/internal/store"
)

// newPluginConnEnrollment seeds a real enrollment + claim so helper_jobs
// FOREIGN KEY (enrollment_id) is satisfied. Returns the post-claim
// enrollment id which the row uses.
func newPluginConnEnrollment(t *testing.T, s *store.Store, owner *store.User, label string, base time.Time) string {
	t.Helper()
	enrollment, secret, err := s.CreateHelperEnrollment(owner.ID, label, []string{"openclaw_config"}, base)
	if err != nil {
		t.Fatalf("CreateHelperEnrollment(%s): %v", label, err)
	}
	claimed, _, err := s.ClaimHelperEnrollment(enrollment.ID, secret, "device-"+label, base.Add(time.Minute))
	if err != nil {
		t.Fatalf("ClaimHelperEnrollment(%s): %v", label, err)
	}
	return claimed.ID
}

func seedPluginConnectionJob(
	t *testing.T,
	s *store.Store,
	owner *store.User,
	enrollmentID, id, jobType, status, payloadJSON string,
	createdAt time.Time,
) {
	t.Helper()
	created := createdAt.UnixMilli()
	completed := createdAt.Add(time.Second).UnixMilli()
	job := &store.HelperJob{
		ID:               id,
		OwnerUserID:      owner.ID,
		OrgID:            owner.OrgID,
		EnrollmentID:     enrollmentID,
		JobType:          jobType,
		Category:         "openclaw_config",
		SchemaVersion:    1,
		PayloadJSON:      payloadJSON,
		PayloadHash:      "sha256:" + id,
		IdempotencyScope: id + "-scope",
		Status:           status,
		CreatedAt:        created,
		UpdatedAt:        created,
		CompletedAt:      &completed,
		ExpiresAt:        createdAt.Add(5 * time.Minute).UnixMilli(),
	}
	if err := s.DB().Create(job).Error; err != nil {
		t.Fatalf("seed plugin-connection job %s: %v", id, err)
	}
}

func TestListPluginConnections_InputValidation(t *testing.T) {
	t.Parallel()
	dl, _, owner := newHelperJobRepoFixture(t, "plugin-conn-input")
	repo := dl.HelperJobRepo

	if _, err := repo.ListPluginConnections(context.Background(), "", owner.OrgID, "enr-1"); err == nil {
		t.Fatal("expected error for empty owner")
	}
	if _, err := repo.ListPluginConnections(context.Background(), owner.ID, "", "enr-1"); err == nil {
		t.Fatal("expected error for empty org")
	}
	if _, err := repo.ListPluginConnections(context.Background(), owner.ID, owner.OrgID, ""); err == nil {
		t.Fatal("expected error for empty enrollment")
	}
}

func TestListPluginConnections_EmptyWhenNoRows(t *testing.T) {
	t.Parallel()
	dl, s, owner := newHelperJobRepoFixture(t, "plugin-conn-empty")
	repo := dl.HelperJobRepo
	now := time.UnixMilli(1778900000000)
	enrID := newPluginConnEnrollment(t, s, owner, "empty", now)
	out, err := repo.ListPluginConnections(context.Background(), owner.ID, owner.OrgID, enrID)
	if err != nil {
		t.Fatalf("ListPluginConnections: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty, got %d rows: %+v", len(out), out)
	}
}

func mustPluginJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestListPluginConnections_ActiveAfterConfigureOnly(t *testing.T) {
	t.Parallel()
	dl, s, owner := newHelperJobRepoFixture(t, "plugin-conn-active")
	repo := dl.HelperJobRepo
	now := time.UnixMilli(1778900000000)
	enrID := newPluginConnEnrollment(t, s, owner, "active", now)

	cfg := mustPluginJSON(t, map[string]any{
		"agent_id":      "agent-A",
		"channel_id":    "chan-A",
		"connection_id": "borgee-plugin:conn-1",
	})
	seedPluginConnectionJob(t, s, owner, enrID, "job-cfg-1",
		"borgee_plugin.configure_connection", "succeeded", cfg, now.Add(2*time.Minute))

	out, err := repo.ListPluginConnections(context.Background(), owner.ID, owner.OrgID, enrID)
	if err != nil {
		t.Fatalf("ListPluginConnections: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 active connection, got %d: %+v", len(out), out)
	}
	if out[0].ConnectionID != "borgee-plugin:conn-1" || out[0].AgentID != "agent-A" || out[0].ChannelID != "chan-A" {
		t.Fatalf("projection mismatch: %+v", out[0])
	}
	if out[0].LastConfiguredAt == 0 {
		t.Fatalf("expected LastConfiguredAt > 0, got %d", out[0].LastConfiguredAt)
	}
}

func TestListPluginConnections_RemoveSupersedesConfigure(t *testing.T) {
	t.Parallel()
	dl, s, owner := newHelperJobRepoFixture(t, "plugin-conn-remove")
	repo := dl.HelperJobRepo
	now := time.UnixMilli(1778900000000)
	enrID := newPluginConnEnrollment(t, s, owner, "remove", now)

	cfgPayload := mustPluginJSON(t, map[string]any{
		"agent_id":      "agent-A",
		"channel_id":    "chan-A",
		"connection_id": "borgee-plugin:conn-1",
	})
	rmPayload := mustPluginJSON(t, map[string]any{
		"agent_id":      "agent-A",
		"connection_id": "borgee-plugin:conn-1",
	})
	seedPluginConnectionJob(t, s, owner, enrID, "job-cfg-1",
		"borgee_plugin.configure_connection", "succeeded", cfgPayload, now.Add(2*time.Minute))
	seedPluginConnectionJob(t, s, owner, enrID, "job-rm-1",
		"borgee_plugin.remove_connection", "succeeded", rmPayload, now.Add(4*time.Minute))

	out, err := repo.ListPluginConnections(context.Background(), owner.ID, owner.OrgID, enrID)
	if err != nil {
		t.Fatalf("ListPluginConnections: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected 0 active after later remove, got %+v", out)
	}
}

func TestListPluginConnections_ReconfigureAfterRemove(t *testing.T) {
	t.Parallel()
	dl, s, owner := newHelperJobRepoFixture(t, "plugin-conn-reconf")
	repo := dl.HelperJobRepo
	now := time.UnixMilli(1778900000000)
	enrID := newPluginConnEnrollment(t, s, owner, "reconf", now)

	cfgA := mustPluginJSON(t, map[string]any{
		"agent_id":      "agent-A",
		"channel_id":    "chan-A",
		"connection_id": "borgee-plugin:conn-1",
	})
	rm := mustPluginJSON(t, map[string]any{
		"agent_id":      "agent-A",
		"connection_id": "borgee-plugin:conn-1",
	})
	cfgB := mustPluginJSON(t, map[string]any{
		"agent_id":      "agent-A",
		"channel_id":    "chan-B",
		"connection_id": "borgee-plugin:conn-1",
	})
	seedPluginConnectionJob(t, s, owner, enrID, "job-cfg-1",
		"borgee_plugin.configure_connection", "succeeded", cfgA, now.Add(2*time.Minute))
	seedPluginConnectionJob(t, s, owner, enrID, "job-rm-1",
		"borgee_plugin.remove_connection", "succeeded", rm, now.Add(4*time.Minute))
	seedPluginConnectionJob(t, s, owner, enrID, "job-cfg-2",
		"borgee_plugin.configure_connection", "succeeded", cfgB, now.Add(6*time.Minute))

	out, err := repo.ListPluginConnections(context.Background(), owner.ID, owner.OrgID, enrID)
	if err != nil {
		t.Fatalf("ListPluginConnections: %v", err)
	}
	if len(out) != 1 || out[0].ChannelID != "chan-B" {
		t.Fatalf("expected 1 active with channel chan-B, got %+v", out)
	}
}

func TestListPluginConnections_SkipsNonSucceededAndMalformed(t *testing.T) {
	t.Parallel()
	dl, s, owner := newHelperJobRepoFixture(t, "plugin-conn-skip")
	repo := dl.HelperJobRepo
	now := time.UnixMilli(1778900000000)
	enrID := newPluginConnEnrollment(t, s, owner, "skip", now)

	// Queued configure → must be skipped.
	queued := mustPluginJSON(t, map[string]any{
		"agent_id":      "agent-Q",
		"channel_id":    "chan-Q",
		"connection_id": "borgee-plugin:queued",
	})
	seedPluginConnectionJob(t, s, owner, enrID, "job-queued",
		"borgee_plugin.configure_connection", "queued", queued, now.Add(2*time.Minute))

	// Failed configure → must be skipped.
	failed := mustPluginJSON(t, map[string]any{
		"agent_id":      "agent-F",
		"channel_id":    "chan-F",
		"connection_id": "borgee-plugin:failed",
	})
	seedPluginConnectionJob(t, s, owner, enrID, "job-failed",
		"borgee_plugin.configure_connection", "failed", failed, now.Add(3*time.Minute))

	// Malformed payload JSON → must be skipped silently.
	seedPluginConnectionJob(t, s, owner, enrID, "job-malformed",
		"borgee_plugin.configure_connection", "succeeded", `{not-json`, now.Add(4*time.Minute))

	// Payload missing borgee-plugin: prefix → must be skipped.
	wrong := mustPluginJSON(t, map[string]any{
		"agent_id":      "agent-W",
		"channel_id":    "chan-W",
		"connection_id": "other-prefix:bad",
	})
	seedPluginConnectionJob(t, s, owner, enrID, "job-wrongprefix",
		"borgee_plugin.configure_connection", "succeeded", wrong, now.Add(5*time.Minute))

	// One real succeeded → must come through.
	ok := mustPluginJSON(t, map[string]any{
		"agent_id":      "agent-OK",
		"channel_id":    "chan-OK",
		"connection_id": "borgee-plugin:ok",
	})
	seedPluginConnectionJob(t, s, owner, enrID, "job-ok",
		"borgee_plugin.configure_connection", "succeeded", ok, now.Add(6*time.Minute))

	out, err := repo.ListPluginConnections(context.Background(), owner.ID, owner.OrgID, enrID)
	if err != nil {
		t.Fatalf("ListPluginConnections: %v", err)
	}
	if len(out) != 1 || out[0].ConnectionID != "borgee-plugin:ok" {
		t.Fatalf("expected only the succeeded+valid configure to project, got %+v", out)
	}
}

func TestListPluginConnections_ScopesByOwnerOrgEnrollment(t *testing.T) {
	t.Parallel()
	dl, s, owner := newHelperJobRepoFixture(t, "plugin-conn-scope")
	repo := dl.HelperJobRepo
	now := time.UnixMilli(1778900000000)
	enrID1 := newPluginConnEnrollment(t, s, owner, "scope-1", now)
	enrID2 := newPluginConnEnrollment(t, s, owner, "scope-2", now)

	cfg := mustPluginJSON(t, map[string]any{
		"agent_id":      "agent-A",
		"channel_id":    "chan-A",
		"connection_id": "borgee-plugin:scoped",
	})
	seedPluginConnectionJob(t, s, owner, enrID1, "job-1",
		"borgee_plugin.configure_connection", "succeeded", cfg, now.Add(2*time.Minute))
	// Same owner, different enrollment.
	seedPluginConnectionJob(t, s, owner, enrID2, "job-2",
		"borgee_plugin.configure_connection", "succeeded", cfg, now.Add(3*time.Minute))

	out, err := repo.ListPluginConnections(context.Background(), owner.ID, owner.OrgID, enrID1)
	if err != nil {
		t.Fatalf("ListPluginConnections enrID1: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("enrID1 expected 1 row, got %d", len(out))
	}

	// Different owner → empty.
	other := strings.Repeat("9", len(owner.ID))
	out, err = repo.ListPluginConnections(context.Background(), other, owner.OrgID, enrID1)
	if err != nil {
		t.Fatalf("cross-owner list: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("cross-owner expected empty, got %+v", out)
	}

	// Different org → empty.
	out, err = repo.ListPluginConnections(context.Background(), owner.ID, "other-org", enrID1)
	if err != nil {
		t.Fatalf("cross-org list: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("cross-org expected empty, got %+v", out)
	}
}

func TestListPluginConnections_MultipleConnectionsSorted(t *testing.T) {
	t.Parallel()
	dl, s, owner := newHelperJobRepoFixture(t, "plugin-conn-sort")
	repo := dl.HelperJobRepo
	now := time.UnixMilli(1778900000000)
	enrID := newPluginConnEnrollment(t, s, owner, "sort", now)

	for i, conn := range []struct {
		id   string
		when time.Duration
	}{
		{"borgee-plugin:older", 2 * time.Minute},
		{"borgee-plugin:newest", 6 * time.Minute},
		{"borgee-plugin:middle", 4 * time.Minute},
	} {
		p := mustPluginJSON(t, map[string]any{
			"agent_id":      "agent-X",
			"channel_id":    "chan-X",
			"connection_id": conn.id,
		})
		seedPluginConnectionJob(t, s, owner, enrID,
			"job-"+conn.id+"-"+string(rune('a'+i)),
			"borgee_plugin.configure_connection", "succeeded", p, now.Add(conn.when))
	}

	out, err := repo.ListPluginConnections(context.Background(), owner.ID, owner.OrgID, enrID)
	if err != nil {
		t.Fatalf("ListPluginConnections: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 active connections, got %d", len(out))
	}
	if out[0].ConnectionID != "borgee-plugin:newest" {
		t.Fatalf("expected newest first, got %s", out[0].ConnectionID)
	}
	if out[2].ConnectionID != "borgee-plugin:older" {
		t.Fatalf("expected older last, got %s", out[2].ConnectionID)
	}
}
