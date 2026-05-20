// Package main — borgee single-binary entry point.
//
// Dispatches to one of six subcommands:
//
//	borgee install ...          # operator one-shot bootstrap: setup → claim → start → wait heartbeat
//	borgee uninstall-host ...   # operator-driven local cleanup (mirror of `install`)
//	borgee daemon ...           # long-lived host-bridge daemon
//	borgee claim ...            # one-time enrollment claim (advanced/recovery)
//	borgee setup ...            # systemd/launchd unit + state-dir bootstrap (advanced/recovery)
//	borgee install-plugin ...   # signed-manifest plugin binary installer (was: borgee install)
//	borgee --version            # version metadata (injected at link time)
//
// The dispatcher is intentionally tiny so each subcommand's flag-parsing,
// help, and exit behavior live in its own package and can be unit-tested
// against a `Run(args, stdout, stderr) error` API.
//
// Rename note (chore/install-onecmd): the prior `borgee install` (HB-1
// signed-manifest plugin installer, package installbutler) moved to
// `borgee install-plugin`. The new `borgee install` is the operator-facing
// one-shot bootstrap that wraps setup + claim + start. The web-UI / install-
// butler workflow continues to invoke `install-plugin` for runtime plugins.
package main

import (
	"fmt"
	"io"
	"os"

	"borgee/internal/cli/claim"
	"borgee/internal/cli/daemon"
	"borgee/internal/cli/install"
	"borgee/internal/cli/installbutler"
	"borgee/internal/cli/setup"
	"borgee/internal/cli/uninstallhost"
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
		return install.Run(args, stdout, stderr)
	case "install-plugin":
		return installbutler.Run(args, stdout, stderr)
	case "setup":
		return setup.Run(args, stdout, stderr)
	case "uninstall-host":
		return uninstallhost.Run(args, stdout, stderr)
	case "uninstall":
		// Helper-side uninstall driven by the server job dispatcher
		// (jobpolicy.JobTypeHelperUninstall, executor in
		// internal/executors/uninstall) is the web-UI path. `uninstall-host`
		// is the operator-driven local cleanup mirror of `install`. Print a
		// pointer so an operator who guesses `uninstall` gets routed.
		fmt.Fprintln(stdout, "Use either:")
		fmt.Fprintln(stdout, "  - Web UI \"Uninstall helper\" (server-job driven), or")
		fmt.Fprintln(stdout, "  - `sudo borgee uninstall-host` (operator-driven local cleanup, mirrors `borgee install`).")
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
	fmt.Fprintln(w, "  install          One-shot operator bootstrap: setup + claim + start + wait heartbeat.")
	fmt.Fprintln(w, "  uninstall-host   Operator-driven local cleanup (mirror of `install`).")
	fmt.Fprintln(w, "  daemon           Long-lived host-bridge daemon (started by systemd / launchd).")
	fmt.Fprintln(w, "  claim            One-time enrollment claim (advanced; `install` invokes this).")
	fmt.Fprintln(w, "  setup            Install systemd unit / launchd plist + state dirs (advanced; `install` invokes this).")
	fmt.Fprintln(w, "  install-plugin   Signed-manifest plugin binary installer (HB-1; was: install).")
	fmt.Fprintln(w, "  version          Print version.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run `borgee <subcommand> --help` for subcommand-specific flags.")
}
