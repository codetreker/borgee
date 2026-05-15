package store

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestHelperJobEnqueueAuthorityAndActiveIdempotency(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-job-owner")
	now := time.UnixMilli(1778840000000)
	enrollment := claimedFreshHelperEnrollment(t, s, owner, []string{"openclaw_config"}, now)
	agent := helperJobAgent(t, s, owner, "openclaw-agent")
	seedAgentConfig(t, s, agent.ID, 3, map[string]any{"name": "OpenClaw", "enabled": true}, now)

	input := EnqueueHelperJobInput{
		OwnerUserID:    owner.ID,
		OrgID:          owner.OrgID,
		EnrollmentID:   enrollment.ID,
		JobType:        "openclaw.configure_agent",
		SchemaVersion:  1,
		PayloadJSON:    `{"agent_id":"` + agent.ID + `"}`,
		IdempotencyKey: "retry-1",
	}
	job, created, err := s.EnqueueHelperJobForUser(input, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("EnqueueHelperJobForUser: %v", err)
	}
	if !created {
		t.Fatalf("first enqueue should create a row")
	}
	if job.ID == "" || job.Status != "queued" || job.Category != "openclaw_config" || job.JobType != "openclaw.configure_agent" {
		t.Fatalf("bad queued job: %+v", job)
	}
	if job.OwnerUserID != owner.ID || job.OrgID != owner.OrgID || job.EnrollmentID != enrollment.ID {
		t.Fatalf("job did not derive owner/org/enrollment from server state: %+v", job)
	}
	if job.ExpiresAt <= job.CreatedAt || job.ExpiresAt-job.CreatedAt > int64((5*time.Minute+time.Second)/time.Millisecond) {
		t.Fatalf("server TTL out of bounds: created=%d expires=%d", job.CreatedAt, job.ExpiresAt)
	}
	if job.ActiveIdempotencyScope == nil || *job.ActiveIdempotencyScope == "" || job.IdempotencyScope == "" || *job.ActiveIdempotencyScope != job.IdempotencyScope {
		t.Fatalf("active idempotency scope not set from server scope: %+v", job)
	}
	if !strings.HasPrefix(job.PayloadHash, "sha256:") || !strings.HasPrefix(job.ManifestDigest, "sha256:") {
		t.Fatalf("missing safe digests: payload=%q manifest=%q", job.PayloadHash, job.ManifestDigest)
	}
	assertHelperJobPayloadBinding(t, job.PayloadJSON, agent.ID, int64(3))

	again, againCreated, err := s.EnqueueHelperJobForUser(input, now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}
	if againCreated || again.ID != job.ID {
		t.Fatalf("same active idempotency scope should converge to existing job, created=%v job=%+v first=%s", againCreated, again, job.ID)
	}
	if count := countHelperJobs(t, s); count != 1 {
		t.Fatalf("idempotent retry inserted %d jobs, want 1", count)
	}

	otherAgent := helperJobAgent(t, s, owner, "openclaw-agent-2")
	seedAgentConfig(t, s, otherAgent.ID, 1, map[string]any{"name": "Other"}, now)
	conflictInput := input
	conflictInput.PayloadJSON = `{"agent_id":"` + otherAgent.ID + `"}`
	if _, _, err := s.EnqueueHelperJobForUser(conflictInput, now.Add(4*time.Minute)); !errors.Is(err, ErrHelperJobIdempotencyConflict) {
		t.Fatalf("same client idempotency key with different effective payload error=%v, want ErrHelperJobIdempotencyConflict", err)
	}
	if count := countHelperJobs(t, s); count != 1 {
		t.Fatalf("idempotency conflict inserted %d jobs, want 1", count)
	}

	freshAt := now.Add(8 * time.Minute).UnixMilli()
	if err := s.DB().Model(&HelperEnrollment{}).Where("id = ?", enrollment.ID).Update("last_seen_at", freshAt).Error; err != nil {
		t.Fatalf("refresh enrollment before post-expiry enqueue: %v", err)
	}
	afterExpiry, createdAfterExpiry, err := s.EnqueueHelperJobForUser(input, now.Add(8*time.Minute))
	if err != nil {
		t.Fatalf("enqueue after active TTL expiry: %v", err)
	}
	if !createdAfterExpiry || afterExpiry.ID == job.ID {
		t.Fatalf("expired active scope should not permanently block new enqueue: created=%v job=%+v first=%s", createdAfterExpiry, afterExpiry, job.ID)
	}
}

