# AP-2 server — capability 透明 UI response shape (≤40 行)

> 落地: feat/ap-2 AP2.1 (`internal/api/users.go` /api/v1/me/permissions 加 capabilities 字段 + deriveAP2Capabilities helper + 5 unit)
> 关联: client `docs/current/client/ap-2-capabilities.md` (14 const + LABEL_MAP byte-identical)

## 1. response shape — `/api/v1/me/permissions`

```json
{
  "user_id": "...",
  "role": "member",          // 历史 caller 兼容 — UI 不显此字段
  "permissions": ["*"],
  "details": [...],
  "capabilities": [           // AP-2 单一来源 (走 14 const byte-identical 跟 auth.ALL; CAPABILITY-DOT 后 dot-notation)
    "channel.read", "channel.write", "channel.delete",
    "artifact.read", "artifact.write", "artifact.commit",
    "artifact.iterate", "artifact.rollback",
    "user.mention", "dm.read", "dm.send",
    "channel.manage_members", "channel.invite", "channel.change_role"
  ]
}
```

## 1.bis CAPABILITY-DOT (post-rename)

CAPABILITY-DOT (migration v=48) — 14 capability const 字符串值改 snake_case → dot-notation 兑现蓝图 auth-permissions.md `<domain>.<verb>` 字面. Go const 名保留 (`auth.ReadChannel` / `auth.CommitArtifact` / etc Go 命名规范不脱节); 仅字符串值改. DB backfill 14 行 per-token UPDATE (反 REPLACE 机械, verb_noun 顺序对调) + idempotent (hasColumns guard 反复跑不破). 0 schema column rename / 0 endpoint URL 改 / 0 routes.go 改 — `user_permissions.capability` TEXT 字段名不动, 仅值改.

## 2. helper — `deriveAP2Capabilities(role, permissions)`

- member + `["*"]` → 全 14 const (AP-0 default)
- agent / bundle-narrowed → permissions[] prefix 解析 + auth.IsValidCapability 反向断言 + dedupe + unknown forward-compat drop

## 3. 反向约束

- ❌ response 不暴露 RBAC role:value (admin/editor/viewer/owner) 字面 (反 role bleed)
- ❌ admin god-mode UI 永久独立 (`/admin-api/users/*` 不走本 helper)
- ❌ 0 schema / 0 endpoint URL / 0 routes.go 改

## 4. tests

- `internal/api/ap_2_capabilities_test.go` 5 unit (member full grant 14 const 顺序 + agent narrowed + agent no grant + only known tokens 反 RBAC + response JSON 反 RBAC role 字面)
