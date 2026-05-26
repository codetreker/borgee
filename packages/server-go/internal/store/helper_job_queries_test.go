package store

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
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
	assertHelperJobManifestBinding(t, job, []string{"openclaw_agent_config"}, nil, nil)
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

func TestHelperJobOpenClawInstallFromManifestIsServerBound(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-job-install-openclaw")
	now := time.UnixMilli(1778840000000)
	enrollment := claimedFreshHelperEnrollment(t, s, owner, []string{"openclaw_lifecycle"}, now)

	job, created, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{
		OwnerUserID:    owner.ID,
		OrgID:          owner.OrgID,
		EnrollmentID:   enrollment.ID,
		JobType:        "openclaw.install_from_manifest",
		SchemaVersion:  1,
		PayloadJSON:    `{"runtime":"openclaw"}`,
		IdempotencyKey: "install-openclaw-1",
	}, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("EnqueueHelperJobForUser install: %v", err)
	}
	if !created || job.Status != HelperJobStatusQueued || job.Category != "openclaw_lifecycle" {
		t.Fatalf("bad install job: created=%v job=%+v", created, job)
	}
	assertHelperJobInstallPayload(t, job.PayloadJSON)
	assertHelperJobManifestBinding(t, job, []string{"openclaw_install", "openclaw_agent_config"}, []string{"openclaw-plugin"}, []string{"https://cdn.borgee.io"})
	if job.ManifestDigest == helperJobDigest([]byte("no-manifest")) {
		t.Fatalf("install job must bind a real manifest digest, got %q", job.ManifestDigest)
	}

	denied := []string{
		`{"runtime":"openclaw","manifest_id":"client"}`,
		`{"runtime":"openclaw","manifest_digest":"sha256:client"}`,
		`{"runtime":"openclaw","artifact_ids":["client"]}`,
		`{"runtime":"openclaw","path_ids":["client"]}`,
		`{"runtime":"openclaw","domain":"https://evil.example"}`,
	}
	for _, payload := range denied {
		_, _, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{
			OwnerUserID:   owner.ID,
			OrgID:         owner.OrgID,
			EnrollmentID:  enrollment.ID,
			JobType:       "openclaw.install_from_manifest",
			SchemaVersion: 1,
			PayloadJSON:   payload,
		}, now.Add(3*time.Minute))
		if !errors.Is(err, ErrHelperJobForbiddenField) {
			t.Fatalf("payload %s error=%v, want ErrHelperJobForbiddenField", payload, err)
		}
	}
}

func TestHelperJobServiceLifecycleIsServerBoundToDeclaredServiceID(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-job-service-lifecycle")
	now := time.UnixMilli(1778840000000)
	enrollment := claimedFreshHelperEnrollment(t, s, owner, []string{"openclaw_lifecycle"}, now)

	job, created, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{
		OwnerUserID:    owner.ID,
		OrgID:          owner.OrgID,
		EnrollmentID:   enrollment.ID,
		JobType:        "service.lifecycle",
		SchemaVersion:  1,
		PayloadJSON:    `{"target":"openclaw","operation":"restart"}`,
		IdempotencyKey: "restart-openclaw-1",
	}, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("EnqueueHelperJobForUser service lifecycle: %v", err)
	}
	if !created || job.Status != HelperJobStatusQueued || job.Category != "openclaw_lifecycle" || job.JobType != "service.lifecycle" {
		t.Fatalf("bad service lifecycle job: created=%v job=%+v", created, job)
	}
	assertHelperJobServiceLifecyclePayload(t, job.PayloadJSON)
	assertHelperJobServiceBinding(t, job, []string{"openclaw-user"})

	for _, payload := range []string{
		`{"target":"openclaw","operation":"restart","service_id":"evil"}`,
		`{"target":"openclaw","operation":"restart","service_unit":"evil.service"}`,
		`{"target":"openclaw","operation":"restart","command":"systemctl restart evil"}`,
	} {
		_, _, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{
			OwnerUserID:   owner.ID,
			OrgID:         owner.OrgID,
			EnrollmentID:  enrollment.ID,
			JobType:       "service.lifecycle",
			SchemaVersion: 1,
			PayloadJSON:   payload,
		}, now.Add(3*time.Minute))
		if !errors.Is(err, ErrHelperJobForbiddenField) {
			t.Fatalf("payload %s error=%v, want ErrHelperJobForbiddenField", payload, err)
		}
	}

	for _, payload := range []string{
		`{"target":"helper","operation":"restart"}`,
		`{"target":"openclaw","operation":"bounce"}`,
		`{"target":"openclaw","operation":""}`,
	} {
		_, _, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{
			OwnerUserID:   owner.ID,
			OrgID:         owner.OrgID,
			EnrollmentID:  enrollment.ID,
			JobType:       "service.lifecycle",
			SchemaVersion: 1,
			PayloadJSON:   payload,
		}, now.Add(3*time.Minute))
		if !errors.Is(err, ErrHelperJobSchemaInvalid) {
			t.Fatalf("payload %s error=%v, want ErrHelperJobSchemaInvalid", payload, err)
		}
	}

	// PR-4 (#1033): server now accepts the full 6-operation set the helper
	// jobpolicy.allowedServiceOperation defines (start/stop/restart/reload
	// /enable/disable). Earlier server-side limit of restart-only was a
	// holdover from the pre-rootd era; the rootd whitelist enforces the
	// same set on the privileged side.
	for _, op := range []string{"start", "stop", "restart", "reload", "enable", "disable"} {
		_, _, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{
			OwnerUserID:    owner.ID,
			OrgID:          owner.OrgID,
			EnrollmentID:   enrollment.ID,
			JobType:        "service.lifecycle",
			SchemaVersion:  1,
			PayloadJSON:    `{"target":"openclaw","operation":"` + op + `"}`,
			IdempotencyKey: "lifecycle-op-" + op,
		}, now.Add(4*time.Minute))
		if err != nil {
			t.Fatalf("lifecycle operation %q rejected: %v", op, err)
		}
	}
}

// PR-4 (#1033): server enum + payload + binding for state.write. Verifies
// the effective payload narrows to {state_key, value_sha256} (operator
// cannot smuggle path/category/credential authority via unknown fields),
// the binding declares borgee_state_config PathID only, and idempotency
// scope round-trips a second equivalent enqueue without creating a
// duplicate row.
func TestHelperJobStateWriteIsServerBoundToBorgeeStateConfigPath(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-job-state-write")
	now := time.UnixMilli(1778840000000)
	enrollment := claimedFreshHelperEnrollment(t, s, owner, []string{"openclaw_config"}, now)

	job, created, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{
		OwnerUserID:    owner.ID,
		OrgID:          owner.OrgID,
		EnrollmentID:   enrollment.ID,
		JobType:        "state.write",
		SchemaVersion:  1,
		PayloadJSON:    `{"state_key":"openclaw/cfg","value_sha256":"sha256:abcd"}`,
		IdempotencyKey: "state-write-cfg-1",
	}, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("EnqueueHelperJobForUser state.write: %v", err)
	}
	if !created || job.Status != HelperJobStatusQueued || job.Category != "openclaw_config" || job.JobType != "state.write" {
		t.Fatalf("bad state.write job: created=%v job=%+v", created, job)
	}
	assertHelperJobStateWritePayload(t, job.PayloadJSON, "openclaw/cfg", "sha256:abcd")
	assertHelperJobManifestBinding(t, job, []string{"borgee_state_config"}, nil, nil)

	// Re-enqueue identical contract: idempotency scope hit, same row.
	job2, created2, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{
		OwnerUserID:    owner.ID,
		OrgID:          owner.OrgID,
		EnrollmentID:   enrollment.ID,
		JobType:        "state.write",
		SchemaVersion:  1,
		PayloadJSON:    `{"state_key":"openclaw/cfg","value_sha256":"sha256:abcd"}`,
		IdempotencyKey: "state-write-cfg-1",
	}, now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("EnqueueHelperJobForUser state.write idempotent: %v", err)
	}
	if created2 || job2.ID != job.ID {
		t.Fatalf("idempotent state.write created a new row: created=%v id=%s want %s", created2, job2.ID, job.ID)
	}

	// Forbidden-field set on state.write payload.
	for _, payload := range []string{
		`{"state_key":"k","path":"/etc/passwd"}`,
		`{"state_key":"k","credential":"secret"}`,
		`{"state_key":"k","path_id":"override"}`,
		`{"state_key":"k","service_id":"evil"}`,
	} {
		_, _, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{
			OwnerUserID:   owner.ID,
			OrgID:         owner.OrgID,
			EnrollmentID:  enrollment.ID,
			JobType:       "state.write",
			SchemaVersion: 1,
			PayloadJSON:   payload,
		}, now.Add(4*time.Minute))
		if !errors.Is(err, ErrHelperJobForbiddenField) {
			t.Fatalf("payload %s error=%v, want ErrHelperJobForbiddenField", payload, err)
		}
	}

	// Schema-invalid: missing state_key, traversal, absolute path, bad
	// value_sha256 prefix.
	for _, payload := range []string{
		`{}`,
		`{"state_key":""}`,
		`{"state_key":"../escape"}`,
		`{"state_key":"/abs/path"}`,
		`{"state_key":"a/./b"}`,
		`{"state_key":"k","value_sha256":"hex-not-sha-prefixed"}`,
	} {
		_, _, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{
			OwnerUserID:   owner.ID,
			OrgID:         owner.OrgID,
			EnrollmentID:  enrollment.ID,
			JobType:       "state.write",
			SchemaVersion: 1,
			PayloadJSON:   payload,
		}, now.Add(5*time.Minute))
		if !errors.Is(err, ErrHelperJobSchemaInvalid) {
			t.Fatalf("payload %s error=%v, want ErrHelperJobSchemaInvalid", payload, err)
		}
	}
}

