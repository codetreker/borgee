// Package auth — abac.go: AP-1 设计 ② ABAC capability check 单 SSOT.
//
// 单 helper `HasCapability(ctx, permission, scope) bool` — 所有 endpoint
// authz 走此一处, 不字面 hardcode permission name (反 grep 守 spec §2 #1).
//
// SSOT: `user_permissions(user_id, permission, scope)` 表 (蓝图 §1.1
// + data-layer.md). agent 是 user_id 一种, 同 ABAC.
//
// Spec锚: docs/implementation/modules/ap-1-spec.md §1 设计 ② + §3 文件
// 清单 (`internal/auth/abac.go` 单 SSOT).
//
// AP-3 (战马C, d69b617): 加 1 层 cross-org owner-only gate — grantee
// `user.org_id` ≠ resource org_id (channel.org_id, artifact 走所属
// channel.org_id) → false. NULL = legacy 行 (跟 AP-1 现网行为零变, 任一
// NULL 走 legacy 路径). admin god-mode 不入此路径 (走 /admin-api/* 单独
// mw, ADM-0 §1.3 红线). grep 检查 `cross.org.*bypass\|skip.*org.*check`
// 在 internal/api/ count==0.
//
// 蓝图锚: docs/blueprint/current/auth-permissions.md §1.1 (ABAC source of truth)
// + §1.2 (Scope 三层 v1: `*` / `channel:<id>` / `artifact:<id>`) + §5
// (cross-org 强制 — AP-3 后续 milestone) + channel-model.md §1.4 (主权列).
//
// 反约束: bundle 字面不入 server (spec §2 #3 — bundle 是 client UI 糖,
// server 端只看 capability list); admin god-mode 不入此 ABAC (spec §2
// #5 — admin 走 /admin-api/* 单独 mw, ADM-0 §1.3 红线).
package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"borgee-server/internal/store"
)

// ErrCodeCrossOrgDenied is the byte-identical error code emitted by
// callers when HasCapability rejects a request due to the AP-3 cross-org
// gate (用于 endpoint 错误响应 body, 跟 AP-1 const 单源同模式).
//
// Drift between this const and handler hardcoded strings is caught by
// reverse grep: `"abac\.cross_org_denied"` in internal/ ≥1 hit
// (capabilities.go const) + 0 hit hardcode in handler — 改 = 改 const
// 一处.
const ErrCodeCrossOrgDenied = "abac.cross_org_denied"

// scopeOrgResolver is the seam into store for resolving a scope string's
// org_id. Mirrors the existing store seam pattern (跟 channelScope helper
// 同精神). nil-safe — if Store is nil the org gate is skipped (跟 NULL
// 行兼容 AP-1 现网精神).
type scopeOrgResolver interface {
	GetChannelByID(id string) (*store.Channel, error)
}

// resolveScopeOrgID extracts the org_id of the resource referenced by
// scope. Returns ("", false) when the scope cannot be resolved (e.g.
// scope == "*" — wildcard, no resource bound; or unknown channel/artifact
// — handler will 404 on its own path). Resolved empty string also
// returns ("", false) — empty == legacy / unset, NOT cross-org evidence.
//
// Scope format (蓝图 §1.2 三层):
//   - "*"               → no resource, ("", false) (skip org gate)
//   - "channel:<id>"    → channel.org_id from store
//   - "artifact:<id>"   → artifact.channel_id → channel.org_id (CV-1 设计
//                         ① 归属=channel, artifact 跟 channel 同 org 是
//                         CM-3 #208 既有不变量)
func resolveScopeOrgID(s scopeOrgResolver, scope string) (string, bool) {
	if s == nil || scope == "" || scope == "*" {
		return "", false
	}
	if strings.HasPrefix(scope, "channel:") {
		channelID := strings.TrimPrefix(scope, "channel:")
		if channelID == "" {
			return "", false
		}
		ch, err := s.GetChannelByID(channelID)
		if err != nil || ch == nil {
			return "", false
		}
		if ch.OrgID == "" {
			return "", false
		}
		return ch.OrgID, true
	}
	if strings.HasPrefix(scope, "artifact:") {
		// artifact 跟 channel 同 org (CV-1 设计 ① 归属=channel + CM-3 #208).
		// Lookup goes through store seam — interface kept slim by adding a
		// store path ad-hoc when needed; for v0 we resolve via raw SQL on
		// the *store.Store concrete type via the wider seam.
		// Defer to the Store-typed path below when Store has artifact lookup.
		if s2, ok := s.(*store.Store); ok {
			var channelID string
			if err := s2.DB().Raw(
				`SELECT channel_id FROM artifacts WHERE id = ?`,
				strings.TrimPrefix(scope, "artifact:"),
			).Row().Scan(&channelID); err != nil || channelID == "" {
				return "", false
			}
			ch, err := s2.GetChannelByID(channelID)
			if err != nil || ch == nil || ch.OrgID == "" {
				return "", false
			}
			return ch.OrgID, true
		}
		return "", false
	}
	// Unknown scope prefix — skip org gate (forward-compat to v2+ scope
	// 层级扩展, 蓝图 §1.2 留账).
	return "", false
}

