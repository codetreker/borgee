// astscan_test.go — self-check for the AST scan helper.
//
// Pins:
//   ① Identifier hit fails the test (positive case)
//   ② Comment-only mention is ignored by default (false-positive guard)
//   ③ String literal is ignored unless IncludeStrings=true
//   ④ _test.go files are always skipped (tests must legally name forbidden)
//   ⑤ SkipFiles glob excludes additional files
//   ⑥ Empty forbidden list fails fatally (programming bug guard)
//   ⑦ ScanOpts zero value is the safe default

package astscan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile is a tiny helper for building a fixture package on disk.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// fakeT captures Errorf / Fatalf calls so the self-check can verify hits
// without actually failing the wrapper test. Implements astscan.TestingT.
type fakeT struct {
	errs  []string
	fatal string
}

func (f *fakeT) Helper() {}

func (f *fakeT) Errorf(format string, args ...any) {
	f.errs = append(f.errs, fmt.Sprintf(format, args...))
}

func (f *fakeT) Fatalf(format string, args ...any) {
	f.fatal = fmt.Sprintf(format, args...)
}

// TestAssertNoForbiddenIdentifiers_HitsIdentifier — invariant ①.
func TestAssertNoForbiddenIdentifiers_HitsIdentifier(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "prod.go", `package fixture

var pendingAcks = map[string]int{}
`)
	ft := &fakeT{}
	AssertNoForbiddenIdentifiers(ft, dir, []ForbiddenIdentifier{
		{Name: "pendingAcks", Reason: "ack best-effort 不重发"},
	}, ScanOpts{})
	if len(ft.errs) == 0 {
		t.Fatal("expected hit on pendingAcks identifier")
	}
	if !strings.Contains(ft.errs[0], "pendingAcks") || !strings.Contains(ft.errs[0], "ack best-effort") {
		t.Errorf("hit msg should include identifier + reason: %q", ft.errs[0])
	}
}

// TestAssertNoForbiddenIdentifiers_IgnoresComments — invariant ②.
func TestAssertNoForbiddenIdentifiers_IgnoresComments(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "prod.go", `package fixture

// pendingAcks: this comment intentionally names the forbidden token.
var harmless int
`)
	ft := &fakeT{}
	AssertNoForbiddenIdentifiers(ft, dir, []ForbiddenIdentifier{
		{Name: "pendingAcks", Reason: "ack best-effort"},
	}, ScanOpts{})
	if len(ft.errs) > 0 {
		t.Errorf("default scan should not hit comments: %v", ft.errs)
	}
	ft2 := &fakeT{}
	AssertNoForbiddenIdentifiers(ft2, dir, []ForbiddenIdentifier{
		{Name: "pendingAcks", Reason: "ack best-effort"},
	}, ScanOpts{IncludeComments: true})
	if len(ft2.errs) == 0 {
		t.Error("IncludeComments=true should hit")
	}
}

// TestAssertNoForbiddenIdentifiers_IgnoresStrings — invariant ③.
func TestAssertNoForbiddenIdentifiers_IgnoresStrings(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "prod.go", `package fixture

var note = "discussion of pendingAcks 约束"
`)
	ft := &fakeT{}
	AssertNoForbiddenIdentifiers(ft, dir, []ForbiddenIdentifier{
		{Name: "pendingAcks", Reason: "ack best-effort"},
	}, ScanOpts{})
	if len(ft.errs) > 0 {
		t.Errorf("default scan should not hit strings: %v", ft.errs)
	}
	ft2 := &fakeT{}
	AssertNoForbiddenIdentifiers(ft2, dir, []ForbiddenIdentifier{
		{Name: "pendingAcks", Reason: "ack best-effort"},
	}, ScanOpts{IncludeStrings: true})
	if len(ft2.errs) == 0 {
		t.Error("IncludeStrings=true should hit string literal")
	}
}

// TestAssertNoForbiddenIdentifiers_SkipsTestFiles — invariant ④.
func TestAssertNoForbiddenIdentifiers_SkipsTestFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "prod.go", `package fixture

var clean int
`)
	writeFile(t, dir, "fixture_test.go", `package fixture

import "testing"

func TestX(t *testing.T) {
	pendingAcks := 0
	_ = pendingAcks
}
`)
	ft := &fakeT{}
	AssertNoForbiddenIdentifiers(ft, dir, []ForbiddenIdentifier{
		{Name: "pendingAcks", Reason: "ack best-effort"},
	}, ScanOpts{})
	if len(ft.errs) > 0 {
		t.Errorf("_test.go must be skipped: %v", ft.errs)
	}
}

// TestAssertNoForbiddenIdentifiers_SkipFilesGlob — invariant ⑤.
func TestAssertNoForbiddenIdentifiers_SkipFilesGlob(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "prod.go", `package fixture

var clean int
`)
	writeFile(t, dir, "generated.pb.go", `package fixture

var pendingAcks int
`)
	ft := &fakeT{}
	AssertNoForbiddenIdentifiers(ft, dir, []ForbiddenIdentifier{
		{Name: "pendingAcks", Reason: "ack best-effort"},
	}, ScanOpts{SkipFiles: []string{"*.pb.go"}})
	if len(ft.errs) > 0 {
		t.Errorf("SkipFiles glob should exclude *.pb.go: %v", ft.errs)
	}
	ft2 := &fakeT{}
	AssertNoForbiddenIdentifiers(ft2, dir, []ForbiddenIdentifier{
		{Name: "pendingAcks", Reason: "ack best-effort"},
	}, ScanOpts{})
	if len(ft2.errs) == 0 {
		t.Error("without SkipFiles, generated.pb.go must hit")
	}
}

// TestAssertNoForbiddenIdentifiers_EmptyListFatal — invariant ⑥.
func TestAssertNoForbiddenIdentifiers_EmptyListFatal(t *testing.T) {
	ft := &fakeT{}
	AssertNoForbiddenIdentifiers(ft, t.TempDir(), nil, ScanOpts{})
	if ft.fatal == "" {
		t.Error("expected Fatalf on empty forbidden list")
	}
	if !strings.Contains(ft.fatal, "forbidden list is empty") {
		t.Errorf("fatal msg should mention empty list: %q", ft.fatal)
	}
}

// TestScanOpts_ZeroValueIsSafe — invariant ⑦.
func TestScanOpts_ZeroValueIsSafe(t *testing.T) {
	var opts ScanOpts
	if opts.IncludeStrings {
		t.Error("zero value should default IncludeStrings=false")
	}
	if opts.IncludeComments {
		t.Error("zero value should default IncludeComments=false")
	}
	if opts.SkipFiles != nil {
		t.Error("zero value should default SkipFiles=nil")
	}
}

// TestAssertNoForbiddenIdentifiers_AcceptsRealTestingT — compile-time check
// that *testing.T satisfies TestingT (the production usage path).
func TestAssertNoForbiddenIdentifiers_AcceptsRealTestingT(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "prod.go", `package fixture

var clean int
`)
	// If *testing.T didn't satisfy TestingT, this wouldn't compile.
	AssertNoForbiddenIdentifiers(t, dir, []ForbiddenIdentifier{
		{Name: "pendingAcks", Reason: "compile-time check"},
	}, ScanOpts{})
}
