//go:build linux || darwin

package statewrite

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
		JobType:             "state.write",
		SchemaVersion:       1,
		Payload:             raw,
		ManifestDigest:      "sha256:test",
		ManifestJSON:        manifestJSON,
		ManifestBindingJSON: bindingJSON,
		LeaseToken:          "lease-1",
	}
}

func TestExecute_HappyPath(t *testing.T) {
	root := t.TempDir()
	exec := &Executor{Now: func() time.Time { return time.Unix(1_780_000_000, 0).UTC() }}
	job := newJob(t,
		map[string]any{"state_key": "alpha/beta", "value_sha256": "sha256:abcd"},
		[]string{PathID},
		[]testfixture.PathSpec{{ID: PathID, Root: root}})

	ts, err := exec.Execute(context.Background(), job)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if ts.Status != dispatch.StatusSucceeded {
		t.Fatalf("status=%s code=%s msg=%s", ts.Status, ts.FailureCode, ts.FailureMessage)
	}
	dest := filepath.Join(root, "alpha", "beta.json")
	raw, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["state_key"] != "alpha/beta" || got["value_sha256"] != "sha256:abcd" {
		t.Fatalf("unexpected content: %v", got)
	}
}

func TestExecute_FailsWhenManifestMissingPathID(t *testing.T) {
	root := t.TempDir()
	exec := &Executor{}
	job := newJob(t,
		map[string]any{"state_key": "k"},
		[]string{PathID},
		[]testfixture.PathSpec{{ID: "other_id", Root: root}})
	ts, _ := exec.Execute(context.Background(), job)
	if ts.Status != dispatch.StatusFailed || ts.FailureCode != "manifest_missing_path_id" {
		t.Fatalf("status=%s code=%s msg=%s", ts.Status, ts.FailureCode, ts.FailureMessage)
	}
}

func TestExecute_FailsWhenBindingMissingPathID(t *testing.T) {
	root := t.TempDir()
	exec := &Executor{}
	job := newJob(t,
		map[string]any{"state_key": "k"},
		[]string{"other_id"},
		[]testfixture.PathSpec{{ID: PathID, Root: root}})
	ts, _ := exec.Execute(context.Background(), job)
	if ts.Status != dispatch.StatusFailed || ts.FailureCode != "manifest_missing_path_id" {
		t.Fatalf("status=%s code=%s msg=%s", ts.Status, ts.FailureCode, ts.FailureMessage)
	}
}

func TestExecute_RejectsPayloadPathEscape(t *testing.T) {
	root := t.TempDir()
	exec := &Executor{}
	job := newJob(t,
		map[string]any{"state_key": "../../etc/passwd"},
		[]string{PathID},
		[]testfixture.PathSpec{{ID: PathID, Root: root}})
	ts, _ := exec.Execute(context.Background(), job)
	if ts.Status != dispatch.StatusFailed || ts.FailureCode != "path_escape" {
		t.Fatalf("status=%s code=%s msg=%s", ts.Status, ts.FailureCode, ts.FailureMessage)
	}
}

func TestExecute_AtomicWrite_NoTempLeftover(t *testing.T) {
	root := t.TempDir()
	exec := &Executor{}
	job := newJob(t,
		map[string]any{"state_key": "k"},
		[]string{PathID},
		[]testfixture.PathSpec{{ID: PathID, Root: root}})
	if _, err := exec.Execute(context.Background(), job); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") && strings.Contains(e.Name(), ".tmp.") {
			t.Fatalf("tempfile leaked: %s", e.Name())
		}
	}
}

func TestExecute_NilJob(t *testing.T) {
	exec := &Executor{}
	ts, err := exec.Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("expected err on nil job")
	}
	if ts.Status != dispatch.StatusFailed || ts.FailureCode != "schema_invalid" {
		t.Fatalf("status=%s code=%s", ts.Status, ts.FailureCode)
	}
}

func TestExecute_MalformedManifest(t *testing.T) {
	exec := &Executor{}
	raw, _ := json.Marshal(map[string]any{"state_key": "k"})
	job := &outbound.LeasedJob{
		JobID:               "j",
		EnrollmentID:        "e",
		JobType:             "state.write",
		SchemaVersion:       1,
		Payload:             raw,
		ManifestJSON:        []byte("{not json"),
		ManifestBindingJSON: []byte(`{"manifest_digest":"sha256:` + strings.Repeat("0", 64) + `","path_ids":["` + PathID + `"]}`),
		LeaseToken:          "l",
	}
	ts, err := exec.Execute(context.Background(), job)
	if err == nil {
		t.Fatal("expected err on malformed manifest")
	}
	if ts.FailureCode != "manifest_invalid" {
		t.Fatalf("code=%s msg=%s", ts.FailureCode, ts.FailureMessage)
	}
}

func TestExecute_EmptyStateKey(t *testing.T) {
	root := t.TempDir()
	exec := &Executor{}
	job := newJob(t,
		map[string]any{"state_key": ""},
		[]string{PathID},
		[]testfixture.PathSpec{{ID: PathID, Root: root}})
	ts, _ := exec.Execute(context.Background(), job)
	if ts.FailureCode != "schema_invalid" {
		t.Fatalf("code=%s", ts.FailureCode)
	}
}
