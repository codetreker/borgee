package outbound

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// HeartbeatInterval is the steady-state cadence at which the daemon posts
// status to the API server. The server's serializer flips an enrollment to
// `offline` after 5 minutes without a heartbeat (see
// helper_enrollments.go::serializeWithConfigure), so 60s gives a 5x safety
// margin against transient network blips. Not a flag — see #968 design notes.
const HeartbeatInterval = 60 * time.Second

// defaultBackoffBase / defaultBackoffCap are the production retry curve.
// Tests override BackoffBase + BackoffCap for fast assertions.
const (
	defaultBackoffBase = 5 * time.Second
	defaultBackoffCap  = 60 * time.Second
)

// Heartbeater periodically POSTs to
// /api/v1/helper/enrollments/{id}/status so the server's freshness window
// keeps the enrollment in the `connected` state. It is safe to fail: network
// errors, 5xx, and 4xx (revoked / unauthorized) all retry with exponential
// backoff without ever panicking the daemon.
//
// #968: this is the missing producer that closes the reboot/crash reconnect
// chain end-to-end. systemd / launchd brings the daemon up post-reboot;
// Heartbeater.Run fires the first POST within 100ms; the server records
// LastSeenAt; the serializer flips status to `connected`.
type Heartbeater struct {
	Client         *http.Client
	ServerOrigin   string // e.g. https://app.borgee.io (no trailing slash)
	EnrollmentID   string
	HelperDeviceID string
	Credential     string
	Interval       time.Duration // 0 → HeartbeatInterval
	BackoffBase    time.Duration // 0 → defaultBackoffBase
	BackoffCap     time.Duration // 0 → defaultBackoffCap

	// Logger lets tests capture log lines. nil → standard log package.
	Logger func(format string, v ...any)
}

func (h *Heartbeater) logf(format string, v ...any) {
	if h.Logger != nil {
		h.Logger(format, v...)
		return
	}
	log.Printf(format, v...)
}

func (h *Heartbeater) interval() time.Duration {
	if h.Interval > 0 {
		return h.Interval
	}
	return HeartbeatInterval
}

func (h *Heartbeater) backoffBase() time.Duration {
	if h.BackoffBase > 0 {
		return h.BackoffBase
	}
	return defaultBackoffBase
}

func (h *Heartbeater) backoffCap() time.Duration {
	if h.BackoffCap > 0 {
		return h.BackoffCap
	}
	return defaultBackoffCap
}

func (h *Heartbeater) httpClient() *http.Client {
	if h.Client != nil {
		return h.Client
	}
	return http.DefaultClient
}

// Run blocks until ctx is cancelled. It fires the first heartbeat
// immediately (no initial sleep), then every Interval. Failures apply
// exponential backoff between Interval bounds: backoff doubles up to
// BackoffCap on each consecutive failure and resets to BackoffBase on
// any success. Safe to fail; never panics on network errors.
func (h *Heartbeater) Run(ctx context.Context) error {
	if strings.TrimSpace(h.ServerOrigin) == "" || strings.TrimSpace(h.EnrollmentID) == "" || strings.TrimSpace(h.Credential) == "" || strings.TrimSpace(h.HelperDeviceID) == "" {
		return fmt.Errorf("heartbeater requires server origin, enrollment id, credential, and helper device id")
	}
	backoff := h.backoffBase()
	for {
		err := h.fire(ctx)
		if ctx.Err() != nil {
			return nil
		}
		var wait time.Duration
		if err != nil {
			h.logf("borgee-helper: heartbeat failed: %v (next attempt in %s)", err, backoff)
			wait = backoff
			backoff *= 2
			if backoff > h.backoffCap() {
				backoff = h.backoffCap()
			}
		} else {
			backoff = h.backoffBase()
			wait = h.interval()
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(wait):
		}
	}
}

// fire issues a single POST. Returns nil on 2xx. Returns a non-nil error
// describing the failure on transport error, 4xx, or 5xx. The caller decides
// retry policy; this function never panics and never logs PII.
func (h *Heartbeater) fire(ctx context.Context) error {
	body, err := json.Marshal(map[string]any{
		"helper_device_id": h.HelperDeviceID,
		"state":            "connected",
	})
	if err != nil {
		return err
	}
	url := strings.TrimRight(h.ServerOrigin, "/") + "/api/v1/helper/enrollments/" + h.EnrollmentID + "/status"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.Credential)
	resp, err := h.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// Drain so the underlying connection can be reused (keep-alive).
	_, _ = bytes.NewBuffer(nil).ReadFrom(resp.Body)
	if resp.StatusCode/100 == 2 {
		return nil
	}
	return fmt.Errorf("status=%d", resp.StatusCode)
}
