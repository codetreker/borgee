// RT-3 ⭐ reverse-grep 反向断言 (rt-3-spec.md §2 反向约束 #3+#4 + content-lock §3+§4).
//
// 原则 (rt-3-spec.md §0):
//   - 原则 ② PresenceState 4 态 enum 单一来源 (count==4)
//   - 原则 ② thinking subject 反向约束 (蓝图 §1.1 ⭐)
//   - content-lock §3 typing 类同义词 0 hit (反 typing-indicator 脱节)
//   - content-lock §4 thinking 5-pattern 守护链第 N+1 处延伸 (RT-3 路径 0 hit)

package datalayer

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestRT3_PresenceState_FourEnumSingleSource pins rt-3-spec.md §2 反向约束 #3 —
// PresenceState 4 态 const 单一来源 count==4 hit (反第 5 态脱节).
func TestRT3_PresenceState_FourEnumSingleSource(t *testing.T) {
	data, err := os.ReadFile("presence.go")
	if err != nil {
		t.Fatalf("read presence.go: %v", err)
	}
	body := string(data)
	expect := []string{
		"PresenceStateOnline",
		"PresenceStateAway",
		"PresenceStateOffline",
		"PresenceStateThinking",
	}
	for _, name := range expect {
		// `PresenceStateXxx PresenceState = "..."` 单一来源 const 各 1 hit.
		re := regexp.MustCompile(`(?m)^\s*` + name + `\s+PresenceState\s*=\s*"`)
		hits := len(re.FindAllString(body, -1))
		if hits != 1 {
			t.Errorf("原则 ② — %s 单一来源 const want ==1 hit, got %d (反 5 态脱节 / 反复制定义)", name, hits)
		}
	}
	// 反第 5 态: PresenceState{Typing,Composing,Idle,Pending,Loading} 等 0 hit.
	forbidden := []string{
		"PresenceStateTyping",
		"PresenceStateComposing",
		"PresenceStateIdle",
		"PresenceStatePending",
		"PresenceStateLoading",
	}
	for _, name := range forbidden {
		if strings.Contains(body, name) {
			t.Errorf("原则 ② 反 5 态脱节 — %s 不应存在 (4 态封闭 enum, 反 typing-indicator 脱节入)", name)
		}
	}
}

// TestRT3_NoTypingIndicator_InRT3Paths pins content-lock §3 反向约束 —
// typing 类同义词 (英 + 中) 在 **RT-3 域新增/修改** 路径 0 hit (反
// typing-indicator 脱节入). RT-2 既有 ws/client.go typing handler 是历史
// 路径不算 RT-3 脱节 (RT-3 是 multi-device fanout + presence 4 态, 不动
// RT-2 typing).
//
// 范围: internal/datalayer/presence.go (RT-3 4 态 enum 单一来源) — 此处不允
// 任何 typing 类字面 (反"假 loading" 脱节入 enum 域).
func TestRT3_NoTypingIndicator_InRT3Paths(t *testing.T) {
	rt3Files := []string{
		"presence.go", // 4 态 enum 单一来源 — typing 类字面禁
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`\b(typing|composing|isTyping|userTyping|composingIndicator)\b`),
		regexp.MustCompile(`正在输入|正在打字|输入中|打字中`),
	}
	hits := []string{}
	for _, f := range rt3Files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		body := string(data)
		for _, re := range patterns {
			if m := re.FindString(body); m != "" {
				hits = append(hits, f+": "+m)
			}
		}
	}
	if len(hits) > 0 {
		t.Errorf("content-lock §3 反向约束 — typing 类同义词 0 hit in RT-3 path, 命中 %d:\n%s",
			len(hits), strings.Join(hits, "\n"))
	}
}

// TestRT3_Thinking5PatternChain_NoFallback pins content-lock §4 — thinking
// 5-pattern 守护链 RT-3 = 第 N+1 处延伸 (反 "AI is thinking…" / "假 loading" 脱节).
//
// 5 字面在 internal/datalayer/ + internal/ws/ presence path 排除 _test.go 0 hit.
func TestRT3_Thinking5PatternChain_NoFallback(t *testing.T) {
	patterns := []string{
		`subject\s*=\s*""`, // empty subject fallback
		`defaultSubject`,
		`fallbackSubject`,
		`"AI is thinking"`,
		`"AI is thinking…"`,
	}
	root := "."
	hits := []string{}
	for _, pat := range patterns {
		re := regexp.MustCompile(pat)
		_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			data, _ := os.ReadFile(p)
			if re.Match(data) {
				hits = append(hits, p+": "+pat)
			}
			return nil
		})
	}
	if len(hits) > 0 {
		t.Errorf("content-lock §4 — thinking 5-pattern 守护链 RT-3 0 hit, 命中 %d:\n%s",
			len(hits), strings.Join(hits, "\n"))
	}
}
