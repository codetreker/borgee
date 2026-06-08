// Regression test for the public dispatch surface after t3a (binary strip)
// removed the high-privilege subcommands. Only `install` and `daemon`
// remain routed; `setup`, `claim`, `rootd`, `install-plugin`, and
// `uninstall-host` must all be rejected with the standard "unknown
// subcommand" error and must not appear in the usage banner.

package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestDispatchPublicSubcommands locks in the public dispatch surface after
// t3a (binary strip) dropped the high-privilege subcommands.
func TestDispatchPublicSubcommands(t *testing.T) {
	t.Run("removed_subcommands_return_unknown", func(t *testing.T) {
		for _, sub := range []string{"setup", "claim", "rootd", "install-plugin", "uninstall-host"} {
			sub := sub
			t.Run(sub, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				err := dispatch(sub, []string{}, &stdout, &stderr)
				if err == nil {
					t.Fatalf("dispatch(%q) returned nil error; want non-nil unknown-subcommand error", sub)
				}
				wantMsg := `unknown subcommand "` + sub + `"`
				if !strings.Contains(err.Error(), wantMsg) {
					t.Errorf("dispatch(%q) err = %q; want substring %q", sub, err.Error(), wantMsg)
				}
				if !strings.Contains(stderr.String(), wantMsg) {
					t.Errorf("dispatch(%q) stderr = %q; want substring %q", sub, stderr.String(), wantMsg)
				}
			})
		}
	})

	t.Run("public_subcommands_route", func(t *testing.T) {
		// Each public subcommand must be routed to its own Run() —
		// confirmed by the dispatcher NOT returning the "unknown
		// subcommand" sentinel error. The downstream Run may itself
		// return an error (e.g. the t3a fail-loud "not implemented"
		// stub) — that is fine; what we are locking in is that
		// dispatch routes the string at all.
		for _, sub := range []string{
			"install",
			"daemon",
		} {
			sub := sub
			t.Run(sub, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				err := dispatch(sub, []string{"--help"}, &stdout, &stderr)
				// Whatever the downstream Run returns, it must
				// not be the dispatcher's unknown-subcommand
				// rejection.
				if err != nil && strings.Contains(err.Error(), "unknown subcommand") {
					t.Fatalf("dispatch(%q, --help) returned unknown-subcommand error; want routed to subcommand: err=%v stderr=%q", sub, err, stderr.String())
				}
				if strings.Contains(stderr.String(), `unknown subcommand "`+sub+`"`) {
					t.Fatalf("dispatch(%q, --help) wrote unknown-subcommand line to stderr; want routed to subcommand", sub)
				}
			})
		}
	})

	t.Run("help_and_version_zero_error", func(t *testing.T) {
		// `--help` and `--version` are handled inside dispatch and
		// must return nil. Version output must contain the binary
		// name + version string (`borgee `).
		for _, sub := range []string{"--help", "-h", "help"} {
			var stdout, stderr bytes.Buffer
			if err := dispatch(sub, nil, &stdout, &stderr); err != nil {
				t.Errorf("dispatch(%q) err = %v; want nil", sub, err)
			}
			if !strings.Contains(stdout.String(), "Usage: borgee") {
				t.Errorf("dispatch(%q) stdout missing usage banner; got %q", sub, stdout.String())
			}
		}
		for _, sub := range []string{"--version", "-v", "version"} {
			var stdout, stderr bytes.Buffer
			if err := dispatch(sub, nil, &stdout, &stderr); err != nil {
				t.Errorf("dispatch(%q) err = %v; want nil", sub, err)
			}
			if !strings.Contains(stdout.String(), "borgee ") {
				t.Errorf("dispatch(%q) stdout = %q; want substring %q", sub, stdout.String(), "borgee ")
			}
		}
	})

	t.Run("usage_banner_lists_public_subcommands_no_removed_ones", func(t *testing.T) {
		var buf bytes.Buffer
		usage(&buf)
		out := buf.String()
		for _, want := range []string{
			"install",
			"daemon",
		} {
			// Match the listed subcommand at the start of a banner
			// line (two-space indent) so a substring like `install`
			// doesn't false-positive on prose.
			marker := "\n  " + want + " "
			if !strings.Contains("\n"+out, marker) {
				t.Errorf("usage() missing subcommand %q; got:\n%s", want, out)
			}
		}
		for _, banned := range []string{
			"\n  setup ",
			"\n  claim ",
			"\n  rootd ",
			"\n  install-plugin ",
			"\n  uninstall-host ",
		} {
			if strings.Contains("\n"+out, banned) {
				t.Errorf("usage() still lists banned subcommand line %q; got:\n%s", strings.TrimSpace(banned), out)
			}
		}
	})

	t.Run("unknown_subcommand_error_shape", func(t *testing.T) {
		// Sanity check on the error sentinel shape so a refactor
		// renaming the error text trips this test.
		var stdout, stderr bytes.Buffer
		err := dispatch("nosuch-subcommand", nil, &stdout, &stderr)
		if err == nil {
			t.Fatal("dispatch(nosuch-subcommand) returned nil; want non-nil")
		}
		if !errors.Is(err, err) { // tautology guard — keeps imports honest
			t.Fatal("unreachable")
		}
		if !strings.Contains(err.Error(), `unknown subcommand "nosuch-subcommand"`) {
			t.Errorf("err = %q; want substring unknown subcommand", err.Error())
		}
	})
}
