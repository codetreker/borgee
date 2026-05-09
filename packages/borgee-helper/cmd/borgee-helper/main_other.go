//go:build !linux && !darwin

// Package main — Windows / 其他 fallback (v1 不挂; HB-2.0 prereq #605
// 已锁定 windows-named-pipe 字面给未来 v0(D) 用 go-winio 真启). hb-2-spec.md
// §5.5 + §5.6.
package main

import (
	"flag"
	"log"
)

func main() {
	flag.Parse()
	log.Println("borgee-helper: this platform not supported in HB-2 v0(C); see hb-2-spec.md §5.5 (Windows = v0(D) via go-winio).")
}
