//go:build linux || darwin

package pluginremove

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"borgee/internal/dispatch"
	"borgee/internal/executors/testfixture"
	"borgee/internal/outbound"
)

func newJob(t *testing.T, payload map[string]any, boundIDs []string, manifestPaths []testfixture.PathSpec) *outbound.LeasedJob {
	t.Helper()
	manifestJSON, bindingJSON := testfixture.Build(t, manifestPaths, boundIDs)
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return &outbound.LeasedJob{
		JobID:               "job-1",
		EnrollmentID:        "enroll-1",
		JobType:             "borgee_plugin.remove_connection",
		SchemaVersion:       1,
		Payload:             raw,
		ManifestDigest:      "sha256:test",
		ManifestJSON:        manifestJSON,
		ManifestBindingJSON: bindingJSON,
		LeaseToken:          "lease-1",
	}
}

func goodPayload() map[string]any {
	return map[string]any{
		"connection_id": "borgee-plugin:abc123",
		"agent_id":      "agent-1",
	}
}

func TestExecute_HappyPath_DeletesExistingFile(t *testing.T) {
	root := t.TempDir()
	// Seed file that should be removed.
	dest := filepath.Join(root, "abc123.json")
	if err := os.WriteFile(dest, []byte(`{"x":1}`), 0o640); err != nil {
		t.Fatalf("seed: %v", err)
	}
	exec := &Executor{}
	job := newJob(t, goodPayload(),
		[]string{PathID},
		[]testfixture.PathSpec{{ID: PathID, Root: root}})
	ts, err := exec.Execute(context.Background(), job)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if ts.Status != dispatch.StatusSucceeded {
		t.Fatalf("status=%s code=%s msg=%s", ts.Status, ts.FailureCode, ts.FailureMessage)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("expected file removed, stat err=%v", err)
	}
}

func TestExecute_Idempotent_MissingFile(t *testing.T) {
	root := t.TempDir()
	exec := &Executor{}
	job := newJob(t, goodPayload(),
		[]string{PathID},
		[]testfixture.PathSpec{{ID: PathID, Root: root}})
	ts, err := exec.Execute(context.Background(), job)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if ts.Status != dispatch.StatusSucceeded {
		t.Fatalf("expected success on missing file, got status=%s code=%s msg=%s", ts.Status, ts.FailureCode, ts.FailureMessage)
	}
}

func TestExecute_RejectsBadConnectionIDPrefix(t *testing.T) {
	root := t.TempDir()
	exec := &Executor{}
	p := goodPayload()
	p["connection_id"] = "wrong-prefix:abc"
	job := newJob(t, p,
		[]string{PathID},
		[]testfixture.PathSpec{{ID: PathID, Root: root}})
	ts, _ := exec.Execute(context.Background(), job)
	if ts.FailureCode != "schema_invalid" {
		t.Fatalf("code=%s", ts.FailureCode)
	}
}

func TestExecute_RejectsConnectionIDPathEscape(t *testing.T) {
	root := t.TempDir()
	exec := &Executor{}
	p := goodPayload()
	p["connection_id"] = "borgee-plugin:../etc/passwd"
	job := newJob(t, p,
		[]string{PathID},
		[]testfixture.PathSpec{{ID: PathID, Root: root}})
	ts, _ := exec.Execute(context.Background(), job)
	if ts.FailureCode != "schema_invalid" && ts.FailureCode != "path_escape" {
		t.Fatalf("code=%s", ts.FailureCode)
	}
}

func TestExecute_RejectsEmptyAgentID(t *testing.T) {
	root := t.TempDir()
	exec := &Executor{}
	p := goodPayload()
	p["agent_id"] = ""
	job := newJob(t, p,
		[]string{PathID},
		[]testfixture.PathSpec{{ID: PathID, Root: root}})
	ts, _ := exec.Execute(context.Background(), job)
	if ts.FailureCode != "schema_invalid" {
		t.Fatalf("code=%s", ts.FailureCode)
	}
}

func TestExecute_NilJob(t *testing.T) {
	exec := &Executor{}
	ts, err := exec.Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("expected err")
	}
	if ts.FailureCode != "schema_invalid" {
		t.Fatalf("code=%s", ts.FailureCode)
	}
}

func TestExecute_FailsWhenManifestMissingPathID(t *testing.T) {
	root := t.TempDir()
	exec := &Executor{}
	job := newJob(t, goodPayload(),
		[]string{PathID},
		[]testfixture.PathSpec{{ID: "other", Root: root}})
	ts, _ := exec.Execute(context.Background(), job)
	if ts.FailureCode != "manifest_missing_path_id" {
		t.Fatalf("code=%s msg=%s", ts.FailureCode, ts.FailureMessage)
	}
}

func TestExecute_HappyPath_DoesNotTouchSiblings(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "abc123.json")
	sibling := filepath.Join(root, "other.json")
	if err := os.WriteFile(target, []byte(`{"x":1}`), 0o640); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	if err := os.WriteFile(sibling, []byte(`{"y":2}`), 0o640); err != nil {
		t.Fatalf("seed sibling: %v", err)
	}
	exec := &Executor{}
	job := newJob(t, goodPayload(),
		[]string{PathID},
		[]testfixture.PathSpec{{ID: PathID, Root: root}})
	if _, err := exec.Execute(context.Background(), job); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target should be gone: %v", err)
	}
	if _, err := os.Stat(sibling); err != nil {
		t.Fatalf("sibling should remain: %v", err)
	}
}

func TestExecute_RejectsMalformedPayload(t *testing.T) {
	root := t.TempDir()
	exec := &Executor{}
	job := newJob(t, goodPayload(),
		[]string{PathID},
		[]testfixture.PathSpec{{ID: PathID, Root: root}})
	job.Payload = []byte("{not json")
	ts, err := exec.Execute(context.Background(), job)
	if err == nil {
		t.Fatal("expected err")
	}
	if ts.FailureCode != "schema_invalid" {
		t.Fatalf("code=%s", ts.FailureCode)
	}
}

func TestExecute_RejectsMissingPrefix(t *testing.T) {
	root := t.TempDir()
	exec := &Executor{}
	p := goodPayload()
	p["connection_id"] = ""
	job := newJob(t, p,
		[]string{PathID},
		[]testfixture.PathSpec{{ID: PathID, Root: root}})
	ts, _ := exec.Execute(context.Background(), job)
	if ts.FailureCode != "schema_invalid" {
		t.Fatalf("code=%s", ts.FailureCode)
	}
}

func TestExecute_LoggerCallable(t *testing.T) {
	root := t.TempDir()
	// Pre-create a file as a DIRECTORY at the target path so os.Remove
	// fails non-idempotently and exercises the remove_failed branch +
	// logger code path.
	if err := os.MkdirAll(filepath.Join(root, "abc123.json", "nested"), 0o750); err != nil {
		t.Fatalf("seed dir: %v", err)
	}
	logged := false
	exec := &Executor{
		Now:    func() time.Time { return time.Unix(0, 0) },
		Logger: func(format string, v ...any) { logged = true },
	}
	job := newJob(t, goodPayload(),
		[]string{PathID},
		[]testfixture.PathSpec{{ID: PathID, Root: root}})
	ts, err := exec.Execute(context.Background(), job)
	if err == nil {
		t.Fatal("expected err removing non-empty dir")
	}
	if ts.FailureCode != "remove_failed" {
		t.Fatalf("code=%s", ts.FailureCode)
	}
	if !logged {
		t.Fatal("logger not called")
	}
	// exercise the now() override path
	if !exec.now().Equal(time.Unix(0, 0)) {
		t.Fatal("now override not used")
	}
}
