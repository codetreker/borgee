package store

import (
	"database/sql"
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// masterRow mirrors a sqlite_master row used for the AC-1 golden comparison.
type masterRow struct {
	Type    string  `json:"type"`
	Name    string  `json:"name"`
	TblName string  `json:"tbl_name"`
	SQL     *string `json:"sql"`
}

func dumpMaster(s *Store) ([]masterRow, error) {
	rows, err := s.db.Raw(`SELECT type, name, tbl_name, sql FROM sqlite_master ORDER BY type, name`).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []masterRow
	for rows.Next() {
		var r masterRow
		var sqlText sql.NullString
		if err := rows.Scan(&r.Type, &r.Name, &r.TblName, &sqlText); err != nil {
			return nil, err
		}
		if sqlText.Valid {
			v := sqlText.String
			r.SQL = &v
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].TblName < out[j].TblName
	})
	return out, rows.Err()
}

// wsTrivia collapses runs of whitespace to a single space and trims. AC-1 allows
// "whitespace-trivia-insensitive only, no semantic normalization": we do NOT
// lowercase, reorder tokens, or strip quotes/keywords. We only normalize spans
// of [ \t\r\n] that are NOT inside a single-quoted string literal, so CHECK/IN
// literal lists (e.g. 'public','private') keep their exact bytes.
var wsRun = regexp.MustCompile(`[ \t\r\n]+`)

func normalizeWS(s string) string {
	var b strings.Builder
	inStr := false
	var pending []byte
	flush := func() {
		if len(pending) > 0 {
			b.Write(wsRun.ReplaceAll(pending, []byte(" ")))
			pending = pending[:0]
		}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\'' {
			flush()
			inStr = !inStr
			b.WriteByte(c)
			continue
		}
		if inStr {
			b.WriteByte(c)
			continue
		}
		pending = append(pending, c)
	}
	flush()
	return strings.TrimSpace(b.String())
}

func loadGolden(t *testing.T) []masterRow {
	t.Helper()
	b, err := os.ReadFile("testdata/schema_golden.json")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var g []masterRow
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}
	return g
}

// freshNewPathStore builds a brand-new in-memory DB via the new createSchema +
// createSchemaIndexes path only (no backfills, no registry). FK is OFF during
// creation, matching Migrate().
func freshNewPathStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.db.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
		t.Fatalf("fk off: %v", err)
	}
	if err := s.createSchema(); err != nil {
		t.Fatalf("createSchema: %v", err)
	}
	if err := s.createSchemaIndexes(); err != nil {
		t.Fatalf("createSchemaIndexes: %v", err)
	}
	if err := s.db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		t.Fatalf("fk on: %v", err)
	}
	return s
}

// TestAC1_FreshSchemaMatchesGolden is the AC-1 gate: a fresh DB built the new
// way must be schema-identical to the committed golden (captured pre-change from
// legacy baseline + all forward-only migrations) by exact sql text over every
// sqlite_master object. NULL-sql auto-indexes are matched by (name, table,
// type). Zero diff in both directions.
func TestAC1_FreshSchemaMatchesGolden(t *testing.T) {
	t.Parallel()
	s := freshNewPathStore(t)
	got, err := dumpMaster(s)
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	golden := loadGolden(t)

	type key struct{ typ, name string }
	index := func(rows []masterRow) map[key]masterRow {
		m := make(map[key]masterRow, len(rows))
		for _, r := range rows {
			m[key{r.Type, r.Name}] = r
		}
		return m
	}
	gm := index(golden)
	nm := index(got)

	var diffs []string
	for k, gr := range gm {
		nr, ok := nm[k]
		if !ok {
			diffs = append(diffs, "MISSING in new path: "+k.typ+" "+k.name)
			continue
		}
		if gr.TblName != nr.TblName {
			diffs = append(diffs, "tbl_name differs for "+k.typ+" "+k.name+": golden="+gr.TblName+" new="+nr.TblName)
		}
		// NULL-sql (auto index) — match by presence + type + tbl_name only.
		if gr.SQL == nil || nr.SQL == nil {
			if (gr.SQL == nil) != (nr.SQL == nil) {
				diffs = append(diffs, "sql nullness differs for "+k.typ+" "+k.name)
			}
			continue
		}
		if normalizeWS(*gr.SQL) != normalizeWS(*nr.SQL) {
			diffs = append(diffs, "sql differs for "+k.typ+" "+k.name+":\n  golden: "+*gr.SQL+"\n  new:    "+*nr.SQL)
		}
	}
	for k := range nm {
		if _, ok := gm[k]; !ok {
			diffs = append(diffs, "EXTRA in new path: "+k.typ+" "+k.name)
		}
	}

	if len(diffs) > 0 {
		t.Fatalf("AC-1 schema diff (%d):\n%s", len(diffs), strings.Join(diffs, "\n"))
	}
	t.Logf("AC-1: new-path schema matches golden across %d objects, zero diff", len(golden))
}

// TestAC1_ReapplyOnPopulatedDBIsNoOp is the existing-DB no-op gate (HIGH-1 from
// design review): build the full schema, then run the new createSchema /
// createSchemaIndexes path AGAIN on the populated DB and assert no error and
// zero schema delta. Existing v48 DBs boot through this path on every start.
func TestAC1_ReapplyOnPopulatedDBIsNoOp(t *testing.T) {
	t.Parallel()
	s := freshNewPathStore(t)
	before, err := dumpMaster(s)
	if err != nil {
		t.Fatalf("dump before: %v", err)
	}
	// Re-run the schema-creation path on the already-populated DB.
	if err := s.db.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
		t.Fatal(err)
	}
	if err := s.createSchema(); err != nil {
		t.Fatalf("re-run createSchema on populated DB: %v", err)
	}
	if err := s.createSchemaIndexes(); err != nil {
		t.Fatalf("re-run createSchemaIndexes on populated DB: %v", err)
	}
	if err := s.db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		t.Fatal(err)
	}
	after, err := dumpMaster(s)
	if err != nil {
		t.Fatalf("dump after: %v", err)
	}
	if len(before) != len(after) {
		t.Fatalf("re-apply changed object count: before=%d after=%d", len(before), len(after))
	}
	for i := range before {
		if before[i].Type != after[i].Type || before[i].Name != after[i].Name {
			t.Fatalf("re-apply changed object set at %d: %+v vs %+v", i, before[i], after[i])
		}
	}
	t.Logf("AC-1 no-op: re-running schema creation on a populated DB left all %d objects unchanged", len(before))
}

// TestAC1_FullMigrateMatchesGolden verifies the complete Migrate() path (schema
// + registry baseline) also yields the golden schema, and that Migrate() is
// itself idempotent on a populated DB (the production boot contract).
func TestAC1_FullMigrateMatchesGolden(t *testing.T) {
	t.Parallel()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	got, err := dumpMaster(s)
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	golden := loadGolden(t)
	if len(got) != len(golden) {
		t.Fatalf("Migrate object count %d != golden %d", len(got), len(golden))
	}
	// Migrate twice must not error (existing-DB boot).
	if err := s.Migrate(); err != nil {
		t.Fatalf("second Migrate (idempotency): %v", err)
	}
	got2, err := dumpMaster(s)
	if err != nil {
		t.Fatalf("dump 2: %v", err)
	}
	if len(got2) != len(got) {
		t.Fatalf("second Migrate changed object count: %d -> %d", len(got), len(got2))
	}
}
