//go:build linux || darwin

// Package daemon — borgee daemon subcommand (HB-2 v0(D) host-bridge).
// 平台 transport: POSIX UDS via net.Listen("unix", path).
//
// hb-2-v0d-spec.md §0.2: real sandbox, real IO, and real SQLite consumer.
// The internal `setup` helper invoked by `borgee install` (or external
// system packaging) installs the systemd unit / launchd plist that
// supervises this subcommand:
//   - Linux: systemd unit + landlock LSM
//   - macOS: launchd + sandbox-exec wrapper
//   - DSN: --grants-db=/var/lib/borgee/server.db?mode=ro
package daemon

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"flag"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"borgee/internal/acl"
	"borgee/internal/audit"
	"borgee/internal/dispatch"
	"borgee/internal/executors/delegationrevoke"
	"borgee/internal/executors/installplugin"
	"borgee/internal/executors/openclawconfigure"
	"borgee/internal/executors/pluginconfigure"
	"borgee/internal/executors/pluginremove"
	"borgee/internal/executors/servicelifecycle"
	"borgee/internal/executors/statuscollect"
	"borgee/internal/executors/statewrite"
	"borgee/internal/executors/uninstall"
	"borgee/internal/grants"
	"borgee/internal/ipc"
	"borgee/internal/jobpolicy"
	"borgee/internal/outbound"
	"borgee/internal/rootdclient"
	"borgee/internal/sandbox"
	"borgee/internal/updatecheck"
)

