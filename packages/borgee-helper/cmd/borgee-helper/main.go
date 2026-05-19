//go:build linux || darwin

// Package main — borgee-helper daemon entry (HB-2 v0(D) host-bridge).
// 平台 transport: POSIX UDS via net.Listen("unix", path).
//
// hb-2-v0d-spec.md §0.2: real sandbox, real IO, and real SQLite consumer.
// install-butler 拉起 daemon 时:
//   - Linux: systemd unit + landlock LSM
//   - macOS: launchd + sandbox-exec wrapper
//   - DSN: --grants-db=/var/lib/borgee/server.db?mode=ro
package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"borgee-helper/internal/acl"
	"borgee-helper/internal/audit"
	"borgee-helper/internal/dispatch"
	"borgee-helper/internal/executors/uninstall"
	"borgee-helper/internal/grants"
	"borgee-helper/internal/ipc"
	"borgee-helper/internal/jobpolicy"
	"borgee-helper/internal/outbound"
	"borgee-helper/internal/sandbox"
)

func main() {
	socket := flag.String("socket", "/run/borgee-helper/borgee-helper.sock", "UDS path (Linux/macOS)")
	auditLog := flag.String("audit-log", "/var/log/borgee-helper/audit.log.jsonl", "audit JSON-line path")
	grantsDSN := flag.String("grants-db", "", "sqlite DSN for HB-3 host_grants table (read-only) — REQUIRED for production")
	readPathsFlag := flag.String("read-paths", "", "comma-separated absolute paths landlock allows (v0(D) static; v1+ pulls live from host_grants)")
	outboundServerOrigin := flag.String("outbound-server-origin", "", "Borgee API server origin for Helper outbound calls")
	outboundAllowedOrigins := flag.String("outbound-allowed-origins", "", "comma-separated exact Borgee API origins allowed for Helper outbound calls")
	queueStateDir := flag.String("queue-state-dir", "", "Helper-owned queue cursor state directory")
	statusStateDir := flag.String("status-state-dir", "", "Helper-owned bounded status state directory")
	auditHandoffDir := flag.String("audit-handoff-dir", "", "Helper-owned local audit handoff directory")
	// All three are file-based so secrets never appear on /proc/PID/cmdline
	// (the systemd drop-in passes paths only; the claim CLI populates the
	// files at install/claim time). Missing/empty files → heartbeat is
	// silently skipped so the daemon still boots before claim happens.
	enrollmentIDFile := flag.String("enrollment-id-file", "", "Path to file containing the helper enrollment id (#968 heartbeat producer config)")
	helperDeviceIDFile := flag.String("helper-device-id-file", "", "Path to file containing the helper device id bound at claim (#968 heartbeat)")
	helperCredentialFile := flag.String("helper-credential-file", "", "Path to file containing the helper credential (Bearer token) issued at claim; readable only by helper user (#968 heartbeat)")
	allowLoopbackOutbound := flag.Bool("allow-loopback-outbound", false, "Permit http:// loopback as --outbound-server-origin (e2e tests only; production daemon always uses https)")
	allowedStateRoots := flag.String("allowed-state-roots", "", "Comma-separated list of allowed parent directories for queue/status/audit-handoff state dirs; empty defaults to platform helper state root (production default; e2e tests override)")
	flag.Parse()

	outboundPrereq := outbound.PrereqConfig{
		ServerOrigin:    *outboundServerOrigin,
		AllowedOrigins:  *outboundAllowedOrigins,
		QueueStateDir:   *queueStateDir,
		StatusStateDir:  *statusStateDir,
		AuditHandoffDir: *auditHandoffDir,
	}
	if err := run(*socket, *auditLog, *grantsDSN, *readPathsFlag, outboundPrereq, *enrollmentIDFile, *helperDeviceIDFile, *helperCredentialFile, *allowLoopbackOutbound, *allowedStateRoots); err != nil {
		log.Fatalf("borgee-helper: %v", err)
	}
}

