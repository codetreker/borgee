//go:build linux || darwin

package installplugin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"borgee/internal/dispatch"
	"borgee/internal/jobpolicy"
	"borgee/internal/outbound"
	"borgee/internal/rootdclient"
)

// fakeRootd records the InstallPlugin invocation + returns the queued
// response/error. Lets tests verify the executor builds the right
// request without dialing a real socket.
type fakeRootd struct {
	gotReq rootdclient.InstallPluginRequest
	resp   *rootdclient.InstallPluginResponse
	err    error
}

func (f *fakeRootd) InstallPlugin(_ context.Context, opts rootdclient.InstallPluginRequest) (*rootdclient.InstallPluginResponse, error) {
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

func fixtureManifest(t *testing.T) (json.RawMessage, json.RawMessage) {
	t.Helper()
	manifest := jobpolicy.PolicyManifest{
		ManifestVersion: 1,
		Artifacts: []jobpolicy.ArtifactDeclaration{
			{ID: "openclaw-plugin", Platform: "linux-x64", Version: "1.0.0", SHA256: "sha256:deadbeef", Origin: "https://cdn.borgee.io"},
		},
		Paths: []jobpolicy.PathDeclaration{
			{ID: "openclaw_install", Root: "/usr/local/lib/borgee", Mode: "write_install"},
		},
		Domains: []string{"https://cdn.borgee.io"},
	}
	binding := jobpolicy.ManifestBinding{
		ManifestDigest: "sha256:deadbeef",
		ArtifactIDs:    []string{"openclaw-plugin"},
		PathIDs:        []string{"openclaw_install"},
		Domains:        []string{"https://cdn.borgee.io"},
	}
	return mustJSON(t, manifest), mustJSON(t, binding)
}

// TestExecuteHappyPathBuildsRequestAndMapsSuccess covers the canonical
// flow: parse payload, resolve target/manifest URL, hand to rootd, mark
// succeeded.
func TestExecuteHappyPathBuildsRequestAndMapsSuccess(t *testing.T) {
	t.Parallel()
	manifest, binding := fixtureManifest(t)
	fake := &fakeRootd{
		resp: &rootdclient.InstallPluginResponse{Installed: true, TargetPath: "/usr/local/lib/borgee/openclaw-plugin"},
	}
	exec := &Executor{Rootd: fake, PubKeyBase64: "AAAA"}
	job := &outbound.LeasedJob{
		JobType:             jobpolicy.JobTypeOpenClawInstallFromManifest,
		Payload:             []byte(`{"install_plan_id":"openclaw-plugin-v1"}`),
		ManifestJSON:        manifest,
		ManifestBindingJSON: binding,
	}
	term, err := exec.Execute(context.Background(), job)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if term.Status != dispatch.StatusSucceeded {
		t.Fatalf("status=%q, want succeeded; full=%+v", term.Status, term)
	}
	wantManifestURL := "https://cdn.borgee.io/dev-artifacts/manifests/openclaw-plugin/linux-x64.json"
	if fake.gotReq.ManifestURL != wantManifestURL {
		t.Fatalf("rootd req manifest_url=%q, want %q", fake.gotReq.ManifestURL, wantManifestURL)
	}
	if fake.gotReq.PluginID != "openclaw-plugin" {
		t.Fatalf("rootd req plugin_id=%q, want openclaw-plugin", fake.gotReq.PluginID)
	}
	if fake.gotReq.TargetPath != "/usr/local/lib/borgee/openclaw-plugin" {
		t.Fatalf("rootd req target_path=%q, want /usr/local/lib/borgee/openclaw-plugin", fake.gotReq.TargetPath)
	}
}

// TestExecuteRejectsNilJob covers the defensive nil guard.
func TestExecuteRejectsNilJob(t *testing.T) {
	t.Parallel()
	exec := &Executor{Rootd: &fakeRootd{}, PubKeyBase64: "AA"}
	term, err := exec.Execute(context.Background(), nil)
	if err == nil || term.Status != dispatch.StatusFailed || term.FailureCode != "schema_invalid" {
		t.Fatalf("nil job not rejected: status=%q code=%q err=%v", term.Status, term.FailureCode, err)
	}
}

// TestExecuteRejectsEmptyPayload guards against missing install_plan_id.
func TestExecuteRejectsEmptyPayload(t *testing.T) {
	t.Parallel()
	exec := &Executor{Rootd: &fakeRootd{}, PubKeyBase64: "AA"}
	term, err := exec.Execute(context.Background(), &outbound.LeasedJob{
		Payload: []byte(`{"install_plan_id":""}`),
	})
	if err == nil || term.FailureCode != "schema_invalid" {
		t.Fatalf("empty install_plan_id not rejected: %+v err=%v", term, err)
	}
}

// TestExecuteRejectsMissingTrustRoot covers the policy_denied path
// when the daemon was launched without BORGEE_MANIFEST_SIGNING_PUBKEY.
func TestExecuteRejectsMissingTrustRoot(t *testing.T) {
	t.Parallel()
	exec := &Executor{Rootd: &fakeRootd{}}
	term, err := exec.Execute(context.Background(), &outbound.LeasedJob{
		Payload: []byte(`{"install_plan_id":"openclaw-plugin-v1"}`),
	})
	if err == nil || term.FailureCode != "policy_denied" {
		t.Fatalf("missing pubkey not rejected: %+v err=%v", term, err)
	}
}

// TestExecuteFailsLoudWhenManifestEmpty mirrors the PR-3 statewrite
// pattern: an empty manifest JSON (today's production state) yields
// manifest_invalid, not silent fallback.
func TestExecuteFailsLoudWhenManifestEmpty(t *testing.T) {
	t.Parallel()
	exec := &Executor{Rootd: &fakeRootd{}, PubKeyBase64: "AA"}
	term, err := exec.Execute(context.Background(), &outbound.LeasedJob{
		Payload:             []byte(`{"install_plan_id":"openclaw-plugin-v1"}`),
		ManifestJSON:        []byte(``),
		ManifestBindingJSON: []byte(`{"manifest_digest":"sha256:x"}`),
	})
	if err == nil || term.FailureCode != "manifest_invalid" {
		t.Fatalf("empty manifest not rejected: %+v err=%v", term, err)
	}
}

// TestExecuteMapsRootdSignatureInvalidToCode propagates install-butler's
// reason:detail back as a server-friendly failure_code.
func TestExecuteMapsRootdSignatureInvalidToCode(t *testing.T) {
	t.Parallel()
	manifest, binding := fixtureManifest(t)
	fake := &fakeRootd{
		err: errors.New("signature_invalid: entry openclaw-plugin signature verification failed"),
	}
	exec := &Executor{Rootd: fake, PubKeyBase64: "AA"}
	term, err := exec.Execute(context.Background(), &outbound.LeasedJob{
		Payload:             []byte(`{"install_plan_id":"openclaw-plugin-v1"}`),
		ManifestJSON:        manifest,
		ManifestBindingJSON: binding,
	})
	if err == nil {
		t.Fatalf("expected error propagation")
	}
	if term.FailureCode != "signature_invalid" {
		t.Fatalf("failure_code=%q, want signature_invalid", term.FailureCode)
	}
	if !strings.Contains(term.FailureMessage, "signature verification failed") {
		t.Fatalf("failure_message lost detail: %q", term.FailureMessage)
	}
}

// TestExecuteMapsRootdSha256Mismatch covers another install-butler
// reason mapping.
func TestExecuteMapsRootdSha256Mismatch(t *testing.T) {
	t.Parallel()
	manifest, binding := fixtureManifest(t)
	fake := &fakeRootd{err: errors.New("sha256_mismatch: entry openclaw-plugin sha256 mismatch")}
	exec := &Executor{Rootd: fake, PubKeyBase64: "AA"}
	term, _ := exec.Execute(context.Background(), &outbound.LeasedJob{
		Payload:             []byte(`{"install_plan_id":"openclaw-plugin-v1"}`),
		ManifestJSON:        manifest,
		ManifestBindingJSON: binding,
	})
	if term.FailureCode != "sha256_mismatch" {
		t.Fatalf("failure_code=%q, want sha256_mismatch", term.FailureCode)
	}
}
