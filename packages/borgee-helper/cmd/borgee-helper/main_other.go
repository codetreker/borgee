//go:build !linux && !darwin

// Package main is the unsupported-platform fallback for non-linux/darwin builds.
// Current helper runtime is linux/darwin only; Windows support remains deferred.
package main

import (
	"flag"
	"log"
)

func main() {
	flag.Parse()
	log.Println("borgee-helper: this platform is not supported; current helper runtime supports linux/darwin only.")
}