// PR-4 (#1033): server enum + payload for status.collect. status.collect
// is not in helper-side jobpolicy.requiresManifest — the job row carries
// no manifest binding (ManifestBindingJSON nil), and the helper executor
// returns the snapshot in ResultSummary without any filesystem write.
func TestHelperJobStatusCollectAcceptsAllowedScopesAndCarriesNoManifestBinding(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-job-status-collect")
	now := time.UnixMilli(1778840000000)
	enrollment := claimedFreshHelperEnrollment(t, s, owner, []string{"status_collect"}, now)

	for _, scope := range []string{"helper", "openclaw", "service"} {
		job, created, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{
			OwnerUserID:    owner.ID,
			OrgID:          owner.OrgID,
			EnrollmentID:   enrollment.ID,
			JobType:        "status.collect",
			SchemaVersion:  1,
			PayloadJSON:    `{"scope":"` + scope + `"}`,
			IdempotencyKey: "status-collect-" + scope,
		}, now.Add(2*time.Minute))
		if err != nil {
			t.Fatalf("EnqueueHelperJobForUser status.collect %s: %v", scope, err)
		}
		if !created || job.Category != "status_collect" || job.JobType != "status.collect" {
			t.Fatalf("bad status.collect job (scope=%s): created=%v job=%+v", scope, created, job)
		}
		if job.ManifestDigest != "" {
			t.Fatalf("status.collect must carry empty manifest digest, got %q", job.ManifestDigest)
		}
		if job.ManifestBindingJSON != nil {
			t.Fatalf("status.collect must carry nil manifest binding, got %v", job.ManifestBindingJSON)
		}
	}

	for _, payload := range []string{
		`{"scope":""}`,
		`{"scope":"unknown"}`,
		`{}`,
		`{"scope":"helper","path":"/etc"}`,
	} {
		_, _, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{
			OwnerUserID:   owner.ID,
			OrgID:         owner.OrgID,
			EnrollmentID:  enrollment.ID,
			JobType:       "status.collect",
			SchemaVersion: 1,
			PayloadJSON:   payload,
		}, now.Add(3*time.Minute))
		if err == nil {
			t.Fatalf("payload %s accepted, want rejection", payload)
		}
	}
}

