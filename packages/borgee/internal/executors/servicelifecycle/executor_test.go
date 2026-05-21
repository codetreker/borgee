//go:build linux || darwin

package servicelifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"borgee/internal/dispatch"
	"borgee/internal/jobpolicy"
	"borgee/internal/outbound"
	"borgee/internal/rootdclient"
)

type fakeRootd struct {
	gotReq rootdclient.ServiceLifecycleRequest
	resp   *rootdclient.ServiceLifecycleResponse
	err    error
}

func (f *fakeRootd) ServiceLifecycle(_ context.Context, opts rootdclient.ServiceLifecycleRequest) (*rootdclient.ServiceLifecycleResponse, error) {
	f.gotReq = opts
	return f.resp, f.err
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func fixtureManifestLinux(t *testing.T) (json.RawMessage, json.RawMessage) {
	t.Helper()
	manifest := jobpolicy.PolicyManifest{
		ManifestVersion: 1,
		Services: []jobpolicy.ServiceDeclaration{
			{ID: "openclaw-user", Platform: "linux", Manager: "systemd", Unit: "openclaw.service"},
		},
	}
	binding := jobpolicy.ManifestBinding{
		ManifestDigest: "sha256:test",
		ServiceIDs:     []string{"openclaw-user"},
	}
	return mustJSON(t, manifest), mustJSON(t, binding)
}

func TestExecuteHappyPathInvokesRootdWithResolvedUnit(t *testing.T) {
	t.Parallel()
	manifest, binding := fixtureManifestLinux(t)
	fake := &fakeRootd{resp: &rootdclient.ServiceLifecycleResponse{ExitCode: 0}}
	exec := &Executor{Rootd: fake}
	term, err := exec.Execute(context.Background(), &outbound.LeasedJob{
		JobType:             jobpolicy.JobTypeServiceLifecycle,
		Payload:             []byte(`{"operation":"restart"}`),
		ManifestJSON:        manifest,
		ManifestBindingJSON: binding,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if term.Status != dispatch.StatusSucceeded {
		t.Fatalf("status=%q, want succeeded; %+v", term.Status, term)
	}
	if fake.gotReq.Manager != "systemd" || fake.gotReq.Unit != "openclaw.service" || fake.gotReq.Operation != "restart" {
		t.Fatalf("rootd req wrong: %+v", fake.gotReq)
	}
}

func TestExecuteRejectsUnknownOperation(t *testing.T) {
	t.Parallel()
	manifest, binding := fixtureManifestLinux(t)
	exec := &Executor{Rootd: &fakeRootd{}}
	for _, op := range []string{"", "exec", "kill", "mask"} {
		term, err := exec.Execute(context.Background(), &outbound.LeasedJob{
			Payload:             []byte(`{"operation":"` + op + `"}`),
			ManifestJSON:        manifest,
			ManifestBindingJSON: binding,
		})
		if err == nil || term.FailureCode != "schema_invalid" {
			t.Fatalf("op %q not rejected: %+v err=%v", op, term, err)
		}
	}
}

func TestExecuteFailsLoudOnEmptyManifest(t *testing.T) {
	t.Parallel()
	exec := &Executor{Rootd: &fakeRootd{}}
	term, err := exec.Execute(context.Background(), &outbound.LeasedJob{
		Payload:             []byte(`{"operation":"restart"}`),
		ManifestJSON:        nil,
		ManifestBindingJSON: nil,
	})
	if err == nil || term.FailureCode != "manifest_invalid" {
		t.Fatalf("empty manifest not rejected: %+v err=%v", term, err)
	}
}

func TestExecuteMapsNonZeroExitToServiceDenied(t *testing.T) {
	t.Parallel()
	manifest, binding := fixtureManifestLinux(t)
	fake := &fakeRootd{resp: &rootdclient.ServiceLifecycleResponse{ExitCode: 5, Stderr: "unit not found"}}
	exec := &Executor{Rootd: fake}
	term, err := exec.Execute(context.Background(), &outbound.LeasedJob{
		Payload:             []byte(`{"operation":"restart"}`),
		ManifestJSON:        manifest,
		ManifestBindingJSON: binding,
	})
	if err == nil || term.FailureCode != "service_denied" {
		t.Fatalf("non-zero exit not mapped: %+v err=%v", term, err)
	}
}

func TestExecutePropagatesRootdError(t *testing.T) {
	t.Parallel()
	manifest, binding := fixtureManifestLinux(t)
	fake := &fakeRootd{err: errors.New("manager_invalid: 'sysvinit'")}
	exec := &Executor{Rootd: fake}
	term, _ := exec.Execute(context.Background(), &outbound.LeasedJob{
		Payload:             []byte(`{"operation":"restart"}`),
		ManifestJSON:        manifest,
		ManifestBindingJSON: binding,
	})
	if term.FailureCode != "service_denied" {
		t.Fatalf("rootd err not propagated: %+v", term)
	}
}

func TestExecuteRejectsManifestMissingServiceID(t *testing.T) {
	t.Parallel()
	manifest, _ := fixtureManifestLinux(t)
	// Binding without ServiceIDs.
	binding := mustJSON(t, jobpolicy.ManifestBinding{ManifestDigest: "sha256:test"})
	exec := &Executor{Rootd: &fakeRootd{}}
	term, err := exec.Execute(context.Background(), &outbound.LeasedJob{
		Payload:             []byte(`{"operation":"restart"}`),
		ManifestJSON:        manifest,
		ManifestBindingJSON: binding,
	})
	if err == nil || term.FailureCode != "manifest_missing_service_id" {
		t.Fatalf("missing service id not rejected: %+v err=%v", term, err)
	}
}
