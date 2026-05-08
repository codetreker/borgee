# AP-2 content lock — capability bundle UI 字面 + 角色无名化 (≤40 行)

> AP-2 UI bundle client 字面 SSOT, 跟 stance §1+§3 byte-identical. **蓝图 §1.3 角色无名化红线** — 反 role.name 字面暴露 UI; 反 typing/loading 类禁词 (跟 RT-3 ⭐ + HB-2 v0(D) + thinking 5-pattern 锁链承袭).

## §1 capability bundle 命名字面 (byte-identical)
> CAPABILITY-DOT 字面承袭 — 14 capability token byte-identical 跟蓝图
> auth-permissions.md §1 `<domain>.<verb>` 字面.

### §1.1 14 capability token 字面 (byte-identical 跟 server `auth.ALL` + client `CAPABILITY_TOKENS`)
| token | 中文 label |
|---|---|
| `channel.read` | `查看频道` |
| `channel.write` | `在频道发消息` |
| `channel.delete` | `删除频道` |
| `artifact.read` | `查看产物` |
| `artifact.write` | `编辑产物` |
| `artifact.commit` | `提交产物` |
| `artifact.iterate` | `迭代产物` |
| `artifact.rollback` | `回滚产物` |
| `user.mention` | `提及用户` |
| `dm.read` | `查看私信` |
| `dm.send` | `发送私信` |
| `channel.manage_members` | `管理频道成员` |
| `channel.invite` | `邀请用户` |
| `channel.change_role` | `调整成员能力` |

### §1.2 3 bundle 命名字面 (byte-identical 跟 client `BUNDLE_LABELS`)
| bundle key | 字面 |
|---|---|
| `workspace` | `工作能力` |
| `reader` | `阅读能力` |
| `mention` | `提及能力` |

## §2 反约束 — role name 字面 user-visible 0 hit (蓝图 §1.3 红线)
反向 grep 真测在 packages/client/src/components/AP2*:
**英**: `"admin"` / `"member"` / `"owner"` / `"guest"` / `"viewer"` 0 hit (user-visible 字面)
**中**: `"管理员"` / `"成员"` / `"所有者"` / `"访客"` / `"观察者"` 0 hit

## §3 反约束 — typing/loading + thinking 类禁词 (反向 grep 0 hit)
**禁词** (跟 BPP-3+CV-7+CV-8/9/11/12/13/14+DM-3/4/9/12+RT-3+HB-2 承袭): `typing` / `composing` / `loading` / `加载中` / `请稍候` / `processing` / `responding` / `thinking` / `analyzing` / `planning` 在 AP-2 UI 0 hit. AP-2 第 N+3 处.

## §4 DOM data-attr SSOT (bundle UI)
| attr | 取值 |
|---|---|
| `data-ap2-bundle-key` | bundle.* (跟 §1 表对应) |
| `data-ap2-bundle-state` | `granted` / `denied` |
| `data-ap2-cap-count` | bundle 内 capability 数字 |
## §5 跨 milestone 字面承袭锁链
- AP-4-enum #591 14-cap byte-identical (bundle ↔ cap mapping)
- 蓝图 §1.3 角色无名化红线 + ADM-0 §1.3 admin god-mode 不挂 bundle
- thinking 5-pattern 锁链 AP-2 = 第 N+3 处延伸 (跟 RT-3 ⭐ + HB-2 v0(D) 同源)

## §6 真测 grep 锚 (CI / PR 真验)
```
git grep -nE '"读取频道"|"管理消息"|"管理成员"|"管理设置"|"查看审计日志"' packages/client/   # ≥5 hit
git grep -nE 'data-ap2-(bundle-key|bundle-state|cap-count)' packages/client/   # ≥3 hit
git grep -nE '"admin"|"member"|"管理员"|"成员"' packages/client/src/components/AP2*   # 0 hit
git grep -nE 'typing|loading|processing|thinking' packages/client/src/components/AP2*   # 0 hit
```