// PR-4 (#1033): server enum + payload for delegation.revoke. Like
// status.collect, no manifest authority is attached — the operation is
// the removal of authority, not the use of it.
func TestHelperJobDelegationRevokeAcceptsCategoriesAndCarriesNoManifestBinding(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-job-delegation-revoke")
	now := time.UnixMilli(1778840000000)
	enrollment := claimedFreshHelperEnrollment(t, s, owner, []string{"helper_lifecycle"}, now)

	for _, category := range []string{"openclaw_config", "openclaw_lifecycle", "status_collect", "helper_lifecycle"} {
		job, created, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{
			OwnerUserID:    owner.ID,
			OrgID:          owner.OrgID,
			EnrollmentID:   enrollment.ID,
			JobType:        "delegation.revoke",
			SchemaVersion:  1,
			PayloadJSON:    `{"target_category":"` + category + `"}`,
			IdempotencyKey: "revoke-" + category,
		}, now.Add(2*time.Minute))
		if err != nil {
			t.Fatalf("EnqueueHelperJobForUser delegation.revoke %s: %v", category, err)
		}
		if !created || job.Category != "helper_lifecycle" || job.JobType != "delegation.revoke" {
			t.Fatalf("bad delegation.revoke job (category=%s): created=%v job=%+v", category, created, job)
		}
		if job.ManifestDigest != "" || job.ManifestBindingJSON != nil {
			t.Fatalf("delegation.revoke must carry no manifest authority, got digest=%q binding=%v", job.ManifestDigest, job.ManifestBindingJSON)
		}
	}

	for _, payload := range []string{
		`{"target_category":""}`,
		`{"target_category":"unknown"}`,
		`{}`,
		`{"target_category":"helper_lifecycle","credential":"secret"}`,
	} {
		_, _, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{
			OwnerUserID:   owner.ID,
			OrgID:         owner.OrgID,
			EnrollmentID:  enrollment.ID,
			JobType:       "delegation.revoke",
			SchemaVersion: 1,
			PayloadJSON:   payload,
		}, now.Add(3*time.Minute))
		if err == nil {
			t.Fatalf("payload %s accepted, want rejection", payload)
		}
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
	missingLastSeen := claimedFreshHelperEnrollment(t, s, owner, []string{"openclaw_config"}, now)
	if err := s.DB().Exec(`UPDATE helper_enrollments SET last_seen_at = NULL WHERE id = ?`, missingLastSeen.ID).Error; err != nil {
		t.Fatalf("seed missing last_seen_at enrollment: %v", err)
	}
	uninstalled, uninstallSecret, err := s.CreateHelperEnrollment(owner.ID, "Uninstalled Helper", []string{"openclaw_config"}, now)
	if err != nil {
		t.Fatalf("CreateHelperEnrollment uninstalled fixture: %v", err)
	}
	_, uninstallCredential, err := s.ClaimHelperEnrollment(uninstalled.ID, uninstallSecret, "device-uninstalled", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ClaimHelperEnrollment uninstalled fixture: %v", err)
	}
	if _, err := s.MarkHelperEnrollmentUninstalled(uninstalled.ID, uninstallCredential, "device-uninstalled", now.Add(2*time.Minute)); err != nil {
		t.Fatalf("MarkHelperEnrollmentUninstalled fixture: %v", err)
	}
	agentOwner := helperJobAgent(t, s, owner, "reject-agent-owner")
	legacyAgentEnrollment := legacyClaimedHelperEnrollmentForOwner(t, s, agentOwner, []string{"openclaw_config"}, now)
	agent := helperJobAgent(t, s, owner, "reject-agent")
	agentOwnedChild := helperJobAgent(t, s, agentOwner, "reject-agent-owned-child")
	otherAgent := helperJobAgent(t, s, other, "reject-other-agent")
	seedAgentConfig(t, s, agent.ID, 1, map[string]any{"name": "A"}, now)
	seedAgentConfig(t, s, agentOwnedChild.ID, 1, map[string]any{"name": "Agent child"}, now)
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
		{"nonexistent enrollment", func(in EnqueueHelperJobInput) EnqueueHelperJobInput {
			in.EnrollmentID = "missing-helper-enrollment"
			return in
		}, ErrHelperJobEnrollmentNotFound},
		{"uninstalled enrollment", func(in EnqueueHelperJobInput) EnqueueHelperJobInput { in.EnrollmentID = uninstalled.ID; return in }, ErrHelperJobEnrollmentUninstalled},
		{"stale enrollment", func(in EnqueueHelperJobInput) EnqueueHelperJobInput { in.EnrollmentID = stale.ID; return in }, ErrHelperJobEnrollmentInactive},
		{"missing last_seen_at", func(in EnqueueHelperJobInput) EnqueueHelperJobInput { in.EnrollmentID = missingLastSeen.ID; return in }, ErrHelperJobEnrollmentInactive},
		{"delegation denied", func(in EnqueueHelperJobInput) EnqueueHelperJobInput { in.EnrollmentID = statusOnly.ID; return in }, ErrHelperJobDelegationDenied},
		{"unknown type", func(in EnqueueHelperJobInput) EnqueueHelperJobInput { in.JobType = "shell"; return in }, ErrHelperJobUnknownType},
		{"install requires lifecycle delegation", func(in EnqueueHelperJobInput) EnqueueHelperJobInput {
			in.JobType = "openclaw.install_from_manifest"
			in.PayloadJSON = `{"runtime":"openclaw"}`
			return in
		}, ErrHelperJobDelegationDenied},
		{"plugin connection requires channel binding", func(in EnqueueHelperJobInput) EnqueueHelperJobInput {
			in.JobType = "borgee_plugin.configure_connection"
			return in
		}, ErrHelperJobSchemaInvalid},
		{"service lifecycle requires lifecycle delegation", func(in EnqueueHelperJobInput) EnqueueHelperJobInput {
			in.JobType = "service.lifecycle"
			in.PayloadJSON = `{"target":"openclaw"}`
			return in
		}, ErrHelperJobDelegationDenied},
		{"state write payload requires state_key", func(in EnqueueHelperJobInput) EnqueueHelperJobInput {
			in.JobType = "state.write"
			// base PayloadJSON is {agent_id:...} which lacks state_key and
			// carries an unknown field under strict decode.
			return in
		}, ErrHelperJobSchemaInvalid},
		{"status collect requires status delegation", func(in EnqueueHelperJobInput) EnqueueHelperJobInput {
			in.JobType = "status.collect"
			in.PayloadJSON = `{"scope":"helper"}`
			return in
		}, ErrHelperJobDelegationDenied},
		{"delegation revoke requires helper-lifecycle delegation", func(in EnqueueHelperJobInput) EnqueueHelperJobInput {
			in.JobType = "delegation.revoke"
			in.PayloadJSON = `{"target_category":"openclaw_config"}`
			return in
		}, ErrHelperJobDelegationDenied},
		{"recognized helper uninstall type", func(in EnqueueHelperJobInput) EnqueueHelperJobInput { in.JobType = "helper.uninstall"; return in }, ErrHelperJobDelegationDenied},
		{"schema version", func(in EnqueueHelperJobInput) EnqueueHelperJobInput { in.SchemaVersion = 2; return in }, ErrHelperJobSchemaInvalid},
		{"cross-owner agent", func(in EnqueueHelperJobInput) EnqueueHelperJobInput {
			in.PayloadJSON = `{"agent_id":"` + otherAgent.ID + `"}`
			return in
		}, ErrHelperJobForbidden},
		{"agent owner authority", func(in EnqueueHelperJobInput) EnqueueHelperJobInput {
			in.OwnerUserID = agentOwner.ID
			in.OrgID = agentOwner.OrgID
			in.EnrollmentID = legacyAgentEnrollment.ID
			in.PayloadJSON = `{"agent_id":"` + agentOwnedChild.ID + `"}`
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

func TestHelperJobChannelBindingRequiresTargetAgentAccess(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-job-channel-owner")
	now := time.UnixMilli(1778840000000)
	enrollment := claimedFreshHelperEnrollment(t, s, owner, []string{"openclaw_config"}, now)
	agent := helperJobAgent(t, s, owner, "channel-bound-agent")
	seedAgentConfig(t, s, agent.ID, 1, map[string]any{"name": "Channel Bound"}, now)
	privateChannel := helperJobChannel(t, s, owner, "helper-job-private", "private")
	if err := s.AddChannelMember(&ChannelMember{ChannelID: privateChannel.ID, UserID: owner.ID}); err != nil {
		t.Fatalf("add owner channel member: %v", err)
	}

	input := EnqueueHelperJobInput{
		OwnerUserID:   owner.ID,
		OrgID:         owner.OrgID,
		EnrollmentID:  enrollment.ID,
		JobType:       "openclaw.configure_agent",
		SchemaVersion: 1,
		PayloadJSON:   `{"agent_id":"` + agent.ID + `","channel_id":"` + privateChannel.ID + `"}`,
	}
	if _, _, err := s.EnqueueHelperJobForUser(input, now.Add(2*time.Minute)); !errors.Is(err, ErrHelperJobForbidden) {
		t.Fatalf("private channel without target agent access error=%v, want ErrHelperJobForbidden", err)
	}
	if count := countHelperJobs(t, s); count != 0 {
		t.Fatalf("denied channel binding inserted %d jobs, want 0", count)
	}

	if err := s.AddChannelMember(&ChannelMember{ChannelID: privateChannel.ID, UserID: agent.ID}); err != nil {
		t.Fatalf("add agent channel member: %v", err)
	}
	job, created, err := s.EnqueueHelperJobForUser(input, now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("private channel with target agent access: %v", err)
	}
	if !created || job.Status != "queued" {
		t.Fatalf("expected queued job after channel access grant, created=%v job=%+v", created, job)
	}
	assertHelperJobPayloadBinding(t, job.PayloadJSON, agent.ID, int64(1))
}

func TestHelperJobPluginConfigureConnectionIsServerBound(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-job-plugin-owner")
	now := time.UnixMilli(1778840000000)
	enrollment := claimedFreshHelperEnrollment(t, s, owner, []string{"openclaw_config"}, now)
	agent := helperJobAgent(t, s, owner, "plugin-bound-agent")
	privateChannel := helperJobChannel(t, s, owner, "helper-job-plugin-private", "private")
	if err := s.AddChannelMember(&ChannelMember{ChannelID: privateChannel.ID, UserID: owner.ID}); err != nil {
		t.Fatalf("add owner channel member: %v", err)
	}

	input := EnqueueHelperJobInput{
		OwnerUserID:    owner.ID,
		OrgID:          owner.OrgID,
		EnrollmentID:   enrollment.ID,
		JobType:        "borgee_plugin.configure_connection",
		SchemaVersion:  1,
		PayloadJSON:    `{"agent_id":"` + agent.ID + `","channel_id":"` + privateChannel.ID + `"}`,
		IdempotencyKey: "plugin-bind-1",
	}
	if _, _, err := s.EnqueueHelperJobForUser(input, now.Add(2*time.Minute)); !errors.Is(err, ErrHelperJobForbidden) {
		t.Fatalf("plugin binding without target agent channel access error=%v, want ErrHelperJobForbidden", err)
	}
	if count := countHelperJobs(t, s); count != 0 {
		t.Fatalf("denied plugin binding inserted %d jobs, want 0", count)
	}

	if err := s.AddChannelMember(&ChannelMember{ChannelID: privateChannel.ID, UserID: agent.ID}); err != nil {
		t.Fatalf("add agent channel member: %v", err)
	}
	job, created, err := s.EnqueueHelperJobForUser(input, now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("plugin binding with target agent channel access: %v", err)
	}
	if !created || job.Status != HelperJobStatusQueued || job.Category != "openclaw_config" {
		t.Fatalf("bad plugin binding job: created=%v job=%+v", created, job)
	}
	assertHelperJobPluginConnectionPayload(t, job.PayloadJSON, agent.ID, privateChannel.ID)
	assertHelperJobManifestBinding(t, job, []string{"borgee_plugin_config"}, nil, nil)
	if !strings.HasPrefix(job.ManifestDigest, "sha256:") {
		t.Fatalf("plugin binding job missing manifest digest: %+v", job)
	}

	again, againCreated, err := s.EnqueueHelperJobForUser(input, now.Add(4*time.Minute))
	if err != nil {
		t.Fatalf("idempotent plugin binding retry: %v", err)
	}
	if againCreated || again.ID != job.ID {
		t.Fatalf("same plugin binding idempotency scope should converge, created=%v again=%+v first=%s", againCreated, again, job.ID)
	}

	for _, payload := range []string{
		`{"agent_id":"` + agent.ID + `"}`,
		`{"agent_id":"` + agent.ID + `","channel_id":"` + privateChannel.ID + `","connection_id":"client"}`,
		`{"agent_id":"` + agent.ID + `","channel_id":"` + privateChannel.ID + `","base_url":"https://evil.example"}`,
		`{"agent_id":"` + agent.ID + `","channel_id":"` + privateChannel.ID + `","api_key":"secret"}`,
	} {
		_, _, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{
			OwnerUserID:   owner.ID,
			OrgID:         owner.OrgID,
			EnrollmentID:  enrollment.ID,
			JobType:       "borgee_plugin.configure_connection",
			SchemaVersion: 1,
			PayloadJSON:   payload,
		}, now.Add(5*time.Minute))
		if err == nil {
			t.Fatalf("plugin binding payload %s unexpectedly succeeded", payload)
		}
	}
}

func TestHelperJobPollAckResultLeaseIdempotencyAndBoundaries(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-job-lease-owner")
	now := time.UnixMilli(1778840000000)
	enrollment, credential := claimedFreshHelperEnrollmentWithCredential(t, s, owner, []string{"openclaw_config"}, now)
	agent := helperJobAgent(t, s, owner, "lease-openclaw-agent")
	seedAgentConfig(t, s, agent.ID, 5, map[string]any{"name": "Lease Agent"}, now)

	job, created, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{
		OwnerUserID:    owner.ID,
		OrgID:          owner.OrgID,
		EnrollmentID:   enrollment.ID,
		JobType:        "openclaw.configure_agent",
		SchemaVersion:  1,
		PayloadJSON:    `{"agent_id":"` + agent.ID + `"}`,
		IdempotencyKey: "lease-result-1",
	}, now.Add(2*time.Minute))
	if err != nil || !created {
		t.Fatalf("EnqueueHelperJobForUser created=%v err=%v", created, err)
	}

	lease, err := s.PollAndLeaseHelperJobForHelper(PollHelperJobInput{
		EnrollmentID:      enrollment.ID,
		HelperCredential:  credential,
		HelperDeviceID:    *enrollment.HelperDeviceID,
		LeaseDuration:     time.Minute,
		RetryAfterNoWork:  5 * time.Second,
		MaxActiveLeases:   1,
		AllowedCategories: []string{"openclaw_config"},
	}, now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("PollAndLeaseHelperJobForHelper: %v", err)
	}
	if lease == nil || lease.Job == nil || lease.Job.ID != job.ID || lease.Job.Status != HelperJobStatusLeased || lease.LeaseToken == "" {
		t.Fatalf("bad lease: %+v", lease)
	}
	if lease.Job.PayloadJSON == "" || lease.Job.OwnerUserID == "" || lease.Job.OrgID == "" {
		t.Fatalf("lease projection must carry payload + owner/org for daemon jobpolicy schema gate (#1050 blocker #2): %+v", lease.Job)
	}
	if lease.RetryAfter != 0 || lease.Attempt != 1 || lease.LeaseExpiresAt <= now.UnixMilli() {
		t.Fatalf("lease metadata not populated: %+v", lease)
	}

	duplicate, err := s.PollAndLeaseHelperJobForHelper(PollHelperJobInput{
		EnrollmentID:     enrollment.ID,
		HelperCredential: credential,
		HelperDeviceID:   *enrollment.HelperDeviceID,
		LeaseDuration:    time.Minute,
		RetryAfterNoWork: 5 * time.Second,
	}, now.Add(3*time.Minute+time.Second))
	if err != nil {
		t.Fatalf("duplicate poll should converge to no work, got err=%v", err)
	}
	if duplicate == nil || duplicate.Job != nil || duplicate.Status != HelperJobPollNoWork || duplicate.RetryAfter != 5*time.Second {
		t.Fatalf("duplicate poll leased extra work: %+v", duplicate)
	}

	acked, err := s.AckHelperJobForHelper(AckHelperJobInput{
		EnrollmentID:     enrollment.ID,
		JobID:            job.ID,
		HelperCredential: credential,
		HelperDeviceID:   *enrollment.HelperDeviceID,
		LeaseToken:       lease.LeaseToken,
		AckStatus:        "received",
	}, now.Add(3*time.Minute+2*time.Second))
	if err != nil || acked == nil || acked.Status != HelperJobStatusRunning {
		t.Fatalf("AckHelperJobForHelper job=%+v err=%v", acked, err)
	}
	ackedAgain, err := s.AckHelperJobForHelper(AckHelperJobInput{
		EnrollmentID:     enrollment.ID,
		JobID:            job.ID,
		HelperCredential: credential,
		HelperDeviceID:   *enrollment.HelperDeviceID,
		LeaseToken:       lease.LeaseToken,
		AckStatus:        "received",
	}, now.Add(3*time.Minute+3*time.Second))
	if err != nil || ackedAgain == nil || ackedAgain.Status != HelperJobStatusRunning {
		t.Fatalf("idempotent ack job=%+v err=%v", ackedAgain, err)
	}

	terminal := CompleteHelperJobInput{
		EnrollmentID:       enrollment.ID,
		JobID:              job.ID,
		HelperCredential:   credential,
		HelperDeviceID:     *enrollment.HelperDeviceID,
		LeaseToken:         lease.LeaseToken,
		Status:             HelperJobStatusFailed,
		FailureCode:        "policy_denied",
		FailureMessage:     "policy handoff denied",
		ResultSummaryJSON:  `{"audit_refs":["audit-1"],"log_refs":[]}`,
		MaxFailureMessage:  256,
		MaxResultSummaries: 4,
	}
	completed, err := s.CompleteHelperJobForHelper(terminal, now.Add(3*time.Minute+4*time.Second))
	if err != nil || completed == nil || completed.Status != HelperJobStatusFailed || completed.ActiveIdempotencyScope != nil || completed.CompletedAt == nil {
		t.Fatalf("CompleteHelperJobForHelper job=%+v err=%v", completed, err)
	}
	completedAgain, err := s.CompleteHelperJobForHelper(terminal, now.Add(3*time.Minute+5*time.Second))
	if err != nil || completedAgain == nil || completedAgain.Status != HelperJobStatusFailed {
		t.Fatalf("same terminal replay job=%+v err=%v", completedAgain, err)
	}
	terminal.FailureCode = "execution_failed"
	if _, err := s.CompleteHelperJobForHelper(terminal, now.Add(3*time.Minute+6*time.Second)); !errors.Is(err, ErrHelperJobTerminalConflict) {
		t.Fatalf("conflicting terminal replay error=%v, want ErrHelperJobTerminalConflict", err)
	}
}

