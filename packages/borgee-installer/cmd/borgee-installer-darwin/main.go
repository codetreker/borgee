//go:build darwin

// Package main — borgee-installer-darwin: HB-1B-INSTALLER macOS .pkg installer.
//
// hb-1b-installer-spec §0.2: ed25519 manifest verification + permission popup
// + sudo /usr/sbin/installer + launchd unit deployment from borgee-helper install assets.
//
// CLI mirrors borgee-installer-linux; .pkg uses /usr/sbin/installer + launchctl.
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
	pkgPath := flag.String("pkg", "", "path to borgee-helper .pkg artifact")
	dryRun := flag.Bool("dry-run", false, "print plan without sudo /usr/sbin/installer")
	flag.Parse()

	if *manifestURL == "" || *pubKeyB64 == "" || *pkgPath == "" {
		fmt.Fprintln(os.Stderr, "usage: borgee-installer-darwin --manifest-url=... --pubkey-base64=... --pkg=...")
		os.Exit(2)
	}

	pubKey, err := base64.StdEncoding.DecodeString(*pubKeyB64)
	if err != nil || len(pubKey) != ed25519.PublicKeySize {
		fmt.Fprintf(os.Stderr, "bad pubkey: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	env, err := manifest.Fetch(ctx, &http.Client{Timeout: 30 * time.Second}, *manifestURL, *bearerToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetch failed: %v\n", err)
		os.Exit(1)
	}

	if err := manifest.Verify(env, ed25519.PublicKey(pubKey)); err != nil {
		fmt.Fprintf(os.Stderr, "manifest verify failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("manifest verified: %d entries signed_at=%d\n", len(env.Entries), env.SignedAt)

	ok, err := dialog.Confirm(os.Stdin, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "confirm failed: %v\n", err)
		os.Exit(1)
	}
	if !ok {
		fmt.Println("user cancelled installation")
		os.Exit(0)
	}

	plan := deploy.DarwinPlan(*pkgPath)
	for _, step := range plan.Steps {
		fmt.Printf("→ %s\n", step)
		if *dryRun {
			continue
		}
		// Keep sudo /usr/sbin/installer and launchctl visible for REG-HB1B-004
		// coverage.
		cmd := exec.CommandContext(ctx, "sh", "-c", step)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "step failed: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Println("borgee-helper installed via launchd ✓")
}
