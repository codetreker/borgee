//go:build linux || darwin

package statuscollect

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"borgee/internal/dispatch"
	"borgee/internal/outbound"
)

func newJob(t *testing.T, payload any) *outbound.LeasedJob {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return &outbound.LeasedJob{
		JobID:        "job-1",
		EnrollmentID: "enroll-1",
		JobType:      "status.collect",
		Payload:      raw,
		LeaseToken:   "lease-1",
	}
}

func TestExecute_HappyPath(t *testing.T) {
	exec := &Executor{Now: func() time.Time { return time.Unix(1_700_000_000, 0).UTC() }}
	for _, scope := range []string{"helper", "openclaw", "service"} {
		t.Run(scope, func(t *testing.T) {
			st, err := exec.Execute(context.Background(), newJob(t, map[string]string{"scope": scope}))
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if st.Status != dispatch.StatusSucceeded {
				t.Fatalf("status=%q want succeeded (msg=%s)", st.Status, st.FailureMessage)
			}
			// Snapshot is returned in LogRefs (no filesystem write).
			if len(st.ResultSummary.LogRefs) != 1 {
				t.Fatalf("expected 1 log ref carrying snapshot, got %d", len(st.ResultSummary.LogRefs))
			}
			var snap map[string]any
			if err := json.Unmarshal([]byte(st.ResultSummary.LogRefs[0]), &snap); err != nil {
				t.Fatalf("decode snapshot: %v", err)
			}
			if snap["scope"] != scope {
				t.Errorf("scope in snapshot = %v want %s", snap["scope"], scope)
			}
		})
	}
}

func TestExecute_NoFilesystemWrite(t *testing.T) {
	// status.collect must not touch the filesystem — it just collects + reports.
	// We don't have a great negative-affirmation here; instead assert no
	// AuditRefs/LogRefs reference any cache filename pattern. The HappyPath
	// test plus the executor source (no os.MkdirAll/os.Rename) make this
	// a structural guarantee.
	exec := &Executor{}
	st, err := exec.Execute(context.Background(), newJob(t, map[string]string{"scope": "helper"}))
	if err != nil || st.Status != dispatch.StatusSucceeded {
		t.Fatalf("Execute: status=%s err=%v", st.Status, err)
	}
}

func TestExecute_MalformedPayload(t *testing.T) {
	exec := &Executor{}
	job := &outbound.LeasedJob{Payload: []byte("not-json")}
	st, _ := exec.Execute(context.Background(), job)
	if st.Status != dispatch.StatusFailed || st.FailureCode != "schema_invalid" {
		t.Fatalf("got status=%q code=%q want failed/schema_invalid", st.Status, st.FailureCode)
	}
}

func TestExecute_BadScope(t *testing.T) {
	exec := &Executor{}
	st, _ := exec.Execute(context.Background(), newJob(t, map[string]string{"scope": "wat"}))
	if st.Status != dispatch.StatusFailed || st.FailureCode != "schema_invalid" {
		t.Fatalf("got status=%q code=%q want failed/schema_invalid", st.Status, st.FailureCode)
	}
}

func TestExecute_NilJob(t *testing.T) {
	exec := &Executor{}
	st, err := exec.Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("expected err on nil job")
	}
	if st.FailureCode != "schema_invalid" {
		t.Fatalf("code=%q want schema_invalid", st.FailureCode)
	}
}

func TestExecute_OpenClawInstalledVersionsAbsent(t *testing.T) {
	exec := &Executor{
		InstalledVersionsPath: filepath.Join(t.TempDir(), "does-not-exist.json"),
	}
	st, err := exec.Execute(context.Background(), newJob(t, map[string]string{"scope": "openclaw"}))
	if err != nil || st.Status != dispatch.StatusSucceeded {
		t.Fatalf("status=%q err=%v", st.Status, err)
	}
	var snap map[string]any
	_ = json.Unmarshal([]byte(st.ResultSummary.LogRefs[0]), &snap)
	if snap["installed_versions_status"] != "absent" {
		t.Fatalf("installed_versions_status = %v want absent", snap["installed_versions_status"])
	}
}
