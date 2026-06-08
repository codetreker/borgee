// Package main — borgee single-binary entry point.
//
// Dispatches to one of three subcommands:
//
//	borgee install ...     # enroll this machine: write a systemd --user service
//	borgee daemon ...      # long-lived reverse-WS daemon serving ls/read/stat
//	borgee uninstall ...   # stop and remove the service
//	borgee --version       # version metadata (injected at link time)
//
// The dispatcher is intentionally tiny so each subcommand's flag-parsing,
// help, and exit behavior live in its own package and can be unit-tested
// against a `Run(args, stdout, stderr) error` API.
package main

import (
	"fmt"
	"io"
	"os"

	"borgee/internal/cli/daemon"
	"borgee/internal/cli/install"
	"borgee/internal/cli/uninstall"
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
	case "install":
		return install.Run(args, stdout, stderr)
	case "uninstall":
		return uninstall.Run(args, stdout, stderr)
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
	fmt.Fprintln(w, "  install          Enroll this machine: --server --token --dirs (writes a systemd --user service).")
	fmt.Fprintln(w, "  daemon           Long-lived reverse-WS daemon that serves ls/read/stat from the allowed dirs.")
	fmt.Fprintln(w, "  uninstall        Stop and remove the service (--purge also wipes the saved token).")
	fmt.Fprintln(w, "  version          Print version.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run `borgee <subcommand> --help` for subcommand-specific flags.")
}
