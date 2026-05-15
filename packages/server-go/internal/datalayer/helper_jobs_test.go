package datalayer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"borgee-server/internal/presence"
	"borgee-server/internal/store"
)

func TestHelperJobRepositoryEnqueueProjectionAndErrorMapping(t *testing.T) {
	t.Parallel()
	dl, s, owner := newHelperJobRepoFixture(t, "helper-job-dl")
	now := time.UnixMilli(1778840000000)
	enrollment, secret, err := s.CreateHelperEnrollment(owner.ID, "Mac Studio", []string{"openclaw_config"}, now)
	if err != nil {
		t.Fatalf("CreateHelperEnrollment: %v", err)
	}
	claimed, _, err := s.ClaimHelperEnrollment(enrollment.ID, secret, "device-1", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ClaimHelperEnrollment: %v", err)
	}
	agent := datalayerHelperJobAgent(t, s, owner, "dl-openclaw-agent")
	seedDatalayerAgentConfig(t, s, agent.ID, 2, map[string]any{"name": "DL Agent"}, now)

	job, created, err := dl.HelperJobRepo.EnqueueForUser(context.Background(), EnqueueHelperJobInput{
		OwnerUserID:    owner.ID,
		OrgID:          owner.OrgID,
		EnrollmentID:   claimed.ID,
		JobType:        "openclaw.configure_agent",
		SchemaVersion:  1,
		PayloadJSON:    `{"agent_id":"` + agent.ID + `"}`,
		IdempotencyKey: "dl-retry",
	}, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("EnqueueForUser: %v", err)
	}
	if !created || job.ID == "" || job.Status != "queued" || job.Category != "openclaw_config" {
		t.Fatalf("bad helper job projection created=%v job=%+v", created, job)
	}
	if job.OwnerUserID != "" || job.OrgID != "" || job.PayloadJSON != "" {
		t.Fatalf("datalayer projection leaked store-only owner/org/raw payload fields: %+v", job)
	}
	if !strings.HasPrefix(job.PayloadHash, "sha256:") || !strings.HasPrefix(job.ManifestDigest, "sha256:") {
		t.Fatalf("safe digest projection missing: %+v", job)
	}

	_, _, err = dl.HelperJobRepo.EnqueueForUser(context.Background(), EnqueueHelperJobInput{
		OwnerUserID:   owner.ID,
		OrgID:         owner.OrgID,
		EnrollmentID:  claimed.ID,
		JobType:       "command.run",
		SchemaVersion: 1,
		PayloadJSON:   `{}`,
	}, now.Add(2*time.Minute))
	if !errors.Is(err, ErrHelperJobUnknownType) {
		t.Fatalf("unknown type error=%v, want ErrHelperJobUnknownType", err)
	}
	if err := mapHelperJobErr(store.ErrHelperJobForbiddenField); !errors.Is(err, ErrHelperJobForbiddenField) {
		t.Fatalf("mapHelperJobErr forbidden field = %v", err)
	}
}

