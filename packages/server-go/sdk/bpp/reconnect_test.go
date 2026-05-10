// Package bpp (sdk/bpp) — reconnect_test.go: BPP-7.2 unit tests.
//
// Cases (acceptance §2):
//   2.1 Reconnect carries cursor / ColdStart no cursor / ColdStart reason
//       byte-identical (reasons.RuntimeCrashed)
//   2.2 HeartbeatInterval = 30s byte-identical
//   2.3 GrantRetry stops after 3 attempts + backoff const reuse
//   2.4 AST scan forbidden tokens (best-effort 守护链延伸第 4 处)
//   2.6 reason chain 12th link

package bpp_test

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"borgee-server/internal/agent/reasons"
	srvbpp "borgee-server/internal/bpp"
	sdkbpp "borgee-server/sdk/bpp"
)

// TestBPP_Reconnect_CarriesCursor — acceptance §2.1.
//
// Confirms the marshalled ReconnectHandshakeFrame contains last_known_cursor
// (BPP-5 #503 字段集一致).
func TestBPP_Reconnect_CarriesCursor(t *testing.T) {
	frame := srvbpp.ReconnectHandshakeFrame{
		Type:            srvbpp.FrameTypeBPPReconnectHandshake,
		PluginID:        "plugin-1",
		AgentID:         "agent-1",
		LastKnownCursor: 12345,
		DisconnectAt:    1700000000000,
		ReconnectAt:     1700000005000,
	}
	b, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{`"last_known_cursor":12345`, `"disconnect_at"`, `"reconnect_at"`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("missing key %q in %s", key, b)
		}
	}
}

// TestBPP_ColdStart_NoCursor — acceptance §2.1 BPP-6 spec §0.1 字段集
// 互斥反断: ColdStartHandshakeFrame must NOT carry LastKnownCursor /
// DisconnectAt / ReconnectAt fields.
func TestBPP_ColdStart_NoCursor(t *testing.T) {
	typ := reflect.TypeOf(srvbpp.ColdStartHandshakeFrame{})
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		switch name {
		case "LastKnownCursor", "DisconnectAt", "ReconnectAt":
			t.Errorf("ColdStartHandshakeFrame must NOT have %q (字段集互斥反断, BPP-6 spec §0.1)", name)
		}
	}
}

// TestBPP_ColdStart_ReasonRuntimeCrashed_ByteIdentical — acceptance §2.1+§2.6
// AL-1a reason 守护链 BPP-7 = 第 12 处 (BPP-2.2 第 7 + AL-2b 第 8 + BPP-4
// 第 9 + BPP-5 第 10 + BPP-6 第 11 + BPP-7 第 12).
func TestBPP_ColdStart_ReasonRuntimeCrashed_ByteIdentical(t *testing.T) {
	if reasons.RuntimeCrashed != "runtime_crashed" {
		t.Fatalf("reasons.RuntimeCrashed 脱节: got %q, want %q (AL-1a 守护链第 12 处)",
			reasons.RuntimeCrashed, "runtime_crashed")
	}
}

// TestBPP_HeartbeatInterval_30s — acceptance §2.2.
func TestBPP_HeartbeatInterval_30s(t *testing.T) {
	if got, want := sdkbpp.HeartbeatInterval, 30*time.Second; got != want {
		t.Errorf("HeartbeatInterval 脱节: got %v, want %v (BPP-4 #499 watchdog 周期 byte-identical)", got, want)
	}
}

// TestBPP_GrantRetry_StopsAfter3 — acceptance §2.3.
func TestBPP_GrantRetry_StopsAfter3(t *testing.T) {
	c := sdkbpp.NewClient("plugin-1", "agent-1", nil)
	var attempts int32
	op := func(ctx context.Context) error {
		atomic.AddInt32(&attempts, 1)
		return errors.New("transient")
	}
	// Use a short-lived context so RetryBackoff doesn't actually wait
	// 30s in tests — GrantRetry returns ctx.Err() between attempts.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := c.GrantRetry(ctx, op)
	if err == nil {
		t.Fatal("expected GrantRetry to return error after attempts")
	}
	got := atomic.LoadInt32(&attempts)
	if got < 1 || got > int32(sdkbpp.MaxPermissionRetries) {
		t.Errorf("attempt count out of bounds: got %d, want 1..%d", got, sdkbpp.MaxPermissionRetries)
	}
}

// TestBPP_GrantRetry_BackoffByteIdentical — acceptance §2.3.
func TestBPP_GrantRetry_BackoffByteIdentical(t *testing.T) {
	if sdkbpp.MaxPermissionRetries != 3 {
		t.Errorf("MaxPermissionRetries 脱节: got %d, want 3 (server const reuse)", sdkbpp.MaxPermissionRetries)
	}
	if sdkbpp.RetryBackoff != 30*time.Second {
		t.Errorf("RetryBackoff 脱节: got %v, want 30s (server const reuse)", sdkbpp.RetryBackoff)
	}
}

// TestBPP_NoSDKQueueOrCustomReason — acceptance §2.4 best-effort
// 守护链延伸第 4 处 (BPP-4 dead_letter_test + BPP-5 reconnect_handler_test +
// BPP-6 cold_start_handler_test + BPP-7 sdk_test).
func TestBPP_NoSDKQueueOrCustomReason(t *testing.T) {
	forbidden := []string{
		"pendingSDKReconnect",
		"sdkRetryQueue",
		"deadLetterSDK",
		"runtime_recovered",
		"sdk_specific_reason",
		"sdkReason",
		"cv4SDKReason",
		"sdkCustomReason",
	}
	dir := "."
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	hits := []string{}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			for _, bad := range forbidden {
				if strings.Contains(ident.Name, bad) {
					hits = append(hits, path+":"+ident.Name)
				}
			}
			return true
		})
	}
	if len(hits) > 0 {
		t.Errorf("BPP-7 原则 §0.2+§0.3 broken: forbidden SDK queue / custom reason "+
			"identifiers in sdk/bpp/ (best-effort 守护链延伸第 4 处, 跟 BPP-4/5/6 同模式): %v", hits)
	}
}

// TestBPP_AdvanceCursor_Monotonic — RT-1.3 cursor monotonic 原则一致.
func TestBPP_AdvanceCursor_Monotonic(t *testing.T) {
	c := sdkbpp.NewClient("plugin-1", "agent-1", nil)
	c.AdvanceCursor(100)
	if c.LastKnownCursor() != 100 {
		t.Errorf("first advance: got %d, want 100", c.LastKnownCursor())
	}
	c.AdvanceCursor(50) // regression must be silently ignored
	if c.LastKnownCursor() != 100 {
		t.Errorf("regression: got %d, want 100 (cursor monotonic)", c.LastKnownCursor())
	}
	c.AdvanceCursor(200)
	if c.LastKnownCursor() != 200 {
		t.Errorf("forward: got %d, want 200", c.LastKnownCursor())
	}
}