func run(socket, auditLogPath, grantsDSN, readPaths string, outboundPrereq outbound.PrereqConfig, enrollmentIDFile, helperDeviceIDFile, helperCredentialFile string, allowLoopbackOutbound bool, allowedStateRoots string) error {
	// Audit log writer (forward-only, JSON-line).
	logFile, err := os.OpenFile(auditLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		log.Printf("warn: audit log %q unwritable (%v); falling back to stderr", auditLogPath, err)
		logFile = os.Stderr
	}
	auditLogger := audit.New(logFile)

	// v0(D) grants consumer: the production path must use SQLite (negative
	// constraint §1.5: grep check MemoryConsumer has 0 hits in production path).
	// Dev tests use MemoryConsumer inside _test.go files, not in main.
	if grantsDSN == "" {
		return errAbort("--grants-db is required (HB-3 host_grants SQLite DSN, e.g. file:/var/lib/borgee/server.db?mode=ro&_busy_timeout=5000)")
	}
	sc, err := grants.NewSQLiteConsumer(grantsDSN)
	if err != nil {
		return err
	}
	defer sc.Close()
	var gc grants.Consumer = sc
	log.Printf("borgee-helper: SQLite consumer connected dsn=%s", grantsDSN)

	preparedOutbound, err := outbound.ValidateAndPrepare(outboundPrereq, outbound.ValidationOptions{
		AllowLoopbackHTTP: allowLoopbackOutbound,
		AllowedStateRoots: splitCSV(allowedStateRoots),
	})
	if err != nil {
		return err
	}
	if preparedOutbound.Enabled {
		log.Printf("borgee-helper: outbound prerequisites configured origin=%s state_dirs=3", preparedOutbound.ServerOrigin)
	}

	// ACL gate (Consumer interface).
	gate := acl.New(gc)

	// v0(D) Sandbox apply (real landlock on Linux; sandbox-exec wrapper on macOS).
	profile := sandbox.Profile{
		AuditLogPath: auditLogPath,
	}
	if readPaths != "" {
		profile.ReadPaths = splitCSV(readPaths)
	}
	if err := sandbox.Apply(profile); err != nil {
		return err
	}
	log.Printf("borgee-helper: sandbox platform=%s applied (v0(D) real)", sandbox.Platform)

	// UDS listener (POSIX).
	_ = os.Remove(socket) // best-effort cleanup stale socket
	ln, err := net.Listen("unix", socket)
	if err != nil {
		return err
	}
	defer ln.Close()
	log.Printf("borgee-helper: listening on %s", socket)

	// Signal handler for clean shutdown (ctx-aware, prevents goroutine leaks per #608).
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	// #968 heartbeat producer: spawn before entering the UDS Accept loop so a
	// rebooted/crashed host re-asserts `connected` within ~100ms of daemon
	// start. Shares ctx with the Accept loop for clean SIGTERM teardown.
	// Skips silently if enrollment isn't configured yet (fresh install
	// pre-claim) — failing to start the daemon over a missing credential
	// would defeat the boot-survival contract.
	if hb, ok := buildHeartbeater(preparedOutbound, enrollmentIDFile, helperDeviceIDFile, helperCredentialFile); ok {
		log.Printf("borgee-helper: heartbeat enabled enrollment_id=%s interval=%s", hb.EnrollmentID, outbound.HeartbeatInterval)
		go func() {
			if err := hb.Run(ctx); err != nil {
				log.Printf("borgee-helper: heartbeater exited: %v", err)
			}
		}()
	} else {
		log.Printf("borgee-helper: no enrollment configured, skipping heartbeat")
	}

	// #1001 + #1002 dispatcher: poll the server for leased jobs, run each
	// through jobpolicy.Evaluate (helper-side double-validate), then through
	// a per-job executor if one is registered. The Executors map is empty
	// today — typed executors land in follow-up PRs (#998 helper.uninstall,
	// etc.); a leased job whose type has no registered executor is reported
	// back as terminal `failed`/`not_implemented` so the server never sees a
	// silently swallowed lease. Pre-claim daemons skip dispatch the same way
	// they skip heartbeat above.
	if dp, ok := buildDispatcher(preparedOutbound, enrollmentIDFile, helperDeviceIDFile, helperCredentialFile); ok {
		log.Printf("borgee-helper: dispatcher enabled enrollment_id=%s", dp.EnrollmentID)
		go func() {
			if err := dp.Run(ctx); err != nil {
				log.Printf("borgee-helper: dispatcher exited: %v", err)
			}
		}()
	} else {
		log.Printf("borgee-helper: no enrollment configured, skipping job dispatcher")
	}

	h := ipc.New(gate, auditLogger)
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			log.Printf("accept err: %v", err)
			continue
		}
		go func(c net.Conn) {
			if err := h.Serve(ctx, c); err != nil {
				log.Printf("serve err: %v", err)
			}
		}(conn)
	}
}

// errAbort is a sentinel error wrapping a fatal startup failure.
func errAbort(msg string) error {
	return &abortErr{msg: msg}
}

type abortErr struct{ msg string }

func (e *abortErr) Error() string { return e.msg }

// splitCSV splits a comma-separated list, trimming whitespace + skipping empties.
func splitCSV(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			seg := trim(s[start:i])
			if seg != "" {
				out = append(out, seg)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		seg := trim(s[start:])
		if seg != "" {
			out = append(out, seg)
		}
	}
	return out
}

