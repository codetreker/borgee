//go:build linux || darwin

package updatecheck

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testServer wraps httptest with an atomic hit counter + a swappable handler.
type testServer struct {
	srv      *httptest.Server
	hits     atomic.Int64
	mu       sync.Mutex
	respBody []byte
	respCode int
	lastReq  []byte
}

func newTestServer(code int, drift []DriftEntry) *testServer {
	ts := &testServer{respCode: code}
	body, _ := json.Marshal(map[string]any{
		"updates_available": drift,
	})
	ts.respBody = body
	ts.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts.hits.Add(1)
		// Capture the last request body for assertions.
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		ts.mu.Lock()
		ts.lastReq = buf
		code := ts.respCode
		body := ts.respBody
		ts.mu.Unlock()
		w.WriteHeader(code)
		_, _ = w.Write(body)
	}))
	return ts
}

func (ts *testServer) close() { ts.srv.Close() }

func newChecker(t *testing.T, origin string, installedPath string) *Checker {
	t.Helper()
	return &Checker{
		Client:                &http.Client{Timeout: 2 * time.Second},
		ServerOrigin:          origin,
		EnrollmentID:          "enroll-1",
		HelperDeviceID:        "device-1",
		Credential:            "helper-token",
		InstalledVersionsPath: installedPath,
		Interval:              50 * time.Millisecond,
		BackoffBase:           20 * time.Millisecond,
		BackoffCap:            80 * time.Millisecond,
		Logger:                func(format string, v ...any) {},
	}
}

func writeInstalled(t *testing.T, path string, plugins map[string]installedVersionRecord) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(installedVersionsFile{Plugins: plugins})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TU-1 DetectsSecurityDrift — server returns class=security; checker logs the
// prominent prompt log line.
func TestChecker_DetectsSecurityDrift(t *testing.T) {
	t.Parallel()
	ts := newTestServer(http.StatusOK, []DriftEntry{
		{PluginID: "openclaw", CurrentVersion: "0.1.0", ManifestVersion: "0.2.0", Class: "security"},
	})
	defer ts.close()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "installed.json")
	writeInstalled(t, path, map[string]installedVersionRecord{
		"openclaw": {Version: "0.1.0", InstalledAt: time.Now().UnixMilli(), SHA256: "deadbeef"},
	})

	c := newChecker(t, ts.srv.URL, path)
	var logged []string
	var mu sync.Mutex
	c.Logger = func(format string, v ...any) {
		mu.Lock()
		defer mu.Unlock()
		logged = append(logged, sprintfLite(format, v...))
	}
	c.Interval = 10 * time.Second // success branch — only first tick matters

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = c.Run(ctx) }()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if ts.hits.Load() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if ts.hits.Load() < 1 {
		t.Fatalf("expected at least 1 POST hit")
	}
	// Allow logger goroutine to flush.
	time.Sleep(20 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	gotSecurity := false
	for _, line := range logged {
		if contains(line, "update-available.security") && contains(line, "plugin=openclaw") && contains(line, "manifest=0.2.0") {
			gotSecurity = true
		}
	}
	if !gotSecurity {
		t.Fatalf("missing security-class log line; got %v", logged)
	}
}

// TU-2 DetectsFeatureDrift — class=feature path.
func TestChecker_DetectsFeatureDrift(t *testing.T) {
	t.Parallel()
	ts := newTestServer(http.StatusOK, []DriftEntry{
		{PluginID: "openclaw", CurrentVersion: "1.0.0", ManifestVersion: "1.1.0", Class: "feature"},
	})
	defer ts.close()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "installed.json")
	writeInstalled(t, path, map[string]installedVersionRecord{
		"openclaw": {Version: "1.0.0"},
	})

	c := newChecker(t, ts.srv.URL, path)
	var logged []string
	var mu sync.Mutex
	c.Logger = func(format string, v ...any) {
		mu.Lock()
		defer mu.Unlock()
		logged = append(logged, sprintfLite(format, v...))
	}
	c.Interval = 10 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = c.Run(ctx) }()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if ts.hits.Load() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	gotFeature := false
	for _, line := range logged {
		if contains(line, "update-available.feature") && contains(line, "plugin=openclaw") {
			gotFeature = true
		}
	}
	if !gotFeature {
		t.Fatalf("missing feature-class log line; got %v", logged)
	}
}

