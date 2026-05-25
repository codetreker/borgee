//go:build linux || darwin

package delegationrevoke

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"borgee/internal/dispatch"
	"borgee/internal/outbound"
	"borgee/internal/rootdclient"
)

type fakeRootd struct {
	gotReq rootdclient.DelegationRevokeRequest
	resp   *rootdclient.DelegationRevokeResponse
	err    error
}

func (f *fakeRootd) DelegationRevoke(_ context.Context, opts rootdclient.DelegationRevokeRequest) (*rootdclient.DelegationRevokeResponse, error) {
	f.gotReq = opts
	return f.resp, f.err
}

type fakeDispatcher struct {
	called  bool
	timeout time.Duration
	err     error
}

func (f *fakeDispatcher) Drain(_ context.Context, t time.Duration) error {
	f.called = true
	f.timeout = t
	return f.err
}

func TestExecuteHappyPathDrainsAndCallsRootd(t *testing.T) {
	t.Parallel()
	fake := &fakeRootd{resp: &rootdclient.DelegationRevokeResponse{Disabled: true, CredentialWiped: true, WipedPaths: []string{"/var/lib/borgee/credential/credential"}}}
	dis := &fakeDispatcher{}
	exec := &Executor{Rootd: fake, Dispatcher: dis, GOOS: "linux"}
	term, err := exec.Execute(context.Background(), &outbound.LeasedJob{
		EnrollmentID: "enroll-1",
		Payload:      []byte(`{"target_category":"openclaw_config"}`),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if term.Status != dispatch.StatusSucceeded {
		t.Fatalf("status=%q, want succeeded; %+v", term.Status, term)
	}
	if !dis.called {
		t.Fatalf("dispatcher Drain was not invoked")
	}
	if fake.gotReq.EnrollmentID != "enroll-1" || fake.gotReq.ServiceName != "borgee.service" {
		t.Fatalf("rootd req wrong: %+v", fake.gotReq)
	}
	if len(fake.gotReq.CredentialPaths) != 3 {
		t.Fatalf("expected 3 credential paths in Linux layout, got %v", fake.gotReq.CredentialPaths)
	}
}

func TestExecuteRejectsUnknownCategory(t *testing.T) {
	t.Parallel()
	exec := &Executor{Rootd: &fakeRootd{}, GOOS: "linux"}
	for _, cat := range []string{"", "evil_category", "unknown"} {
		payload, _ := json.Marshal(map[string]string{"target_category": cat})
		term, err := exec.Execute(context.Background(), &outbound.LeasedJob{
			EnrollmentID: "enroll-x", Payload: payload,
		})
		if err == nil || term.FailureCode != "schema_invalid" {
			t.Fatalf("category %q not rejected: %+v err=%v", cat, term, err)
		}
	}
}

func TestExecuteRejectsNilRootd(t *testing.T) {
	t.Parallel()
	exec := &Executor{GOOS: "linux"}
	term, err := exec.Execute(context.Background(), &outbound.LeasedJob{
		EnrollmentID: "enroll-x",
		Payload:      []byte(`{"target_category":"helper_lifecycle"}`),
	})
	if err == nil || term.FailureCode != "executor_error" {
		t.Fatalf("nil rootd not rejected: %+v err=%v", term, err)
	}
}

func TestExecutePropagatesRootdError(t *testing.T) {
	t.Parallel()
	fake := &fakeRootd{err: errors.New("credential_wipe_failed: /var/lib/borgee/credential/credential: permission denied")}
	exec := &Executor{Rootd: fake, GOOS: "linux"}
	term, err := exec.Execute(context.Background(), &outbound.LeasedJob{
		EnrollmentID: "enroll-x",
		Payload:      []byte(`{"target_category":"helper_lifecycle"}`),
	})
	if err == nil || term.FailureCode != "execution_failed" {
		t.Fatalf("rootd err not propagated: %+v err=%v", term, err)
	}
	if !strings.Contains(term.FailureMessage, "credential_wipe_failed") {
		t.Fatalf("failure_message lost detail: %q", term.FailureMessage)
	}
}

func TestExecuteRejectsRootdCredentialNotWiped(t *testing.T) {
	t.Parallel()
	fake := &fakeRootd{resp: &rootdclient.DelegationRevokeResponse{Disabled: false, CredentialWiped: false}}
	exec := &Executor{Rootd: fake, GOOS: "linux"}
	term, err := exec.Execute(context.Background(), &outbound.LeasedJob{
		EnrollmentID: "enroll-x",
		Payload:      []byte(`{"target_category":"helper_lifecycle"}`),
	})
	if err == nil || term.FailureCode != "execution_failed" {
		t.Fatalf("credential-not-wiped not rejected: %+v err=%v", term, err)
	}
}

func TestExecuteDrainTimeoutDoesNotBlockRevoke(t *testing.T) {
	t.Parallel()
	fake := &fakeRootd{resp: &rootdclient.DelegationRevokeResponse{Disabled: true, CredentialWiped: true}}
	dis := &fakeDispatcher{err: errors.New("drain timeout")}
	exec := &Executor{Rootd: fake, Dispatcher: dis, GOOS: "linux"}
	term, err := exec.Execute(context.Background(), &outbound.LeasedJob{
		EnrollmentID: "enroll-x",
		Payload:      []byte(`{"target_category":"helper_lifecycle"}`),
	})
	if err != nil {
		t.Fatalf("drain timeout should not fail the revoke: %v", err)
	}
	if term.Status != dispatch.StatusSucceeded {
		t.Fatalf("status=%q want succeeded; %+v", term.Status, term)
	}
}

func TestExecuteRejectsNilJob(t *testing.T) {
	t.Parallel()
	exec := &Executor{Rootd: &fakeRootd{}, GOOS: "linux"}
	term, err := exec.Execute(context.Background(), nil)
	if err == nil || term.FailureCode != "schema_invalid" {
		t.Fatalf("nil job not rejected: %+v err=%v", term, err)
	}
}
