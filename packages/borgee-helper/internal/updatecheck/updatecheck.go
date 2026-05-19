//go:build linux || darwin

// Package updatecheck implements the helper-side update detection loop
// (#999). Blueprint锚: docs/blueprint/current/host-bridge.md §1.3
// "更新策略: 分类, 不自动" — 自动更新仍是反模式. The loop:
//
//  1. reads /var/lib/borgee-helper/installed-versions.json (written by
//     install-butler after each successful install)
//  2. POSTs the snapshot to /api/v1/helper/enrollments/{id}/installed-versions
//  3. server computes drift vs the signed manifest (with class
//     normalization) and returns the drift list
//  4. helper logs one event per drift entry; class drives severity:
//       - "security" → log.Printf prominent (operator must surface to user)
//       - "feature"  → log.Printf informational (settings panel only)
//
// Apply is NOT in this package — per blueprint, application happens only on
// explicit user confirmation. The dispatcher (#1001+#1002) will pick that
// up via a future plugin.update typed job.
package updatecheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// DefaultInterval is the steady-state cadence between update checks. Chosen
// at 15 minutes — drift is not time-critical and we do NOT want to add load
// to the manifest endpoint by colliding with the 60s heartbeat tick. Tests
// inject a much smaller value via Checker.Interval.
const DefaultInterval = 15 * time.Minute

// DefaultBackoffBase / DefaultBackoffCap are the retry curve on POST failure.
// Mirrors heartbeat.go's pattern so operator-side log analysis can use the
// same shape.
const (
	DefaultBackoffBase = 30 * time.Second
	DefaultBackoffCap  = 5 * time.Minute
)

// DefaultInstalledVersionsPath is the canonical on-disk location install-
// butler writes after each successful install. Override via Checker.
// InstalledVersionsPath for tests / non-default deploys.
const DefaultInstalledVersionsPath = "/var/lib/borgee-helper/installed-versions.json"

// installedVersionsFile mirrors the install-butler write shape byte-for-byte.
// Kept duplicated here (vs imported) because borgee-helper internal packages
// must not import from cmd/. Field tags identical.
type installedVersionsFile struct {
	Plugins map[string]installedVersionRecord `json:"plugins"`
}

type installedVersionRecord struct {
	Version     string `json:"version"`
	InstalledAt int64  `json:"installed_at"`
	SHA256      string `json:"sha256"`
}

// postRequest mirrors api.installedVersionsRequest (server side). Field
// tags byte-identical so the contract is wire-stable.
type postRequest struct {
	HelperDeviceID string                `json:"helper_device_id"`
	Installed      []postInstalledEntry  `json:"installed"`
}

type postInstalledEntry struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

// postResponse mirrors the server-side response shape — we only consume the
// drift list. enrollment / last_update_check_at are present too but the
// helper doesn't need them for its log path.
type postResponse struct {
	UpdatesAvailable []DriftEntry `json:"updates_available"`
}

// DriftEntry is one update available reported by the server (after class
// normalization). Exported so tests + main.go can construct it.
type DriftEntry struct {
	PluginID        string `json:"plugin_id"`
	CurrentVersion  string `json:"current_version"`
	ManifestVersion string `json:"manifest_version"`
	Class           string `json:"class"`
}

// Class constants — kept in sync with api.UpdateClass* literals. Helper
// reads server-normalized values; this is purely a defensive default if
// the server ever returns an unknown class.
const (
	ClassSecurity = "security"
	ClassFeature  = "feature"
)

// Checker runs the periodic update-detection POST loop. Zero-value safe
// only when Client + ServerOrigin + EnrollmentID + HelperDeviceID +
// Credential are set; tests / main.go fill these in.
type Checker struct {
	Client                *http.Client
	ServerOrigin          string
	EnrollmentID          string
	HelperDeviceID        string
	Credential            string
	InstalledVersionsPath string // "" → DefaultInstalledVersionsPath
	Interval              time.Duration
	BackoffBase           time.Duration
	BackoffCap            time.Duration

	// Logger lets tests capture log lines. nil → standard log package.
	// Production wires to log.Printf so security drift surfaces in the
	// daemon journal where operators / monitoring read it.
	Logger func(format string, v ...any)

	// Now is injected for deterministic tests. nil → time.Now.
	Now func() time.Time
}

func (c *Checker) logf(format string, v ...any) {
	if c.Logger != nil {
		c.Logger(format, v...)
		return
	}
	log.Printf(format, v...)
}

func (c *Checker) httpClient() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return http.DefaultClient
}

