// Package api — cv_8_thread_validator.go: CV-8 artifact comment thread reply
// thinking-subject 5-pattern validator (5-pattern 第 6 处链 byte-identical 跟
// RT-3 + BPP-2.2 + AL-1b + CV-5 + CV-7 同字符).
//
// Blueprint锚: docs/blueprint/current/realtime.md §1.1 ⭐ "thinking 必须带 subject".
// Spec: docs/implementation/modules/cv-8-spec.md §0 立场 ③.
//
// 5-pattern (改 = 改 6 处 byte-identical):
//   1. trailing "thinking" — body 末为 "thinking"
//   2. defaultSubject literal
//   3. fallbackSubject literal
//   4. "AI is thinking" 字面
//   5. subject="" 空字符串 / whitespace-only (caught upstream by "Message
//      content is required" guard, present here for SSOT completeness)
//
// 反向 grep 锚 (CI lint 必跑):
//   `body.*"thinking"$|defaultSubject|fallbackSubject|"AI is thinking"|subject\s*=\s*""`
//   在 internal/api/ 排除 _test.go count==0.
package api

import (
	"regexp"
	"strings"
)

var thinkingSubjectSentinelsCV8 = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bthinking\s*$`),
	regexp.MustCompile(`defaultSubject`),
	regexp.MustCompile(`fallbackSubject`),
	regexp.MustCompile(`AI is thinking`),
}

// violatesThinkingSubjectCV8 returns true iff body fails the 5-pattern guard
// (CV-8 6th-site lock, byte-identical CV-5 / CV-7).
func violatesThinkingSubjectCV8(body string) bool {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return true
	}
	for _, re := range thinkingSubjectSentinelsCV8 {
		if re.MatchString(trimmed) {
			return true
		}
	}
	return false
}
