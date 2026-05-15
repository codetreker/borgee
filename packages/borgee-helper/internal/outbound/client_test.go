package outbound

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestClientPollAckResultUseFixedPathsAndHelperCredential(t *testing.T) {
	ctx := context.Background()
	var paths []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.String())
		if r.Header.Get("Authorization") != "Bearer helper-token" {
			t.Fatalf("Authorization header=%q", r.Header.Get("Authorization"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body["helper_device_id"] != "device-1" {
			t.Fatalf("request missing helper_device_id: %v", body)
		}
		switch r.URL.Path {
		case "/api/v1/helper/enrollments/enroll-1/jobs/poll":
			writeOutboundTestJSON(w, map[string]any{
				"status": "leased",
				"job": map[string]any{
					"job_id":           "job-1",
					"enrollment_id":    "enroll-1",
					"job_type":         "openclaw.configure_agent",
					"schema_version":   1,
					"payload":          map[string]any{"agent_id": "agent-1"},
					"manifest_digest":  "sha256:manifest",
					"lease_token":      "lease-token",
					"lease_expires_at": time.Now().Add(time.Minute).UnixMilli(),
					"attempt":          1,
				},
			})
		case "/api/v1/helper/enrollments/enroll-1/jobs/job-1/ack":
			if body["lease_token"] != "lease-token" || body["ack_status"] != "received" {
				t.Fatalf("bad ack body: %v", body)
			}
			writeOutboundTestJSON(w, map[string]any{"job": map[string]any{"job_id": "job-1", "status": "running"}})
		case "/api/v1/helper/enrollments/enroll-1/jobs/job-1/result":
			if body["lease_token"] != "lease-token" || body["status"] != "failed" || body["failure_code"] != "policy_denied" {
				t.Fatalf("bad result body: %v", body)
			}
			if _, ok := body["url"]; ok {
				t.Fatalf("result body must not include arbitrary URL override: %v", body)
			}
			writeOutboundTestJSON(w, map[string]any{"job": map[string]any{"job_id": "job-1", "status": "failed", "failure_code": "policy_denied"}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	t.Cleanup(ts.Close)

	client, err := NewClient(PreparedConfig{Enabled: true, ServerOrigin: ts.URL}, StaticCredentialSource{Credential: "helper-token", HelperDeviceID: "device-1"}, WithHTTPClient(ts.Client()))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	poll, err := client.Poll(ctx, "enroll-1", PollOptions{})
	if err != nil || poll.Status != PollStatusLeased || poll.Directive != DirectiveProcess || poll.Job == nil || poll.Job.LeaseToken != "lease-token" {
		t.Fatalf("Poll result=%+v err=%v", poll, err)
	}
	ack, err := client.Ack(ctx, "enroll-1", "job-1", "lease-token")
	if err != nil || ack.Status != "running" {
		t.Fatalf("Ack result=%+v err=%v", ack, err)
	}
	result, err := client.Result(ctx, "enroll-1", "job-1", ResultRequest{
		LeaseToken:     "lease-token",
		Status:         "failed",
		FailureCode:    "policy_denied",
		FailureMessage: "policy handoff denied",
		ResultSummary:  ResultSummary{AuditRefs: []string{"audit-1"}},
	})
	if err != nil || result.Status != "failed" || result.FailureCode != "policy_denied" {
		t.Fatalf("Result=%+v err=%v", result, err)
	}
	want := []string{
		"POST /api/v1/helper/enrollments/enroll-1/jobs/poll",
		"POST /api/v1/helper/enrollments/enroll-1/jobs/job-1/ack",
		"POST /api/v1/helper/enrollments/enroll-1/jobs/job-1/result",
	}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths=%v want %v", paths, want)
	}
}

func TestClientMapsNoWorkTransientAndStopDirectives(t *testing.T) {
	ctx := context.Background()
	for name, tc := range map[string]struct {
		statusCode int
		body       map[string]any
		want       Directive
		retryAfter time.Duration
	}{
		"no work":          {statusCode: http.StatusOK, body: map[string]any{"status": "no_work", "retry_after_ms": 5000}, want: DirectiveRetry, retryAfter: 5 * time.Second},
		"transient server": {statusCode: http.StatusBadGateway, body: map[string]any{"code": "temporary"}, want: DirectiveRetry},
		"stale credential": {statusCode: http.StatusForbidden, body: map[string]any{"code": "stale_credential"}, want: DirectiveStopStaleCredential},
		"revoked":          {statusCode: http.StatusForbidden, body: map[string]any{"code": "revoked"}, want: DirectiveStopRevoked},
		"uninstalled":      {statusCode: http.StatusForbidden, body: map[string]any{"code": "uninstalled"}, want: DirectiveStopUninstalled},
	} {
		t.Run(name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				writeOutboundTestJSON(w, tc.body)
			}))
			t.Cleanup(ts.Close)
			client, err := NewClient(PreparedConfig{Enabled: true, ServerOrigin: ts.URL}, StaticCredentialSource{Credential: "helper-token", HelperDeviceID: "device-1"}, WithHTTPClient(ts.Client()))
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			poll, err := client.Poll(ctx, "enroll-1", PollOptions{})
			if err != nil {
				t.Fatalf("Poll: %v", err)
			}
			if poll.Directive != tc.want || poll.RetryAfter != tc.retryAfter {
				t.Fatalf("directive=%s retry=%s want %s/%s", poll.Directive, poll.RetryAfter, tc.want, tc.retryAfter)
			}
		})
	}
}

func TestClientRejectsFullURLOrTraversalIdentifiers(t *testing.T) {
	ctx := context.Background()
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(ts.Close)
	client, err := NewClient(PreparedConfig{Enabled: true, ServerOrigin: ts.URL}, StaticCredentialSource{Credential: "helper-token", HelperDeviceID: "device-1"}, WithHTTPClient(ts.Client()))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	for _, id := range []string{"https://evil.example/x", "../other", "job/with/slash", ""} {
		if _, err := client.Poll(ctx, id, PollOptions{}); err == nil {
			t.Fatalf("Poll accepted unsafe enrollment id %q", id)
		}
	}
	if requests != 0 {
		t.Fatalf("unsafe identifiers should not send requests, got %d", requests)
	}
}

func writeOutboundTestJSON(w http.ResponseWriter, body map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}