func TestHelperJobTerminalInputRequiresReasonAndRedactsSensitiveFailureMessage(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-job-terminal-redaction")
	now := time.UnixMilli(1778840000000)
	enrollment, credential := claimedFreshHelperEnrollmentWithCredential(t, s, owner, []string{"openclaw_config"}, now)
	agent := helperJobAgent(t, s, owner, "terminal-redaction-agent")
	seedAgentConfig(t, s, agent.ID, 1, map[string]any{"name": "Terminal Redaction"}, now)

	job, _, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{OwnerUserID: owner.ID, OrgID: owner.OrgID, EnrollmentID: enrollment.ID, JobType: "openclaw.configure_agent", SchemaVersion: 1, PayloadJSON: `{"agent_id":"` + agent.ID + `"}`}, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("enqueue fixture: %v", err)
	}
	lease, err := s.PollAndLeaseHelperJobForHelper(PollHelperJobInput{EnrollmentID: enrollment.ID, HelperCredential: credential, HelperDeviceID: *enrollment.HelperDeviceID, LeaseDuration: time.Minute}, now.Add(3*time.Minute))
	if err != nil || lease == nil {
		t.Fatalf("lease fixture=%+v err=%v", lease, err)
	}
	if _, err := s.AckHelperJobForHelper(AckHelperJobInput{EnrollmentID: enrollment.ID, JobID: job.ID, HelperCredential: credential, HelperDeviceID: *enrollment.HelperDeviceID, LeaseToken: lease.LeaseToken, AckStatus: "received"}, now.Add(3*time.Minute+time.Second)); err != nil {
		t.Fatalf("ack fixture: %v", err)
	}

	if _, err := s.CompleteHelperJobForHelper(CompleteHelperJobInput{EnrollmentID: enrollment.ID, JobID: job.ID, HelperCredential: credential, HelperDeviceID: *enrollment.HelperDeviceID, LeaseToken: lease.LeaseToken, Status: HelperJobStatusCancelled}, now.Add(3*time.Minute+2*time.Second)); !errors.Is(err, ErrHelperJobSchemaInvalid) {
		t.Fatalf("cancelled without reason error=%v, want ErrHelperJobSchemaInvalid", err)
	}

	completed, err := s.CompleteHelperJobForHelper(CompleteHelperJobInput{
		EnrollmentID:       enrollment.ID,
		JobID:              job.ID,
		HelperCredential:   credential,
		HelperDeviceID:     *enrollment.HelperDeviceID,
		LeaseToken:         lease.LeaseToken,
		Status:             HelperJobStatusFailed,
		FailureCode:        "execution_failed",
		FailureMessage:     "Authorization: Bearer secret-token credential=my-secret env=OPENAI_API_KEY=sk-test private message content /Users/alice/private.txt",
		ResultSummaryJSON:  `{"audit_refs":["audit-1"],"log_refs":["log-1"]}`,
		MaxFailureMessage:  512,
		MaxResultSummaries: 4,
	}, now.Add(3*time.Minute+3*time.Second))
	if err != nil {
		t.Fatalf("CompleteHelperJobForHelper redacted terminal: %v", err)
	}
	if completed.FailureMessage == nil {
		t.Fatalf("expected redacted failure message, got nil")
	}
	msg := *completed.FailureMessage
	for _, forbidden := range []string{"secret-token", "my-secret", "sk-test", "private message content", "/Users/alice/private.txt"} {
		if strings.Contains(msg, forbidden) {
			t.Fatalf("failure message leaked %q: %q", forbidden, msg)
		}
	}
	if !strings.Contains(msg, "[redacted]") {
		t.Fatalf("failure message should contain redaction marker, got %q", msg)
	}
}

