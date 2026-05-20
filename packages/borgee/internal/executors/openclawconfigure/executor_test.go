//go:build linux || darwin

package openclawconfigure

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
		JobType:             "openclaw.configure_agent",
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
		"agent_id":              "agent-1",
		"channel_id":            "channel-1",
		"config_schema_version": 2,
		"config_hash":           "sha256:abc",
	}
}

func TestExecute_HappyPath(t *testing.T) {
	root := t.TempDir()
	exec := &Executor{Now: func() time.Time { return time.Unix(1_780_000_000, 0).UTC() }}
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
	dest := filepath.Join(root, "agent-1.json")
	raw, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["agent_id"] != "agent-1" || got["config_hash"] != "sha256:abc" {
		t.Fatalf("unexpected content: %v", got)
	}
}

func TestExecute_FailsWhenManifestMissingPathID(t *testing.T) {
	root := t.TempDir()
	exec := &Executor{}
	job := newJob(t, goodPayload(),
		[]string{PathID},
		[]testfixture.PathSpec{{ID: "other_id", Root: root}})
	ts, _ := exec.Execute(context.Background(), job)
	if ts.FailureCode != "manifest_missing_path_id" {
		t.Fatalf("code=%s msg=%s", ts.FailureCode, ts.FailureMessage)
	}
}

func TestExecute_FailsWhenBindingMissingPathID(t *testing.T) {
	root := t.TempDir()
	exec := &Executor{}
	job := newJob(t, goodPayload(),
		[]string{"other_id"},
		[]testfixture.PathSpec{{ID: PathID, Root: root}})
	ts, _ := exec.Execute(context.Background(), job)
	if ts.FailureCode != "manifest_missing_path_id" {
		t.Fatalf("code=%s msg=%s", ts.FailureCode, ts.FailureMessage)
	}
}

func TestExecute_RejectsPayloadPathEscape(t *testing.T) {
	root := t.TempDir()
	exec := &Executor{}
	p := goodPayload()
	p["agent_id"] = "../etc/passwd"
	job := newJob(t, p,
		[]string{PathID},
		[]testfixture.PathSpec{{ID: PathID, Root: root}})
	ts, _ := exec.Execute(context.Background(), job)
	// agent_id validator catches this first; both are correct rejections.
	if ts.FailureCode != "schema_invalid" && ts.FailureCode != "path_escape" {
		t.Fatalf("code=%s msg=%s", ts.FailureCode, ts.FailureMessage)
	}
}

func TestExecute_AtomicWrite_NoTempLeftover(t *testing.T) {
	root := t.TempDir()
	exec := &Executor{}
	job := newJob(t, goodPayload(),
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
	if ts.FailureCode != "schema_invalid" {
		t.Fatalf("code=%s", ts.FailureCode)
	}
}

func TestExecute_MalformedManifest(t *testing.T) {
	exec := &Executor{}
	raw, _ := json.Marshal(goodPayload())
	job := &outbound.LeasedJob{
		JobID:               "j",
		EnrollmentID:        "e",
		JobType:             "openclaw.configure_agent",
		SchemaVersion:       1,
		Payload:             raw,
		ManifestJSON:        []byte("{not json"),
		ManifestBindingJSON: []byte(`{"manifest_digest":"sha256:` + strings.Repeat("0", 64) + `","path_ids":["` + PathID + `"]}`),
		LeaseToken:          "l",
	}
	ts, err := exec.Execute(context.Background(), job)
	if err == nil {
		t.Fatal("expected err")
	}
	if ts.FailureCode != "manifest_invalid" {
		t.Fatalf("code=%s msg=%s", ts.FailureCode, ts.FailureMessage)
	}
}
