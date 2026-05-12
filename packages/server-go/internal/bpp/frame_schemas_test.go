// Package bpp_test — frame_schemas_test.go: BPP-1 (#274/#280) envelope
// CI lint. Reflection-based reverse assertions that pin the 5
// invariants the spec brief calls out:
//
//   ① Each registered envelope has the RT-0 #237 matching layout
//      (`Type` is field 0, tagged `json:"type"`, no extra envelope-
//      level meta fields like `v` / `ts` — the discriminator IS the
//      envelope).
//   ② Control plane (6 frames) — direction lock = Server→Plugin.
//   ③ Data plane (3 frames)    — direction lock = Plugin→Server.
//   ④ Allow-list closure — every exported `*Frame` struct in package bpp
//      that satisfies BPPEnvelope is in `BPPEnvelopeWhitelist`, and
//      every allow-list entry has a matching struct (no orphans).
//   ⑤ godoc anchor — `BPP-1.*byte-identical.*RT-0` count >= 1 in the
//      bpp package source.
//
// This file backs `scripts/lint-bpp-envelope.sh`, which is in turn
// invoked by the `bpp-envelope-lint` job in `.github/workflows/ci.yml`.

package bpp_test

import (
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"borgee-server/internal/bpp"
)

// TestBPPEnvelopeFrameWhitelist pins invariant ④. The lint walks every
// envelope returned by AllBPPEnvelopes(), maps it to its FrameType(),
// then asserts the allow-list exactly covers that set.
//
// AL-2b (#452) extended data plane to 4 frames (added agent_config_ack)
// → total 10 envelopes (6 control + 4 data). 跟 acceptance §1.2 字面
// exact match — direction lock plugin→server.
func TestBPPEnvelopeFrameWhitelist(t *testing.T) {
	t.Parallel()
	envs := bpp.AllBPPEnvelopes()
	if got, want := len(envs), 15; got != want {
		t.Fatalf("BPP envelope count: got %d, want %d (BPP-1 control 6 + data 3 + AL-2b ack +1 + BPP-2.2 task +2 + BPP-3.1 permission_denied +1 + BPP-5 reconnect_handshake +1 + BPP-6 cold_start_handshake +1 = 15)", got, want)
	}
	wl := bpp.BPPEnvelopeWhitelist()
	if got, want := len(wl), 15; got != want {
		t.Fatalf("allow-list size: got %d, want %d", got, want)
	}
	seen := map[string]struct{}{}
	for _, e := range envs {
		ft := e.FrameType()
		if ft == "" {
			t.Fatalf("envelope %T has empty FrameType()", e)
		}
		if _, dup := seen[ft]; dup {
			t.Fatalf("duplicate FrameType %q across envelopes", ft)
		}
		seen[ft] = struct{}{}
		if _, ok := wl[ft]; !ok {
			t.Fatalf("envelope %T (%q) is not in BPPEnvelopeWhitelist", e, ft)
		}
	}
	for ft := range wl {
		if _, ok := seen[ft]; !ok {
			t.Fatalf("allow-list has %q but no matching envelope struct", ft)
		}
	}
}

// TestBPPEnvelopeDirectionLock pins invariants ② + ③. Walks every
// envelope, calls FrameDirection(), and asserts it matches the
// allow-list value. Also asserts the control-plane / data-plane counts
// match the §2.1 / §2.2 row counts.
func TestBPPEnvelopeDirectionLock(t *testing.T) {
	t.Parallel()
	wl := bpp.BPPEnvelopeWhitelist()
	var ctrl, data int
	for _, e := range bpp.AllBPPEnvelopes() {
		ft := e.FrameType()
		want := wl[ft]
		got := e.FrameDirection()
		if got != want {
			t.Errorf("%s direction: got %q, want %q", ft, got, want)
		}
		switch got {
		case bpp.DirectionServerToPlugin:
			ctrl++
		case bpp.DirectionPluginToServer:
			data++
		default:
			t.Errorf("%s has invalid direction %q", ft, got)
		}
	}
	if ctrl != 7 {
		t.Errorf("control-plane envelope count: got %d, want 7 (§2.1 + BPP-3.1 permission_denied)", ctrl)
	}
	if data != 8 {
		t.Errorf("data-plane envelope count: got %d, want 8 (§2.2 + AL-2b agent_config_ack + BPP-2.2 task_started/task_finished + BPP-5 reconnect_handshake + BPP-6 cold_start_handshake)", data)
	}
}

