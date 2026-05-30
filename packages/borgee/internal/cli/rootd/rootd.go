//go:build linux || darwin

// Package rootd — `borgee rootd` subcommand: privilege-separated companion
// daemon to `borgee daemon`. Runs as root, listens on a UDS, accepts only
// a hardcoded command whitelist. The main daemon (running as the installing
// user) forwards root-requiring jobs over this IPC.
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
	"strconv"
	"syscall"
)

const (
	// defaultSocketLinux is the legacy fallback UDS path. Installed units pass
	// a per-uid --socket path under /run/borgee/<uid>/.
	defaultSocketLinux = "/run/borgee/borgee-rootd.sock"
	// defaultSocketDarwin is the legacy fallback UDS path. Installed units pass
	// a per-uid --socket path under /Users/Shared/Borgee/<uid>/.
	defaultSocketDarwin = "/Users/Shared/Borgee/borgee-rootd.sock"

	// PeerGroup is the legacy group-gated fallback. Installed units pass
	// --allowed-peer-uid and disable group-based authorization.
	PeerGroup = "borgee"
)

// Run is the entry called by the dispatcher in cmd/borgee/main.go.
func Run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("borgee rootd", flag.ContinueOnError)
	fs.SetOutput(stderr)
	socket := fs.String("socket", defaultSocket(), "UDS path to listen on")
	allowedPeerUID := fs.Int("allowed-peer-uid", -1, "Only allow this uid to connect to rootd")
	socketOwnerUID := fs.Int("socket-owner-uid", -1, "UID to chown the rootd socket to")
	socketOwnerGID := fs.Int("socket-owner-gid", -1, "GID to chown the rootd socket to")
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

	var allowed *uint32
	if *allowedPeerUID >= 0 {
		v := uint32(*allowedPeerUID)
		allowed = &v
	}
	var ownerUID *int
	if *socketOwnerUID >= 0 {
		v := *socketOwnerUID
		ownerUID = &v
	}
	var ownerGID *int
	if *socketOwnerGID >= 0 {
		v := *socketOwnerGID
		ownerGID = &v
	}
	peerGroup := PeerGroup
	if allowed != nil {
		peerGroup = ""
	}

	logger := log.New(stderr, "borgee-rootd: ", log.LstdFlags|log.Lmicroseconds)
	srv := &Server{
		SocketPath:     *socket,
		PeerGroup:      peerGroup,
		AllowedPeerUID: allowed,
		SocketOwnerUID: ownerUID,
		SocketOwnerGID: ownerGID,
		Logger:         logger.Printf,
		Handlers:       DefaultHandlers(),
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	uidNote := "group=" + peerGroup
	if allowed != nil {
		uidNote = "uid=" + strconv.Itoa(int(*allowed))
	}
	fmt.Fprintf(stdout, "borgee rootd: listening on %s (%s whitelist: %v)\n", *socket, uidNote, srv.handlerNames())
	return srv.Serve(ctx)
}

func defaultSocket() string {
	if runtime.GOOS == "darwin" {
		return defaultSocketDarwin
	}
	return defaultSocketLinux
}
