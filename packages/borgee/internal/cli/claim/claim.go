//go:build linux || darwin

// Package claim — borgee claim subcommand (closes #968 reboot/crash chain end-to-end).
//
// Operator workflow:
//   1. Operator generates an enrollment on the web UI; the server returns an
//      enrollment ID + one-time enrollment_secret (15min TTL per
//      server-go/internal/store/helper_enrollment_queries.go).
//   2. Operator runs this CLI on the target machine, typically as root via
//      sudo, supplying the ID + secret + server origin:
//        sudo borgee claim \
//          --enrollment-id <id> \
//          --enrollment-secret <secret> \
//          --server-origin https://app.borgee.io
//   3. CLI derives a stable helper_device_id (machine-id / IOPlatformUUID),
//      POSTs /api/v1/helper/enrollments/{id}/claim with body
//      {"enrollment_secret":..., "helper_device_id":...}, receives
//      helper_credential, and persists the three files the daemon needs into
//      the helper's StateDirectory:
//        - --credential-file       (0600, owned by helper)
//        - --enrollment-id-file    (0644)
//        - --device-id-file        (0644)
//   4. On the next start (immediate via `systemctl restart` or on reboot),
//      the daemon reads those files and spawns its heartbeat producer.
//
// The CLI is intentionally local-only: enrollment_secret is too sensitive to
// bake into the install bundle, and a single one-shot subcommand the operator
// runs once keeps the install asset surface smaller.
package claim

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	defaultLinuxStateDir = "/var/lib/borgee"
	defaultMacStateDir   = "/Library/Application Support/Borgee/Helper"

	defaultHelperUserLinux = "borgee"
	defaultHelperUserMac   = "_borgee"

	httpTimeout = 30 * time.Second
)

// Run is the entry for `borgee claim`. The dispatcher in cmd/borgee passes
// the remaining argv + stdio.
func Run(args []string, stdout, stderr io.Writer) error {
	if err := runCLI(args, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, "borgee claim:", err)
		return err
	}
	return nil
}

func runCLI(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("borgee claim", flag.ContinueOnError)
	fs.SetOutput(stderr)
	enrollmentID := fs.String("enrollment-id", "", "Helper enrollment id from web UI (required)")
	enrollmentSecret := fs.String("enrollment-secret", "", "One-time enrollment secret from web UI (15min TTL) (required)")
	serverOrigin := fs.String("server-origin", "https://app.borgee.io", "Borgee API server origin (e.g. https://app.borgee.io)")

	defaultStateDir := defaultLinuxStateDir
	defaultHelperUser := defaultHelperUserLinux
	if runtime.GOOS == "darwin" {
		defaultStateDir = defaultMacStateDir
		defaultHelperUser = defaultHelperUserMac
	}
	// #1017 bug 1 fix: setup.go creates `<state>/credential/` as a DIRECTORY
	// and the systemd unit + launchd plist read
	// `<state>/credential/{credential,enrollment-id,device-id}`. Earlier
	// defaults wrote the three files directly under `<state>/`, which
	// collided with the directory (claim refused to overwrite the dir as a
	// file, or wrote next to it where the daemon never reads). Align the
	// defaults with the daemon's actual file paths so `borgee install`
	// (setup → claim → start) lands a working credential out of the box.
	credentialSubdir := filepath.Join(defaultStateDir, "credential")
	credentialFile := fs.String("credential-file", filepath.Join(credentialSubdir, "credential"), "Path to write helper credential (0600)")
	enrollmentIDFile := fs.String("enrollment-id-file", filepath.Join(credentialSubdir, "enrollment-id"), "Path to write enrollment id")
	deviceIDFile := fs.String("device-id-file", filepath.Join(credentialSubdir, "device-id"), "Path to read/write helper device id")
	helperUser := fs.String("helper-user", defaultHelperUser, "OS user to chown credential file to (when running as root)")
	helperGroup := fs.String("helper-group", "", "OS group to chown credential file to (defaults to helper-user)")
	allowInsecure := fs.Bool("allow-insecure-server-origin", false, "Allow http:// or non-public server-origin (test only)")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *enrollmentID == "" {
		return errors.New("--enrollment-id is required")
	}
	if *enrollmentSecret == "" {
		return errors.New("--enrollment-secret is required")
	}
	origin, err := normalizeServerOrigin(*serverOrigin, *allowInsecure)
	if err != nil {
		return fmt.Errorf("--server-origin: %w", err)
	}

	deviceID, err := resolveDeviceID(*deviceIDFile)
	if err != nil {
		return fmt.Errorf("resolve device id: %w", err)
	}

	// Ensure parent dirs exist (idempotent — setup typically pre-creates them).
	// When running as root we also chown to the helper user so the daemon
	// (running as `borgee`) can stat-and-read the files inside.
	for _, p := range []string{*credentialFile, *enrollmentIDFile, *deviceIDFile} {
		parent := filepath.Dir(p)
		if err := os.MkdirAll(parent, 0o750); err != nil {
			return fmt.Errorf("mkdir parent for %q: %w", p, err)
		}
		if os.Geteuid() == 0 {
			group := *helperGroup
			if group == "" {
				group = *helperUser
			}
			if err := chownToUser(parent, *helperUser, group); err != nil {
				// Non-fatal: parent may already exist from setup; chown best
				// effort. Daemon-side read still works as long as perms
				// allow.
				_ = err
			}
		}
	}

	credential, err := postClaim(context.Background(), origin, *enrollmentID, *enrollmentSecret, deviceID)
	if err != nil {
		return err
	}

	// Persist enrollment-id and device-id first; if anything later fails we
	// still leave behind state that lets the operator re-run cleanly.
	if err := writeFileAtomic(*enrollmentIDFile, []byte(*enrollmentID+"\n"), 0o644); err != nil {
		return fmt.Errorf("write enrollment-id-file: %w", err)
	}
	if err := writeFileAtomic(*deviceIDFile, []byte(deviceID+"\n"), 0o644); err != nil {
		return fmt.Errorf("write device-id-file: %w", err)
	}

	// Credential gets the tightest perms + chown to the helper user/group when
	// we have permission (root). If chown fails because we are running as a
	// non-root user (e.g. inside a test), we leave the file with the current
	// owner — perms are still 0600 so it is not world-readable.
	if err := writeFileAtomic(*credentialFile, []byte(credential), 0o600); err != nil {
		return fmt.Errorf("write credential-file: %w", err)
	}
	if os.Geteuid() == 0 {
		group := *helperGroup
		if group == "" {
			group = *helperUser
		}
		if err := chownToUser(*credentialFile, *helperUser, group); err != nil {
			// chown failure is non-fatal but reported: on a fresh install
			// where `borgee setup` has not yet created the user, the
			// operator may need to re-run after setup completes.
			fmt.Fprintf(stderr, "borgee claim: warn: chown %q to %s:%s failed: %v\n", *credentialFile, *helperUser, group, err)
		}
	}

	fmt.Fprintf(stdout, "claimed enrollment %s, credential written to %s\n", *enrollmentID, *credentialFile)
	return nil
}

