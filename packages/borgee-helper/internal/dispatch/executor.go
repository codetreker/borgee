package dispatch

import (
	"context"

	"borgee-helper/internal/outbound"
)

// TerminalNotImplemented is the failure_code reported via Result when a leased
// job's job_type has no executor registered. #1001 closes the helper dispatch
// wiring; the real per-job executors land in follow-up PRs (#998 etc) and
// register themselves into Dispatcher.Executors at main.go construction time.
const TerminalNotImplemented = "not_implemented"

// Terminal status string values mirror the server's helper_job_queries
// state machine: any leased job must be reported back as one of these.
const (
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
)

// TerminalStatus is the executor's contract back to the dispatcher: which
// terminal state to report via /result, and optional summary refs.
type TerminalStatus struct {
	Status         string
	FailureCode    string
	FailureMessage string
	ResultSummary  outbound.ResultSummary
}

// Executor runs a single leased job and reports the terminal status. The
// returned error is logged by the dispatcher; if Status is empty and err is
// non-nil, the dispatcher reports `failed` / `executor_error`. Implementations
// must honor ctx (it is cancelled on daemon SIGTERM and on lease teardown).
type Executor interface {
	Execute(ctx context.Context, job *outbound.LeasedJob) (TerminalStatus, error)
}