func TestHelperJobErrorMapping(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   error
		want error
	}{
		{"invalid input", store.ErrHelperJobInvalidInput, ErrHelperJobInvalidInput},
		{"unknown type", store.ErrHelperJobUnknownType, ErrHelperJobUnknownType},
		{"type not enabled", store.ErrHelperJobTypeNotEnabled, ErrHelperJobTypeNotEnabled},
		{"schema invalid", store.ErrHelperJobSchemaInvalid, ErrHelperJobSchemaInvalid},
		{"forbidden field", store.ErrHelperJobForbiddenField, ErrHelperJobForbiddenField},
		{"enrollment not found", store.ErrHelperJobEnrollmentNotFound, ErrHelperJobEnrollmentNotFound},
		{"wrong owner", store.ErrHelperJobWrongOwner, ErrHelperJobWrongOwner},
		{"wrong org", store.ErrHelperJobWrongOrg, ErrHelperJobWrongOrg},
		{"forbidden", store.ErrHelperJobForbidden, ErrHelperJobForbidden},
		{"enrollment unclaimed", store.ErrHelperJobEnrollmentUnclaimed, ErrHelperJobEnrollmentUnclaimed},
		{"enrollment revoked", store.ErrHelperJobEnrollmentRevoked, ErrHelperJobEnrollmentRevoked},
		{"enrollment uninstalled", store.ErrHelperJobEnrollmentUninstalled, ErrHelperJobEnrollmentUninstalled},
		{"stale enrollment", store.ErrHelperJobStaleEnrollment, ErrHelperJobStaleEnrollment},
		{"enrollment inactive", store.ErrHelperJobEnrollmentInactive, ErrHelperJobEnrollmentInactive},
		{"delegation denied", store.ErrHelperJobDelegationDenied, ErrHelperJobDelegationDenied},
		{"manifest required", store.ErrHelperJobManifestRequired, ErrHelperJobManifestRequired},
		{"idempotency conflict", store.ErrHelperJobIdempotencyConflict, ErrHelperJobIdempotencyConflict},
		{"expired", store.ErrHelperJobExpired, ErrHelperJobExpired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := mapHelperJobErr(tc.in); !errors.Is(err, tc.want) {
				t.Fatalf("mapHelperJobErr(%v)=%v, want %v", tc.in, err, tc.want)
			}
		})
	}
	if err := mapHelperJobErr(nil); err != nil {
		t.Fatalf("mapHelperJobErr(nil)=%v", err)
	}
	unknown := errors.New("other")
	if err := mapHelperJobErr(unknown); !errors.Is(err, unknown) {
		t.Fatalf("mapHelperJobErr(other)=%v, want original", err)
	}
	if helperJobFromStore(nil) != nil {
		t.Fatal("helperJobFromStore(nil) should return nil")
	}
}

func newHelperJobRepoFixture(t *testing.T, name string) (*DataLayer, *store.Store, *store.User) {
	t.Helper()
	s := store.MigratedStoreFromTemplate(t)
	t.Cleanup(func() { _ = s.Close() })
	pt, err := presence.NewSessionsTracker(s.DB())
	if err != nil {
		t.Fatalf("presence.NewSessionsTracker: %v", err)
	}
	dl := NewDataLayer(s, pt, slog.New(slog.NewTextHandler(io.Discard, nil)))
	email := name + "@example.com"
	owner := &store.User{DisplayName: name, Role: "member", Email: &email, PasswordHash: "hash"}
	if err := dl.UserRepo.Create(context.Background(), owner); err != nil {
		t.Fatalf("UserRepo.Create: %v", err)
	}
	if _, err := s.CreateOrgForUser(owner, name+" Org"); err != nil {
		t.Fatalf("CreateOrgForUser: %v", err)
	}
	reloaded, err := s.GetUserByID(owner.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	return dl, s, reloaded
}

func datalayerHelperJobAgent(t *testing.T, s *store.Store, owner *store.User, name string) *store.User {
	t.Helper()
	apiKey := name + "-key"
	agent := &store.User{DisplayName: name, Role: "agent", OwnerID: &owner.ID, APIKey: &apiKey, OrgID: owner.OrgID, PasswordHash: "hash"}
	if err := s.CreateUser(agent); err != nil {
		t.Fatalf("CreateUser agent: %v", err)
	}
	return agent
}

func seedDatalayerAgentConfig(t *testing.T, s *store.Store, agentID string, version int64, blob map[string]any, now time.Time) {
	t.Helper()
	b, err := json.Marshal(blob)
	if err != nil {
		t.Fatalf("marshal config blob: %v", err)
	}
	if err := s.DB().Exec(`INSERT INTO agent_configs (agent_id, schema_version, blob, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, agentID, version, string(b), now.UnixMilli(), now.UnixMilli()).Error; err != nil {
		t.Fatalf("seed agent config: %v", err)
	}
}
