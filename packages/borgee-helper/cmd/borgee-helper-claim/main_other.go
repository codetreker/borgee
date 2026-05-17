//go:build !linux && !darwin

// Package main — fallback for non-linux/darwin (no claim CLI on Windows yet).
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	flag.Parse()
	fmt.Fprintln(os.Stderr, "borgee-helper-claim: this platform is not supported; helper runtime supports linux/darwin only.")
	os.Exit(2)
}