func (c *Checker) installedVersionsPath() string {
	if strings.TrimSpace(c.InstalledVersionsPath) != "" {
		return c.InstalledVersionsPath
	}
	return DefaultInstalledVersionsPath
}

func (c *Checker) interval() time.Duration {
	if c.Interval > 0 {
		return c.Interval
	}
	return DefaultInterval
}

func (c *Checker) backoffBase() time.Duration {
	if c.BackoffBase > 0 {
		return c.BackoffBase
	}
	return DefaultBackoffBase
}

func (c *Checker) backoffCap() time.Duration {
	if c.BackoffCap > 0 {
		return c.BackoffCap
	}
	return DefaultBackoffCap
}

// Run blocks until ctx is cancelled. Fires the first check immediately,
// then every Interval. Failures back off; success resets. Mirrors
// heartbeat.go's shape so the production operator sees a consistent
// retry/log pattern across helper loops.
func (c *Checker) Run(ctx context.Context) error {
	if strings.TrimSpace(c.ServerOrigin) == "" || strings.TrimSpace(c.EnrollmentID) == "" || strings.TrimSpace(c.Credential) == "" || strings.TrimSpace(c.HelperDeviceID) == "" {
		return fmt.Errorf("updatecheck.Checker requires server origin, enrollment id, credential, and helper device id")
	}
	backoff := c.backoffBase()
	for {
		err := c.fire(ctx)
		if ctx.Err() != nil {
			return nil
		}
		var wait time.Duration
		if err != nil {
			c.logf("borgee-helper: update-check failed: %v (next attempt in %s)", err, backoff)
			wait = backoff
			backoff *= 2
			if backoff > c.backoffCap() {
				backoff = c.backoffCap()
			}
		} else {
			backoff = c.backoffBase()
			wait = c.interval()
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(wait):
		}
	}
}

// fire executes one check: read snapshot, POST, log drift. Returns nil on
// HTTP 2xx (regardless of drift list content). Caller decides retry policy.
func (c *Checker) fire(ctx context.Context) error {
	installed, err := readInstalledVersions(c.installedVersionsPath())
	if err != nil {
		// Missing/unreadable file is NOT a hard error — fresh pre-install
		// daemons have no snapshot yet. Treat as "no installed plugins"
		// and let the server compute drift vs the manifest (which yields
		// "every manifest entry is an available install opportunity").
		c.logf("borgee-helper: update-check: snapshot %q unavailable (%v); posting empty installed list", c.installedVersionsPath(), err)
		installed = nil
	}

	entries := make([]postInstalledEntry, 0, len(installed))
	for id, rec := range installed {
		entries = append(entries, postInstalledEntry{ID: id, Version: rec.Version})
	}

	body, err := json.Marshal(postRequest{
		HelperDeviceID: c.HelperDeviceID,
		Installed:      entries,
	})
	if err != nil {
		return err
	}
	url := strings.TrimRight(c.ServerOrigin, "/") + "/api/v1/helper/enrollments/" + c.EnrollmentID + "/installed-versions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Credential)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var parsed postResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		// Server returned 2xx but malformed body — log + treat as success
		// so we don't backoff a transient encoding glitch.
		c.logf("borgee-helper: update-check: response decode failed (%v); treating as success", err)
		return nil
	}

	logDrift(c.logf, parsed.UpdatesAvailable)
	return nil
}

// logDrift emits one log line per drift entry with class-driven severity.
// Operators / monitoring filter on "update-available.security" to surface
// security drift to end users (desktop notification etc); feature drift is
// settings-panel only per blueprint §1.3.
func logDrift(logf func(format string, v ...any), drift []DriftEntry) {
	for _, d := range drift {
		class := d.Class
		if class != ClassSecurity {
			class = ClassFeature
		}
		// Format: structured key=value so journalctl / log scrapers can
		// pick out "update-available.security" as a high-priority signal.
		// current_version="" means "not yet installed" — operator sees a
		// fresh-install opportunity rather than an upgrade.
		current := d.CurrentVersion
		if current == "" {
			current = "(not installed)"
		}
		logf("borgee-helper: update-available.%s plugin=%s current=%s manifest=%s",
			class, d.PluginID, current, d.ManifestVersion)
	}
}

// readInstalledVersions parses the install-butler-written snapshot file.
// Returns (nil, nil) when the file is empty (not yet populated). Returns
// (nil, err) only when the file is present but unreadable.
func readInstalledVersions(path string) (map[string]installedVersionRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	var parsed installedVersionsFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}
	return parsed.Plugins, nil
}
