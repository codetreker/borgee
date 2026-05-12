//go:build linux

// Package main — borgee-installer-linux: HB-1B-INSTALLER Linux .deb installer.
//
// hb-1b-installer-spec §0.2: ed25519 manifest verification + permission popup
// + sudo apt install + systemd unit deployment from borgee-helper install assets.
//
// CLI:
//
//	borgee-installer-linux \
//	  --manifest-url=https://server/api/v1/plugin-manifest \
//	  --pubkey-base64=... \
//	  --deb=./borgee-helper_0.1.0_amd64.deb
//
// Boundary: installer implementation stays in this module and uses user sudo;
// it does not add an installer admin API path.
package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"borgee-installer/internal/deploy"
	"borgee-installer/internal/dialog"
	"borgee-installer/internal/manifest"
)

func main() {
	manifestURL := flag.String("manifest-url", "", "HB-1 server endpoint URL")
	pubKeyB64 := flag.String("pubkey-base64", "", "ed25519 public key (base64)")
	bearerToken := flag.String("bearer-token", "", "owner Bearer api-key (HB-1 owner-only auth)")
	debPath := flag.String("deb", "", "path to borgee-helper .deb artifact")
	dryRun := flag.Bool("dry-run", false, "print plan without sudo apt install")
	flag.Parse()

	if *manifestURL == "" || *pubKeyB64 == "" || *debPath == "" {
		fmt.Fprintln(os.Stderr, "usage: borgee-installer-linux --manifest-url=... --pubkey-base64=... --deb=...")
		os.Exit(2)
	}

	pubKey, err := base64.StdEncoding.DecodeString(*pubKeyB64)
	if err != nil || len(pubKey) != ed25519.PublicKeySize {
		fmt.Fprintf(os.Stderr, "bad pubkey: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Step 1: fetch manifest from HB-1 server endpoint.
	env, err := manifest.Fetch(ctx, &http.Client{Timeout: 30 * time.Second}, *manifestURL, *bearerToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetch failed: %v\n", err)
		os.Exit(1)
	}

	// Step 2: ed25519 verification is required; do not skip it.
	if err := manifest.Verify(env, ed25519.PublicKey(pubKey)); err != nil {
		fmt.Fprintf(os.Stderr, "manifest verify failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("manifest verified: %d entries signed_at=%d\n", len(env.Entries), env.SignedAt)

	// Step 3: permission popup UX (4 grant_type values match HB-3 #520).
	ok, err := dialog.Confirm(os.Stdin, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "confirm failed: %v\n", err)
		os.Exit(1)
	}
	if !ok {
		fmt.Println("user cancelled installation")
		os.Exit(0)
	}

	// Step 4: deploy plan (sudo apt install + systemd enable).
	plan := deploy.LinuxPlan(*debPath)
	for _, step := range plan.Steps {
		fmt.Printf("→ %s\n", step)
		if *dryRun {
			continue
		}
		// Keep the sudo command visible for REG-HB1B-004 coverage; installer plan
		// steps are simple shell commands.
		cmd := exec.CommandContext(ctx, "sh", "-c", step)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "step failed: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Println("borgee-helper installed via systemd ✓")
}
