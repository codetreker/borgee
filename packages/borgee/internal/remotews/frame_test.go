package remotews

import (
	"encoding/json"
	"strings"
	"testing"

	"borgee/internal/fsops"
)

// TestRequestData_FlatShape_NoParams is the AC-5 invariant: a marshaled
// request frame carries data.action + data.path and NO params key. It mirrors
// the server's TestBuildRequestData_FlatShape so both ends assert the same
// literal wire shape.
func TestRequestData_FlatShape_NoParams(t *testing.T) {
	rd := RequestData{Action: "ls", Path: "/x"}
	b, err := json.Marshal(rd)
	if err != nil {
		t.Fatalf("marshal RequestData: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `"action":"ls"`) {
		t.Errorf("RequestData JSON %q missing \"action\":\"ls\"", s)
	}
	if !strings.Contains(s, `"path":"/x"`) {
		t.Errorf("RequestData JSON %q missing \"path\":\"/x\"", s)
	}
	if strings.Contains(s, "params") {
		t.Errorf("RequestData JSON %q must NOT contain a params key", s)
	}

	// The same invariant must hold when RequestData is embedded as a request
	// frame's data payload (this is the byte sequence the server decodes).
	payload, err := json.Marshal(rd)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	fr := Frame{Type: "request", ID: "srv-1", Data: payload}
	fb, err := json.Marshal(fr)
	if err != nil {
		t.Fatalf("marshal Frame: %v", err)
	}
	fs := string(fb)
	if !strings.Contains(fs, `"action":"ls"`) || !strings.Contains(fs, `"path":"/x"`) {
		t.Errorf("request Frame JSON %q missing flat action/path", fs)
	}
	if strings.Contains(fs, "params") {
		t.Errorf("request Frame JSON %q must NOT contain a params key", fs)
	}
}

// TestResponseFrame_RoundTrip builds a response frame carrying a marshaled
// fsops.LsResult and asserts the type/echoed-id/camelCase data shape.
func TestResponseFrame_RoundTrip(t *testing.T) {
	res := fsops.LsResult{Entries: []fsops.DirEntry{
		{Name: "a.txt", IsDirectory: false, Size: 12, Mtime: "2026-01-02T03:04:05.000Z"},
	}}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal LsResult: %v", err)
	}
	fr := Frame{Type: "response", ID: "srv-7", Data: data}
	b, err := json.Marshal(fr)
	if err != nil {
		t.Fatalf("marshal response Frame: %v", err)
	}

	// Decode back and assert the structural shape.
	var got Frame
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal Frame: %v", err)
	}
	if got.Type != "response" {
		t.Errorf("Type = %q; want response", got.Type)
	}
	if got.ID != "srv-7" {
		t.Errorf("ID = %q; want srv-7 (echoed)", got.ID)
	}
	var dataObj struct {
		Entries []struct {
			Name        string `json:"name"`
			IsDirectory bool   `json:"isDirectory"`
			Size        int64  `json:"size"`
			Mtime       string `json:"mtime"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(got.Data, &dataObj); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if len(dataObj.Entries) != 1 || dataObj.Entries[0].Name != "a.txt" {
		t.Errorf("entries = %+v; want one entry named a.txt", dataObj.Entries)
	}
	// camelCase keys present on the wire.
	s := string(got.Data)
	if !strings.Contains(s, `"isDirectory"`) || !strings.Contains(s, `"mtime"`) {
		t.Errorf("response data JSON %q missing camelCase keys", s)
	}
}