func TestHelperJobHelperAuthorityAndExpiryFailures(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-job-authority-owner")
	now := time.UnixMilli(1778840000000)
	enrollment, credential := claimedFreshHelperEnrollmentWithCredential(t, s, owner, []string{"openclaw_config"}, now)
	agent := helperJobAgent(t, s, owner, "authority-openclaw-agent")
	seedAgentConfig(t, s, agent.ID, 1, map[string]any{"name": "Authority Agent"}, now)
	job, _, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{
		OwnerUserID:   owner.ID,
		OrgID:         owner.OrgID,
		EnrollmentID:  enrollment.ID,
		JobType:       "openclaw.configure_agent",
		SchemaVersion: 1,
		PayloadJSON:   `{"agent_id":"` + agent.ID + `"}`,
	}, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("EnqueueHelperJobForUser: %v", err)
	}

	if _, err := s.PollAndLeaseHelperJobForHelper(PollHelperJobInput{EnrollmentID: enrollment.ID, HelperCredential: "wrong", HelperDeviceID: *enrollment.HelperDeviceID}, now.Add(3*time.Minute)); !errors.Is(err, ErrHelperJobUnauthorized) {
		t.Fatalf("wrong credential poll error=%v, want ErrHelperJobUnauthorized", err)
	}
	if _, err := s.PollAndLeaseHelperJobForHelper(PollHelperJobInput{EnrollmentID: enrollment.ID, HelperCredential: credential, HelperDeviceID: "other-device"}, now.Add(3*time.Minute)); !errors.Is(err, ErrHelperJobDeviceMismatch) {
		t.Fatalf("wrong device poll error=%v, want ErrHelperJobDeviceMismatch", err)
	}

	lease, err := s.PollAndLeaseHelperJobForHelper(PollHelperJobInput{EnrollmentID: enrollment.ID, HelperCredential: credential, HelperDeviceID: *enrollment.HelperDeviceID, LeaseDuration: time.Second}, now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("lease for expiry case: %v", err)
	}
	if _, err := s.AckHelperJobForHelper(AckHelperJobInput{EnrollmentID: enrollment.ID, JobID: job.ID, HelperCredential: credential, HelperDeviceID: *enrollment.HelperDeviceID, LeaseToken: lease.LeaseToken, AckStatus: "received"}, now.Add(3*time.Minute+2*time.Second)); !errors.Is(err, ErrHelperJobLeaseLost) {
		t.Fatalf("late ack error=%v, want ErrHelperJobLeaseLost", err)
	}

	fresh, freshCredential := claimedFreshHelperEnrollmentWithCredential(t, s, owner, []string{"openclaw_config"}, now)
	freshJob, _, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{OwnerUserID: owner.ID, OrgID: owner.OrgID, EnrollmentID: fresh.ID, JobType: "openclaw.configure_agent", SchemaVersion: 1, PayloadJSON: `{"agent_id":"` + agent.ID + `"}`}, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("enqueue revoke fixture: %v", err)
	}
	if _, err := s.RevokeHelperEnrollmentForUser(fresh.ID, owner.ID, owner.OrgID, now.Add(3*time.Minute)); err != nil {
		t.Fatalf("revoke fixture: %v", err)
	}
	if _, err := s.PollAndLeaseHelperJobForHelper(PollHelperJobInput{EnrollmentID: fresh.ID, HelperCredential: freshCredential, HelperDeviceID: *fresh.HelperDeviceID}, now.Add(4*time.Minute)); !errors.Is(err, ErrHelperJobEnrollmentRevoked) {
		t.Fatalf("revoked poll error=%v, want ErrHelperJobEnrollmentRevoked", err)
	}
	var revokedJob HelperJob
	if err := s.DB().Where("id = ?", freshJob.ID).First(&revokedJob).Error; err != nil {
		t.Fatalf("load revoked job: %v", err)
	}
	if revokedJob.Status != HelperJobStatusCancelled || revokedJob.FailureCode == nil || *revokedJob.FailureCode != "revoked" || revokedJob.ActiveIdempotencyScope != nil {
		t.Fatalf("revoked poll should settle queued job, got %+v", revokedJob)
	}
}

func TestHelperJobStaleCredentialSettlesActiveJobsAndCurrentCredentialCanPoll(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		settle      func(t *testing.T, s *Store, enrollment *HelperEnrollment, oldCredential string, jobID, leaseToken string, now time.Time)
		activeState string
	}{
		{
			name:        "poll settles leased job after credential rotation",
			activeState: HelperJobStatusLeased,
			settle: func(t *testing.T, s *Store, enrollment *HelperEnrollment, oldCredential string, _ string, _ string, now time.Time) {
				t.Helper()
				if _, err := s.PollAndLeaseHelperJobForHelper(PollHelperJobInput{EnrollmentID: enrollment.ID, HelperCredential: oldCredential, HelperDeviceID: *enrollment.HelperDeviceID, MaxActiveLeases: 1}, now); !errors.Is(err, ErrHelperJobStaleCredential) {
					t.Fatalf("stale poll error=%v, want ErrHelperJobStaleCredential", err)
				}
			},
		},
		{
			name:        "ack settles leased job after credential rotation",
			activeState: HelperJobStatusLeased,
			settle: func(t *testing.T, s *Store, enrollment *HelperEnrollment, oldCredential string, jobID, leaseToken string, now time.Time) {
				t.Helper()
				if _, err := s.AckHelperJobForHelper(AckHelperJobInput{EnrollmentID: enrollment.ID, JobID: jobID, HelperCredential: oldCredential, HelperDeviceID: *enrollment.HelperDeviceID, LeaseToken: leaseToken, AckStatus: "received"}, now); !errors.Is(err, ErrHelperJobStaleCredential) {
					t.Fatalf("stale ack error=%v, want ErrHelperJobStaleCredential", err)
				}
			},
		},
		{
			name:        "result settles running job after credential rotation",
			activeState: HelperJobStatusRunning,
			settle: func(t *testing.T, s *Store, enrollment *HelperEnrollment, oldCredential string, jobID, leaseToken string, now time.Time) {
				t.Helper()
				if _, err := s.CompleteHelperJobForHelper(CompleteHelperJobInput{EnrollmentID: enrollment.ID, JobID: jobID, HelperCredential: oldCredential, HelperDeviceID: *enrollment.HelperDeviceID, LeaseToken: leaseToken, Status: HelperJobStatusSucceeded}, now); !errors.Is(err, ErrHelperJobStaleCredential) {
					t.Fatalf("stale result error=%v, want ErrHelperJobStaleCredential", err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := migratedStore(t)
			owner := helperOwner(t, s, "helper-job-stale-"+strings.ReplaceAll(tc.name, " ", "-"))
			now := time.UnixMilli(1778840000000)
			enrollment, oldCredential := claimedFreshHelperEnrollmentWithCredential(t, s, owner, []string{"openclaw_config"}, now)
			agent := helperJobAgent(t, s, owner, "stale-openclaw-agent-"+strings.ReplaceAll(tc.name, " ", "-"))
			seedAgentConfig(t, s, agent.ID, 1, map[string]any{"name": "Stale Agent"}, now)

			activeJob, _, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{OwnerUserID: owner.ID, OrgID: owner.OrgID, EnrollmentID: enrollment.ID, JobType: "openclaw.configure_agent", SchemaVersion: 1, PayloadJSON: `{"agent_id":"` + agent.ID + `"}`}, now.Add(2*time.Minute))
			if err != nil {
				t.Fatalf("enqueue active fixture: %v", err)
			}
			lease, err := s.PollAndLeaseHelperJobForHelper(PollHelperJobInput{EnrollmentID: enrollment.ID, HelperCredential: oldCredential, HelperDeviceID: *enrollment.HelperDeviceID, LeaseDuration: time.Minute, MaxActiveLeases: 1}, now.Add(3*time.Minute))
			if err != nil || lease == nil || lease.Job == nil {
				t.Fatalf("lease active fixture=%+v err=%v", lease, err)
			}
			if tc.activeState == HelperJobStatusRunning {
				if _, err := s.AckHelperJobForHelper(AckHelperJobInput{EnrollmentID: enrollment.ID, JobID: activeJob.ID, HelperCredential: oldCredential, HelperDeviceID: *enrollment.HelperDeviceID, LeaseToken: lease.LeaseToken, AckStatus: "received"}, now.Add(3*time.Minute+time.Second)); err != nil {
					t.Fatalf("ack running fixture: %v", err)
				}
			}
			nextJob, _, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{OwnerUserID: owner.ID, OrgID: owner.OrgID, EnrollmentID: enrollment.ID, JobType: "openclaw.configure_agent", SchemaVersion: 1, PayloadJSON: `{"agent_id":"` + agent.ID + `"}`, IdempotencyKey: "next-" + tc.activeState}, now.Add(3*time.Minute+2*time.Second))
			if err != nil {
				t.Fatalf("enqueue next fixture: %v", err)
			}

			_, currentCredential, err := s.RotateHelperEnrollmentCredential(enrollment.ID, oldCredential, *enrollment.HelperDeviceID, now.Add(3*time.Minute+3*time.Second))
			if err != nil {
				t.Fatalf("rotate fixture: %v", err)
			}
			tc.settle(t, s, enrollment, oldCredential, activeJob.ID, lease.LeaseToken, now.Add(3*time.Minute+4*time.Second))

			var settled HelperJob
			if err := s.DB().Where("id = ?", activeJob.ID).First(&settled).Error; err != nil {
				t.Fatalf("load settled job: %v", err)
			}
			if settled.Status != HelperJobStatusCancelled || settled.FailureCode == nil || *settled.FailureCode != "stale_credential" || settled.ActiveIdempotencyScope != nil {
				t.Fatalf("stale credential should settle active job, got %+v", settled)
			}

			nextLease, err := s.PollAndLeaseHelperJobForHelper(PollHelperJobInput{EnrollmentID: enrollment.ID, HelperCredential: currentCredential, HelperDeviceID: *enrollment.HelperDeviceID, LeaseDuration: time.Minute, MaxActiveLeases: 1}, now.Add(3*time.Minute+5*time.Second))
			if err != nil {
				t.Fatalf("current credential poll after stale settlement: %v", err)
			}
			if nextLease == nil || nextLease.Job == nil || nextLease.Job.ID != nextJob.ID {
				t.Fatalf("current credential should lease queued next job, got %+v want %s", nextLease, nextJob.ID)
			}
		})
	}
}

