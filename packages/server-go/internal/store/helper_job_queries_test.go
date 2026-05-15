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
		{"recognized install type", func(in EnqueueHelperJobInput) EnqueueHelperJobInput {
			in.JobType = "openclaw.install_from_manifest"
			return in
		}, ErrHelperJobManifestRequired},
		{"recognized plugin connection type", func(in EnqueueHelperJobInput) EnqueueHelperJobInput {
			in.JobType = "borgee_plugin.configure_connection"
			return in
		}, ErrHelperJobTypeNotEnabled},
		{"recognized service lifecycle type", func(in EnqueueHelperJobInput) EnqueueHelperJobInput { in.JobType = "service.lifecycle"; return in }, ErrHelperJobTypeNotEnabled},
		{"recognized state write type", func(in EnqueueHelperJobInput) EnqueueHelperJobInput { in.JobType = "state.write"; return in }, ErrHelperJobTypeNotEnabled},
		{"recognized status collect type", func(in EnqueueHelperJobInput) EnqueueHelperJobInput { in.JobType = "status.collect"; return in }, ErrHelperJobTypeNotEnabled},
		{"recognized delegation revoke type", func(in EnqueueHelperJobInput) EnqueueHelperJobInput { in.JobType = "delegation.revoke"; return in }, ErrHelperJobTypeNotEnabled},
		{"recognized helper uninstall type", func(in EnqueueHelperJobInput) EnqueueHelperJobInput { in.JobType = "helper.uninstall"; return in }, ErrHelperJobTypeNotEnabled},
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
	if lease.Job.PayloadJSON == "" || lease.Job.OwnerUserID != "" || lease.Job.OrgID != "" {
		t.Fatalf("lease projection should include payload but not owner/org internals: %+v", lease.Job)
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

func countHelperJobs(t *testing.T, s *Store) int64 {
	t.Helper()
	var count int64
	if err := s.DB().Table("helper_jobs").Count(&count).Error; err != nil {
		t.Fatalf("count helper_jobs: %v", err)
	}
	return count
}
