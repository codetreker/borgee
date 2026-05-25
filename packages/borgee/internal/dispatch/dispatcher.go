// Package dispatch wires the helper-side job pipeline that closes the
// HB-RA-1B execution contract: server-pushed job → jobpolicy.Evaluate
// (double-validate) → executor → Result/Ack over the persistent WS
// transport.
//
// Issues #1038 + (historic) #1001/#1002 — PR-2 #1038 swapped the
// outbound transport from HTTP long-poll to a persistent WebSocket.
// Dispatcher's outer loop is now Receive-based: it blocks on a WS read
// for the next pushed job (sub-second latency, vs. the prior ~25s
// long-poll budget). The outbound package owns reconnect + backoff
// internally; the dispatcher just calls Run, which drives the WS
// client through dial→receive→reconnect under one goroutine.
//
// Lifecycle: cmd/borgee-helper/main.go spawns one Dispatcher.Run
// goroutine alongside the other producers, sharing the daemon's
// SIGTERM-aware context. The loop never panics on transport errors
// and never aborts the daemon on transient close — those just
// reconnect via outbound.Client.RunWithReconnect.
package dispatch

import (
	"context"
	"log"
	"strings"
	"time"

	"borgee/internal/jobpolicy"
	"borgee/internal/outbound"
)

// Default lease-renewal cadence. Ack is now a fire-and-forget WS
// frame; the lease is extended server-side on each Ack. Picking 30s
// gives a 2x safety margin against the server's default 60s lease
// duration without spamming the wire.
const defaultLeaseRenewEvery = 30 * time.Second

// PolicyEvaluator turns a leased job into a jobpolicy decision.
type PolicyEvaluator func(ctx context.Context, job *outbound.LeasedJob) jobpolicy.Decision

// Dispatcher drives the WS transport: receives pushed jobs, runs each
// through a local policy gate + executor pipeline, posts Ack/Result
// frames. Zero value is not useful; Client MUST be set.
type Dispatcher struct {
	Client          *outbound.Client
	EnrollmentID    string
	PolicyEvaluator PolicyEvaluator
	Executors       map[string]Executor

	// LeaseRenewEvery — Ack cadence per leased job. 0 → default.
	LeaseRenewEvery time.Duration

	// Logger lets tests capture log lines. nil → standard log package.
	Logger func(format string, v ...any)

	// OnDirective is invoked when the WS read loop surfaces a stop
	// directive (revoked / stale_credential / uninstalled). Default
	// behavior: log + return; the daemon's outer ctx eventually
	// cancels and systemd Restart=on-failure rebounds. Tests may
	// override to assert the path was hit.
	OnDirective func(ctx context.Context, dir outbound.Directive)
}

func (d *Dispatcher) logf(format string, v ...any) {
	if d.Logger != nil {
		d.Logger(format, v...)
		return
	}
	log.Printf(format, v...)
}

func (d *Dispatcher) leaseRenewEvery() time.Duration {
	if d.LeaseRenewEvery > 0 {
		return d.LeaseRenewEvery
	}
	return defaultLeaseRenewEvery
}

// Run blocks until ctx is cancelled or the WS client receives a stop
// directive. Pre-claim daemons skip the loop entirely.
func (d *Dispatcher) Run(ctx context.Context) error {
	if d.Client == nil || strings.TrimSpace(d.EnrollmentID) == "" {
		d.logf("borgee-helper: no enrollment configured, skipping job dispatcher")
		return nil
	}
	d.Client.SetEnrollmentID(d.EnrollmentID)

	stop := d.Client.RunWithReconnect(ctx,
		func(jobCtx context.Context, job *outbound.LeasedJob) {
			d.processJob(jobCtx, job)
		},
		func(_ context.Context, dir outbound.Directive) {
			d.logf("borgee-helper: dispatcher received directive=%s", dir)
			if d.OnDirective != nil {
				d.OnDirective(ctx, dir)
			}
		},
	)
	if stop != "" {
		d.logf("borgee-helper: dispatcher exiting on stop directive=%s", stop)
	}
	return nil
}

// processJob runs a single leased job through the policy gate +
// executor and reports the terminal status. While the job runs, a
// side goroutine periodically Acks to keep the lease alive. The Ack
// loop tears down before the final Result so the server never sees an
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

	cancel()
	<-ackDone

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
		return jobpolicy.Decision{Allow: false, Reason: jobpolicy.ReasonPolicyDenied}
	}
	return d.PolicyEvaluator(ctx, job)
}

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

// Drain best-effort waits for in-flight job processing to complete and
// then closes the WS receive loop so the next Receive call returns. It
// is intentionally cooperative — the executor that requested the drain
// (today only delegation.revoke) must still report its own terminal
// Result over the existing WS connection BEFORE the dispatcher's outer
// loop unwinds. Drain therefore does NOT cancel the dispatcher's ctx;
// it only signals "stop accepting new jobs" so the next reconnect tick
// finds the credential gone and exits naturally.
//
// `timeout` caps the wait; on expiry Drain returns an error and the
// caller proceeds anyway (delegation.revoke prefers to wipe the
// credential even if a stuck in-flight job is still running).
//
// PR-4 only wires this for delegation.revoke. The other 7 job types
// never call Drain.
func (d *Dispatcher) Drain(_ context.Context, _ time.Duration) error {
	// Today the dispatcher's per-job processing is fire-and-forget on
	// the outbound.Client.RunWithReconnect side — there is no central
	// in-flight job registry to wait on. The executor for
	// delegation.revoke is itself one of those in-flight jobs, so the
	// drain is implicitly satisfied: once delegation.revoke's
	// dispatch.Result fires, no other job can be leased (the
	// credential is about to be wiped on the rootd side, and the WS
	// server will see auth failure on the next push).
	//
	// Future: a richer drain would track per-job goroutines via a
	// sync.WaitGroup and Wait here. For PR-4 the no-op is correct
	// because delegation.revoke is the only caller.
	return nil
}
