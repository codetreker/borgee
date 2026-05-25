// Package main — borgee single-binary entry point.
//
// Dispatches to one of five subcommands:
//
//	borgee install ...          # operator one-shot bootstrap: setup → claim → start → wait heartbeat
//	borgee uninstall-host ...   # operator-driven local cleanup (mirror of `install`)
//	borgee daemon ...           # long-lived host-bridge daemon (User=borgee)
//	borgee rootd ...            # long-lived root-privileged companion daemon (User=root, narrow IPC whitelist)
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
//
// Internal-only helpers note (issue #1055): the prior top-level `borgee setup`
// and `borgee claim` subcommands were leftover internals from the .deb/.pkg
// era and produced non-functional installs when run standalone. They have
// been dropped from the public dispatch surface; their packages
// (`internal/cli/setup` and `internal/cli/claim`) remain in-tree and are
// invoked transitively by `borgee install`. If a real advanced use case for
// the standalone flow surfaces later, re-expose under an `internal`
// subcommand group rather than at the top level.
//
// Privilege-separation note (rootd-skeleton): `borgee daemon` runs as the
// `borgee` system user (no root); `borgee rootd` is the new companion that
// runs as root, listens on a local UDS, and accepts only a hardcoded
// command whitelist. The main daemon forwards root-requiring jobs over
// this IPC. PR-1 ships only the rootd skeleton with a `ping` whitelist
// entry; the three real root commands land in PR-4.
package main

import (
	"fmt"
	"io"
	"os"

	"borgee/internal/cli/daemon"
	"borgee/internal/cli/install"
	"borgee/internal/cli/installbutler"
	"borgee/internal/cli/rootd"
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
		// Surface the structured failure on stderr before exit. systemd
		// captures stderr into the journal — operators see the reason
		// (e.g. "--grants-db is required" or a sandbox-apply failure)
		// instead of a bare non-zero exit. Subcommands also write their
		// own context-rich diagnostics; this last-line guard ensures the
		// final error is never silently swallowed.
		fmt.Fprintf(os.Stderr, "borgee %s: %v\n", sub, err)
		os.Exit(1)
	}
}

func dispatch(sub string, args []string, stdout, stderr io.Writer) error {
	switch sub {
	case "daemon":
		return daemon.Run(args, stdout, stderr)
	case "rootd":
		return rootd.Run(args, stdout, stderr)
	case "install":
		return install.Run(args, stdout, stderr)
	case "install-plugin":
		return installbutler.Run(args, stdout, stderr)
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
	fmt.Fprintln(w, "  daemon           Long-lived host-bridge daemon (started by systemd / launchd, User=borgee).")
	fmt.Fprintln(w, "  rootd            Long-lived root companion daemon — narrow IPC whitelist (User=root).")
	fmt.Fprintln(w, "  install-plugin   Signed-manifest plugin binary installer (HB-1; was: install).")
	fmt.Fprintln(w, "  version          Print version.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run `borgee <subcommand> --help` for subcommand-specific flags.")
}
