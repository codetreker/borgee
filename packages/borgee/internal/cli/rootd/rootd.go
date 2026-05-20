//go:build linux || darwin

// Package rootd — `borgee rootd` subcommand: privilege-separated companion
// daemon to `borgee daemon`. Runs as root, listens on a UDS, accepts only
// a hardcoded command whitelist. The main daemon (running as the `borgee`
// system user) forwards root-requiring jobs over this IPC.
//
// PR-1 scope: skeleton only — the whitelist contains a single `ping`
// handler that round-trips a `{"pong": true, "time": <unix ms>}` envelope.
// Three real commands (install_plugin, service_lifecycle,
// delegation_revoke) land in PR-4 by extending the whitelist; no other
// part of the wire protocol changes.
//
// Threat model:
//
//   - The main daemon parses untrusted server payloads over WebSocket and
//     is the high-attack-surface side. It deliberately does not hold root.
//   - rootd holds root but exposes only a tiny, hardcoded API surface
//     (the Handlers map) over a peer-credential-gated Unix socket. The
//     UDS is locked to mode 0660 and chown root:borgee at Listen time;
//     each accepted connection is then verified to belong to the `borgee`
//     group (SO_PEERCRED on Linux, getpeereid on macOS).
//   - Every accepted/rejected request is logged with cmd + peer uid +
//     ok — this is the audit trail an operator (or post-incident review)
//     uses to scope blast radius.
package rootd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

const (
	// defaultSocketLinux is the canonical UDS path baked into the
	// borgee-rootd.service unit. Lives under /run/borgee so reboot does
	// not leave a stale socket (tmpfs).
	defaultSocketLinux = "/run/borgee/borgee-rootd.sock"
	// defaultSocketDarwin lives under /Users/Shared so non-root accounts
	// in the `borgee` group can find it; the file itself is chmod 0660
	// + chown root:borgee.
	defaultSocketDarwin = "/Users/Shared/Borgee/borgee-rootd.sock"

	// PeerGroup is the unix group whose members are allowed to connect
	// to the rootd UDS. The main `borgee daemon` runs as user `borgee`
	// which is in the `borgee` group; rootd refuses peers whose primary
	// gid is not in this group.
	PeerGroup = "borgee"
)

// Run is the entry called by the dispatcher in cmd/borgee/main.go.
func Run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("borgee rootd", flag.ContinueOnError)
	fs.SetOutput(stderr)
	socket := fs.String("socket", defaultSocket(), "UDS path to listen on")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return fmt.Errorf("unsupported platform %q (linux/darwin only)", runtime.GOOS)
	}
	if os.Geteuid() != 0 {
		fmt.Fprintln(stderr, "borgee rootd: must be run as root")
		return errors.New("not root")
	}

	logger := log.New(stderr, "borgee-rootd: ", log.LstdFlags|log.Lmicroseconds)
	srv := &Server{
		SocketPath: *socket,
		PeerGroup:  PeerGroup,
		Logger:     logger.Printf,
		Handlers:   DefaultHandlers(),
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	fmt.Fprintf(stdout, "borgee rootd: listening on %s (whitelist: %v)\n", *socket, srv.handlerNames())
	return srv.Serve(ctx)
}

func defaultSocket() string {
	if runtime.GOOS == "darwin" {
		return defaultSocketDarwin
	}
	return defaultSocketLinux
}
