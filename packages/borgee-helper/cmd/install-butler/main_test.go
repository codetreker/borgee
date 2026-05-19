//go:build linux || darwin

// Package main tests for install-butler (#996).
//
// All seven test buckets (TB-1..7) drive the install-butler binary as a
// child process via exec.Command. This is required by acceptance criteria —
// we must exercise the real exit code + stderr surface that operators (and
// downstream borgee-installer scripts) will rely on.
//
// httptest serves both the manifest and the binary; ed25519 keys are
// generated per-test so we never depend on real production keys.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// installButlerBinary is built once per test process via TestMain so the
// seven subprocess tests share one binary on disk.
var installButlerBinary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "install-butler-bin-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "TestMain: mktemp:", err)
		os.Exit(2)
	}
	bin := filepath.Join(dir, "install-butler")
	// Build the binary in this same package — the test file lives next to
	// main.go so `go build .` covers both.
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Stdout = os.Stderr
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "TestMain: build install-butler:", err)
		os.Remove(bin)
		os.RemoveAll(dir)
		os.Exit(2)
	}
	installButlerBinary = bin

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// signedManifestFixture builds a valid manifest payload signed by `key`,
// for a single entry whose binary content is `binary` and whose other
// fields are taken from `entry` (Signature + SHA256 are filled in).
type signedManifestFixture struct {
	payload pluginManifestPayload
	entry   pluginManifestEntry
	binary  []byte
}

func buildFixture(t *testing.T, key ed25519.PrivateKey, binaryURL string, binary []byte) signedManifestFixture {
	t.Helper()
	sum := sha256.Sum256(binary)
	entry := pluginManifestEntry{
		ID:        "openclaw",
		Version:   "1.2.3",
		BinaryURL: binaryURL,
		SHA256:    hex.EncodeToString(sum[:]),
		Platforms: []string{"linux-x64"},
	}
	sig := ed25519.Sign(key, entryCanonicalBytes(entry))
	entry.Signature = base64.StdEncoding.EncodeToString(sig)
	return signedManifestFixture{
		payload: pluginManifestPayload{
			ManifestVersion: 1,
			IssuedAt:        time.Now().UnixMilli(),
			ExpiresAt:       time.Now().Add(24 * time.Hour).UnixMilli(),
			Signature:       "test-top-level-signature",
			Plugins:         []pluginManifestEntry{entry},
		},
		entry:  entry,
		binary: binary,
	}
}

// startBinaryServer spins up an httptest server that serves `binary` at
// /binary. Returns the full URL. Caller is responsible for the cleanup via
// t.Cleanup which startBinaryServer registers.
func startBinaryServer(t *testing.T, binary []byte) string {
	t.Helper()
	binSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/binary" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(binary)
	}))
	t.Cleanup(binSrv.Close)
	return binSrv.URL + "/binary"
}

// startManifestServer serves the (already-signed) payload at /manifest.
func startManifestServer(t *testing.T, payload pluginManifestPayload) string {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	manSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/manifest" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	t.Cleanup(manSrv.Close)
	return manSrv.URL + "/manifest"
}

// runCLI invokes the install-butler binary with the given args, returning
// combined stdout/stderr + exit-code (0 on success). It never panics on
// non-zero exit so tests can assert on the failure.
func runCLI(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(installButlerBinary, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()
	if err == nil {
		return stdout, stderr, 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return stdout, stderr, ee.ExitCode()
	}
	t.Fatalf("runCLI: unexpected error type %T: %v", err, err)
	return
}

func genKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey: %v", err)
	}
	return pub, priv
}

