//go:build linux || darwin

// Package main — install-butler: HB-1 short-lived signed-manifest binary
// installer (#996).
//
// One-shot CLI. Lifecycle:
//
//	HTTPS GET manifest → parse → locate entry by --plugin-id
//	  → ed25519 verify entry signature (canonical: ID|Version|BinaryURL|SHA256,
//	    byte-identical with server-go/internal/api/manifest_signing.go::
//	    EntryCanonicalBytes — see comment on entryCanonicalBytes below for the
//	    one-line algorithm)
//	  → HTTPS GET BinaryURL → stream to tempfile on same filesystem as --target
//	  → streamed SHA256 → compare to entry.SHA256 (reject sha256_mismatch)
//	  → atomic os.Rename(tempfile, --target) → chmod 0755
//	  → chown borgee-helper:borgee-helper when running as root
//	  → exit 0 (or 1 on any of the seven failure modes)
//
// DO NOT turn install-butler into a long-lived daemon. The whole point of
// this binary is "one-shot + drop privilege by exiting". If you need
// background scheduling (e.g. nightly update polling), wrap it from a
// systemd timer or launchd `StartCalendarInterval`, NOT from this binary.
// Lingering as root after the write would defeat the install-butler /
// host-bridge daemon-split that the host-bridge blueprint requires.
//
// 蓝图锚: docs/blueprint/current/host-bridge.md §1.2 + §4.5 + manifest-signing
// operational contract docs/current/host-bridge/manifest-signing.md.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Reason strings — install-butler structured stderr error codes. Each maps
// to one of the seven acceptance-criteria failure modes (#996).
const (
	reasonManifestFetchFailed = "manifest_fetch_failed"
	reasonManifestParseFailed = "manifest_parse_failed"
	reasonPluginNotFound      = "plugin_not_found"
	reasonSignatureInvalid    = "signature_invalid"
	reasonBinaryFetchFailed   = "binary_fetch_failed"
	reasonSHA256Mismatch      = "sha256_mismatch"
	reasonWriteFailed         = "write_failed"
)

// pluginManifestEntry mirrors server-go/internal/api/host_manifest.go
// `PluginManifestEntry` field-by-field (`json:"…"` tags byte-identical).
// Kept as a local struct because borgee-helper is a separate Go module and
// must not import server-go internal packages.
type pluginManifestEntry struct {
	ID        string   `json:"id"`
	Version   string   `json:"version"`
	BinaryURL string   `json:"binary_url"`
	SHA256    string   `json:"sha256"`
	Signature string   `json:"signature"`
	Platforms []string `json:"platforms"`
}

// pluginManifestPayload mirrors server-go top-level shape.
type pluginManifestPayload struct {
	ManifestVersion int                   `json:"manifest_version"`
	IssuedAt        int64                 `json:"issued_at"`
	ExpiresAt       int64                 `json:"expires_at"`
	Signature       string                `json:"signature"`
	Plugins         []pluginManifestEntry `json:"plugins"`
}

// entrySigSeparator — canonical-form field separator. Single ASCII "|"
// (0x7C). Byte-identical with
// server-go/internal/api/manifest_signing.go::entrySigSeparator.
const entrySigSeparator = "|"

// entryCanonicalBytes returns the byte sequence ed25519-signed for a single
// manifest entry. MUST stay byte-identical with
// server-go/internal/api/manifest_signing.go::EntryCanonicalBytes — any drift
// silently breaks verification. Platforms field is intentionally excluded
// (client-side metadata, not security relevant).
//
// Canonical form: ID + "|" + Version + "|" + BinaryURL + "|" + SHA256
func entryCanonicalBytes(e pluginManifestEntry) []byte {
	return []byte(e.ID + entrySigSeparator + e.Version + entrySigSeparator + e.BinaryURL + entrySigSeparator + e.SHA256)
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		// run() already wrote a `<reason>: <detail>` line to stderr; exit 1.
		os.Exit(1)
	}
}

