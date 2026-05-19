// Package dispatch wires the helper-side job pipeline that closes the
// HB-RA-1B execution contract: Poll → jobpolicy.Evaluate (double-validate) →
// executor → Result/Ack.
//
// Issues #1001 + #1002 — before this package landed, outbound.Client.Poll/
// Ack/Result existed but no production code ever invoked them, and
// jobpolicy.Evaluate had 0 callers outside its own tests. The blueprint
// (`docs/blueprint/current/host-bridge.md §3` + §1.2) advertised
// defense-in-depth that was effectively single-sided (server only). Wiring
// them here through one long-lived goroutine fixes both gaps in one place;
// per-job executors plug in via the Executors map in follow-up PRs.
//
// Lifecycle: cmd/borgee-helper/main.go spawns one Dispatcher.Run goroutine
// next to Heartbeater.Run, sharing the daemon's SIGTERM-aware context. The
// loop never panics on transport errors and never aborts the daemon on
// 4xx — those just back off and continue trying, matching Heartbeater's
// "safe to fail" stance so an admin-side revoke + re-claim can recover
// without bouncing the process.
package dispatch

import (
	"context"
	"log"
	"strings"
	"time"

	"borgee-helper/internal/jobpolicy"
	"borgee-helper/internal/outbound"
)

// Default cadences. Production picks sane defaults; tests override via the
// public fields to keep assertions sub-second.
const (
	defaultPollWait        = 25 * time.Second // server-side long-poll budget (handlePoll caps at 30s)
	defaultPollRetry       = 5 * time.Second  // gap between polls when the server says no_work and didn't include a hint
	defaultBackoffBase     = 5 * time.Second
	defaultBackoffCap      = 60 * time.Second
	defaultLeaseRenewEvery = 30 * time.Second
)

// PolicyEvaluator turns a leased job into a jobpolicy decision. Wrapping the
// real jobpolicy.Evaluate keeps the dispatcher decoupled from EvaluationInput
// construction (which needs enrollment / sandbox state the dispatcher does
// not own). Tests inject fakes via this field.
type PolicyEvaluator func(ctx context.Context, job *outbound.LeasedJob) jobpolicy.Decision

// Dispatcher polls the server for leased helper jobs and runs each through a
// local policy gate + executor pipeline. Zero value is not useful; Client
// MUST be set (otherwise Run returns immediately with no error so a pre-claim
// daemon can still boot — see also cmd/borgee-helper/main.go).
type Dispatcher struct {
	Client          *outbound.Client
	EnrollmentID    string
	PolicyEvaluator PolicyEvaluator
	Executors       map[string]Executor

	// PollWait caps how long the server may block a single /poll call (long-poll).
	// PollRetry is the gap between polls when the server returns no_work with no
	// explicit retry hint. LeaseRenewEvery controls the Ack cadence per leased job.
	// BackoffBase / BackoffCap apply to consecutive Poll transport / 5xx failures
	// (mirrors Heartbeater behavior so reviewers don't have to learn a second
	// retry curve).
	PollWait        time.Duration
	PollRetry       time.Duration
	LeaseRenewEvery time.Duration
	BackoffBase     time.Duration
	BackoffCap      time.Duration

	// Logger lets tests capture log lines. nil → standard log package.
	Logger func(format string, v ...any)
}

func (d *Dispatcher) logf(format string, v ...any) {
	if d.Logger != nil {
		d.Logger(format, v...)
		return
	}
	log.Printf(format, v...)
}

func (d *Dispatcher) pollWait() time.Duration {
	if d.PollWait > 0 {
		return d.PollWait
	}
	return defaultPollWait
}

func (d *Dispatcher) pollRetry() time.Duration {
	if d.PollRetry > 0 {
		return d.PollRetry
	}
	return defaultPollRetry
}

func (d *Dispatcher) leaseRenewEvery() time.Duration {
	if d.LeaseRenewEvery > 0 {
		return d.LeaseRenewEvery
	}
	return defaultLeaseRenewEvery
}

