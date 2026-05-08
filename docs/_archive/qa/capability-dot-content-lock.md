# CAPABILITY-DOT content lock — capability dot-notation 字面 byte-identical 跟蓝图 §3 (≤40 行)

> capability 字面 SSOT, 跟 stance §1+§2+§4 byte-identical. **蓝图 auth-permissions.md §3 字面单源真值锚** — 改一处 = 改两处 (蓝图 + capabilities.go) reflect-lint 守门. user-visible UI / DM body / error code 真改字面.

## §1 17 capability 字面 byte-identical 跟蓝图 §3
| domain | cap (蓝图字面) |
|---|---|
| messaging | `message.send` / `message.read` / `message.edit_own` / `message.delete_own` / `mention.user` |
| workspace | `workspace.read` / `artifact.create` / `artifact.edit_content` / `artifact.modify_structure` |
| channel | `channel.create` / `channel.invite_user` / `channel.invite_agent` / `channel.manage_members` / `channel.set_topic` / `channel.delete` |
| org | `agent.manage` / `*` (admin only) |

## §2 user-visible UI / DM body 字面真改
- BPP-3.2 system DM body: `{agent_name} 想 {attempted_action} 但缺权限 {required_capability}` — `{required_capability}` 渲染 dot-notation 字面 (例: `message.send`)
- DM body 模板字面 byte-identical 跟 BPP-3.2 #495 既有 + 字面值改 dot-notation
- error code 在 BPP-3.1 PermissionDeniedFrame `required_capability` 字段渲染 dot-notation

## §3 反约束 — snake_case 字面真清 (反向 grep 0 hit)
反向 grep `"[a-z]+_[a-z_]+"` 在 capabilities.go + UI / DM body / error code 渲染路径 0 hit (snake_case literal 真清). 注: 内部 i18n key 用 read_channel.* 不算违规, 仅 user-visible string literal + capability literal 违规.

## §4 反约束 — typing/loading + thinking 类禁词 (反向 grep 0 hit)
**禁词** (跟 BPP-3+CV-7+...+RT-3+HB-2+AP-2+CS-2+ADM-2 承袭): `typing` / `composing` / `loading` / `加载中` / `processing` / `responding` / `thinking` / `analyzing` / `planning` 在 capability rename 路径 0 hit.

## §5 DOM data-attr SSOT (UI 渲染 capability 字面)
| attr | 取值 |
|---|---|
| `data-cap-required` | dot-notation 字面 (例: `message.send`) |
| `data-cap-domain` | `message` / `workspace` / `channel` / `org` |

## §6 真测 grep 锚 (CI / PR 真验)
```
git grep -nE '"message\.send"|"channel\.read"|"artifact\.create"|"agent\.manage"' packages/server-go/internal/auth/   # ≥4 hit
git grep -nE '"read_channel"|"write_channel"|"commit_artifact"|"manage_members"' packages/server-go/internal/   # 0 hit (snake_case 真清)
git grep -nE 'data-cap-(required|domain)' packages/client/   # ≥2 hit
```