// resolveDeviceID returns a stable identifier for the helper.
//
// Priority:
//   1. Existing content of --device-id-file (so re-claim preserves the id).
//   2. Linux: /etc/machine-id (systemd-managed, regenerated on image clone).
//   3. macOS: IOPlatformUUID from ioreg.
//   4. Fallback: random UUID v4 (only persisted via writeFileAtomic later).
func resolveDeviceID(path string) (string, error) {
	if path != "" {
		if b, err := os.ReadFile(path); err == nil {
			if id := strings.TrimSpace(string(b)); id != "" {
				return id, nil
			}
		}
	}
	if runtime.GOOS == "linux" {
		if b, err := os.ReadFile("/etc/machine-id"); err == nil {
			if id := strings.TrimSpace(string(b)); id != "" {
				return id, nil
			}
		}
	}
	if runtime.GOOS == "darwin" {
		if id, err := readIOPlatformUUID(); err == nil && id != "" {
			return id, nil
		}
	}
	return generateUUIDv4()
}

func readIOPlatformUUID() (string, error) {
	out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		// Looking for: "IOPlatformUUID" = "<UUID>"
		if !strings.Contains(line, "IOPlatformUUID") {
			continue
		}
		idx := strings.Index(line, "= \"")
		if idx < 0 {
			continue
		}
		rest := line[idx+3:]
		end := strings.Index(rest, "\"")
		if end < 0 {
			continue
		}
		return rest[:end], nil
	}
	return "", errors.New("IOPlatformUUID not found in ioreg output")
}

func generateUUIDv4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16])), nil
}

func postClaim(ctx context.Context, origin, enrollmentID, secret, deviceID string) (string, error) {
	payload, err := json.Marshal(map[string]string{
		"enrollment_secret": secret,
		"helper_device_id":  deviceID,
	})
	if err != nil {
		return "", err
	}
	url := strings.TrimRight(origin, "/") + "/api/v1/helper/enrollments/" + enrollmentID + "/claim"
	ctx, cancel := context.WithTimeout(ctx, httpTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("POST claim: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		// Trim very large server bodies to keep the operator-facing error legible.
		snippet := string(body)
		if len(snippet) > 512 {
			snippet = snippet[:512] + "..."
		}
		return "", fmt.Errorf("claim failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(snippet))
	}
	var parsed struct {
		HelperCredential string `json:"helper_credential"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("decode claim response: %w", err)
	}
	if strings.TrimSpace(parsed.HelperCredential) == "" {
		return "", errors.New("claim response missing helper_credential")
	}
	return parsed.HelperCredential, nil
}

// writeFileAtomic writes data to path via a tmpfile + rename, applying mode.
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Chmod(mode); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func chownToUser(path, username, groupname string) error {
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

// normalizeServerOrigin keeps the producer side's contract: HTTPS required
// except when --allow-insecure-server-origin is explicitly opted in (used by
// the e2e test, never by production).
func normalizeServerOrigin(raw string, allowInsecure bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("required")
	}
	if !allowInsecure && !strings.HasPrefix(strings.ToLower(raw), "https://") {
		return "", errors.New("https is required (re-run with --allow-insecure-server-origin only for local testing)")
	}
	return strings.TrimRight(raw, "/"), nil
}
