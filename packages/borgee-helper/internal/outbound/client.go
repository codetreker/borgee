package outbound

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Directive string

const (
	DirectiveProcess             Directive = "process"
	DirectiveRetry               Directive = "retry"
	DirectiveStopUnauthorized    Directive = "stop_unauthorized"
	DirectiveStopStaleCredential Directive = "stop_stale_credential"
	DirectiveStopRevoked         Directive = "stop_revoked"
	DirectiveStopUninstalled     Directive = "stop_uninstalled"
)

const (
	PollStatusLeased = "leased"
	PollStatusNoWork = "no_work"
)

type StaticCredentialSource struct {
	Credential     string
	HelperDeviceID string
}

type ClientOption func(*Client)

func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

type Client struct {
	serverOrigin string
	credential   StaticCredentialSource
	httpClient   *http.Client
}

type PollOptions struct {
	WaitMS int `json:"wait_ms,omitempty"`
}

type PollResult struct {
	Status     string
	Directive  Directive
	RetryAfter time.Duration
	Job        *LeasedJob
}

type LeasedJob struct {
	JobID          string          `json:"job_id"`
	EnrollmentID   string          `json:"enrollment_id"`
	JobType        string          `json:"job_type"`
	SchemaVersion  int             `json:"schema_version"`
	Payload        json.RawMessage `json:"payload"`
	ManifestDigest string          `json:"manifest_digest"`
	LeaseToken     string          `json:"lease_token"`
	LeaseExpiresAt int64           `json:"lease_expires_at"`
	Attempt        int             `json:"attempt"`
}

type JobState struct {
	JobID       string `json:"job_id"`
	Status      string `json:"status"`
	FailureCode string `json:"failure_code,omitempty"`
}

type ResultSummary struct {
	AuditRefs []string `json:"audit_refs,omitempty"`
	LogRefs   []string `json:"log_refs,omitempty"`
}

type ResultRequest struct {
	LeaseToken     string
	Status         string
	FailureCode    string
	FailureMessage string
	ResultSummary  ResultSummary
}

func NewClient(cfg PreparedConfig, credential StaticCredentialSource, opts ...ClientOption) (*Client, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("outbound client requires enabled prepared config")
	}
	if strings.TrimSpace(cfg.ServerOrigin) == "" {
		return nil, fmt.Errorf("outbound client requires server origin")
	}
	u, err := url.Parse(cfg.ServerOrigin)
	if err != nil || u.Scheme == "" || u.Host == "" || u.Path != "" {
		return nil, fmt.Errorf("invalid prepared server origin")
	}
	credential.Credential = strings.TrimSpace(credential.Credential)
	credential.HelperDeviceID = strings.TrimSpace(credential.HelperDeviceID)
	if credential.Credential == "" || credential.HelperDeviceID == "" {
		return nil, fmt.Errorf("helper credential and device id are required")
	}
	c := &Client{serverOrigin: strings.TrimRight(cfg.ServerOrigin, "/"), credential: credential, httpClient: http.DefaultClient}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

func (c *Client) Poll(ctx context.Context, enrollmentID string, opts PollOptions) (PollResult, error) {
	if err := validatePathID(enrollmentID); err != nil {
		return PollResult{}, err
	}
	body := map[string]any{"helper_device_id": c.credential.HelperDeviceID}
	if opts.WaitMS > 0 {
		body["wait_ms"] = opts.WaitMS
	}
	var resp struct {
		Status       string          `json:"status"`
		RetryAfterMS int             `json:"retry_after_ms"`
		Job          json.RawMessage `json:"job"`
	}
	statusCode, code, err := c.doJSON(ctx, "/api/v1/helper/enrollments/"+enrollmentID+"/jobs/poll", body, &resp)
	if err != nil {
		return PollResult{}, err
	}
	if statusCode >= 500 {
		return PollResult{Directive: DirectiveRetry}, nil
	}
	if stop := stopDirective(statusCode, code); stop != "" {
		return PollResult{Directive: stop}, nil
	}
	if statusCode != http.StatusOK {
		return PollResult{}, fmt.Errorf("helper poll failed: status=%d code=%s", statusCode, code)
	}
	if resp.Status == PollStatusNoWork {
		return PollResult{Status: PollStatusNoWork, Directive: DirectiveRetry, RetryAfter: time.Duration(resp.RetryAfterMS) * time.Millisecond}, nil
	}
	if resp.Status != PollStatusLeased || len(resp.Job) == 0 {
		return PollResult{}, errors.New("invalid helper poll response")
	}
	var job LeasedJob
	dec := json.NewDecoder(bytes.NewReader(resp.Job))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&job); err != nil {
		return PollResult{}, err
	}
	if job.JobID == "" || job.LeaseToken == "" {
		return PollResult{}, errors.New("invalid leased job response")
	}
	return PollResult{Status: PollStatusLeased, Directive: DirectiveProcess, Job: &job}, nil
}