// TB-1 HappyPath — full chain: manifest fetch → sig verify → binary fetch
// → sha256 match → atomic rename → chmod 0755.
func TestInstallButler_TB1_HappyPath(t *testing.T) {
	t.Parallel()
	pub, priv := genKey(t)
	binary := []byte("hello-openclaw-binary-bytes")
	binaryURL := startBinaryServer(t, binary)
	fixture := buildFixture(t, priv, binaryURL, binary)
	manifestURL := startManifestServer(t, fixture.payload)

	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "openclaw")
	stdout, stderr, code := runCLI(t,
		"--manifest-url", manifestURL,
		"--pubkey-base64", base64.StdEncoding.EncodeToString(pub),
		"--plugin-id", "openclaw",
		"--target", target,
		"--allow-insecure-manifest-url",
		"--allow-insecure-binary-url",
	)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q stdout=%q", code, stderr, stdout)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if !bytes.Equal(got, binary) {
		t.Fatalf("target contents mismatch: got %d bytes want %d", len(got), len(binary))
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("target perms: got %o want 0755", info.Mode().Perm())
	}
	if !strings.Contains(stdout, "installed plugin openclaw version 1.2.3") {
		t.Fatalf("stdout missing success line: %q", stdout)
	}
}

// TB-2 SignatureInvalid — tamper the entry signature, expect rejection +
// no target file written.
func TestInstallButler_TB2_SignatureInvalid(t *testing.T) {
	t.Parallel()
	pub, priv := genKey(t)
	binary := []byte("payload")
	binaryURL := startBinaryServer(t, binary)
	fixture := buildFixture(t, priv, binaryURL, binary)
	// Tamper: flip one byte of the signature after base64-decoding.
	sigBytes, _ := base64.StdEncoding.DecodeString(fixture.payload.Plugins[0].Signature)
	sigBytes[0] ^= 0xFF
	fixture.payload.Plugins[0].Signature = base64.StdEncoding.EncodeToString(sigBytes)

	manifestURL := startManifestServer(t, fixture.payload)
	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "openclaw")
	_, stderr, code := runCLI(t,
		"--manifest-url", manifestURL,
		"--pubkey-base64", base64.StdEncoding.EncodeToString(pub),
		"--plugin-id", "openclaw",
		"--target", target,
		"--allow-insecure-manifest-url",
		"--allow-insecure-binary-url",
	)
	if code == 0 {
		t.Fatalf("expected non-zero exit, got 0; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "signature_invalid") {
		t.Fatalf("expected signature_invalid in stderr, got %q", stderr)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target must not exist after signature_invalid; stat err=%v", err)
	}
}

// TB-3 SHA256Mismatch — server returns extra bytes, sha256 should not match;
// target file must NOT be created (NOT half-written).
func TestInstallButler_TB3_SHA256Mismatch(t *testing.T) {
	t.Parallel()
	pub, priv := genKey(t)
	binary := []byte("the-real-bytes")
	// Server lies about the bytes — serve tampered binary while the
	// fixture's SHA256 still describes the original.
	tampered := append([]byte{}, binary...)
	tampered = append(tampered, "EXTRA"...)
	binaryURL := startBinaryServer(t, tampered)
	fixture := buildFixture(t, priv, binaryURL, binary)
	manifestURL := startManifestServer(t, fixture.payload)

	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "openclaw")
	_, stderr, code := runCLI(t,
		"--manifest-url", manifestURL,
		"--pubkey-base64", base64.StdEncoding.EncodeToString(pub),
		"--plugin-id", "openclaw",
		"--target", target,
		"--allow-insecure-manifest-url",
		"--allow-insecure-binary-url",
	)
	if code == 0 {
		t.Fatalf("expected non-zero exit, got 0; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "sha256_mismatch") {
		t.Fatalf("expected sha256_mismatch in stderr, got %q", stderr)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target must not exist after sha256_mismatch; stat err=%v", err)
	}
	// Also assert no .partial leftover in target dir.
	entries, _ := os.ReadDir(targetDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".partial") {
			t.Fatalf("leftover tempfile %q after sha256_mismatch", e.Name())
		}
	}
}