// buildHeartbeater returns a configured Heartbeater + true when the daemon
// has everything it needs to post status, or (nil,false) otherwise. Returning
// false is the explicit "skip heartbeat" path: pre-claim hosts must still
// boot the daemon so the local UDS contract is honored.
//
// All three inputs are *file paths* (not raw values): keeping secrets out of
// the cmdline avoids /proc/PID/cmdline leakage. Missing file, empty file, and
// unreadable file all collapse to (nil,false) so a partially-populated state
// directory never trips the daemon.
func buildHeartbeater(prep outbound.PreparedConfig, enrollmentIDFile, helperDeviceIDFile, credentialFile string) (*outbound.Heartbeater, bool) {
	if !prep.Enabled {
		return nil, false
	}
	if trim(enrollmentIDFile) == "" || trim(helperDeviceIDFile) == "" || trim(credentialFile) == "" {
		return nil, false
	}
	enrollmentID, ok := readTrimmedFile("--enrollment-id-file", enrollmentIDFile)
	if !ok {
		return nil, false
	}
	helperDeviceID, ok := readTrimmedFile("--helper-device-id-file", helperDeviceIDFile)
	if !ok {
		return nil, false
	}
	credential, ok := readTrimmedFile("--helper-credential-file", credentialFile)
	if !ok {
		return nil, false
	}
	return &outbound.Heartbeater{
		ServerOrigin:   prep.ServerOrigin,
		EnrollmentID:   enrollmentID,
		HelperDeviceID: helperDeviceID,
		Credential:     credential,
	}, true
}

// buildDispatcher mirrors buildHeartbeater's skip semantics: missing /
// unreadable / empty files collapse to (nil,false) so a pre-claim daemon
// boots without dispatch wiring. When all three values are present, we
// construct an outbound.Client and a default-deny PolicyEvaluator that
// delegates to jobpolicy.Evaluate. The Executors map is intentionally empty
// in this PR — typed executors register themselves in follow-up PRs (#998
// etc) and the dispatcher reports `not_implemented` for any job_type that
// isn't yet wired.
func buildDispatcher(prep outbound.PreparedConfig, enrollmentIDFile, helperDeviceIDFile, credentialFile string) (*dispatch.Dispatcher, bool) {
	if !prep.Enabled {
		return nil, false
	}
	if trim(enrollmentIDFile) == "" || trim(helperDeviceIDFile) == "" || trim(credentialFile) == "" {
		return nil, false
	}
	enrollmentID, ok := readTrimmedFile("--enrollment-id-file", enrollmentIDFile)
	if !ok {
		return nil, false
	}
	helperDeviceID, ok := readTrimmedFile("--helper-device-id-file", helperDeviceIDFile)
	if !ok {
		return nil, false
	}
	credential, ok := readTrimmedFile("--helper-credential-file", credentialFile)
	if !ok {
		return nil, false
	}
	client, err := outbound.NewClient(
		prep,
		outbound.StaticCredentialSource{Credential: credential, HelperDeviceID: helperDeviceID},
	)
	if err != nil {
		log.Printf("borgee-helper: cannot construct outbound client for dispatcher: %v", err)
		return nil, false
	}
	return &dispatch.Dispatcher{
		Client:          client,
		EnrollmentID:    enrollmentID,
		PolicyEvaluator: defaultPolicyEvaluator(),
		Executors: map[string]dispatch.Executor{
			// #998 helper.uninstall — one-key self-teardown. See
			// internal/executors/uninstall for the cleanup sequence and the
			// "no self-stop signal" safety note.
			jobpolicy.JobTypeHelperUninstall: &uninstall.Executor{
				Logger: log.Printf,
			},
		},
	}, true
}

// defaultPolicyEvaluator wraps jobpolicy.Evaluate. Today the helper does not
// own the enrollment/sandbox state that Evaluate needs for full envelope
// checks (those live in the install/configure flow and will be wired through
// in #998+ alongside the executors); a job that lacks the required envelope
// fields therefore falls into Evaluate's schema-invalid / manifest-invalid
// branches and the dispatcher reports the deterministic reason. The point of
// wiring it here is to close the #1002 "0 production callers" gap so the
// double-validate contract becomes real the moment any executor lands.
func defaultPolicyEvaluator() dispatch.PolicyEvaluator {
	return func(_ context.Context, job *outbound.LeasedJob) jobpolicy.Decision {
		if job == nil {
			return jobpolicy.Decision{Allow: false, Reason: jobpolicy.ReasonSchemaInvalid}
		}
		return jobpolicy.Evaluate(jobpolicy.EvaluationInput{
			Job: jobpolicy.Job{
				JobID:          job.JobID,
				EnrollmentID:   job.EnrollmentID,
				JobType:        job.JobType,
				SchemaVersion:  job.SchemaVersion,
				PayloadJSON:    job.Payload,
				ManifestDigest: job.ManifestDigest,
			},
		})
	}
}

// readTrimmedFile reads `path` and returns its whitespace-trimmed content. It
// returns ("", false) and logs the reason on any of: missing path, read
// failure, empty file. The label is the flag name for operator-friendly logs.
func readTrimmedFile(label, path string) (string, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		log.Printf("borgee-helper: cannot read %s %q (%v); skipping heartbeat", label, path, err)
		return "", false
	}
	value := trim(string(raw))
	if value == "" {
		log.Printf("borgee-helper: %s %q is empty; skipping heartbeat", label, path)
		return "", false
	}
	return value, true
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n') {
		s = s[:len(s)-1]
	}
	return s
}
