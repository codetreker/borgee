package store

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// schemaBaselineStatements is generated from testdata/schema_golden.json. This
// test regenerates it (GEN_BASELINE=1) and, by default, verifies the committed
// file is in sync with the golden so the consolidated baseline can never drift
// silently from the captured schema.
//
//	GEN_BASELINE=1 go test -tags sqlite_fts5 -run TestGenerateBaseline ./internal/store/
func TestGenerateBaseline(t *testing.T) {
	golden := loadGolden(t)

	// Filter out auto-created objects: sqlite_sequence, FTS5 shadow tables, and
	// NULL-sql auto indexes. SQLite materializes these from AUTOINCREMENT, the
	// virtual table, and PK/UNIQUE constraints respectively.
	isAuto := func(r masterRow) bool {
		if r.SQL == nil {
			return true
		}
		if r.Name == "sqlite_sequence" || strings.HasPrefix(r.Name, "artifacts_fts_") {
			return true
		}
		return false
	}

	var tables, fts, indexes, views, triggers []masterRow
	for _, r := range golden {
		if isAuto(r) {
			continue
		}
		switch {
		case r.Name == "artifacts_fts":
			fts = append(fts, r)
		case r.Type == "table":
			tables = append(tables, r)
		case r.Type == "index":
			indexes = append(indexes, r)
		case r.Type == "view":
			views = append(views, r)
		case r.Type == "trigger":
			triggers = append(triggers, r)
		}
	}
	// Dependency order: tables -> FTS5 virtual table -> indexes -> view ->
	// triggers. golden is already sorted by (type,name); within each group the
	// order is stable. FK is OFF during creation so inter-table order is free.
	var ordered []masterRow
	ordered = append(ordered, tables...)
	ordered = append(ordered, fts...)
	ordered = append(ordered, indexes...)
	ordered = append(ordered, views...)
	ordered = append(ordered, triggers...)

	want := renderBaselineFile(ordered)

	const path = "schema_baseline_gen.go"
	if os.Getenv("GEN_BASELINE") != "" {
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		out, err := exec.Command("gofmt", "-w", path).CombinedOutput()
		if err != nil {
			t.Fatalf("gofmt: %v\n%s", err, out)
		}
		t.Logf("regenerated %s with %d statements", path, len(ordered))
		return
	}

	// Sync check: committed file must equal a gofmt'd render of the golden.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	// gofmt the want for an apples-to-apples comparison.
	tmp, err := os.CreateTemp(t.TempDir(), "baseline*.go")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmp.WriteString(want); err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	if out, err := exec.Command("gofmt", "-w", tmp.Name()).CombinedOutput(); err != nil {
		t.Fatalf("gofmt want: %v\n%s", err, out)
	}
	wantFmt, err := os.ReadFile(tmp.Name())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(wantFmt) {
		t.Fatalf("schema_baseline_gen.go is out of sync with testdata/schema_golden.json; run GEN_BASELINE=1 go test -tags sqlite_fts5 -run TestGenerateBaseline ./internal/store/")
	}
}

func renderBaselineFile(objs []masterRow) string {
	var b strings.Builder
	b.WriteString("// Code generated from internal/store/testdata/schema_golden.json by\n")
	b.WriteString("// schema_baseline_gen_test.go (TestGenerateBaseline, GEN_BASELINE=1). DO NOT EDIT.\n")
	b.WriteString("//\n")
	b.WriteString("// This is the one-time re-baselined schema: the verbatim sqlite_master `sql`\n")
	b.WriteString("// text of every non-auto object (tables, real indexes, view, triggers, FTS5\n")
	b.WriteString("// virtual table) as it stood after the legacy baseline + all forward-only\n")
	b.WriteString("// migrations. Auto objects (sqlite_sequence, FTS shadow tables,\n")
	b.WriteString("// sqlite_autoindex_*) are omitted — sqlite materializes them automatically.\n")
	b.WriteString("package store\n\n")
	b.WriteString("// schemaBaselineStatements holds the consolidated CREATE statements in\n")
	b.WriteString("// dependency order: tables, then the FTS5 virtual table, then indexes, then\n")
	b.WriteString("// the view, then triggers. Each entry is the verbatim golden `sql`; the exec\n")
	b.WriteString("// path (withIfNotExists) injects IF NOT EXISTS so re-running on an existing DB\n")
	b.WriteString("// is a no-op while the text here stays byte-identical to the golden for AC-1.\n")
	b.WriteString("var schemaBaselineStatements = []string{\n")
	for _, r := range objs {
		b.WriteString("\t" + goStringLit(*r.SQL) + ",\n")
	}
	b.WriteString("}\n")
	return b.String()
}

func goStringLit(s string) string {
	if !strings.Contains(s, "`") {
		return "`" + s + "`"
	}
	q, _ := json.Marshal(s)
	return string(q)
}