// TestBPPEnvelopeFieldOrder pins invariant ① — the wire-layout lock
// with RT-0 #237 / RT-1.1 #290. Every envelope's first struct field
// MUST be `Type string` tagged `json:"type"`. This is the dispatcher
// contract — change it and every wire decoder breaks at once.
func TestBPPEnvelopeFieldOrder(t *testing.T) {
	t.Parallel()
	for _, e := range bpp.AllBPPEnvelopes() {
		typ := reflect.TypeOf(e)
		if typ.Kind() != reflect.Struct {
			t.Fatalf("%T is not a struct", e)
		}
		if typ.NumField() == 0 {
			t.Fatalf("%T has zero fields", e)
		}
		f0 := typ.Field(0)
		if f0.Name != "Type" {
			t.Errorf("%s field 0 name: got %q, want \"Type\"", typ.Name(), f0.Name)
		}
		if f0.Type.Kind() != reflect.String {
			t.Errorf("%s field 0 kind: got %v, want string", typ.Name(), f0.Type.Kind())
		}
		if got := f0.Tag.Get("json"); got != "type" {
			t.Errorf("%s field 0 json tag: got %q, want \"type\"", typ.Name(), got)
		}
	}
}

// TestBPPEnvelopeGodocAnchor pins invariant ⑤. The package documents
// the RT-0 wire-layout lock in a godoc comment so a reverse grep
// catches deletions. We accept any file in internal/bpp/ matching
// `BPP-1.*byte-identical.*RT-0` (count >= 1).
func TestBPPEnvelopeGodocAnchor(t *testing.T) {
	t.Parallel()
	dir := bppPkgDir(t)
	pat := regexp.MustCompile(`BPP-1.*byte-identical.*RT-0`)
	hits := 0
	walkGoFiles(t, dir, func(path string, body []byte) {
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		if pat.Match(body) {
			hits++
		}
	})
	if hits < 1 {
		t.Fatalf("godoc anchor `BPP-1.*byte-identical.*RT-0` count: got %d, want >= 1", hits)
	}
}

// TestBPPEnvelopeReverseGrepNoFullDefault pins the RT-1.3 hardline
// carried into BPP-1: no implicit `full` replay default may live in
// the bpp package non-test sources. The patterns mirror the spec
// brief's reverse-grep list. Comments are stripped before scanning so
// docstrings that describe the forbidden patterns don't self-trip.
func TestBPPEnvelopeReverseGrepNoFullDefault(t *testing.T) {
	t.Parallel()
	bad := []*regexp.Regexp{
		regexp.MustCompile(`replay_mode\s*=\s*"full"`),
		regexp.MustCompile(`default.*ResumeModeFull`),
		regexp.MustCompile(`\bdefaultReplayMode\b`),
	}
	dir := bppPkgDir(t)
	walkGoFiles(t, dir, func(path string, body []byte) {
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		code, err := stripGoComments(body)
		if err != nil {
			t.Fatalf("strip comments %s: %v", path, err)
		}
		for _, re := range bad {
			if loc := re.FindIndex(code); loc != nil {
				t.Errorf("forbidden pattern %q hit in %s at byte %d (negative assertion: server must not default to full replay)",
					re.String(), filepath.Base(path), loc[0])
			}
		}
	})
}

// stripGoComments returns the source body with all line and block
// comments replaced by spaces (preserving byte offsets). Implemented
// via go/scanner so it handles strings / rune literals correctly.
func stripGoComments(body []byte) ([]byte, error) {
	fset := token.NewFileSet()
	f := fset.AddFile("", fset.Base(), len(body))
	out := make([]byte, len(body))
	copy(out, body)
	var s scanner.Scanner
	s.Init(f, body, nil, scanner.ScanComments)
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.COMMENT {
			start := fset.Position(pos).Offset
			end := start + len(lit)
			for i := start; i < end && i < len(out); i++ {
				if out[i] != '\n' {
					out[i] = ' '
				}
			}
		}
	}
	return out, nil
}

// TestBPPEnvelopeAllExportedStructsCovered pins invariant ④ from the
// other direction: parse the AST of envelope.go and assert every
// exported `*Frame` struct declared there is in AllBPPEnvelopes(). An
// engineer who adds a struct but forgets the registry entry trips this.
func TestBPPEnvelopeAllExportedStructsCovered(t *testing.T) {
	t.Parallel()
	dir := bppPkgDir(t)
	src := filepath.Join(dir, "envelope.go")
	body, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read envelope.go: %v", err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, src, body, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse envelope.go: %v", err)
	}
	registered := map[string]struct{}{}
	for _, e := range bpp.AllBPPEnvelopes() {
		registered[reflect.TypeOf(e).Name()] = struct{}{}
	}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts := spec.(*ast.TypeSpec)
			if !ts.Name.IsExported() {
				continue
			}
			if !strings.HasSuffix(ts.Name.Name, "Frame") {
				continue
			}
			if _, ok := ts.Type.(*ast.StructType); !ok {
				continue
			}
			if _, ok := registered[ts.Name.Name]; !ok {
				t.Errorf("exported struct %s in envelope.go is not in AllBPPEnvelopes()", ts.Name.Name)
			}
		}
	}
}

// --- helpers ---

func bppPkgDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// Tests run from the package directory itself.
	return wd
}

func walkGoFiles(t *testing.T, dir string, fn func(path string, body []byte)) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".go") {
			continue
		}
		p := filepath.Join(dir, ent.Name())
		body, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		fn(p, body)
	}
}