func TestHelperJobEnqueueRejectsInactiveDelegationAndClosedTaxonomy(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-job-reject")
	other := helperOwner(t, s, "helper-job-other")
	now := time.UnixMilli(1778840000000)
	fresh := claimedFreshHelperEnrollment(t, s, owner, []string{"openclaw_config"}, now)
	statusOnly := claimedFreshHelperEnrollment(t, s, owner, []string{"status_collect"}, now)
	stale := claimedFreshHelperEnrollment(t, s, owner, []string{"openclaw_config"}, now)
	oldLastSeen := now.Add(-10 * time.Minute).UnixMilli()
	if err := s.DB().Model(&HelperEnrollment{}).Where("id = ?", stale.ID).Update("last_seen_at", oldLastSeen).Error; err != nil {
		t.Fatalf("seed stale enrollment: %v", err)
	}
	agent := helperJobAgent(t, s, owner, "reject-agent")
	otherAgent := helperJobAgent(t, s, other, "reject-other-agent")
	seedAgentConfig(t, s, agent.ID, 1, map[string]any{"name": "A"}, now)
	seedAgentConfig(t, s, otherAgent.ID, 1, map[string]any{"name": "B"}, now)

	base := EnqueueHelperJobInput{
		OwnerUserID:   owner.ID,
		OrgID:         owner.OrgID,
		EnrollmentID:  fresh.ID,
		JobType:       "openclaw.configure_agent",
		SchemaVersion: 1,
		PayloadJSON:   `{"agent_id":"` + agent.ID + `"}`,
	}
	cases := []struct {
		name string
		mut  func(EnqueueHelperJobInput) EnqueueHelperJobInput
		want error
	}{
		{"wrong owner", func(in EnqueueHelperJobInput) EnqueueHelperJobInput { in.OwnerUserID = other.ID; return in }, ErrHelperJobForbidden},
		{"wrong org", func(in EnqueueHelperJobInput) EnqueueHelperJobInput { in.OrgID = other.OrgID; return in }, ErrHelperJobForbidden},
		{"stale enrollment", func(in EnqueueHelperJobInput) EnqueueHelperJobInput { in.EnrollmentID = stale.ID; return in }, ErrHelperJobEnrollmentInactive},
		{"delegation denied", func(in EnqueueHelperJobInput) EnqueueHelperJobInput { in.EnrollmentID = statusOnly.ID; return in }, ErrHelperJobDelegationDenied},
		{"unknown type", func(in EnqueueHelperJobInput) EnqueueHelperJobInput { in.JobType = "shell"; return in }, ErrHelperJobUnknownType},
		{"recognized disabled type", func(in EnqueueHelperJobInput) EnqueueHelperJobInput { in.JobType = "service.lifecycle"; return in }, ErrHelperJobTypeNotEnabled},
		{"schema version", func(in EnqueueHelperJobInput) EnqueueHelperJobInput { in.SchemaVersion = 2; return in }, ErrHelperJobSchemaInvalid},
		{"cross-owner agent", func(in EnqueueHelperJobInput) EnqueueHelperJobInput {
			in.PayloadJSON = `{"agent_id":"` + otherAgent.ID + `"}`
			return in
		}, ErrHelperJobForbidden},
		{"payload forbidden field", func(in EnqueueHelperJobInput) EnqueueHelperJobInput {
			in.PayloadJSON = `{"agent_id":"` + agent.ID + `","shell":"whoami"}`
			return in
		}, ErrHelperJobForbiddenField},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := s.EnqueueHelperJobForUser(tc.mut(base), now.Add(2*time.Minute))
			if !errors.Is(err, tc.want) {
				t.Fatalf("error=%v, want %v", err, tc.want)
			}
		})
	}
	if count := countHelperJobs(t, s); count != 0 {
		t.Fatalf("rejected enqueue attempts inserted %d jobs, want 0", count)
	}
}

func claimedFreshHelperEnrollment(t *testing.T, s *Store, owner *User, categories []string, now time.Time) *HelperEnrollment {
	t.Helper()
	enrollment, secret, err := s.CreateHelperEnrollment(owner.ID, "Mac Studio", categories, now)
	if err != nil {
		t.Fatalf("CreateHelperEnrollment: %v", err)
	}
	claimed, _, err := s.ClaimHelperEnrollment(enrollment.ID, secret, "device-"+enrollment.ID, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ClaimHelperEnrollment: %v", err)
	}
	if _, err := s.UpdateHelperEnrollmentLastSeen(claimed.ID, *claimed.PersistentCredentialDigest, "device-"+enrollment.ID, now.Add(90*time.Second)); err == nil {
		t.Fatalf("test fixture accidentally authenticated with digest as credential")
	}
	return claimed
}

func helperJobAgent(t *testing.T, s *Store, owner *User, name string) *User {
	t.Helper()
	apiKey := name + "-key"
	agent := &User{DisplayName: name, Role: "agent", OwnerID: &owner.ID, APIKey: &apiKey, OrgID: owner.OrgID, PasswordHash: "hash"}
	if err := s.CreateUser(agent); err != nil {
		t.Fatalf("CreateUser agent: %v", err)
	}
	return agent
}

func seedAgentConfig(t *testing.T, s *Store, agentID string, version int64, blob map[string]any, now time.Time) {
	t.Helper()
	b, err := json.Marshal(blob)
	if err != nil {
		t.Fatalf("marshal config blob: %v", err)
	}
	if err := s.DB().Exec(`INSERT INTO agent_configs (agent_id, schema_version, blob, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, agentID, version, string(b), now.UnixMilli(), now.UnixMilli()).Error; err != nil {
		t.Fatalf("seed agent config: %v", err)
	}
}

func assertHelperJobPayloadBinding(t *testing.T, payload string, agentID string, version int64) {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if got["agent_id"] != agentID {
		t.Fatalf("payload agent_id=%v, want %s in %v", got["agent_id"], agentID, got)
	}
	if got["config_schema_version"] != float64(version) {
		t.Fatalf("payload config_schema_version=%v, want %d in %v", got["config_schema_version"], version, got)
	}
	if hash, _ := got["config_hash"].(string); !strings.HasPrefix(hash, "sha256:") {
		t.Fatalf("payload missing config_hash sha256 digest: %v", got)
	}
	for _, key := range []string{"owner_user_id", "org_id", "credential", "token", "shell", "argv", "script", "service_unit", "path", "domain", "url"} {
		if _, ok := got[key]; ok {
			t.Fatalf("payload leaked forbidden key %q: %v", key, got)
		}
	}
}

func countHelperJobs(t *testing.T, s *Store) int64 {
	t.Helper()
	var count int64
	if err := s.DB().Table("helper_jobs").Count(&count).Error; err != nil {
		t.Fatalf("count helper_jobs: %v", err)
	}
	return count
}
