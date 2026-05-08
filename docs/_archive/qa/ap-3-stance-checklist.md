# AP-3 立场反查清单 (战马C v0)

> 战马C · 2026-04-29 · 立场 review checklist (跟 AP-1 #493 + AL-5 #d2622ec + REFACTOR-REASONS #496 同模式)
> **目的**: AP-3 三段实施 (3.1 schema + 错码 / 3.2 abac.HasCapability 加 org gate / 3.3 e2e + closure) PR review 时, 飞马 / 烈马按此清单逐立场 sign-off, 反向断言代码层守住每条立场.
> **关联**: spec `docs/implementation/modules/ap-3-spec.md` (战马C v0, d69b617) + acceptance `docs/qa/acceptance-templates/ap-3.md`. 复用 AP-1 #493 HasCapability SSOT + capabilities.go 14 const + AP-1.1 #493 user_permissions.expires_at ALTER ADD COLUMN NULL 模式 + CM-3 #208 cross-org 资源归属 + ADM-0 §1.3 admin god-mode 红线.

## §0 立场总表 (3 立场 + 5 边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | cross-org owner-only 强制 | auth-permissions.md §5 + channel-model.md §1.4 + CM-3 #208 字面承袭 | `HasCapability` 加 1 层 org gate (取 grantee `user.org_id` + resource org from channel.org_id); cross-org → false; 反向 grep `cross.org.*bypass\|skip.*org.*check` count==0 |
| ② | user_permissions 加 org_id scope (NULL 兼容 legacy) | auth-permissions.md §1.2 + AP-1.1 #493 expires_at ALTER ADD COLUMN NULL 同模式 | `user_permissions.org_id TEXT NULL` — NULL = legacy 行 inheritance 不破 AP-1 现网行为; sparse index `idx_user_permissions_org_id WHERE org_id IS NOT NULL`; 反约束: 不挂 NOT NULL / 不挂 default / 不挂 FK organizations(id) (跟 user.org_id 同精神, 业务校验 server 层) |
| ③ | 反约束 grep cross-org bypass 0 hit | 跟 AP-1 #493 5 grep 反约束同模式守 future drift | `internal/api/` 反向 grep cross-org bypass / admin HasCapability.*org / agent cross-org permission 全 count==0 |
| ④ (边界) | 错码字面单源 (跟 AP-1 const 单源同模式) | AP-1 #493 立场 ② SSOT helper | `auth.ErrCodeCrossOrgDenied = "abac.cross_org_denied"` const (capabilities.go 字面单源, 反向 grep handler 内 hardcode `"abac.cross_org_denied"` count==0 — 改 = 改 const 一处) |
| ⑤ (边界) | admin god-mode 不入 (ADM-0 §1.3 红线) | admin-model.md §1.3 + AP-1 立场 ⑤ | admin path 走 `/admin-api/*` 单独 mw 不调 HasCapability; 反向 grep `admin.*HasCapability.*\.org\|HasCapability\(.*admin_` count==0 |
| ⑥ (边界) | NULL 行兼容 AP-1 现网 | AP-1.1 #493 expires_at NULL = 永久 同精神 | grantee 跟 resource 都有 org_id 时才 enforce, 任一 NULL 走 legacy 路径 (跟 AP-1 既有 ABAC 行为零变); 反向 grep `org_id IS NULL.*reject\|org_id\s*=\s*""\s*reject` count==0 |
| ⑦ (边界) | agent runtime 走 agent.org_id gate (BPP-1 #304 既有 org sandbox 同源) | BPP-1 #304 agent runtime org sandbox + 蓝图 §1.4 立场 "agent 不享 wildcard" | agent path 复用 abac.HasCapability 同 SSOT (agent 是 user_id 一种, AP-1 #493 立场 ④ 同精神); 反向 grep `agent.*cross.*org.*permission\|agent.*org_id.*ignore` count==0 (BPP-1 envelope whitelist 不裂) |
| ⑧ (边界) | endpoint 不改 (改 = 改 abac.go 一处) | AP-1 #493 立场 ② SSOT 单 helper | `internal/api/channels.go` / `artifacts.go` / `messages.go` / `mentions.go` 等 endpoint 文件 0 行改 (走既有 HasCapability 单 helper, AP-3 仅扩 helper 内部); git diff 验证 packages/server-go/internal/api/ 仅 _test.go 加, 0 production 改 |

## §1 立场 ① cross-org owner-only 强制 (AP-3.2 守)

**蓝图字面源**: `auth-permissions.md` §5 与现状的差距 ("cross-org 强制 — AP-3 后续 milestone") + `channel-model.md` §1.4 主权列 + CM-3 #208 cross-org 资源归属字面承袭

**反约束清单**:

- [ ] `abac.HasCapability` 加 1 层 org gate — 取 grantee `user.org_id` (auth.UserFromContext 既有路径) + resource org (channel.org_id from store.GetChannelByID, 蓝图 channel-model §1 既有列); cross-org → false 直返
- [ ] 反向 grep `cross.org.*bypass\|skip.*org.*check\|bypass.*org_id` 在 `internal/api/` count==0 (单测 TestAP32_ReverseGrep_NoCrossOrgBypass)
- [ ] artifact path 走 channel.org_id (artifact 跟 channel 同 org 是 CV-1 #334 立场 ① "归属 = channel" 既有不变量)

## §2 立场 ② user_permissions 加 org_id scope (AP-3.1 守)

**蓝图字面源**: `auth-permissions.md` §1.2 (Scope 层级 v1 三层) + AP-1.1 #493 ALTER ADD COLUMN NULL 模式 + CM-3 #208 org_id 资源归属

**反约束清单**:

- [ ] `ALTER TABLE user_permissions ADD COLUMN org_id TEXT` (NULL nullable, 不挂 default, 不挂 NOT NULL — 跟 AP-1.1 expires_at 同模式)
- [ ] `CREATE INDEX idx_user_permissions_org_id ON user_permissions(org_id) WHERE org_id IS NOT NULL` sparse index (跟 expires_at 同模式)
- [ ] 不挂 FK `org_id REFERENCES organizations(id)` (跟 user.org_id 同精神, 蓝图 §5 业务校验 server 层做; 反向 grep `user_permissions.*FOREIGN KEY.*organizations` count==0)
- [ ] 现网行 INSERT 后 org_id=NULL = legacy (TestAP31_LegacyRowsNullPreserved 守)

## §3 立场 ③ 反约束 grep cross-org bypass 0 hit (AP-3.3 守)

**蓝图字面源**: 跟 AP-1 #493 5 grep 反约束同模式守 future drift

**反约束清单**:

- [ ] `cross.org.*bypass\|skip.*org.*check\|bypass.*org_id` 在 internal/api/ count==0
- [ ] `admin.*HasCapability.*\.org\|HasCapability\(.*admin_` count==0 (admin 走 /admin-api 单独 mw, ADM-0 §1.3 红线)
- [ ] `agent.*cross.*org.*permission\|agent.*org_id.*ignore` count==0 (BPP-1 #304 既有 org sandbox 同源)
- [ ] `user_permissions.*FOREIGN KEY.*organizations` count==0 (跟 user.org_id 同精神, 业务校验 server 层)
- [ ] `"abac\.cross_org_denied"` 在 internal/ ≥1 hit (capabilities.go const) + 0 hit hardcode in handler (反 hardcode error string)

## §4 联签清单 (实施 PR 时填)

- [ ] 飞马 (spec ↔ 立场对齐): _(签)_
- [ ] 烈马 (反向 grep + 单测覆盖率 ≥84% + 5 反约束全 count==0): _(签)_
- [ ] 战马C (实施代码 ↔ 立场反查 8 项全过): _(签)_