func TestHelperJobUninstallSettlementCoversRunningJob(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-job-uninstall-running")
	now := time.UnixMilli(1778840000000)
	enrollment, credential := claimedFreshHelperEnrollmentWithCredential(t, s, owner, []string{"openclaw_config"}, now)
	agent := helperJobAgent(t, s, owner, "uninstall-running-agent")
	seedAgentConfig(t, s, agent.ID, 1, map[string]any{"name": "Uninstall Agent"}, now)

	activeJob, _, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{OwnerUserID: owner.ID, OrgID: owner.OrgID, EnrollmentID: enrollment.ID, JobType: "openclaw.configure_agent", SchemaVersion: 1, PayloadJSON: `{"agent_id":"` + agent.ID + `"}`}, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("enqueue active fixture: %v", err)
	}
	lease, err := s.PollAndLeaseHelperJobForHelper(PollHelperJobInput{EnrollmentID: enrollment.ID, HelperCredential: credential, HelperDeviceID: *enrollment.HelperDeviceID, LeaseDuration: time.Minute}, now.Add(3*time.Minute))
	if err != nil || lease == nil {
		t.Fatalf("lease active fixture=%+v err=%v", lease, err)
	}
	if _, err := s.AckHelperJobForHelper(AckHelperJobInput{EnrollmentID: enrollment.ID, JobID: activeJob.ID, HelperCredential: credential, HelperDeviceID: *enrollment.HelperDeviceID, LeaseToken: lease.LeaseToken, AckStatus: "received"}, now.Add(3*time.Minute+time.Second)); err != nil {
		t.Fatalf("ack running fixture: %v", err)
	}

	if _, err := s.MarkHelperEnrollmentUninstalled(enrollment.ID, credential, *enrollment.HelperDeviceID, now.Add(3*time.Minute+2*time.Second)); err != nil {
		t.Fatalf("uninstall fixture: %v", err)
	}
	if _, err := s.CompleteHelperJobForHelper(CompleteHelperJobInput{EnrollmentID: enrollment.ID, JobID: activeJob.ID, HelperCredential: credential, HelperDeviceID: *enrollment.HelperDeviceID, LeaseToken: lease.LeaseToken, Status: HelperJobStatusSucceeded}, now.Add(3*time.Minute+3*time.Second)); !errors.Is(err, ErrHelperJobEnrollmentUninstalled) {
		t.Fatalf("uninstalled result error=%v, want ErrHelperJobEnrollmentUninstalled", err)
	}

	var settled HelperJob
	if err := s.DB().Where("id = ?", activeJob.ID).First(&settled).Error; err != nil {
		t.Fatalf("load settled job: %v", err)
	}
	if settled.Status != HelperJobStatusCancelled || settled.FailureCode == nil || *settled.FailureCode != "uninstalled" || settled.ActiveIdempotencyScope != nil {
		t.Fatalf("uninstall should settle running job, got %+v", settled)
	}
}

func TestHelperJobAckSettlesRevokedAndUninstalledJobs(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		settle   func(t *testing.T, s *Store, owner *User, enrollment *HelperEnrollment, credential string, now time.Time)
		wantErr  error
		wantCode string
	}{
		{
			name: "revoked",
			settle: func(t *testing.T, s *Store, owner *User, enrollment *HelperEnrollment, _ string, now time.Time) {
				t.Helper()
				if _, err := s.RevokeHelperEnrollmentForUser(enrollment.ID, owner.ID, owner.OrgID, now); err != nil {
					t.Fatalf("revoke fixture: %v", err)
				}
			},
			wantErr:  ErrHelperJobEnrollmentRevoked,
			wantCode: "revoked",
		},
		{
			name: "uninstalled",
			settle: func(t *testing.T, s *Store, _ *User, enrollment *HelperEnrollment, credential string, now time.Time) {
				t.Helper()
				if _, err := s.MarkHelperEnrollmentUninstalled(enrollment.ID, credential, *enrollment.HelperDeviceID, now); err != nil {
					t.Fatalf("uninstall fixture: %v", err)
				}
			},
			wantErr:  ErrHelperJobEnrollmentUninstalled,
			wantCode: "uninstalled",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := migratedStore(t)
			owner := helperOwner(t, s, "helper-job-ack-"+tc.name)
			now := time.UnixMilli(1778840000000)
			enrollment, credential := claimedFreshHelperEnrollmentWithCredential(t, s, owner, []string{"openclaw_config"}, now)
			agent := helperJobAgent(t, s, owner, "ack-settle-agent-"+tc.name)
			seedAgentConfig(t, s, agent.ID, 1, map[string]any{"name": "Ack Settle"}, now)

			job, _, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{OwnerUserID: owner.ID, OrgID: owner.OrgID, EnrollmentID: enrollment.ID, JobType: "openclaw.configure_agent", SchemaVersion: 1, PayloadJSON: `{"agent_id":"` + agent.ID + `"}`}, now.Add(2*time.Minute))
			if err != nil {
				t.Fatalf("enqueue fixture: %v", err)
			}
			lease, err := s.PollAndLeaseHelperJobForHelper(PollHelperJobInput{EnrollmentID: enrollment.ID, HelperCredential: credential, HelperDeviceID: *enrollment.HelperDeviceID, LeaseDuration: time.Minute}, now.Add(3*time.Minute))
			if err != nil || lease == nil {
				t.Fatalf("lease fixture=%+v err=%v", lease, err)
			}

			tc.settle(t, s, owner, enrollment, credential, now.Add(3*time.Minute+time.Second))
			if _, err := s.AckHelperJobForHelper(AckHelperJobInput{EnrollmentID: enrollment.ID, JobID: job.ID, HelperCredential: credential, HelperDeviceID: *enrollment.HelperDeviceID, LeaseToken: lease.LeaseToken, AckStatus: "received"}, now.Add(3*time.Minute+2*time.Second)); !errors.Is(err, tc.wantErr) {
				t.Fatalf("ack after %s error=%v, want %v", tc.name, err, tc.wantErr)
			}

			var settled HelperJob
			if err := s.DB().Where("id = ?", job.ID).First(&settled).Error; err != nil {
				t.Fatalf("load settled job: %v", err)
			}
			if settled.Status != HelperJobStatusCancelled || settled.FailureCode == nil || *settled.FailureCode != tc.wantCode || settled.ActiveIdempotencyScope != nil {
				t.Fatalf("ack after %s should settle job, got %+v", tc.name, settled)
			}
		})
	}
}

