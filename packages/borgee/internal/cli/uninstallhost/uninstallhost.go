//go:build linux || darwin

// Package uninstallhost — `borgee uninstall-host` subcommand: operator-
// driven local cleanup mirror of `borgee install`. Distinct from the
// server-job `helper.uninstall` executor (internal/executors/uninstall)
// because uninstall-host:
//
//   - runs as a SEPARATE process from the daemon, so it is safe to issue
//     `systemctl stop borgee` first (the server-job executor can't
//     stop itself without SIGTERM-ing mid-cleanup before /result lands)
//   - is invoked directly by the operator via
//     `sudo npx @codetreker/borgee-remote-agent uninstall-host`
//   - shares the cleanup buckets (state dirs, runtime dir, sandbox
//     profile, service unit, OS user) with the server-job executor by
//     calling the same `uninstall.Executor` underneath
//
// Out of scope: removing the npm package itself. `/usr/local/bin/borgee`
// (if present from `npm i -g`) is an npm-owned symlink; we print a
// pointer telling the operator to run `npm uninstall -g
// @codetreker/borgee-remote-agent`. We can't reliably do that from
// inside an npx-invoked process because npx uses a temporary cache, not
// a global install.
package uninstallhost

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"

	"borgee/internal/cli/setup"
	"borgee/internal/dispatch"
	"borgee/internal/executors/uninstall"
	"borgee/internal/outbound"
)

// Run is the entry for `borgee uninstall-host`. Dispatcher in cmd/borgee
// passes the remaining argv + stdio.
func Run(args []string, stdout, stderr io.Writer) error {
	cfg, err := parseArgs(args, stderr)
	if err != nil {
		return err
	}
	return run(cfg, stdout, stderr)
}

type config struct {
	preserveState bool
	yes           bool
	skipStop      bool // testing hook
	skipExecutor  bool // testing hook
	skipRootCheck bool // testing hook: bypass sudo gate
	runner        systemRunner
	executor      executorRunner
}

type systemRunner interface {
	Run(ctx context.Context, name string, args ...string) error
}

type realRunner struct{}

func (realRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// executorRunner abstracts the helper-uninstall executor so tests can
// observe the cleanup call without touching the real filesystem.
type executorRunner interface {
	Execute(ctx context.Context, payload map[string]any) (dispatch.TerminalStatus, error)
}

func parseArgs(args []string, stderr io.Writer) (*config, error) {
	fs := flag.NewFlagSet("borgee uninstall-host", flag.ContinueOnError)
	fs.SetOutput(stderr)
	preserve := fs.Bool("preserve-state", false, "Keep state directories (queue / status / audit-handoff / credential) — operator can resume later by re-running claim")
	yes := fs.Bool("yes", false, "Skip the confirmation prompt (required for non-interactive flows)")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return &config{preserveState: *preserve, yes: *yes}, nil
}

// run is the testable entry. Returns nil on success.
func run(cfg *config, stdout, stderr io.Writer) error {
	if !cfg.skipRootCheck && os.Geteuid() != 0 {
		fmt.Fprintln(stderr, "borgee uninstall-host: must be run as root (use sudo)")
		return errors.New("not root")
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return fmt.Errorf("unsupported platform %q", runtime.GOOS)
	}

	fmt.Fprintln(stdout, "borgee uninstall-host: tearing down local helper footprint")

	// 1. Stop + disable the service first. This is safe BECAUSE we are
	//    a separate process from the daemon (unlike the server-job
	//    helper.uninstall executor which runs inside the daemon and so
	//    must NOT stop itself).
	if !cfg.skipStop {
		if err := stopService(cfg, stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "borgee uninstall-host: warn: stop service: %v\n", err)
		}
	}

	// 2. Run the cleanup executor (state dirs, runtime dir, sandbox
	//    profile, service unit, OS user). Shares its bucket model with
	//    the server-job executor so cleanup semantics are identical.
	if !cfg.skipExecutor {
		if err := runExecutor(cfg, stdout, stderr); err != nil {
			return fmt.Errorf("executor: %w", err)
		}
	}

	// 3. npm-aware pointer. The CLI cannot reliably `npm uninstall -g`
	//    itself because npx uses a temporary cache, not a global
	//    install. Print the operator-facing follow-up.
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "borgee uninstall-host: local cleanup done.")
	fmt.Fprintln(stdout, "If you installed via `npm i -g @codetreker/borgee-remote-agent`, finish with:")
	fmt.Fprintln(stdout, "  sudo npm uninstall -g @codetreker/borgee-remote-agent")
	fmt.Fprintln(stdout, "If you installed via `npx ...`, no further step is needed (npx cache is ephemeral).")
	return nil
}

