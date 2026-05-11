// Package audit - HB-2 audit log writer for JSON lines. The schema must stay
// byte-identical with the HB-1 audit log across milestones: actor / action /
// target / when / scope. Changes are covered by tests in both locations and by
// the hb-2-spec.md section 4 required invariant #5.
package audit

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

// Event is the HB-2 IPC audit row, including rejected calls.
type Event struct {
	Actor  string `json:"actor"`  // agent_id used by cross-agent ACL checks
	Action string `json:"action"` // list_files / read_file / network_egress, including rejects
	Target string `json:"target"` // path / url / scope
	When   int64  `json:"when"`   // unix millis
	Scope  string `json:"scope"`  // host_grants scope (e.g. "fs:/Users/me/projects")
}

// Logger writes JSON lines in order. A single mutex serializes audit writes.
type Logger struct {
	mu sync.Mutex
	w  io.Writer
}

// New constructs a logger for an audit.log.jsonl file, stdout, or a test writer.
func New(w io.Writer) *Logger {
	return &Logger{w: w}
}

// Write appends one serialized line per call. It returns errors without blocking
// the IPC path, so callers can handle audit writes as best-effort, matching the
// BPP-4/5 mode.
func (l *Logger) Write(e Event) error {
	if e.When == 0 {
		e.When = time.Now().UnixMilli()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = l.w.Write(b)
	return err
}
