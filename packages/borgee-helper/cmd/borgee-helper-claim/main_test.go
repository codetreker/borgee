//go:build linux || darwin

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// TestClaim_HappyPath exercises the full claim CLI flow against an in-process
// httptest server that mirrors the real /claim handler shape: it returns 201
// with helper_credential when enrollment_secret + helper_device_id are
// non-empty.
//
// What the test locks (#968 R4 end-to-end claim half):
//   - POST URL matches /api/v1/helper/enrollments/{id}/claim.
//   - Request body carries the two-field JSON shape the server expects.
//   - Successful response writes credential (0600), enrollment-id, device-id
//     into the three file paths.
//   - Re-run reuses the persisted device-id (no fresh UUID each invocation).
func TestClaim_HappyPath(t *testing.T) {
	const (
		fakeEnrollment = "enr-test-1"
		fakeSecret     = "sec-1"
		fakeCredential = "tok-1"
	)
	var lastBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "want POST", http.StatusMethodNotAllowed)
			return
		}
		wantPath := "/api/v1/helper/enrollments/" + fakeEnrollment + "/claim"
		if r.URL.Path != wantPath {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		lastBody.Store(body)
		var req struct {
			EnrollmentSecret string `json:"enrollment_secret"`
			HelperDeviceID   string `json:"helper_device_id"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.EnrollmentSecret != fakeSecret || req.HelperDeviceID == "" {
			http.Error(w, "bad creds", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"helper_credential": fakeCredential,
			"enrollment":        map[string]any{"enrollment_id": fakeEnrollment},
		})
	}))
	t.Cleanup(srv.Close)

	tmp := t.TempDir()
	credPath := filepath.Join(tmp, "credential")
	idPath := filepath.Join(tmp, "enrollment-id")
	devPath := filepath.Join(tmp, "device-id")

	var out, errBuf bytes.Buffer
	if err := runCLI([]string{
		"--enrollment-id=" + fakeEnrollment,
		"--enrollment-secret=" + fakeSecret,
		"--server-origin=" + srv.URL,
		"--allow-insecure-server-origin",
		"--credential-file=" + credPath,
		"--enrollment-id-file=" + idPath,
		"--device-id-file=" + devPath,
	}, &out, &errBuf); err != nil {
		t.Fatalf("runCLI: %v (stderr=%s)", err, errBuf.String())
	}
	if !strings.Contains(out.String(), "claimed enrollment "+fakeEnrollment) {
		t.Errorf("stdout missing claim line; got %q", out.String())
	}

	// Credential persisted with the issued token + 0600 perms.
	body, err := os.ReadFile(credPath)
	if err != nil {
		t.Fatalf("read credential: %v", err)
	}
	if string(body) != fakeCredential {
		t.Errorf("credential content = %q, want %q", string(body), fakeCredential)
	}
	info, err := os.Stat(credPath)
	if err != nil {
		t.Fatalf("stat credential: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("credential perm = %o, want 0600", mode)
	}

	// Enrollment + device id files were written and survive across re-claim.
	if got, _ := os.ReadFile(idPath); strings.TrimSpace(string(got)) != fakeEnrollment {
		t.Errorf("enrollment-id file = %q, want %q", string(got), fakeEnrollment)
	}
	devBefore, _ := os.ReadFile(devPath)
	if strings.TrimSpace(string(devBefore)) == "" {
		t.Fatalf("device-id file is empty after first claim")
	}

	// Re-run: device-id file must be reused, not regenerated.
	out.Reset()
	errBuf.Reset()
	if err := runCLI([]string{
		"--enrollment-id=" + fakeEnrollment,
		"--enrollment-secret=" + fakeSecret,
		"--server-origin=" + srv.URL,
		"--allow-insecure-server-origin",
		"--credential-file=" + credPath,
		"--enrollment-id-file=" + idPath,
		"--device-id-file=" + devPath,
	}, &out, &errBuf); err != nil {
		t.Fatalf("runCLI re-run: %v (stderr=%s)", err, errBuf.String())
	}
	devAfter, _ := os.ReadFile(devPath)
	if string(devBefore) != string(devAfter) {
		t.Errorf("device-id changed across re-claim: before=%q after=%q", string(devBefore), string(devAfter))
	}
}

// TestClaim_ServerRejects covers the 4xx error path. On non-201 we must not
// overwrite the credential file; the previous credential is still valid until
// an admin rotates it server-side.
func TestClaim_ServerRejects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Helper enrollment already claimed", http.StatusConflict)
	}))
	t.Cleanup(srv.Close)

	tmp := t.TempDir()
	credPath := filepath.Join(tmp, "credential")
	if err := os.WriteFile(credPath, []byte("old"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var out, errBuf bytes.Buffer
	err := runCLI([]string{
		"--enrollment-id=enr",
		"--enrollment-secret=sec",
		"--server-origin=" + srv.URL,
		"--allow-insecure-server-origin",
		"--credential-file=" + credPath,
		"--enrollment-id-file=" + filepath.Join(tmp, "enrollment-id"),
		"--device-id-file=" + filepath.Join(tmp, "device-id"),
	}, &out, &errBuf)
	if err == nil {
		t.Fatalf("expected error on 409; got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 409") {
		t.Errorf("error missing HTTP 409 marker: %v", err)
	}
	body, _ := os.ReadFile(credPath)
	if string(body) != "old" {
		t.Errorf("credential clobbered on failure: %q", string(body))
	}
}

// TestClaim_HTTPSRequired guards the production safety: without
// --allow-insecure-server-origin, plain-http origins must be rejected. This
// keeps the operator from accidentally posting enrollment_secret in clear.
func TestClaim_HTTPSRequired(t *testing.T) {
	var out, errBuf bytes.Buffer
	err := runCLI([]string{
		"--enrollment-id=enr",
		"--enrollment-secret=sec",
		"--server-origin=http://example.test",
		"--credential-file=" + filepath.Join(t.TempDir(), "credential"),
	}, &out, &errBuf)
	if err == nil || !strings.Contains(err.Error(), "https is required") {
		t.Fatalf("want https-required error; got %v", err)
	}
}

// TestResolveDeviceID_PrefersPersistedFile ensures re-claim uses the existing
// device-id file content rather than regenerating from machine-id or UUID.
func TestResolveDeviceID_PrefersPersistedFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "device-id")
	const want = "persisted-id-xyz"
	if err := os.WriteFile(path, []byte(want+"\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := resolveDeviceID(path)
	if err != nil {
		t.Fatalf("resolveDeviceID: %v", err)
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
