// Package bpp_test — al_2b_frames_test.go: AL-2b (#452) BPP frame
// byte-identity locks for AgentConfigUpdateFrame (7 字段) +
// AgentConfigAckFrame (7 字段) + status enum CHECK.
//
// 出处: docs/qa/acceptance-templates/al-2b.md §1.1 / §1.2 + 蓝图
// `plugin-protocol.md` §1.5 (热更新分级 + 幂等 reload + runtime 不缓存)
// + §2.1 (control-plane row `agent_config_update` + data-plane row
// `agent_config_ack`).
//
// 反向约束: 字段顺序脱节 = lint fail = PR 卡 (跟 BPP-1 #304 envelope CI
// lint reflect 自动覆盖同模式 — frame_schemas_test.go ④ 已守 whitelist
// closure, 此 file 加 §1.1/§1.2 byte-identical 字面串锁定 + status enum
// CHECK reject).
package bpp_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"borgee-server/internal/bpp"
)

// TestAL_AgentConfigUpdateFrameFieldOrder pins acceptance §1.1 — 7
// 字段 byte-identical envelope:
//
//   {type, cursor, agent_id, schema_version, blob, idempotency_key, created_at}
//
// JSON key order follows struct declaration order. Drift here breaks
// the BPP-2 dispatcher contract + AL-2a 单一来源 round-trip simultaneously.
func TestAL_AgentConfigUpdateFrameFieldOrder(t *testing.T) {
	t.Parallel()

	frame := bpp.AgentConfigUpdateFrame{
		Type:           bpp.FrameTypeBPPAgentConfigUpdate,
		Cursor:         42,
		AgentID:        "agent-A",
		SchemaVersion:  3,
		Blob:           `{"name":"BotZ","prompt":"…"}`,
		IdempotencyKey: "idem-X",
		CreatedAt:      1700000000000,
	}
	b, err := json.Marshal(&frame)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"type":"agent_config_update","cursor":42,"agent_id":"agent-A","schema_version":3,"blob":"{\"name\":\"BotZ\",\"prompt\":\"…\"}","idempotency_key":"idem-X","created_at":1700000000000}`
	if string(b) != want {
		t.Fatalf("AgentConfigUpdate envelope byte-identity broken (acceptance §1.1):\n got: %s\nwant: %s", string(b), want)
	}

	// Zero-valued tail (cursor=0, schema_version=0, created_at=0) — 始终
	// 序列化, 不挂 omitempty (跟 IterationStateChangedFrame.CompletedAt
	// 同模式 — JSON byte-identity 不分支).
	zero := bpp.AgentConfigUpdateFrame{
		Type:           bpp.FrameTypeBPPAgentConfigUpdate,
		AgentID:        "agent-B",
		Blob:           "",
		IdempotencyKey: "idem-Y",
	}
	b, err = json.Marshal(&zero)
	if err != nil {
		t.Fatal(err)
	}
	wantZero := `{"type":"agent_config_update","cursor":0,"agent_id":"agent-B","schema_version":0,"blob":"","idempotency_key":"idem-Y","created_at":0}`
	if string(b) != wantZero {
		t.Fatalf("AgentConfigUpdate zero-valued envelope byte-identity broken:\n got: %s\nwant: %s", string(b), wantZero)
	}
}

// TestAL_AgentConfigAckFrameFieldOrder pins acceptance §1.2 — 7 字段
// byte-identical:
//
//   {type, cursor, agent_id, schema_version, status, reason, applied_at}
//
// Direction lock = plugin→server (反向断言 server_to_plugin 不在此 frame).
func TestAL_AgentConfigAckFrameFieldOrder(t *testing.T) {
	t.Parallel()

	applied := bpp.AgentConfigAckFrame{
		Type:          bpp.FrameTypeBPPAgentConfigAck,
		Cursor:        42,
		AgentID:       "agent-A",
		SchemaVersion: 3,
		Status:        bpp.AgentConfigAckStatusApplied,
		Reason:        "",
		AppliedAt:     1700000000001,
	}
	b, err := json.Marshal(&applied)
	if err != nil {
		t.Fatal(err)
	}
	wantApplied := `{"type":"agent_config_ack","cursor":42,"agent_id":"agent-A","schema_version":3,"status":"applied","reason":"","applied_at":1700000000001}`
	if string(b) != wantApplied {
		t.Fatalf("AgentConfigAck applied envelope byte-identity broken (acceptance §1.2):\n got: %s\nwant: %s", string(b), wantApplied)
	}

	// stale 路径 — schema_version 落后于 server, ack 携带 plugin 已知值,
	// reason 复用 AL-1a #249 reason 字面 byte-identical.
	stale := bpp.AgentConfigAckFrame{
		Type:          bpp.FrameTypeBPPAgentConfigAck,
		Cursor:        43,
		AgentID:       "agent-A",
		SchemaVersion: 2, // plugin 旧值
		Status:        bpp.AgentConfigAckStatusStale,
		Reason:        "unknown",
		AppliedAt:     0, // stale 态时 0 (反向约束 不挂 omitempty 始终序列化)
	}
	b, err = json.Marshal(&stale)
	if err != nil {
		t.Fatal(err)
	}
	wantStale := `{"type":"agent_config_ack","cursor":43,"agent_id":"agent-A","schema_version":2,"status":"stale","reason":"unknown","applied_at":0}`
	if string(b) != wantStale {
		t.Fatalf("AgentConfigAck stale envelope byte-identity broken:\n got: %s\nwant: %s", string(b), wantStale)
	}

	// rejected 路径 — schema_version 不接受, plugin 拒绝 apply.
	rejected := bpp.AgentConfigAckFrame{
		Type:          bpp.FrameTypeBPPAgentConfigAck,
		Cursor:        44,
		AgentID:       "agent-B",
		SchemaVersion: 3,
		Status:        bpp.AgentConfigAckStatusRejected,
		Reason:        "runtime_crashed",
		AppliedAt:     0,
	}
	b, err = json.Marshal(&rejected)
	if err != nil {
		t.Fatal(err)
	}
	wantRejected := `{"type":"agent_config_ack","cursor":44,"agent_id":"agent-B","schema_version":3,"status":"rejected","reason":"runtime_crashed","applied_at":0}`
	if string(b) != wantRejected {
		t.Fatalf("AgentConfigAck rejected envelope byte-identity broken:\n got: %s\nwant: %s", string(b), wantRejected)
	}
}

// TestAL_AgentConfigAckDirectionLock pins acceptance §1.2 direction
// 锁定 — plugin→server only (反向断言 DirectionServerToPlugin 不在此
// frame). 跟 BPP-1 #304 direction lock 同模式 reflect 自动覆盖.
func TestAL_AgentConfigAckDirectionLock(t *testing.T) {
	t.Parallel()

	got := bpp.AgentConfigAckFrame{}.FrameDirection()
	if got != bpp.DirectionPluginToServer {
		t.Errorf("AgentConfigAckFrame direction: got %q, want %q (acceptance §1.2 plugin→server lock)",
			got, bpp.DirectionPluginToServer)
	}

	// AgentConfigUpdate 是 server→plugin (跟 ack 反向).
	gotUpdate := bpp.AgentConfigUpdateFrame{}.FrameDirection()
	if gotUpdate != bpp.DirectionServerToPlugin {
		t.Errorf("AgentConfigUpdateFrame direction: got %q, want %q (acceptance §1.1 server→plugin lock)",
			gotUpdate, bpp.DirectionServerToPlugin)
	}
}

// TestAL_AgentConfigAckStatusEnum pins acceptance §1.2 status CHECK
// — 3 态 byte-identical ('applied' | 'rejected' | 'stale'). 反向约束:
// fail-closed 校验 reject 'unknown' / '' / 同义词脱节.
//
// schema 层无 CHECK enum (BPP frame 是 wire format, 不是 SQL); 此 test
// 跟 al_4_1 TestAL41_RejectsInvalidStatus / cv_3_2 ValidArtifactKinds
// 同模式 — server-side validator function gate.
func TestAL_AgentConfigAckStatusEnum(t *testing.T) {
	t.Parallel()

	// 白名单 3 态合法.
	for _, ok := range []string{
		bpp.AgentConfigAckStatusApplied,
		bpp.AgentConfigAckStatusRejected,
		bpp.AgentConfigAckStatusStale,
	} {
		if !isValidAckStatus(ok) {
			t.Errorf("status %q rejected — should accept (acceptance §1.2 CHECK)", ok)
		}
	}

	// 枚举外值 reject.
	for _, bad := range []string{
		"unknown", "ok", "fail", "",
		"APPLIED",   // 大小写脱节
		"applying",  // 中间态脱节
		"completed", // CV-4 状态脱节
	} {
		if isValidAckStatus(bad) {
			t.Errorf("status %q accepted — should reject (acceptance §1.2 CHECK 3 态 fail-closed)", bad)
		}
	}
}

// isValidAckStatus is the AL-2b ack status enum validator. Mirrors the
// CHECK constraint a SQL schema would enforce; lives in test until
// AL-2b.2 server hook lands (then it'll move to a proper validator
// alongside cv_3_2 ValidateArtifactKind).
func isValidAckStatus(s string) bool {
	return s == bpp.AgentConfigAckStatusApplied ||
		s == bpp.AgentConfigAckStatusRejected ||
		s == bpp.AgentConfigAckStatusStale
}

// TestAL_AgentConfigUpdate7Fields + Ack7Fields pin acceptance §4.1
// — 字段顺序漂移防御. Reflection-based field count + JSON tag scan.
// 跟 BPP-1 #304 reflect 自动覆盖同模式.
func TestAL_AgentConfigUpdate7Fields(t *testing.T) {
	t.Parallel()

	want := []struct {
		name string
		json string
	}{
		{"Type", "type"},
		{"Cursor", "cursor"},
		{"AgentID", "agent_id"},
		{"SchemaVersion", "schema_version"},
		{"Blob", "blob"},
		{"IdempotencyKey", "idempotency_key"},
		{"CreatedAt", "created_at"},
	}

	typ := reflect.TypeOf(bpp.AgentConfigUpdateFrame{})
	if got := typ.NumField(); got != len(want) {
		t.Fatalf("AgentConfigUpdateFrame field count: got %d, want %d (acceptance §1.1 7 字段)", got, len(want))
	}
	for i, w := range want {
		f := typ.Field(i)
		if f.Name != w.name {
			t.Errorf("field %d name: got %q, want %q", i, f.Name, w.name)
		}
		if tag := f.Tag.Get("json"); tag != w.json {
			t.Errorf("field %d json tag: got %q, want %q", i, tag, w.json)
		}
	}
}

func TestAL_AgentConfigAck7Fields(t *testing.T) {
	t.Parallel()

	want := []struct {
		name string
		json string
	}{
		{"Type", "type"},
		{"Cursor", "cursor"},
		{"AgentID", "agent_id"},
		{"SchemaVersion", "schema_version"},
		{"Status", "status"},
		{"Reason", "reason"},
		{"AppliedAt", "applied_at"},
	}

	typ := reflect.TypeOf(bpp.AgentConfigAckFrame{})
	if got := typ.NumField(); got != len(want) {
		t.Fatalf("AgentConfigAckFrame field count: got %d, want %d (acceptance §1.2 7 字段)", got, len(want))
	}
	for i, w := range want {
		f := typ.Field(i)
		if f.Name != w.name {
			t.Errorf("field %d name: got %q, want %q", i, f.Name, w.name)
		}
		if tag := f.Tag.Get("json"); tag != w.json {
			t.Errorf("field %d json tag: got %q, want %q", i, tag, w.json)
		}
	}
}

// TestAL_NoBlobRuntimeOnlyFields pins acceptance §3.2 — 单一来源原则一致.
// frame `Blob` 是 opaque JSON wire payload (server 端 marshal 单一来源 字段);
// 此 test 反向断言 Blob 不是结构体直嵌 runtime-only 字段 (api_key /
// temperature / token_limit / retry_policy). 真实校验在 AL-2b.2 server
// PATCH hook (fail-closed 跟 AL-2a #447 TestAL2A1_NoDomainBleed 同源),
// 此 PR frame 层仅锁定 Blob 是 string opaque (反向约束 schema 层 NoDomainBleed).
func TestAL_NoBlobRuntimeOnlyFields(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(bpp.AgentConfigUpdateFrame{})
	blobField, ok := typ.FieldByName("Blob")
	if !ok {
		t.Fatal("AgentConfigUpdateFrame missing Blob field")
	}
	if blobField.Type.Kind() != reflect.String {
		t.Errorf("Blob must be opaque string (server 端 marshal 后传 wire); got Kind=%v",
			blobField.Type.Kind())
	}

	// 反向: frame 不直接暴露 runtime-only 字段名 (AL-2a #447 单一来源 原则
	// 反向约束 — api_key/temperature 是 plugin 内部事, 不进 server 单一来源).
	for _, forbidden := range []string{
		"APIKey", "ApiKey", "Temperature", "TokenLimit", "RetryPolicy",
		"LLMProvider", "ModelName", // AL-4.1 #398 反向约束同源
	} {
		if _, has := typ.FieldByName(forbidden); has {
			t.Errorf("AgentConfigUpdateFrame leaks runtime-only field %q — 反向约束 broken (acceptance §3.2 + AL-2a #447 单一来源)", forbidden)
		}
	}
}
