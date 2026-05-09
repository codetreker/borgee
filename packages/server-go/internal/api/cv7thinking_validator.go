// Package api — cv_7_thinking_validator.go: CV-7 thinking-subject 5-pattern
// validator (5-pattern 第 5 处链 byte-identical 跟 RT-3 + BPP-2.2 + AL-1b +
// CV-5 同字符).
//
// Blueprint锚: docs/blueprint/current/realtime.md §1.1 ⭐ "thinking 必须带 subject".
// Spec: docs/implementation/modules/cv-7-spec.md §0 设计 ③ + §3 grep 检查项.
//
// 5-pattern (改 = 改 5 处 byte-identical):
//   1. trailing "thinking" — body 末为 "thinking"
//   2. defaultSubject literal
//   3. fallbackSubject literal
//   4. "AI is thinking" 字面
//   5. subject="" 空字符串 / whitespace-only
//
// grep 检查项 (CI lint 必跑):
//   `body.*"thinking"$|defaultSubject|fallbackSubject|"AI is thinking"|subject\s*=\s*""`
//   在 internal/api/ 排除 _test.go count==0.
//
// 此函数由 messages.go::handleUpdateMessage 在 content_type='artifact_comment'
// + sender Role=='agent' 时调用; 命中任一 pattern → 400
// `comment.thinking_subject_required` byte-identical 跟 CV-5 #530.
package api

import (
	"regexp"
	"strings"
)

var thinkingSubjectSentinelsCV7 = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bthinking\s*$`),
	regexp.MustCompile(`defaultSubject`),
	regexp.MustCompile(`fallbackSubject`),
	regexp.MustCompile(`AI is thinking`),
}

// violatesThinkingSubjectCV7 mirrors the CV-5 SSOT check (lock chain 5th
// site). Returns true iff body fails the 5-pattern guard.
func violatesThinkingSubjectCV7(body string) bool {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return true // pattern 5: subject="" 空
	}
	for _, re := range thinkingSubjectSentinelsCV7 {
		if re.MatchString(trimmed) {
			return true
		}
	}
	return false
}
