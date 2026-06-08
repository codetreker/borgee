// Package main — borgee single-binary entry point.
//
// Dispatches to one of two subcommands:
//
//	borgee install ...          # operator one-shot bootstrap (rebuilt by T3b)
//	borgee daemon ...           # long-lived host-bridge daemon (rebuilt by T3b)
//	borgee --version            # version metadata (injected at link time)
//
// The dispatcher is intentionally tiny so each subcommand's flag-parsing,
// help, and exit behavior live in its own package and can be unit-tested
// against a `Run(args, stdout, stderr) error` API.
//
// t3a (binary strip) note: the high-privilege host subcommands (rootd,
// install-plugin, uninstall-host) and their backing packages were removed.
// The `install` and `daemon` subcommands are preserved as fail-loud stubs;
// T3b rebuilds their bodies (reverse-WS daemon + operator bootstrap).
package main

import (
	"fmt"
	"io"
	"os"

	"borgee/internal/cli/daemon"
	"borgee/internal/cli/install"
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
	fmt.Fprintln(w, "  daemon           Long-lived host-bridge daemon (started by systemd / launchd, User=borgee).")
	fmt.Fprintln(w, "  version          Print version.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run `borgee <subcommand> --help` for subcommand-specific flags.")
}
