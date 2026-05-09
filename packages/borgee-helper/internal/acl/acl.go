// Package acl — HB-2 IPC request gate (path normalization +
// cross-agent ACL + grants 校验). 所有 IPC call 入口必经.
//
// hb-2-spec.md §4 反向约束 #2 (路径越界 100% reject) + #4 (cross-agent
// ACL) + #7 (写类 IPC 100% reject).
package acl

import (
	"context"
	"path/filepath"
	"strings"

	"borgee-helper/internal/grants"
	"borgee-helper/internal/reasons"
)

// Action 是 IPC request action (read-only set; 写类全 reject).
type Action string

const (
	ActionListFiles      Action = "list_files"
	ActionReadFile       Action = "read_file"
	ActionNetworkEgress  Action = "network_egress"
)

// readOnlyActions = 反向约束 #7 白名单 (所有非此 set 的 action 全 reject).
var readOnlyActions = map[Action]bool{
	ActionListFiles:     true,
	ActionReadFile:      true,
	ActionNetworkEgress: true,
}

// IsReadOnly 反向枚举出处 — 单测覆盖每种写法 (write_file / delete_file /
// chmod / chown / mkdir / rmdir / mv / cp ...) 全 reject.
func IsReadOnly(a Action) bool {
	return readOnlyActions[a]
}

// Gate 决定 IPC request 通过/拒绝; 不依赖 IO 真启 (单测可注入 mock consumer).
type Gate struct {
	Grants grants.Consumer
}

// New 构造 gate (consumer 由 caller 注入; v0(C) 走 mock, HB-3 后真接 SQL).
func New(c grants.Consumer) *Gate {
	return &Gate{Grants: c}
}

// Decision 是 ACL 决策结果.
type Decision struct {
	Allow  bool
	Reason reasons.Reason // 拒绝理由 (Allow=true 时 = OK)
	Scope  string         // matched grant scope (audit 写 target/scope 用)
}

// Decide 主入口 — 按 (handshakeAgentID, requestAgentID, action, target) 决策.
//
// handshakeAgentID = IPC 连接握手注册的 agent_id (daemon 持有);
// requestAgentID = 当前 request payload 携带的 agent_id;
// 二者不一致 → cross_agent_reject (反向约束 #4).
func (g *Gate) Decide(ctx context.Context, handshakeAgentID, requestAgentID string, action Action, target string) Decision {
	// ① 写类 100% reject (反向约束 #7) — 单测反向枚举守.
	if !IsReadOnly(action) {
		return Decision{Allow: false, Reason: reasons.IOFailed}
	}
	// ② cross-agent ACL (反向约束 #4).
	if handshakeAgentID == "" || requestAgentID == "" || handshakeAgentID != requestAgentID {
		return Decision{Allow: false, Reason: reasons.CrossAgentReject}
	}
	// ③ 路径 normalization + traversal reject (反向约束 #2; 仅文件类 action).
	scope := target
	if action == ActionListFiles || action == ActionReadFile {
		clean, ok := normalizePath(target)
		if !ok {
			return Decision{Allow: false, Reason: reasons.PathOutsideGrants}
		}
		scope = "fs:" + clean
	} else if action == ActionNetworkEgress {
		// network_egress: scope = "egress:<host>"; caller 已 normalize URL.
		scope = "egress:" + target
	}
	// ④ grants lookup (read-only consumer; 反向约束 #3 不缓存).
	mc, ok := g.Grants.(interface {
		LookupRaw(context.Context, string, string) (grants.Grant, bool, bool, error)
	})
	if ok {
		_, exists, expired, err := mc.LookupRaw(ctx, requestAgentID, scope)
		if err != nil {
			return Decision{Allow: false, Reason: reasons.IOFailed}
		}
		if !exists {
			return Decision{Allow: false, Reason: reasons.GrantNotFound}
		}
		if expired {
			return Decision{Allow: false, Reason: reasons.GrantExpired}
		}
		return Decision{Allow: true, Reason: reasons.OK, Scope: scope}
	}
	gr, ok2, err := g.Grants.Lookup(ctx, requestAgentID, scope)
	if err != nil {
		return Decision{Allow: false, Reason: reasons.IOFailed}
	}
	if !ok2 {
		return Decision{Allow: false, Reason: reasons.GrantNotFound}
	}
	return Decision{Allow: true, Reason: reasons.OK, Scope: gr.Scope}
}

// normalizePath 反 traversal — 拒 .. 分量 + 必须 abs + 拒 NUL byte.
// 不解符号链接 (运行期 IO; v0(C) 留 OS-layer landlock + sandbox-exec
// 守, hb-2-spec.md §5.5 sandbox build tag 拆).
func normalizePath(p string) (string, bool) {
	if p == "" || strings.ContainsRune(p, 0) {
		return "", false
	}
	if !filepath.IsAbs(p) {
		return "", false
	}
	clean := filepath.Clean(p)
	// Clean 不解 .. 跨根 (Linux Clean("/a/../b") = "/b") 但拒输入显式含 ../
	// 段后再 Clean 等价 ascend — 用原始字符串扫一遍守.
	for _, seg := range strings.Split(p, string(filepath.Separator)) {
		if seg == ".." {
			return "", false
		}
	}
	return clean, true
}