// HasCapability is the single SSOT capability check — all endpoint
// authz routes through this helper. Returns true iff the user (from
// context) has been granted (permission, scope), with the v1 scope
// fallback hierarchy:
//
//   - explicit (permission, scope) row → grant
//   - explicit (permission, "*") row → grant (wildcard scope)
//   - explicit ("*", "*") wildcard row → grant (human admin / AP-0
//     default; agent 不享, 蓝图 §1.4 字面承袭, 见 IsAgent 守)
//
// AP-3 cross-org gate (战马C v0): 在 above 任意 grant 命中前先过 org
// gate — grantee `user.org_id` 与 resource org_id (resolveScopeOrgID)
// 都非空且不等 → false 直返 (cross-org owner-only). 任一 NULL/empty 走
// legacy 路径 (跟 AP-1 现网行为零变, 设计 ⑥).
//
// 反约束 (spec §2 + 蓝图 §1.4 + §2 不变量 + AP-3 设计 ①③⑤⑦):
//   - admin 不入此路径 — admin god-mode 走 /admin-api/* (ADM-0 §1.3).
//   - agent 不享 (*,*) wildcard — IsAgent 守, owner 即使误 grant 也 403.
//   - bundle 字面不入 — bundle 是 client UI 糖, server 只看 capability.
//   - cross-org agent path 走同 SSOT (BPP-1 #304 org sandbox 同源).
//
// Returns (granted bool). Use store.Store.ListUserPermissions through
// the package var hook so tests can swap.
func HasCapability(ctx context.Context, s *store.Store, permission, scope string) bool {
	user := UserFromContext(ctx)
	if user == nil {
		return false
	}
	// AP-3 设计 ① cross-org owner-only gate — 高于 wildcard 短路, 在
	// permission 命中前先 reject.
	if resourceOrgID, ok := resolveScopeOrgID(s, scope); ok {
		if user.OrgID != "" && user.OrgID != resourceOrgID {
			return false
		}
	}
	perms, err := s.ListUserPermissions(user.ID)
	if err != nil {
		return false
	}
	isAgent := user.Role == "agent"
	for _, p := range perms {
		// agent 不享 (*,*) 短路 — 蓝图 §1.4 设计字面.
		if !isAgent {
			if p.Permission == "*" && p.Scope == "*" {
				return true
			}
		}
		if p.Permission == permission && (p.Scope == "*" || p.Scope == scope) {
			return true
		}
	}
	return false
}

// ChannelScopeStr / ArtifactScopeStr build canonical scope strings
// for use as the `scope` argument to HasCapability. Mirrors the
// existing channelScope() pattern in api/channels.go (single source).
func ChannelScopeStr(channelID string) string  { return fmt.Sprintf("channel:%s", channelID) }
func ArtifactScopeStr(artifactID string) string { return fmt.Sprintf("artifact:%s", artifactID) }

// ArtifactScope is a request-time helper: extracts {artifactId}
// PathValue and returns "artifact:{id}". Mirrors channelScope() for
// use as the scopeResolver in middleware-style call sites.
func ArtifactScope(r *http.Request) string {
	return ArtifactScopeStr(r.PathValue("artifactId"))
}