func TestSettleHelperJobWritesFailureMessage(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-job-settle-direct")
	now := time.UnixMilli(1778840000000)
	enrollment := claimedFreshHelperEnrollment(t, s, owner, []string{"openclaw_config"}, now)
	agent := helperJobAgent(t, s, owner, "settle-direct-agent")
	seedAgentConfig(t, s, agent.ID, 1, map[string]any{"name": "Settle Direct"}, now)
	job, _, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{OwnerUserID: owner.ID, OrgID: owner.OrgID, EnrollmentID: enrollment.ID, JobType: "openclaw.configure_agent", SchemaVersion: 1, PayloadJSON: `{"agent_id":"` + agent.ID + `"}`}, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("enqueue fixture: %v", err)
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		return settleHelperJob(tx, job.ID, now.Add(3*time.Minute), HelperJobStatusFailed, "execution_failed", "short failure")
	}); err != nil {
		t.Fatalf("settleHelperJob: %v", err)
	}
	var settled HelperJob
	if err := s.DB().Where("id = ?", job.ID).First(&settled).Error; err != nil {
		t.Fatalf("load settled job: %v", err)
	}
	if settled.Status != HelperJobStatusFailed || settled.FailureCode == nil || *settled.FailureCode != "execution_failed" || settled.FailureMessage == nil || *settled.FailureMessage != "short failure" || settled.ActiveIdempotencyScope != nil {
		t.Fatalf("settleHelperJob did not persist terminal metadata: %+v", settled)
	}
}

// TestHelperJobCompleteHelperUninstallSucceededFlipsEnrollment (#998) — when
// CompleteHelperJobForHelper records terminal `succeeded` for a
// `helper.uninstall` job, the same transaction flips the enrollment to
// `uninstalled`. Sibling check: terminal `failed` for the same job type
// leaves the enrollment alone so an operator can retry.
func TestHelperJobCompleteHelperUninstallSucceededFlipsEnrollment(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-uninstall-complete")
	now := time.UnixMilli(1778840000000)

	// Happy path: succeeded → enrollment uninstalled.
	enrollment, credential := claimedFreshHelperEnrollmentWithCredential(t, s, owner, []string{"helper_lifecycle"}, now)
	job, _, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{
		OwnerUserID:   owner.ID,
		OrgID:         owner.OrgID,
		EnrollmentID:  enrollment.ID,
		JobType:       HelperJobTypeHelperUninstall,
		SchemaVersion: 1,
		PayloadJSON:   `{"scope":"helper"}`,
	}, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("EnqueueHelperJobForUser uninstall: %v", err)
	}
	lease, err := s.PollAndLeaseHelperJobForHelper(PollHelperJobInput{
		EnrollmentID:      enrollment.ID,
		HelperCredential:  credential,
		HelperDeviceID:    *enrollment.HelperDeviceID,
		LeaseDuration:     time.Minute,
		RetryAfterNoWork:  5 * time.Second,
		MaxActiveLeases:   1,
		AllowedCategories: []string{"helper_lifecycle"},
	}, now.Add(2*time.Minute))
	if err != nil || lease == nil || lease.Job == nil {
		t.Fatalf("PollAndLeaseHelperJobForHelper: lease=%+v err=%v", lease, err)
	}
	if _, err := s.AckHelperJobForHelper(AckHelperJobInput{
		EnrollmentID:     enrollment.ID,
		JobID:            job.ID,
		HelperCredential: credential,
		HelperDeviceID:   *enrollment.HelperDeviceID,
		LeaseToken:       lease.LeaseToken,
		AckStatus:        "received",
	}, now.Add(2*time.Minute+time.Second)); err != nil {
		t.Fatalf("AckHelperJobForHelper: %v", err)
	}
	completed, err := s.CompleteHelperJobForHelper(CompleteHelperJobInput{
		EnrollmentID:     enrollment.ID,
		JobID:            job.ID,
		HelperCredential: credential,
		HelperDeviceID:   *enrollment.HelperDeviceID,
		LeaseToken:       lease.LeaseToken,
		Status:           HelperJobStatusSucceeded,
	}, now.Add(2*time.Minute+2*time.Second))
	if err != nil || completed == nil || completed.Status != HelperJobStatusSucceeded {
		t.Fatalf("CompleteHelperJobForHelper succeeded: completed=%+v err=%v", completed, err)
	}
	got, err := s.GetHelperEnrollment(enrollment.ID)
	if err != nil {
		t.Fatalf("GetHelperEnrollment post-success: %v", err)
	}
	if got.Status != "uninstalled" || got.UninstalledAt == nil {
		t.Fatalf("succeeded terminal must flip enrollment to uninstalled, got status=%s uninstalled_at=%v", got.Status, got.UninstalledAt)
	}

	// Failure path: failed terminal must leave enrollment untouched.
	enrollment2, credential2 := claimedFreshHelperEnrollmentWithCredential(t, s, owner, []string{"helper_lifecycle"}, now)
	job2, _, err := s.EnqueueHelperJobForUser(EnqueueHelperJobInput{
		OwnerUserID:   owner.ID,
		OrgID:         owner.OrgID,
		EnrollmentID:  enrollment2.ID,
		JobType:       HelperJobTypeHelperUninstall,
		SchemaVersion: 1,
		PayloadJSON:   `{"scope":"helper"}`,
	}, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("EnqueueHelperJobForUser uninstall failure fixture: %v", err)
	}
	lease2, err := s.PollAndLeaseHelperJobForHelper(PollHelperJobInput{
		EnrollmentID:      enrollment2.ID,
		HelperCredential:  credential2,
		HelperDeviceID:    *enrollment2.HelperDeviceID,
		LeaseDuration:     time.Minute,
		RetryAfterNoWork:  5 * time.Second,
		MaxActiveLeases:   1,
		AllowedCategories: []string{"helper_lifecycle"},
	}, now.Add(2*time.Minute))
	if err != nil || lease2 == nil || lease2.Job == nil {
		t.Fatalf("PollAndLeaseHelperJobForHelper failure fixture: %v", err)
	}
	if _, err := s.AckHelperJobForHelper(AckHelperJobInput{
		EnrollmentID:     enrollment2.ID,
		JobID:            job2.ID,
		HelperCredential: credential2,
		HelperDeviceID:   *enrollment2.HelperDeviceID,
		LeaseToken:       lease2.LeaseToken,
		AckStatus:        "received",
	}, now.Add(2*time.Minute+time.Second)); err != nil {
		t.Fatalf("Ack failure fixture: %v", err)
	}
	failed, err := s.CompleteHelperJobForHelper(CompleteHelperJobInput{
		EnrollmentID:     enrollment2.ID,
		JobID:            job2.ID,
		HelperCredential: credential2,
		HelperDeviceID:   *enrollment2.HelperDeviceID,
		LeaseToken:       lease2.LeaseToken,
		Status:           HelperJobStatusFailed,
		FailureCode:      "execution_failed",
		FailureMessage:   "simulated",
	}, now.Add(2*time.Minute+2*time.Second))
	if err != nil || failed == nil || failed.Status != HelperJobStatusFailed {
		t.Fatalf("CompleteHelperJobForHelper failed: completed=%+v err=%v", failed, err)
	}
	got2, err := s.GetHelperEnrollment(enrollment2.ID)
	if err != nil {
		t.Fatalf("GetHelperEnrollment post-failure: %v", err)
	}
	if got2.Status == "uninstalled" || got2.UninstalledAt != nil {
		t.Fatalf("failed terminal must NOT flip enrollment, got status=%s uninstalled_at=%v", got2.Status, got2.UninstalledAt)
	}
}