// Run is the entry for `borgee daemon`. Dispatcher in cmd/borgee passes the
// remaining argv + stdio so subcommand --help/error output stays consistent
// across the four subcommands.
func Run(args []string, _ io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("borgee daemon", flag.ContinueOnError)
	fs.SetOutput(stderr)
	socket := fs.String("socket", "/run/borgee/borgee.sock", "UDS path (Linux/macOS)")
	auditLog := fs.String("audit-log", "/var/log/borgee/audit.log.jsonl", "audit JSON-line path")
	grantsDSN := fs.String("grants-db", "", "sqlite DSN for HB-3 host_grants table (read-only) — REQUIRED for production")
	readPathsFlag := fs.String("read-paths", "", "comma-separated absolute paths landlock allows (v0(D) static; v1+ pulls live from host_grants)")
	outboundServerOrigin := fs.String("outbound-server-origin", "", "Borgee API server origin for Helper outbound calls")
	outboundAllowedOrigins := fs.String("outbound-allowed-origins", "", "comma-separated exact Borgee API origins allowed for Helper outbound calls")
	queueStateDir := fs.String("queue-state-dir", "", "Helper-owned queue cursor state directory")
	statusStateDir := fs.String("status-state-dir", "", "Helper-owned bounded status state directory")
	auditHandoffDir := fs.String("audit-handoff-dir", "", "Helper-owned local audit handoff directory")
	// All three are file-based so secrets never appear on /proc/PID/cmdline
	// (the systemd drop-in passes paths only; the claim CLI populates the
	// files at install/claim time). Missing/empty files → heartbeat is
	// silently skipped so the daemon still boots before claim happens.
	enrollmentIDFile := fs.String("enrollment-id-file", "", "Path to file containing the helper enrollment id (#968 heartbeat producer config)")
	helperDeviceIDFile := fs.String("helper-device-id-file", "", "Path to file containing the helper device id bound at claim (#968 heartbeat)")
	helperCredentialFile := fs.String("helper-credential-file", "", "Path to file containing the helper credential (Bearer token) issued at claim; readable only by helper user (#968 heartbeat)")
	allowLoopbackOutbound := fs.Bool("allow-loopback-outbound", false, "Permit http:// loopback as --outbound-server-origin (e2e tests only; production daemon always uses https)")
	allowedStateRoots := fs.String("allowed-state-roots", "", "Comma-separated list of allowed parent directories for queue/status/audit-handoff state dirs; empty defaults to platform helper state root (production default; e2e tests override)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	outboundPrereq := outbound.PrereqConfig{
		ServerOrigin:    *outboundServerOrigin,
		AllowedOrigins:  *outboundAllowedOrigins,
		QueueStateDir:   *queueStateDir,
		StatusStateDir:  *statusStateDir,
		AuditHandoffDir: *auditHandoffDir,
	}
	return run(*socket, *auditLog, *grantsDSN, *readPathsFlag, outboundPrereq, *enrollmentIDFile, *helperDeviceIDFile, *helperCredentialFile, *allowLoopbackOutbound, *allowedStateRoots)
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

	// PR-2 #1038: heartbeat is now WS ping/pong (built into the
	// outbound.Client.pingLoop spawned by Client.Dial). The legacy
	// standalone POST /status producer was replaced by the persistent
	// WS transport's pong-side last_seen_at update on the server. The
	// dispatcher's outbound client is therefore the single producer
	// of the freshness signal — no separate Heartbeater wiring.

	// #1001 + #1002 dispatcher: WS-pushed leased jobs → policy gate →
	// per-job executor → Ack/Result frames. Reconnect with exponential
	// backoff (1s base, 30s cap, ±20% jitter) is handled inside
	// outbound.Client. Pre-claim daemons skip dispatch — the persistent
	// WS dial requires credential + device id + enrollment id, which
	// the operator populates by running `borgee install` (which invokes
	// the internal `claim` helper).
	if dp, ok := buildDispatcher(preparedOutbound, enrollmentIDFile, helperDeviceIDFile, helperCredentialFile); ok {
		log.Printf("borgee-helper: dispatcher enabled enrollment_id=%s (WS transport)", dp.EnrollmentID)
		go func() {
			if err := dp.Run(ctx); err != nil {
				log.Printf("borgee-helper: dispatcher exited: %v", err)
			}
		}()
	} else {
		log.Printf("borgee-helper: no enrollment configured, skipping job dispatcher")
	}

	// #999 update-detection: piggy-back the same enrollment + credential
	// state the heartbeat / dispatcher already require. Same skip-on-
	// pre-claim semantics so a fresh install boots without the loop;
	// the install-butler / claim sequence will populate the files and
	// next daemon restart picks them up.
	if uc, ok := buildUpdateChecker(preparedOutbound, enrollmentIDFile, helperDeviceIDFile, helperCredentialFile); ok {
		log.Printf("borgee-helper: update-checker enabled enrollment_id=%s interval=%s", uc.EnrollmentID, updatecheck.DefaultInterval)
		go func() {
			if err := uc.Run(ctx); err != nil {
				log.Printf("borgee-helper: update-checker exited: %v", err)
			}
		}()
	} else {
		log.Printf("borgee-helper: no enrollment configured, skipping update-checker")
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

// buildDispatcher (PR-2 #1038) constructs the persistent WS-transport
// dispatcher when the daemon has everything it needs (enrollment id +
// device id + credential present + outbound prereqs validated). When
// any input is missing/unreadable, returns (nil,false) so the daemon
// still boots and serves the local UDS contract — claim populates the
// state directory after first start.
//
// All three inputs are *file paths* (not raw values): keeping secrets
// out of the cmdline avoids /proc/PID/cmdline leakage.
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
	// PR-4 #1033 — rootd companion client. Uses the platform-default
	// UDS path (`/run/borgee/borgee-rootd.sock` on Linux,
	// `/Users/Shared/Borgee/borgee-rootd.sock` on darwin). The
	// install_plugin / service.lifecycle / delegation.revoke executors
	// dial this client to forward root-requiring operations into the
	// privileged `borgee rootd` companion daemon.
	rootd := &rootdclient.Client{SocketPath: rootdclient.DefaultSocket()}
	dispatcher := &dispatch.Dispatcher{
		Client:          client,
		EnrollmentID:    enrollmentID,
		PolicyEvaluator: defaultPolicyEvaluator(),
	}
	dispatcher.Executors = map[string]dispatch.Executor{
		// #998 helper.uninstall — one-key self-teardown.
		jobpolicy.JobTypeHelperUninstall: &uninstall.Executor{Logger: log.Printf},
		// PR-3 #1041 — the four no-root executors.
		jobpolicy.JobTypeStatusCollect: &statuscollect.Executor{
			InstalledVersionsPath: updatecheck.DefaultInstalledVersionsPath,
			Logger:                log.Printf,
		},
		jobpolicy.JobTypeStateWrite:                &statewrite.Executor{Logger: log.Printf},
		jobpolicy.JobTypeOpenClawConfigureAgent:    &openclawconfigure.Executor{Logger: log.Printf},
		jobpolicy.JobTypePluginConfigureConnection: &pluginconfigure.Executor{Logger: log.Printf},
		// #1049 — remove a previously-configured per-connection plugin record.
		// Mirrors pluginconfigure but deletes instead of writes; idempotent
		// on missing file.
		jobpolicy.JobTypePluginRemoveConnection: &pluginremove.Executor{Logger: log.Printf},
		// PR-4 #1033 — three root-requiring executors. All three
		// forward the actual privileged operation into `borgee rootd`
		// via the shared rootdclient. Paths / unit names / target
		// directories come from the signed manifest carried in each
		// leased job, NOT from any daemon-startup flag.
		jobpolicy.JobTypeOpenClawInstallFromManifest: &installplugin.Executor{
			Rootd:        rootd,
			PubKeyBase64: os.Getenv("BORGEE_MANIFEST_SIGNING_PUBKEY"),
			Logger:       log.Printf,
		},
		jobpolicy.JobTypeServiceLifecycle: &servicelifecycle.Executor{
			Rootd:  rootd,
			Logger: log.Printf,
		},
		jobpolicy.JobTypeDelegationRevoke: &delegationrevoke.Executor{
			Rootd:      rootd,
			Dispatcher: dispatcher,
			Logger:     log.Printf,
		},
	}
	return dispatcher, true
}

// buildUpdateChecker mirrors the buildHeartbeater / buildDispatcher
// pre-claim skip semantics. #999 update detection — the loop POSTs the
// installed-versions snapshot every updatecheck.DefaultInterval; server
// computes drift authoritatively against the signed manifest and returns
// the per-class list. Helper logs each drift entry with class-driven
// severity. Apply is NOT wired here per blueprint §1.3 (auto-apply banned).
func buildUpdateChecker(prep outbound.PreparedConfig, enrollmentIDFile, helperDeviceIDFile, credentialFile string) (*updatecheck.Checker, bool) {
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
	return &updatecheck.Checker{
		ServerOrigin:   prep.ServerOrigin,
		EnrollmentID:   enrollmentID,
		HelperDeviceID: helperDeviceID,
		Credential:     credential,
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
//
// PR-4 amend (#1033): TrustRoots is populated from
// BORGEE_MANIFEST_SIGNING_PUBKEY (same env var the installplugin executor
// already reads) so jobpolicy.verifyManifestAuthority can verify the
// signed manifest body the server now emits in every leased-job
// payload. Empty env → empty TrustRoots → manifest-required jobs land
// in ReasonManifestInvalid (Evaluate's documented "no trust roots"
// path); that is the safe production default until ops rotates a key in.
//
// PR-4 amend gap #1: the envelope fields (owner_user_id, org_id,
// helper_device_id, category, payload_hash, expires_at) now flow from
// the server's lease frame into jobpolicy.Job, so validateJobSchema
// passes. Without those six fields the prior code constructed an
// envelope of empty strings and every pushed job got rejected with
// ReasonSchemaInvalid before the executor ran. Enrollment is built
// from the same envelope (the server vouches for the binding via the
// WS credential gate, so the daemon doesn't need a duplicate local
// store yet) — this keeps validateLocalState a no-op until the helper
// installs its own enrollment cache in a later milestone.
func defaultPolicyEvaluator() dispatch.PolicyEvaluator {
	trustRoots := loadHelperManifestTrustRoots()
	return func(_ context.Context, job *outbound.LeasedJob) jobpolicy.Decision {
		if job == nil {
			return jobpolicy.Decision{Allow: false, Reason: jobpolicy.ReasonSchemaInvalid}
		}
		var expires time.Time
		if job.ExpiresAt > 0 {
			expires = time.UnixMilli(job.ExpiresAt)
		}
		return jobpolicy.Evaluate(jobpolicy.EvaluationInput{
			TrustRoots: trustRoots,
			Job: jobpolicy.Job{
				JobID:                job.JobID,
				OwnerUserID:          job.OwnerUserID,
				OrgID:                job.OrgID,
				EnrollmentID:         job.EnrollmentID,
				HelperDeviceID:       job.HelperDeviceID,
				CredentialGeneration: 1,
				JobType:              job.JobType,
				Category:             job.Category,
				SchemaVersion:        job.SchemaVersion,
				PayloadJSON:          job.Payload,
				PayloadHash:          job.PayloadHash,
				ManifestDigest:       job.ManifestDigest,
				ManifestJSON:         job.ManifestJSON,
				ManifestBindingJSON:  job.ManifestBindingJSON,
				ExpiresAt:            expires,
			},
			Enrollment: jobpolicy.EnrollmentState{
				OwnerUserID:          job.OwnerUserID,
				OrgID:                job.OrgID,
				EnrollmentID:         job.EnrollmentID,
				HelperDeviceID:       job.HelperDeviceID,
				CredentialGeneration: 1,
				Status:               "active",
				AllowedCategories:    []string{job.Category},
			},
		})
	}
}

// loadHelperManifestTrustRoots decodes the daemon's startup
// BORGEE_MANIFEST_SIGNING_PUBKEY env var into an ed25519 public key
// slice for jobpolicy.EvaluationInput.TrustRoots. Multiple roots are
// supported via comma-separation so future key rotations can run with a
// grace window (current+next pubkey both valid). Empty / malformed env
// produces an empty slice — Evaluate rejects manifest-required jobs
// safely under that condition.
func loadHelperManifestTrustRoots() []ed25519.PublicKey {
	raw := strings.TrimSpace(os.Getenv("BORGEE_MANIFEST_SIGNING_PUBKEY"))
	if raw == "" {
		return nil
	}
	var roots []ed25519.PublicKey
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(entry)
		if err != nil || len(decoded) != ed25519.PublicKeySize {
			log.Printf("borgee-helper: BORGEE_MANIFEST_SIGNING_PUBKEY entry invalid (len=%d err=%v); skipping", len(decoded), err)
			continue
		}
		roots = append(roots, ed25519.PublicKey(decoded))
	}
	return roots
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