func (d *Dispatcher) backoffBase() time.Duration {
	if d.BackoffBase > 0 {
		return d.BackoffBase
	}
	return defaultBackoffBase
}

func (d *Dispatcher) backoffCap() time.Duration {
	if d.BackoffCap > 0 {
		return d.BackoffCap
	}
	return defaultBackoffCap
}

// Run blocks until ctx is cancelled. Caller's responsibility: pass the
// daemon's SIGTERM-aware context so teardown closes within ~100ms.
//
// Skip behavior — if Client is nil or EnrollmentID is empty, Run returns
// nil immediately. This mirrors the heartbeater's "no enrollment configured,
// skipping" path: a pre-claim daemon must still boot the UDS contract; the
// dispatcher is opt-in once an operator runs the claim CLI.
func (d *Dispatcher) Run(ctx context.Context) error {
	if d.Client == nil || strings.TrimSpace(d.EnrollmentID) == "" {
		d.logf("borgee-helper: no enrollment configured, skipping job dispatcher")
		return nil
	}
	backoff := d.backoffBase()
	for {
		if ctx.Err() != nil {
			return nil
		}
		waitMS := int(d.pollWait() / time.Millisecond)
		poll, err := d.Client.Poll(ctx, d.EnrollmentID, outbound.PollOptions{WaitMS: waitMS})
		if ctx.Err() != nil {
			return nil
		}
		switch {
		case err != nil:
			d.logf("borgee-helper: dispatcher poll failed: %v (next attempt in %s)", err, backoff)
			if !sleepOrDone(ctx, backoff) {
				return nil
			}
			backoff = nextBackoff(backoff, d.backoffCap())
			continue
		case isStopDirective(poll.Directive):
			d.logf("borgee-helper: dispatcher poll stop directive=%s (continuing with backoff)", poll.Directive)
			if !sleepOrDone(ctx, backoff) {
				return nil
			}
			backoff = nextBackoff(backoff, d.backoffCap())
			continue
		case poll.Directive == outbound.DirectiveRetry && poll.Job == nil:
			backoff = d.backoffBase()
			wait := poll.RetryAfter
			if wait <= 0 {
				wait = d.pollRetry()
			}
			if !sleepOrDone(ctx, wait) {
				return nil
			}
			continue
		case poll.Directive == outbound.DirectiveProcess && poll.Job != nil:
			backoff = d.backoffBase()
			d.processJob(ctx, poll.Job)
			continue
		default:
			// Unknown shape — apply backoff so we don't tight-loop on a server
			// regression.
			d.logf("borgee-helper: dispatcher poll returned unexpected shape directive=%s status=%s", poll.Directive, poll.Status)
			if !sleepOrDone(ctx, backoff) {
				return nil
			}
			backoff = nextBackoff(backoff, d.backoffCap())
		}
	}
}