func claimedFreshHelperEnrollment(t *testing.T, s *Store, owner *User, categories []string, now time.Time) *HelperEnrollment {
	t.Helper()
	claimed, _ := claimedFreshHelperEnrollmentWithCredential(t, s, owner, categories, now)
	return claimed
}

func claimedFreshHelperEnrollmentWithCredential(t *testing.T, s *Store, owner *User, categories []string, now time.Time) (*HelperEnrollment, string) {
	t.Helper()
	enrollment, secret, err := s.CreateHelperEnrollment(owner.ID, "Mac Studio", categories, now)
	if err != nil {
		t.Fatalf("CreateHelperEnrollment: %v", err)
	}
	claimed, credential, err := s.ClaimHelperEnrollment(enrollment.ID, secret, "device-"+enrollment.ID, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ClaimHelperEnrollment: %v", err)
	}
	if _, err := s.UpdateHelperEnrollmentLastSeen(claimed.ID, *claimed.PersistentCredentialDigest, "device-"+enrollment.ID, now.Add(90*time.Second)); err == nil {
		t.Fatalf("test fixture accidentally authenticated with digest as credential")
	}
	return claimed, credential
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

func helperJobChannel(t *testing.T, s *Store, owner *User, name, visibility string) *Channel {
	t.Helper()
	ch := &Channel{Name: name, Visibility: visibility, CreatedBy: owner.ID, Type: "channel", OrgID: owner.OrgID}
	if err := s.CreateChannel(ch); err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	return ch
}

func legacyClaimedHelperEnrollmentForOwner(t *testing.T, s *Store, owner *User, categories []string, now time.Time) *HelperEnrollment {
	t.Helper()
	b, err := json.Marshal(categories)
	if err != nil {
		t.Fatalf("marshal categories: %v", err)
	}
	deviceID := "legacy-device-" + owner.ID
	digest := "sha256:legacy-digest"
	ts := now.UnixMilli()
	row := &HelperEnrollment{
		ID:                         "legacy-helper-enrollment-" + owner.ID,
		OwnerUserID:                owner.ID,
		OrgID:                      owner.OrgID,
		HostLabel:                  "Legacy Helper",
		HelperDeviceID:             &deviceID,
		AllowedCategories:          string(b),
		Status:                     "connected",
		LastSeenAt:                 &ts,
		CreatedAt:                  ts,
		UpdatedAt:                  ts,
		ClaimedAt:                  &ts,
		PersistentCredentialDigest: &digest,
		CredentialCreatedAt:        &ts,
		CredentialGeneration:       1,
	}
	if err := s.DB().Create(row).Error; err != nil {
		t.Fatalf("seed legacy helper enrollment: %v", err)
	}
	return row
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

func assertHelperJobInstallPayload(t *testing.T, payload string) {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if got["install_plan_id"] != "openclaw-plugin-v1" {
		t.Fatalf("install payload did not use server-owned install plan: %v", got)
	}
	for _, forbidden := range []string{"runtime", "manifest_id", "manifest_digest", "artifact_ids", "path_ids", "domain", "url", "command", "service_unit"} {
		if _, ok := got[forbidden]; ok {
			t.Fatalf("install payload leaked client authority field %q: %v", forbidden, got)
		}
	}
}

func assertHelperJobPluginConnectionPayload(t *testing.T, payload string, agentID string, channelID string) {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if got["agent_id"] != agentID {
		t.Fatalf("payload agent_id=%v, want %s in %v", got["agent_id"], agentID, got)
	}
	if got["channel_id"] != channelID {
		t.Fatalf("payload channel_id=%v, want %s in %v", got["channel_id"], channelID, got)
	}
	connectionID, _ := got["connection_id"].(string)
	if !strings.HasPrefix(connectionID, "borgee-plugin:") {
		t.Fatalf("payload missing server-owned connection_id: %v", got)
	}
	for _, key := range []string{"owner_user_id", "org_id", "credential", "credentials", "token", "api_key", "base_url", "shell", "argv", "script", "service_unit", "path", "domain", "url"} {
		if _, ok := got[key]; ok {
			t.Fatalf("plugin payload leaked forbidden key %q: %v", key, got)
		}
	}
}

func assertHelperJobManifestBinding(t *testing.T, job *HelperJob, wantPaths, wantArtifacts, wantDomains []string) {
	t.Helper()
	if job.ManifestBindingJSON == nil || strings.TrimSpace(*job.ManifestBindingJSON) == "" {
		t.Fatalf("job missing manifest binding: %+v", job)
	}
	var binding struct {
		ManifestDigest string   `json:"manifest_digest"`
		ArtifactIDs    []string `json:"artifact_ids"`
		PathIDs        []string `json:"path_ids"`
		Domains        []string `json:"domains"`
		ServiceIDs     []string `json:"service_ids"`
	}
	if err := json.Unmarshal([]byte(*job.ManifestBindingJSON), &binding); err != nil {
		t.Fatalf("manifest binding is not JSON: %v", err)
	}
	if binding.ManifestDigest == "" || binding.ManifestDigest != job.ManifestDigest {
		t.Fatalf("binding digest %q did not match job digest %q", binding.ManifestDigest, job.ManifestDigest)
	}
	assertStringSet(t, "path_ids", binding.PathIDs, wantPaths)
	assertStringSet(t, "artifact_ids", binding.ArtifactIDs, wantArtifacts)
	assertStringSet(t, "domains", binding.Domains, wantDomains)
	if len(binding.ServiceIDs) != 0 {
		t.Fatalf("Task9 must not grant service IDs, got %v", binding.ServiceIDs)
	}
}

func assertHelperJobServiceBinding(t *testing.T, job *HelperJob, wantServiceIDs []string) {
	t.Helper()
	if job.ManifestBindingJSON == nil || strings.TrimSpace(*job.ManifestBindingJSON) == "" {
		t.Fatalf("job missing manifest binding: %+v", job)
	}
	var binding struct {
		ManifestDigest string   `json:"manifest_digest"`
		ArtifactIDs    []string `json:"artifact_ids"`
		PathIDs        []string `json:"path_ids"`
		Domains        []string `json:"domains"`
		ServiceIDs     []string `json:"service_ids"`
	}
	if err := json.Unmarshal([]byte(*job.ManifestBindingJSON), &binding); err != nil {
		t.Fatalf("manifest binding is not JSON: %v", err)
	}
	if binding.ManifestDigest == "" || binding.ManifestDigest != job.ManifestDigest {
		t.Fatalf("binding digest %q did not match job digest %q", binding.ManifestDigest, job.ManifestDigest)
	}
	assertStringSet(t, "service_ids", binding.ServiceIDs, wantServiceIDs)
	if len(binding.ArtifactIDs) != 0 || len(binding.PathIDs) != 0 || len(binding.Domains) != 0 {
		t.Fatalf("service lifecycle binding should not grant artifacts/paths/domains: %+v", binding)
	}
}

func assertHelperJobServiceLifecyclePayload(t *testing.T, payload string) {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if got["operation"] != "restart" {
		t.Fatalf("payload operation=%v, want restart in %v", got["operation"], got)
	}
	for _, key := range []string{"target", "service_id", "service_ids", "service_unit", "command", "shell", "argv", "path", "domain", "credential"} {
		if _, ok := got[key]; ok {
			t.Fatalf("service lifecycle payload leaked forbidden key %q: %v", key, got)
		}
	}
}

func assertHelperJobStateWritePayload(t *testing.T, payload string, wantKey, wantSHA string) {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if got["state_key"] != wantKey {
		t.Fatalf("payload state_key=%v, want %s in %v", got["state_key"], wantKey, got)
	}
	if got["value_sha256"] != wantSHA {
		t.Fatalf("payload value_sha256=%v, want %s in %v", got["value_sha256"], wantSHA, got)
	}
	for _, key := range []string{"path", "path_id", "path_ids", "service_id", "credential", "shell", "argv", "command"} {
		if _, ok := got[key]; ok {
			t.Fatalf("state.write payload leaked forbidden key %q: %v", key, got)
		}
	}
}

func assertStringSet(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s got %v, want %v", label, got, want)
	}
	seen := map[string]bool{}
	for _, value := range got {
		seen[value] = true
	}
	for _, value := range want {
		if !seen[value] {
			t.Fatalf("%s got %v, missing %q", label, got, value)
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
