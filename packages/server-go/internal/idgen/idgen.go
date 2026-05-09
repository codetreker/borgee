// Package idgen — ID generator SSOT (蓝图 §4.A.1 ULID lock-in 兑现).
//
// Spec: docs/implementation/modules/ulid-migration-spec.md §0 设计 ① +
// §1 UM.1 + 蓝图 data-layer.md §4.A.1 字面 "ID 方案 = ULID 所有业务表
// 主键, 禁 INTEGER PK / Snowflake / KSUID / UUIDv7".
//
// 设计 SSOT: 一处生成 (NewID), 反 inline `ulid.Make()` / `uuid.NewString()`
// 散落 (post-ULID-MIGRATION grep 检查 guard 0 hit).
//
// Forward-compat: NewID 返 26-char canonical ULID (Crockford base32, lex
// sortable by time). 既有 UUID-36 行不动 (db column TEXT 不限长度);
// 新行 ULID-26. 既有 UUID 比较 lex 跟 ULID 不同序 — caller 不依赖 ID lex
// (RT-1 cursor 走独立 lex_id ULID 蓝图 §4.A.4, 跟 PK 解耦).

package idgen

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	mu      sync.Mutex
	entropy = ulid.Monotonic(rand.Reader, 0)
)

// NewID returns a fresh ULID string (26 chars, monotonic within
// millisecond). Goroutine-safe via mutex around shared entropy reader
// (跟 DL-2 #615 events_store newULID 同精神 single-writer 串行, 反
// monotonic violation across goroutines).
//
// 反约束: 不暴露 ulid.ULID type — caller 仅消费 string ID (db column TEXT
// + json string 字段). 反 type ID string 抽象漂 (蓝图 §v0 代码债 audit
// 表 line 219 字面 "v1 切回 永久混用 + type ID string 抽象" — 但本 v1
// MIGRATION 仅切生成器, 不切类型抽象 留 v2+).
func NewID() string {
	mu.Lock()
	defer mu.Unlock()
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}