// processJob runs a single leased job through the policy gate + executor and
// reports the terminal status. While the job runs, a side goroutine
// periodically Acks to keep the lease alive. The Ack loop tears down
// deterministically before the final Result post so the server never sees an
// Ack after the terminal state.
func (d *Dispatcher) processJob(ctx context.Context, job *outbound.LeasedJob) {
	jobCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	ackDone := make(chan struct{})
	go d.ackLoop(jobCtx, job, ackDone)

	result := outbound.ResultRequest{LeaseToken: job.LeaseToken}

	decision := d.evaluate(jobCtx, job)
	if !decision.Allow {
		result.Status = StatusFailed
		result.FailureCode = string(decision.Reason)
		result.FailureMessage = "helper local policy rejected job"
		d.logf("borgee-helper: dispatcher policy rejected job=%s type=%s reason=%s", job.JobID, job.JobType, decision.Reason)
	} else if exec, ok := d.lookupExecutor(job.JobType); ok {
		terminal, execErr := exec.Execute(jobCtx, job)
		result.Status = terminal.Status
		result.FailureCode = terminal.FailureCode
		result.FailureMessage = terminal.FailureMessage
		result.ResultSummary = terminal.ResultSummary
		if execErr != nil {
			d.logf("borgee-helper: dispatcher executor job=%s type=%s err=%v", job.JobID, job.JobType, execErr)
			if result.Status == "" {
				result.Status = StatusFailed
				result.FailureCode = "executor_error"
				result.FailureMessage = execErr.Error()
			}
		}
		if result.Status == "" {
			// Executor returned (nil, nil) — treat as failure rather than
			// silently dropping the lease.
			result.Status = StatusFailed
			result.FailureCode = "executor_error"
			result.FailureMessage = "executor returned no terminal status"
		}
	} else {
		result.Status = StatusFailed
		result.FailureCode = TerminalNotImplemented
		result.FailureMessage = "no executor registered for job_type=" + job.JobType
		d.logf("borgee-helper: dispatcher no executor for job=%s type=%s reporting not_implemented", job.JobID, job.JobType)
	}

	// Stop the ack loop before posting Result so the server never sees an Ack
	// after the terminal state lands.
	cancel()
	<-ackDone

	// Use the parent ctx for the final Result so we still post even if jobCtx
	// has been cancelled. If the daemon itself is shutting down, the parent
	// ctx is already done — accept the lease will expire server-side.
	if ctx.Err() != nil {
		return
	}
	if _, err := d.Client.Result(ctx, job.EnrollmentID, job.JobID, result); err != nil {
		d.logf("borgee-helper: dispatcher result post failed job=%s: %v", job.JobID, err)
	}
}

func (d *Dispatcher) lookupExecutor(jobType string) (Executor, bool) {
	if d.Executors == nil {
		return nil, false
	}
	exec, ok := d.Executors[jobType]
	if !ok || exec == nil {
		return nil, false
	}
	return exec, true
}

func (d *Dispatcher) evaluate(ctx context.Context, job *outbound.LeasedJob) jobpolicy.Decision {
	if d.PolicyEvaluator == nil {
		// Default deny — a dispatcher with no evaluator wired up must not
		// silently allow jobs through. Production main.go always wires
		// jobpolicy.Evaluate; tests can substitute their own.
		return jobpolicy.Decision{Allow: false, Reason: jobpolicy.ReasonPolicyDenied}
	}
	return d.PolicyEvaluator(ctx, job)
}

// ackLoop posts /ack on a fixed cadence to extend the server-side lease while
// the executor runs. It returns when ctx is cancelled (by processJob after
// the terminal Result is decided, or by daemon SIGTERM).
func (d *Dispatcher) ackLoop(ctx context.Context, job *outbound.LeasedJob, done chan<- struct{}) {
	defer close(done)
	tick := time.NewTicker(d.leaseRenewEvery())
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if ctx.Err() != nil {
				return
			}
			if _, err := d.Client.Ack(ctx, job.EnrollmentID, job.JobID, job.LeaseToken); err != nil {
				d.logf("borgee-helper: dispatcher ack failed job=%s: %v", job.JobID, err)
			}
		}
	}
}

// isStopDirective collapses all three "stop processing this enrollment"
// signals into one branch. Today the dispatcher just backs off and keeps
// trying so an operator-side re-claim recovers without process bounce — the
// same stance Heartbeater takes on 401.
func isStopDirective(d outbound.Directive) bool {
	switch d {
	case outbound.DirectiveStopUnauthorized, outbound.DirectiveStopStaleCredential, outbound.DirectiveStopRevoked, outbound.DirectiveStopUninstalled:
		return true
	}
	return false
}

func nextBackoff(current, cap time.Duration) time.Duration {
	next := current * 2
	if next > cap {
		return cap
	}
	return next
}

// sleepOrDone returns true if the wait elapsed, false if ctx was cancelled.
func sleepOrDone(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
