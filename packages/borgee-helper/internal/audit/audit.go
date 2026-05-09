// Package audit — HB-2 audit log writer (JSON line; schema byte-identical
// 跟 HB-1 audit log 跨 milestone 5-field 单一来源: actor / action / target /
// when / scope. 改 = 改两处单测锁定, hb-2-spec.md §4 反向约束 #5).
package audit

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

// Event 是 HB-2 IPC call 的审计行 (含 reject); 5 字段单一来源.
type Event struct {
	Actor  string `json:"actor"`  // agent_id (cross-agent ACL 出处)
	Action string `json:"action"` // list_files / read_file / network_egress (含 reject 时)
	Target string `json:"target"` // path / url / scope
	When   int64  `json:"when"`   // unix millis
	Scope  string `json:"scope"`  // host_grants scope (e.g. "fs:/Users/me/projects")
}

// Logger 是顺序 JSON-line writer (单 mutex 守 forward-only audit, 反 race).
type Logger struct {
	mu sync.Mutex
	w  io.Writer
}

// New 构造 logger (writer = audit.log.jsonl 文件 / stdout / mock).
func New(w io.Writer) *Logger {
	return &Logger{w: w}
}

// Write 顺序写一行 (atomic per call). 失败返回 err 但不阻 IPC 路径
// (caller 可选择 best-effort, 跟 BPP-4/5 同模式).
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
