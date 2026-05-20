package audit

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestHB22_AuditEvent5FieldSchemaByteIdentical(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	l := New(buf)
	if err := l.Write(Event{
		Actor:  "agent-uuid-1",
		Action: "read_file",
		Target: "/Users/me/projects/foo.txt",
		When:   1714492800000,
		Scope:  "fs:/Users/me/projects",
	}); err != nil {
		t.Fatalf("Write err: %v", err)
	}
	line := strings.TrimSpace(buf.String())
	// Literal byte-identical 5-field schema (same source as the HB-1 audit log).
	want := `{"actor":"agent-uuid-1","action":"read_file","target":"/Users/me/projects/foo.txt","when":1714492800000,"scope":"fs:/Users/me/projects"}`
	if line != want {
		t.Errorf("audit JSON 脱节:\n got=%s\nwant=%s", line, want)
	}
}

func TestHB22_AuditEvent5FieldSetExact(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	l := New(buf)
	_ = l.Write(Event{Actor: "a", Action: "list_files", Target: "/x", When: 1, Scope: "fs:/x"})
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := []string{"actor", "action", "target", "when", "scope"}
	if len(m) != len(want) {
		t.Errorf("audit field count 脱节: got=%d want=%d (forbid 6th 脱节)", len(m), len(want))
	}
	for _, k := range want {
		if _, ok := m[k]; !ok {
			t.Errorf("missing field %q (5-field 单一来源 跟 HB-1 byte-identical)", k)
		}
	}
}

func TestHB22_WhenAutoFillIfZero(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	l := New(buf)
	_ = l.Write(Event{Actor: "a", Action: "read_file", Target: "/x", Scope: "fs:/x"})
	var m map[string]any
	_ = json.Unmarshal(buf.Bytes(), &m)
	if w, _ := m["when"].(float64); w == 0 {
		t.Error("zero When 应自动填 unix millis")
	}
}