// TU-3 NoDriftQuiet — server returns empty drift, no update-available log line.
func TestChecker_NoDriftQuiet(t *testing.T) {
	t.Parallel()
	ts := newTestServer(http.StatusOK, []DriftEntry{})
	defer ts.close()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "installed.json")
	writeInstalled(t, path, map[string]installedVersionRecord{
		"openclaw": {Version: "1.0.0"},
	})

	c := newChecker(t, ts.srv.URL, path)
	var logged []string
	var mu sync.Mutex
	c.Logger = func(format string, v ...any) {
		mu.Lock()
		defer mu.Unlock()
		logged = append(logged, sprintfLite(format, v...))
	}
	c.Interval = 10 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = c.Run(ctx) }()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if ts.hits.Load() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	for _, line := range logged {
		if contains(line, "update-available") {
			t.Fatalf("unexpected update-available log line on empty drift: %s", line)
		}
	}
}

// TU-4 MissingInstalledVersionsFile — fresh helper pre-install. fires POST
// with empty installed list, no panic.
func TestChecker_MissingInstalledVersionsFile(t *testing.T) {
	t.Parallel()
	ts := newTestServer(http.StatusOK, []DriftEntry{})
	defer ts.close()

	tmp := t.TempDir()
	missing := filepath.Join(tmp, "no-such-file.json")
	c := newChecker(t, ts.srv.URL, missing)
	c.Interval = 10 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = c.Run(ctx) }()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if ts.hits.Load() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if ts.hits.Load() < 1 {
		t.Fatalf("missing-file path should still POST; got %d hits", ts.hits.Load())
	}
	ts.mu.Lock()
	defer ts.mu.Unlock()
	var got postRequest
	if err := json.Unmarshal(ts.lastReq, &got); err != nil {
		t.Fatalf("unmarshal request: %v body=%s", err, string(ts.lastReq))
	}
	if got.HelperDeviceID != "device-1" {
		t.Fatalf("device id missing in POST: %+v", got)
	}
	if len(got.Installed) != 0 {
		t.Fatalf("expected empty installed list on missing file, got %+v", got.Installed)
	}
}

// TU-5 RequiresAuth — Run returns error when credential is empty.
func TestChecker_RequiresAuthFields(t *testing.T) {
	t.Parallel()
	c := &Checker{
		ServerOrigin:   "https://example",
		EnrollmentID:   "enroll-1",
		HelperDeviceID: "device-1",
		Credential:     "", // missing
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Run(ctx); err == nil {
		t.Fatalf("expected error on missing credential")
	}
}

// TU-6 ContextCancel — Run returns promptly on ctx cancel.
func TestChecker_ContextCancel(t *testing.T) {
	t.Parallel()
	ts := newTestServer(http.StatusOK, []DriftEntry{})
	defer ts.close()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "installed.json")
	writeInstalled(t, path, map[string]installedVersionRecord{})
	c := newChecker(t, ts.srv.URL, path)
	c.Interval = 10 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	// Wait for first hit to ensure we're in the wait branch.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && ts.hits.Load() < 1 {
		time.Sleep(5 * time.Millisecond)
	}
	if ts.hits.Load() < 1 {
		t.Fatalf("first POST never fired")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("Run did not return within 200ms of cancel")
	}
}

// helpers ----------------------------------------------------------------

func contains(haystack, needle string) bool {
	return indexOf(haystack, needle) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// sprintfLite mirrors fmt.Sprintf for the format strings the checker uses
// (single-pass %s / %v substitution). Avoids pulling fmt into test-only
// expansion paths.
func sprintfLite(format string, v ...any) string {
	out := make([]byte, 0, len(format))
	vi := 0
	for i := 0; i < len(format); i++ {
		if format[i] == '%' && i+1 < len(format) {
			i++
			if vi < len(v) {
				out = append(out, []byte(toString(v[vi]))...)
				vi++
			}
			continue
		}
		out = append(out, format[i])
	}
	return string(out)
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case error:
		return t.Error()
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// TestDeriveHTTPOrigin_Schemes (amend gap #7) — the daemon's
// ServerOrigin is `wss://...` (the persistent WS transport URL), but
// the installed-versions endpoint is REST. Without scheme rewrite, the
// underlying http.Transport rejects "wss" with `unsupported protocol
// scheme`. The mapping is wss→https, ws→http, anything else passes
// through unchanged.
func TestDeriveHTTPOrigin_Schemes(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"wss://x.example", "https://x.example"},
		{"ws://x.example:1234", "http://x.example:1234"},
		{"https://x.example", "https://x.example"},
		{"http://localhost", "http://localhost"},
		{"  wss://x.example/  ", "https://x.example"},
	}
	for _, c := range cases {
		if got := deriveHTTPOrigin(c.in); got != c.want {
			t.Errorf("deriveHTTPOrigin(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