// TB-4 PluginNotFound — request id that isn't in the manifest.
func TestInstallButler_TB4_PluginNotFound(t *testing.T) {
	t.Parallel()
	pub, priv := genKey(t)
	binary := []byte("x")
	binaryURL := startBinaryServer(t, binary)
	fixture := buildFixture(t, priv, binaryURL, binary)
	manifestURL := startManifestServer(t, fixture.payload)

	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "ghost")
	_, stderr, code := runCLI(t,
		"--manifest-url", manifestURL,
		"--pubkey-base64", base64.StdEncoding.EncodeToString(pub),
		"--plugin-id", "does-not-exist",
		"--target", target,
		"--allow-insecure-manifest-url",
		"--allow-insecure-binary-url",
	)
	if code == 0 {
		t.Fatalf("expected non-zero exit, got 0; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "plugin_not_found") {
		t.Fatalf("expected plugin_not_found in stderr, got %q", stderr)
	}
}

// TB-5 ManifestFetchFailed — server returns 500.
func TestInstallButler_TB5_ManifestFetchFailed(t *testing.T) {
	t.Parallel()
	pub, _ := genKey(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "openclaw")
	_, stderr, code := runCLI(t,
		"--manifest-url", srv.URL+"/manifest",
		"--pubkey-base64", base64.StdEncoding.EncodeToString(pub),
		"--plugin-id", "openclaw",
		"--target", target,
		"--allow-insecure-manifest-url",
		"--allow-insecure-binary-url",
	)
	if code == 0 {
		t.Fatalf("expected non-zero exit, got 0; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "manifest_fetch_failed") {
		t.Fatalf("expected manifest_fetch_failed in stderr, got %q", stderr)
	}
}

// TB-6 DryRun — --dry-run verifies everything but never writes the target.
func TestInstallButler_TB6_DryRun(t *testing.T) {
	t.Parallel()
	pub, priv := genKey(t)
	binary := []byte("hello-dry-run")
	binaryURL := startBinaryServer(t, binary)
	fixture := buildFixture(t, priv, binaryURL, binary)
	manifestURL := startManifestServer(t, fixture.payload)

	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "openclaw")
	stdout, stderr, code := runCLI(t,
		"--manifest-url", manifestURL,
		"--pubkey-base64", base64.StdEncoding.EncodeToString(pub),
		"--plugin-id", "openclaw",
		"--target", target,
		"--dry-run",
		"--allow-insecure-manifest-url",
		"--allow-insecure-binary-url",
	)
	if code != 0 {
		t.Fatalf("expected exit 0 on dry-run, got %d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "would write to "+target) {
		t.Fatalf("expected dry-run plan in stdout, got %q", stdout)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target must NOT exist after dry-run; stat err=%v", err)
	}
}

// TB-7 AtomicWriteOnFail — server closes connection mid-stream; --target
// must keep its prior contents (atomic rename never happened).
func TestInstallButler_TB7_AtomicWritePreservesPriorTarget(t *testing.T) {
	t.Parallel()
	pub, priv := genKey(t)
	// 32 KiB so the server has enough room to short-write before completing.
	binary := bytes.Repeat([]byte("AB"), 16*1024)

	// Custom binary server that flushes a few bytes then yanks the
	// connection (no proper Content-Length close → io.Copy returns
	// unexpected EOF).
	var hits atomic.Int32
	binSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/octet-stream")
		// Advertise full length so the client expects more than we send.
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(binary)))
		w.WriteHeader(http.StatusOK)
		// Write only a tiny prefix, then hijack + close.
		_, _ = w.Write(binary[:64])
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		hj, ok := w.(http.Hijacker)
		if !ok {
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			return
		}
		_ = conn.Close()
	}))
	t.Cleanup(binSrv.Close)

	// Build the fixture once we know the truncating server's URL so the
	// signature actually covers the URL the client will fetch from.
	fixture := buildFixture(t, priv, binSrv.URL+"/binary", binary)
	manifestURL := startManifestServer(t, fixture.payload)

	// Pre-existing target with known content — must remain untouched.
	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "openclaw")
	priorContent := []byte("PREVIOUSLY-INSTALLED-VERSION")
	if err := os.WriteFile(target, priorContent, 0o755); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	_, stderr, code := runCLI(t,
		"--manifest-url", manifestURL,
		"--pubkey-base64", base64.StdEncoding.EncodeToString(pub),
		"--plugin-id", "openclaw",
		"--target", target,
		"--allow-insecure-manifest-url",
		"--allow-insecure-binary-url",
	)
	if code == 0 {
		t.Fatalf("expected non-zero exit on truncated stream, got 0; stderr=%q", stderr)
	}
	// EITHER binary_fetch_failed (server cut) OR sha256_mismatch (server
	// sent partial bytes that happened to read as EOF cleanly). Both
	// outcomes leave the target untouched.
	if !strings.Contains(stderr, "binary_fetch_failed") && !strings.Contains(stderr, "sha256_mismatch") {
		t.Fatalf("expected binary_fetch_failed or sha256_mismatch in stderr, got %q", stderr)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if !bytes.Equal(got, priorContent) {
		t.Fatalf("target was replaced! got %q want %q", got, priorContent)
	}
	// Also assert no leftover .partial files.
	entries, _ := os.ReadDir(targetDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".partial") {
			t.Fatalf("leftover tempfile %q after failed download", e.Name())
		}
	}
}