func (c *Client) Ack(ctx context.Context, enrollmentID, jobID, leaseToken string) (JobState, error) {
	if err := validatePathID(enrollmentID); err != nil {
		return JobState{}, err
	}
	if err := validatePathID(jobID); err != nil {
		return JobState{}, err
	}
	body := map[string]any{"helper_device_id": c.credential.HelperDeviceID, "lease_token": leaseToken, "ack_status": "received"}
	var resp struct {
		Job JobState `json:"job"`
	}
	statusCode, code, err := c.doJSON(ctx, "/api/v1/helper/enrollments/"+enrollmentID+"/jobs/"+jobID+"/ack", body, &resp)
	if err != nil {
		return JobState{}, err
	}
	if statusCode != http.StatusOK {
		return JobState{}, fmt.Errorf("helper ack failed: status=%d code=%s", statusCode, code)
	}
	return resp.Job, nil
}

func (c *Client) Result(ctx context.Context, enrollmentID, jobID string, result ResultRequest) (JobState, error) {
	if err := validatePathID(enrollmentID); err != nil {
		return JobState{}, err
	}
	if err := validatePathID(jobID); err != nil {
		return JobState{}, err
	}
	body := map[string]any{
		"helper_device_id": c.credential.HelperDeviceID,
		"lease_token":      strings.TrimSpace(result.LeaseToken),
		"status":           strings.TrimSpace(result.Status),
	}
	if strings.TrimSpace(result.FailureCode) != "" {
		body["failure_code"] = strings.TrimSpace(result.FailureCode)
	}
	if strings.TrimSpace(result.FailureMessage) != "" {
		body["failure_message"] = strings.TrimSpace(result.FailureMessage)
	}
	if len(result.ResultSummary.AuditRefs) > 0 || len(result.ResultSummary.LogRefs) > 0 {
		body["result_summary"] = result.ResultSummary
	}
	var resp struct {
		Job JobState `json:"job"`
	}
	statusCode, code, err := c.doJSON(ctx, "/api/v1/helper/enrollments/"+enrollmentID+"/jobs/"+jobID+"/result", body, &resp)
	if err != nil {
		return JobState{}, err
	}
	if statusCode != http.StatusOK {
		return JobState{}, fmt.Errorf("helper result failed: status=%d code=%s", statusCode, code)
	}
	return resp.Job, nil
}

func (c *Client) doJSON(ctx context.Context, path string, body any, out any) (int, string, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return 0, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.serverOrigin+path, bytes.NewReader(b))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.credential.Credential)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, "", err
	}
	if resp.StatusCode != http.StatusOK {
		var errBody struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(raw, &errBody)
		return resp.StatusCode, errBody.Code, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return resp.StatusCode, "", err
	}
	return resp.StatusCode, "", nil
}

func stopDirective(statusCode int, code string) Directive {
	if statusCode == http.StatusUnauthorized {
		return DirectiveStopUnauthorized
	}
	if statusCode != http.StatusForbidden {
		return ""
	}
	switch code {
	case "stale_credential":
		return DirectiveStopStaleCredential
	case "revoked":
		return DirectiveStopRevoked
	case "uninstalled":
		return DirectiveStopUninstalled
	default:
		return ""
	}
}

func validatePathID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" || strings.Contains(id, "://") || strings.ContainsAny(id, "/\\?#") || strings.Contains(id, "..") {
		return fmt.Errorf("unsafe helper identifier")
	}
	return nil
}
