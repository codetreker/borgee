// Command migrate runs Borgee's forward-only schema migrations.
//
// Usage:
//
//	borgee-migrate up                # apply all pending migrations
//	borgee-migrate up --target 5     # apply pending migrations up to version 5
//	borgee-migrate status            # list applied vs pending
//
// The same engine runs automatically on server startup (cmd/collab); this CLI
// exists for ops / CI verification (G0.1 acceptance).
package main

import (
	"flag"
	"fmt"
	"os"

	"borgee-server/internal/config"
	"borgee-server/internal/migrations"
	"borgee-server/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}
	s, err := store.Open(cfg.DatabasePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "store.Open: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	// Run the full boot path first: the consolidated schema baseline creates
	// the schema (a no-op on an existing DB), the forward-only registry records
	// the baseline version, and the system user is seeded. cmd/collab does the
	// same on startup; this CLI mirrors it so `status` / `up` reflect the real
	// boot state. The trailing engine.Run below is then idempotent.
	if err := s.Migrate(); err != nil {
		fmt.Fprintf(os.Stderr, "store.Migrate (schema baseline): %v\n", err)
		os.Exit(1)
	}

	engine := migrations.Default(s.DB())

	switch cmd {
	case "up":
		fs := flag.NewFlagSet("up", flag.ExitOnError)
		target := fs.Int("target", 0, "apply migrations up to this version (0 = all pending)")
		_ = fs.Parse(os.Args[2:])
		if err := engine.Run(*target); err != nil {
			fmt.Fprintf(os.Stderr, "migrate up: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("ok")
	case "status":
		applied, err := engine.Applied()
		if err != nil {
			fmt.Fprintf(os.Stderr, "status: %v\n", err)
			os.Exit(1)
		}
		pending, err := engine.Pending()
		if err != nil {
			fmt.Fprintf(os.Stderr, "status: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("applied: %d  pending: %d\n", len(applied), len(pending))
		for _, m := range pending {
			fmt.Printf("  pending v%d  %s\n", m.Version, m.Name)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: borgee-migrate <up|status> [--target N]")
}