// run is the testable entrypoint. Returns a non-nil error when install-butler
// should exit 1. Side-effects (writes / chown) only happen on the success
// path or — for the tempfile — get cleaned up in failure paths.
func run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("install-butler", flag.ContinueOnError)
	fs.SetOutput(stderr)
	manifestURL := fs.String("manifest-url", "", "HTTPS URL of the plugin manifest endpoint (required)")
	pubkeyB64 := fs.String("pubkey-base64", "", "Base64-encoded ed25519 public key (32 bytes after decode) matching BORGEE_MANIFEST_SIGNING_KEY on server (required)")
	pluginID := fs.String("plugin-id", "", "Plugin id to install (must match an entry in the manifest, e.g. openclaw) (required)")
	target := fs.String("target", "", "Absolute path to write the verified binary to (e.g. /usr/local/lib/borgee/openclaw) (required)")
	dryRun := fs.Bool("dry-run", false, "Verify manifest + signature + sha256 without writing the target file")
	httpTimeout := fs.Duration("http-timeout", 60*time.Second, "Per-request HTTP timeout (default 60s)")
	helperUser := fs.String("helper-user", "borgee-helper", "OS user to chown the target to when running as root (default borgee-helper)")
	helperGroup := fs.String("helper-group", "", "OS group to chown the target to (defaults to --helper-user)")
	allowInsecureManifest := fs.Bool("allow-insecure-manifest-url", false, "Allow http:// manifest-url (test only; production must be https://)")
	allowInsecureBinary := fs.Bool("allow-insecure-binary-url", false, "Allow http:// binary-url (test only; production must be https://)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *manifestURL == "" {
		return writeReason(stderr, reasonManifestFetchFailed, "--manifest-url is required")
	}
	if *pubkeyB64 == "" {
		return writeReason(stderr, reasonSignatureInvalid, "--pubkey-base64 is required")
	}
	if *pluginID == "" {
		return writeReason(stderr, reasonPluginNotFound, "--plugin-id is required")
	}
	if *target == "" {
		return writeReason(stderr, reasonWriteFailed, "--target is required")
	}
	if !filepath.IsAbs(*target) {
		return writeReason(stderr, reasonWriteFailed, fmt.Sprintf("--target must be absolute path, got %q", *target))
	}
	if !*allowInsecureManifest && !strings.HasPrefix(strings.ToLower(*manifestURL), "https://") {
		return writeReason(stderr, reasonManifestFetchFailed, "--manifest-url must be https:// (use --allow-insecure-manifest-url for local testing)")
	}

	pubKey, err := decodePubKey(*pubkeyB64)
	if err != nil {
		return writeReason(stderr, reasonSignatureInvalid, fmt.Sprintf("--pubkey-base64 decode: %v", err))
	}

	ctx := context.Background()
	client := &http.Client{Timeout: *httpTimeout}

	// 1+2. Fetch + parse manifest.
	payload, err := fetchManifest(ctx, client, *manifestURL)
	if err != nil {
		// fetchManifest returns a *reasonErr distinguishing fetch vs parse.
		return writeReasonErr(stderr, err)
	}

	// 3. Locate entry by --plugin-id.
	var entry pluginManifestEntry
	found := false
	for _, e := range payload.Plugins {
		if e.ID == *pluginID {
			entry = e
			found = true
			break
		}
	}
	if !found {
		return writeReason(stderr, reasonPluginNotFound, fmt.Sprintf("plugin id %q not in manifest (%d entries)", *pluginID, len(payload.Plugins)))
	}

	// 4. ed25519 verify entry signature.
	if entry.Signature == "" {
		return writeReason(stderr, reasonSignatureInvalid, fmt.Sprintf("entry %q has empty signature (production server must set BORGEE_MANIFEST_SIGNING_KEY)", entry.ID))
	}
	sig, err := base64.StdEncoding.DecodeString(entry.Signature)
	if err != nil {
		return writeReason(stderr, reasonSignatureInvalid, fmt.Sprintf("entry %q signature base64 decode: %v", entry.ID, err))
	}
	if !ed25519.Verify(pubKey, entryCanonicalBytes(entry), sig) {
		return writeReason(stderr, reasonSignatureInvalid, fmt.Sprintf("entry %q signature verification failed", entry.ID))
	}

	// Validate sha256 hex shape before downloading so we fail fast on
	// obviously broken manifests.
	wantSHA := strings.ToLower(strings.TrimSpace(entry.SHA256))
	if len(wantSHA) != 64 {
		return writeReason(stderr, reasonSHA256Mismatch, fmt.Sprintf("entry %q SHA256 must be 64 hex chars, got %d", entry.ID, len(wantSHA)))
	}
	if _, err := hex.DecodeString(wantSHA); err != nil {
		return writeReason(stderr, reasonSHA256Mismatch, fmt.Sprintf("entry %q SHA256 hex decode: %v", entry.ID, err))
	}

	if !*allowInsecureBinary && !strings.HasPrefix(strings.ToLower(entry.BinaryURL), "https://") {
		return writeReason(stderr, reasonBinaryFetchFailed, fmt.Sprintf("entry %q binary_url must be https:// (use --allow-insecure-binary-url for local testing), got %q", entry.ID, entry.BinaryURL))
	}

	// 5. Stream binary to a tempfile on the SAME filesystem as --target so
	// the eventual rename is atomic. SHA256 is computed in-stream so we
	// never buffer the whole binary in memory.
	targetDir := filepath.Dir(*target)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return writeReason(stderr, reasonWriteFailed, fmt.Sprintf("mkdir %q: %v", targetDir, err))
	}
	tmp, err := os.CreateTemp(targetDir, ".install-butler-*.partial")
	if err != nil {
		return writeReason(stderr, reasonWriteFailed, fmt.Sprintf("create tempfile in %q: %v", targetDir, err))
	}
	tmpPath := tmp.Name()
	// In any failure path below we must remove the tempfile so a partial
	// download never lingers next to --target.
	cleanupTmp := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}

	gotSHA, downloadErr := downloadAndHash(ctx, client, entry.BinaryURL, tmp)
	if downloadErr != nil {
		cleanupTmp()
		return writeReasonErr(stderr, downloadErr)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return writeReason(stderr, reasonWriteFailed, fmt.Sprintf("close tempfile: %v", err))
	}

	// 6. Compare SHA256. NEVER replace --target if mismatch (acceptance TB-3).
	if gotSHA != wantSHA {
		_ = os.Remove(tmpPath)
		return writeReason(stderr, reasonSHA256Mismatch, fmt.Sprintf("entry %q sha256 mismatch: want=%s got=%s", entry.ID, wantSHA, gotSHA))
	}

	// 7. Dry-run short-circuit: keep --target untouched and exit 0.
	if *dryRun {
		_ = os.Remove(tmpPath)
		fmt.Fprintf(stdout, "would write to %s, verified plan: plugin=%s version=%s sha256=%s\n",
			*target, entry.ID, entry.Version, wantSHA)
		return nil
	}

	// 8. Atomic rename. chmod first on the tempfile so the target appears
	// with final perms in one shot (rename is atomic on POSIX; rename never
	// half-replaces).
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		_ = os.Remove(tmpPath)
		return writeReason(stderr, reasonWriteFailed, fmt.Sprintf("chmod tempfile: %v", err))
	}
	if err := os.Rename(tmpPath, *target); err != nil {
		_ = os.Remove(tmpPath)
		return writeReason(stderr, reasonWriteFailed, fmt.Sprintf("rename %q -> %q: %v", tmpPath, *target, err))
	}

	// 9. chown when running as root. Non-fatal warning when the helper user
	// is missing (same posture as borgee-helper-claim) so a single-machine
	// test rig without the borgee-helper user can still install.
	if os.Geteuid() == 0 {
		group := *helperGroup
		if group == "" {
			group = *helperUser
		}
		if cerr := chownTarget(*target, *helperUser, group); cerr != nil {
			fmt.Fprintf(stderr, "install-butler: warn: chown %q to %s:%s failed: %v\n", *target, *helperUser, group, cerr)
		}
	}

	// 10. Drop privilege by exiting. install-butler is one-shot; there is
	// no long-lived process here. main() returns → kernel reaps.
	fmt.Fprintf(stdout, "installed plugin %s version %s to %s (sha256=%s)\n",
		entry.ID, entry.Version, *target, wantSHA)
	return nil
}