func stopService(cfg *config, stdout, stderr io.Writer) error {
	r := cfg.runner
	if r == nil {
		r = realRunner{}
	}
	ctx := context.Background()
	switch runtime.GOOS {
	case "linux":
		fmt.Fprintln(stdout, "borgee uninstall-host: systemctl stop + disable (main + rootd companion)")
		// stop + disable are intentionally best-effort — a fresh host
		// where the service was never enabled returns non-zero but
		// the file-level cleanup should still proceed. Stop the main
		// daemon first so it stops forwarding to rootd, then stop
		// rootd. Disable in the same order.
		_ = r.Run(ctx, "systemctl", "stop", setup.LinuxServiceName)
		_ = r.Run(ctx, "systemctl", "stop", setup.LinuxRootdServiceName)
		_ = r.Run(ctx, "systemctl", "disable", setup.LinuxServiceName)
		_ = r.Run(ctx, "systemctl", "disable", setup.LinuxRootdServiceName)
	case "darwin":
		fmt.Fprintln(stdout, "borgee uninstall-host: launchctl bootout (main + rootd companion)")
		_ = r.Run(ctx, "launchctl", "bootout", "system/"+setup.DarwinPlistLabel)
		_ = r.Run(ctx, "launchctl", "bootout", "system/"+setup.DarwinRootdPlistLabel)
	}
	return nil
}

// realExecutor adapts the shared uninstall.Executor to the local
// executorRunner shape (which takes a payload map). The executor expects
// an *outbound.LeasedJob with a JSON-encoded payload; we build one here.
type realExecutor struct {
	exec *uninstall.Executor
}

func (e *realExecutor) Execute(ctx context.Context, payload map[string]any) (dispatch.TerminalStatus, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return dispatch.TerminalStatus{}, err
	}
	job := &outbound.LeasedJob{
		JobID:         "uninstall-host-cli",
		EnrollmentID:  "local",
		JobType:       "helper.uninstall",
		SchemaVersion: 1,
		Payload:       raw,
		LeaseToken:    "v1:local",
	}
	return e.exec.Execute(ctx, job)
}

func runExecutor(cfg *config, stdout, stderr io.Writer) error {
	var runner executorRunner
	if cfg.executor != nil {
		runner = cfg.executor
	} else {
		runner = &realExecutor{exec: &uninstall.Executor{
			Layout: uninstall.DefaultLayout(runtime.GOOS),
			GOOS:   runtime.GOOS,
			Logger: func(format string, v ...any) {
				fmt.Fprintf(stderr, "borgee uninstall-host: "+format+"\n", v...)
			},
		}}
	}
	terminal, err := runner.Execute(context.Background(), map[string]any{
		"scope":          "helper",
		"preserve_state": cfg.preserveState,
	})
	if err != nil {
		return err
	}
	if terminal.Status != dispatch.StatusSucceeded {
		return fmt.Errorf("executor reported terminal=%s code=%s msg=%s", terminal.Status, terminal.FailureCode, terminal.FailureMessage)
	}
	fmt.Fprintf(stdout, "borgee uninstall-host: executor succeeded (audit_refs=%v)\n", terminal.ResultSummary.AuditRefs)
	return nil
}
