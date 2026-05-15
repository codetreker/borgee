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
	"borgee-helper/internal/grants"
	"borgee-helper/internal/ipc"
	"borgee-helper/internal/outbound"
	"borgee-helper/internal/sandbox"
)

func main() {
	socket := flag.String("socket", "/run/borgee-helper/borgee-helper.sock", "UDS path (Linux/macOS)")
	auditLog := flag.String("audit-log", "/var/log/borgee-helper/audit.log.jsonl", "audit JSON-line path")
	grantsDSN := flag.String("grants-db", "", "sqlite DSN for HB-3 host_grants table (read-only) — REQUIRED for production")
	readPathsFlag := flag.String("read-paths", "", "comma-separated absolute paths landlock allows (v0(D) static; v1+ pulls live from host_grants)")
	outboundServerOrigin := flag.String("outbound-server-origin", "", "Borgee API server origin for future Helper outbound polling prerequisites")
	outboundAllowedOrigins := flag.String("outbound-allowed-origins", "", "comma-separated exact Borgee API origins allowed for future Helper outbound polling prerequisites")
	queueStateDir := flag.String("queue-state-dir", "", "Helper-owned queue cursor state directory for future outbound polling prerequisites")
	statusStateDir := flag.String("status-state-dir", "", "Helper-owned bounded status state directory for future outbound polling prerequisites")
	auditHandoffDir := flag.String("audit-handoff-dir", "", "Helper-owned local audit handoff directory for future outbound polling prerequisites")
	flag.Parse()

	outboundPrereq := outbound.PrereqConfig{
		ServerOrigin:    *outboundServerOrigin,
		AllowedOrigins:  *outboundAllowedOrigins,
		QueueStateDir:   *queueStateDir,
		StatusStateDir:  *statusStateDir,
		AuditHandoffDir: *auditHandoffDir,
	}
	if err := run(*socket, *auditLog, *grantsDSN, *readPathsFlag, outboundPrereq); err != nil {
		log.Fatalf("borgee-helper: %v", err)
	}
}

func run(socket, auditLogPath, grantsDSN, readPaths string, outboundPrereq outbound.PrereqConfig) error {
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

	preparedOutbound, err := outbound.ValidateAndPrepare(outboundPrereq, outbound.ValidationOptions{})
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

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n') {
		s = s[:len(s)-1]
	}
	return s
}