// reasonErr wraps a reason code with a human-readable detail so the caller
// (run) can format `<reason>: <detail>` once and exit 1.
type reasonErr struct {
	reason string
	detail string
}

func (e *reasonErr) Error() string {
	return e.reason + ": " + e.detail
}

func newReasonErr(reason, detail string) error {
	return &reasonErr{reason: reason, detail: detail}
}

func writeReasonErr(stderr io.Writer, err error) error {
	var re *reasonErr
	if errors.As(err, &re) {
		fmt.Fprintf(stderr, "install-butler: %s: %s\n", re.reason, re.detail)
		return err
	}
	// Generic write_failed bucket — shouldn't be reachable since run() funnels
	// all errors through writeReason / writeReasonErr, but kept as safety net.
	fmt.Fprintf(stderr, "install-butler: %s: %v\n", reasonWriteFailed, err)
	return err
}

func writeReason(stderr io.Writer, reason, detail string) error {
	err := newReasonErr(reason, detail)
	fmt.Fprintf(stderr, "install-butler: %s: %s\n", reason, detail)
	return err
}

func decodePubKey(b64 string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return nil, err
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("pubkey length: got %d, want %d", len(raw), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(raw), nil
}

func fetchManifest(ctx context.Context, client *http.Client, url string) (*pluginManifestPayload, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, newReasonErr(reasonManifestFetchFailed, fmt.Sprintf("build request: %v", err))
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, newReasonErr(reasonManifestFetchFailed, fmt.Sprintf("GET %s: %v", url, err))
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, newReasonErr(reasonManifestFetchFailed,
			fmt.Sprintf("GET %s: HTTP %d: %s", url, resp.StatusCode, strings.TrimSpace(string(body))))
	}
	// Cap manifest size at 1 MiB — manifests are small JSON blobs; a server
	// returning megabytes is a misconfiguration we should reject loudly.
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, newReasonErr(reasonManifestFetchFailed, fmt.Sprintf("read body: %v", err))
	}
	var payload pluginManifestPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, newReasonErr(reasonManifestParseFailed, fmt.Sprintf("decode JSON: %v", err))
	}
	return &payload, nil
}

