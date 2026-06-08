// Package daemon implements `borgee daemon` — the long-lived reverse-WS
// runner. It resolves the server URL, token, and allowed directories, then
// drives a remotews.Client until SIGINT/SIGTERM (graceful, exit 0) or the
// server rejects the token (exit non-zero).
package daemon

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"borgee/internal/remotews"
	"borgee/internal/tokenstore"
)

// Run parses the daemon flags and blocks serving the reverse-WS connection.
func Run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("daemon", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		server    = fs.String("server", "", "Borgee server WebSocket URL (e.g. ws://localhost:4900)")
		token     = fs.String("token", "", "Connection token (first run; persisted on first handshake). Optional if a token is already persisted.")
		tokenFile = fs.String("token-file", tokenstore.DefaultTokenPath(), "Path to the persisted token file (mode 0600, owner-only)")
		dirs      = fs.String("dirs", "", "Comma-separated list of directories to expose (read-only)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *server == "" {
		return errors.New("--server is required")
	}
	if err := requireNonRoot(os.Getuid); err != nil {
		return err
	}

	allowed := splitDirs(*dirs)
	if len(allowed) == 0 {
		return errors.New("--dirs is required (at least one directory)")
	}

	// Resolve the token: --token wins; else fall back to the persisted file.
	// Persist-on-first-handshake only when the token came from --token (a
	// fresh enrollment), mirroring cli.ts's persistFirstHandshake gate.
	cliToken := strings.TrimSpace(*token)
	resolvedToken := cliToken
	persistFirstHandshake := cliToken != ""
	if resolvedToken == "" {
		persisted, ok := tokenstore.ReadToken(*tokenFile)
		if !ok {
			return fmt.Errorf(
				"no token provided via --token and no persisted token at %s.\n"+
					"Run with --token <one-shot from the Borgee UI> the first time; "+
					"subsequent runs (including after reboot) read the persisted file automatically.",
				*tokenFile,
			)
		}
		resolvedToken = persisted
		fmt.Fprintf(stdout, "borgee daemon: loaded persisted token from %s\n", *tokenFile)
	}

	fmt.Fprintf(stdout, "borgee daemon: serving %s\n", strings.Join(allowed, ", "))

	cfg := remotews.Config{
		ServerURL:   *server,
		Token:       resolvedToken,
		AllowedDirs: allowed,
	}
	if persistFirstHandshake {
		tf := *tokenFile
		cfg.OnFirstHandshake = func(t string) {
			if err := tokenstore.WriteToken(tf, t); err != nil {
				// Non-fatal: losing the connection is worse than failing to
				// persist; the operator can re-pass --token.
				fmt.Fprintf(stderr, "borgee daemon: failed to persist token to %s: %v\n", tf, err)
				return
			}
			fmt.Fprintf(stdout, "borgee daemon: persisted token to %s\n", tf)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	client := remotews.New(cfg)
	if err := client.Run(ctx); err != nil {
		if errors.Is(err, remotews.ErrAuthRejected) {
			return errors.New("server rejected the token; the persisted token may have been revoked — re-run with --token <new token>")
		}
		return err
	}
	return nil
}

// requireNonRoot refuses to run as root. The daemon must run as the install
// user — it never needs nor should hold root. getuid is injected for testing.
func requireNonRoot(getuid func() int) error {
	if getuid() == 0 {
		return errors.New("refusing to run as root: borgee daemon must run as the install user (non-root)")
	}
	return nil
}

// splitDirs splits a comma-separated list, trims, and drops empties (mirrors
// cli.ts:85).
func splitDirs(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