// Race-clean helper — exercised by `go test -race`; runs runCLI from
// several goroutines against an independent fixture each. Guards against
// process-global state in install-butler subprocess invocations.
func TestInstallButler_HappyPath_Concurrent(t *testing.T) {
	t.Parallel()
	const N = 4
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			pub, priv := genKey(t)
			binary := []byte(fmt.Sprintf("payload-%d", i))
			binaryURL := startBinaryServer(t, binary)
			fixture := buildFixture(t, priv, binaryURL, binary)
			manifestURL := startManifestServer(t, fixture.payload)
			targetDir := t.TempDir()
			target := filepath.Join(targetDir, "openclaw")
			_, stderr, code := runCLI(t,
				"--manifest-url", manifestURL,
				"--pubkey-base64", base64.StdEncoding.EncodeToString(pub),
				"--plugin-id", "openclaw",
				"--target", target,
				"--allow-insecure-manifest-url",
				"--allow-insecure-binary-url",
			)
			if code != 0 {
				t.Errorf("goroutine %d: exit=%d stderr=%q", i, code, stderr)
				return
			}
			got, err := os.ReadFile(target)
			if err != nil {
				t.Errorf("goroutine %d: read target: %v", i, err)
				return
			}
			if !bytes.Equal(got, binary) {
				t.Errorf("goroutine %d: target mismatch", i)
			}
		}(i)
	}
	wg.Wait()
}

// Cross-check canonical bytes byte-identical with the four-field "|"
// separator — guards against silent drift from the server's
// EntryCanonicalBytes in manifest_signing.go.
func TestEntryCanonicalBytes_ByteIdentical(t *testing.T) {
	t.Parallel()
	e := pluginManifestEntry{
		ID:        "openclaw",
		Version:   "1.0.0",
		BinaryURL: "https://cdn.borgee.io/plugins/openclaw-1.0.0-linux-x64",
		SHA256:    "0000000000000000000000000000000000000000000000000000000000000000",
		// Platforms intentionally not in canonical bytes — proves exclusion.
		Platforms: []string{"linux-x64"},
	}
	got := string(entryCanonicalBytes(e))
	want := "openclaw|1.0.0|https://cdn.borgee.io/plugins/openclaw-1.0.0-linux-x64|0000000000000000000000000000000000000000000000000000000000000000"
	if got != want {
		t.Fatalf("canonical drift:\n got=%q\nwant=%q", got, want)
	}
}

// Unused import insurance — make sure io stays in use even after future
// editing trims happen.
var _ = io.Discard
