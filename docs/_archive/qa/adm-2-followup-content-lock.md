# ADM-2 followup content lock — admin SPA `/admin/audit-log` 页 字面 (≤40 行)

> admin SPA UI 字面 SSOT, 跟 stance §2 + ADM-2 既有 5 模板字面 byte-identical 承袭. **PM 必修 #2 G4.x #4 双截屏归档锚** (docs/qa/signoffs/g4-screenshots/g4-2-adm2-{audit-list,red-banner}.png).

## §1 admin SPA `/admin/audit-log` 页字面 (byte-identical)
| 槽位 | 字面 |
|---|---|
| 页面 title | `审计日志` |
| filter `actor_kind` 选项 | `全部` / `用户` / `Agent` / `管理员` / `混合来源` |
| empty state | `暂无审计记录` |
| red banner (impersonate active) | `当前以业主身份操作 — 该会话受 24h 时限` |
| pagination | `第 ${page} 页 / 共 ${total} 页` |

## §2 actor_kind 4-enum 字面 byte-identical 跟 ADM-3 #619 承袭
- `human` → `用户` (UI 字面)
- `agent` → `Agent` (UI 字面)
- `admin` → `管理员` (UI 字面)
- `mixed` → `混合来源` (UI 字面)
- 反向 grep `"用户/客户"|"机器人"|"系统管理员"|"复合"` 0 hit (反同义词漂)

## §3 反约束 — admin role name user-visible 0 hit (蓝图 §1.3 角色无名化承袭)
反向 grep `"admin"` / `"member"` / `"owner"` / `"管理员账号"` 等内部 role name 字面 0 hit (UI 仅显 actor_kind 4-enum 字面). 注: i18n key 内部用 admin.* 不算违规.

## §4 反约束 — typing/loading + thinking 类禁词 (反向 grep 0 hit)
**禁词** (跟 BPP-3+CV-7+CV-8/9/11/12/13/14+DM-3/4/9/12+RT-3+HB-2+AP-2+CS-2 承袭): `typing` / `composing` / `loading` / `加载中` / `请稍候` / `processing` / `responding` / `thinking` / `analyzing` / `planning` 在 ADM-2 SPA 0 hit. 第 N+5 处.

## §5 DOM data-attr SSOT
| attr | 取值 |
|---|---|
| `data-adm2-audit-list` | `true` (audit-log 页面 root) |
| `data-adm2-red-banner` | `active` (impersonate active 时) |
| `data-adm2-actor-kind` | `human` / `agent` / `admin` / `mixed` |

## §6 真测 grep 锚 (CI / PR 真验)
```
git grep -nE '"审计日志"|"暂无审计记录"|"当前以业主身份操作"' packages/client/   # ≥3 hit
git grep -nE 'data-adm2-(audit-list|red-banner|actor-kind)' packages/client/   # ≥3 hit
git grep -nE '"用户/客户"|"机器人"|"复合"|typing|loading|thinking' packages/client/src/components/Adm2*   # 0 hit
```