// downloadAndHash streams the response body into w while computing SHA256.
// Returns the lowercase hex sha256 of the streamed bytes. Caller MUST close
// w (we don't close so the caller controls the tempfile lifecycle).
func downloadAndHash(ctx context.Context, client *http.Client, url string, w io.Writer) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", newReasonErr(reasonBinaryFetchFailed, fmt.Sprintf("build request: %v", err))
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", newReasonErr(reasonBinaryFetchFailed, fmt.Sprintf("GET %s: %v", url, err))
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", newReasonErr(reasonBinaryFetchFailed,
			fmt.Sprintf("GET %s: HTTP %d: %s", url, resp.StatusCode, strings.TrimSpace(string(body))))
	}
	h := sha256.New()
	mw := io.MultiWriter(w, h)
	if _, err := io.Copy(mw, resp.Body); err != nil {
		// EOF mid-stream from server is a fetch failure — the tempfile is
		// partial and the caller will Remove() it. Critical: --target stays
		// untouched (TB-7 acceptance).
		return "", newReasonErr(reasonBinaryFetchFailed, fmt.Sprintf("stream body: %v", err))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func chownTarget(path, username, groupname string) error {
	u, err := user.Lookup(username)
	if err != nil {
		return err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return err
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return err
	}
	if groupname != "" {
		if g, gErr := user.LookupGroup(groupname); gErr == nil {
			if parsed, pErr := strconv.Atoi(g.Gid); pErr == nil {
				gid = parsed
			}
		}
	}
	return os.Chown(path, uid, gid)
}
