// Package main — borgee single-binary entry point.
//
// Dispatches to one of four subcommands:
//
//	borgee daemon ...           # long-lived host-bridge daemon (was borgee-helper)
//	borgee claim ...            # one-time enrollment claim (was borgee-helper-claim)
//	borgee install ...          # signed-manifest binary installer (was install-butler)
//	borgee setup ...            # systemd/launchd unit + state-dir bootstrap (new; was .deb postinstall)
//	borgee uninstall            # convenience pointer to the helper.uninstall job (web UI)
//	borgee --version            # version metadata (injected at link time)
//
// The dispatcher is intentionally tiny so each subcommand's flag-parsing,
// help, and exit behavior live in its own package and can be unit-tested
// against a `Run(args, stdout, stderr) error` API.
package main

import (
	"fmt"
	"io"
	"os"

	"borgee/internal/cli/claim"
	"borgee/internal/cli/daemon"
	"borgee/internal/cli/installbutler"
	"borgee/internal/cli/setup"
)

// version is overridden via `-ldflags "-X main.version=..."` at release time.
// The default keeps `borgee --version` usable from a local `go build .` so an
// operator running off main can still report something coherent.
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(2)
	}
	sub := os.Args[1]
	args := os.Args[2:]
	if err := dispatch(sub, args, os.Stdout, os.Stderr); err != nil {
		// Subcommands write their own structured error to stderr; the
		// dispatcher only forwards the non-zero exit code.
		os.Exit(1)
	}
}

func dispatch(sub string, args []string, stdout, stderr io.Writer) error {
	switch sub {
	case "daemon":
		return daemon.Run(args, stdout, stderr)
	case "claim":
		return claim.Run(args, stdout, stderr)
	case "install":
		return installbutler.Run(args, stdout, stderr)
	case "setup":
		return setup.Run(args, stdout, stderr)
	case "uninstall":
		// Helper-side uninstall is performed by the helper.uninstall job
		// dispatcher (jobpolicy.JobTypeHelperUninstall, executor in
		// internal/executors/uninstall) so the operator runs it via the web
		// UI — keep the CLI a one-line pointer rather than a parallel path
		// that would drift from the job's audited cleanup sequence.
		fmt.Fprintln(stdout, "Use the web UI \"Uninstall helper\" action to invoke the helper.uninstall job. The daemon will tear down its state and exit.")
		return nil
	case "-h", "--help", "help":
		usage(stdout)
		return nil
	case "-v", "--version", "version":
		fmt.Fprintf(stdout, "borgee %s\n", version)
		return nil
	default:
		fmt.Fprintf(stderr, "borgee: unknown subcommand %q\n", sub)
		usage(stderr)
		return fmt.Errorf("unknown subcommand %q", sub)
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "Usage: borgee <subcommand> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Subcommands:")
	fmt.Fprintln(w, "  daemon       Long-lived host-bridge daemon (started by systemd / launchd).")
	fmt.Fprintln(w, "  claim        One-time enrollment claim (writes credential + enrollment-id + device-id files).")
	fmt.Fprintln(w, "  install      Signed-manifest binary installer (one-shot; verifies + atomically writes).")
	fmt.Fprintln(w, "  setup        Install systemd unit (Linux) or launchd plist (macOS), system user, and state dirs.")
	fmt.Fprintln(w, "  uninstall    Pointer to the helper.uninstall job (run via web UI).")
	fmt.Fprintln(w, "  version      Print version.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run `borgee <subcommand> --help` for subcommand-specific flags.")
}
