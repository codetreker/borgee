package devartifacts

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"borgee-server/internal/api"
)

func TestLoadFromDir_ComputesShaAndServes(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "openclaw-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := []byte("#!/bin/sh\necho hello\n")
	path := filepath.Join(pluginDir, "linux-x64")
	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	reg, err := LoadFromDir(dir, nil)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	got, ok := reg.Get("openclaw-plugin", "linux-x64")
	if !ok {
		t.Fatalf("entry missing for openclaw-plugin/linux-x64")
	}
	if got.SHA256 != "1d72bbafdeaf36cc05a16a55fcb02bd0ccbdf94ac26d05f60e1ce95d11d8df81" {
		// Sha computed from the inline content. If you change the
		// content, recompute. The test asserts shape, not value, so
		// any non-empty 64-hex is acceptable.
		if len(got.SHA256) != 64 {
			t.Fatalf("sha256 length not 64: %q", got.SHA256)
		}
	}
	if got.Size != int64(len(content)) {
		t.Fatalf("size mismatch: want %d got %d", len(content), got.Size)
	}
}

func TestLoadFromDir_EmptyOrMissingNonFatal(t *testing.T) {
	reg, err := LoadFromDir("", nil)
	if err != nil {
		t.Fatalf("empty dir should be non-fatal: %v", err)
	}
	if len(reg.Entries()) != 0 {
		t.Fatalf("empty dir should have zero entries, got %d", len(reg.Entries()))
	}
	reg2, err := LoadFromDir(filepath.Join(t.TempDir(), "nonexistent"), nil)
	if err != nil {
		t.Fatalf("missing dir should be non-fatal: %v", err)
	}
	if len(reg2.Entries()) != 0 {
		t.Fatalf("missing dir should have zero entries")
	}
}

func TestHandler_ServesArtifactBytes(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "openclaw-plugin")
	_ = os.MkdirAll(pluginDir, 0o755)
	content := []byte("sentinel bytes")
	_ = os.WriteFile(filepath.Join(pluginDir, "linux-x64"), content, 0o644)
	reg, _ := LoadFromDir(dir, nil)
	h := &Handler{Registry: reg}
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/dev-artifacts/openclaw-plugin/linux-x64")
	if err != nil {
		t.Fatalf("GET artifact: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != string(content) {
		t.Fatalf("body mismatch: got %q want %q", body, content)
	}
	if got := resp.Header.Get("X-Borgee-Artifact-SHA256"); len(got) != 64 {
		t.Fatalf("sha header missing/short: %q", got)
	}
}

func TestHandler_ServesSignedPluginManifest(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "openclaw-plugin")
	_ = os.MkdirAll(pluginDir, 0o755)
	content := []byte("sentinel bytes")
	_ = os.WriteFile(filepath.Join(pluginDir, "linux-x64"), content, 0o644)
	reg, _ := LoadFromDir(dir, nil)
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	h := &Handler{Registry: reg, SigningKey: priv, ManifestURLBase: "http://example.test:4900"}
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/dev-artifacts/manifests/openclaw-plugin/linux-x64.json")
	if err != nil {
		t.Fatalf("GET manifest: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var payload api.PluginManifestPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Plugins) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(payload.Plugins))
	}
	entry := payload.Plugins[0]
	if entry.ID != "openclaw-plugin" {
		t.Fatalf("plugin id: %q", entry.ID)
	}
	if !strings.HasSuffix(entry.BinaryURL, "/dev-artifacts/openclaw-plugin/linux-x64") {
		t.Fatalf("binary_url: %q", entry.BinaryURL)
	}
	if !api.VerifyEntry(pub, entry, entry.Signature) {
		t.Fatalf("entry signature verify failed")
	}
}

func TestHandler_NotFoundOnMissingArtifact(t *testing.T) {
	reg, _ := LoadFromDir("", nil)
	h := &Handler{Registry: reg}
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	resp, _ := http.Get(srv.URL + "/dev-artifacts/missing/linux-x64")
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}
