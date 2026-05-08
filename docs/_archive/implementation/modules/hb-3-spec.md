# HB-3 spec brief — host_grants schema SSOT + 情境化授权 (≤80 行)

> 战马A · Phase 5 host-bridge · ≤80 行 · 蓝图 [`host-bridge.md`](../../blueprint/current/host-bridge.md) §1.3 (情境化授权 4 类) + §1.5 release gate 第 5 行 (撤销 grant → daemon < 100ms 拒绝). 模块锚 [`host-bridge.md`](host-bridge.md) §HB-3. 依赖 HB-1 #491 install-butler spec + HB-2 #491 host-bridge daemon spec (HB-2 §3.2 grants store contract 已锁: read-only consumer + HB-3 持 schema). Blocked-by: HB-1 / HB-2 真实施 (Go binary skeleton 等 DL-4 manifest endpoint; HB stack Go 重审拍板, 撤 Rust crate 路径).

## 0. 关键约束 (3 条立场, 蓝图 §1.3 字面)

1. **host_grants schema 单源 (HB-3 ownership)** — HB-2 host-bridge daemon 跟 install-butler **read-only consumer** (HB-2 spec §3.2 已锁). HB-3 持 schema migration + REST CRUD endpoints (server 端管理 + client SPA 弹窗写); daemon 仅 SELECT, 不 INSERT/UPDATE. **反约束**: 反向 grep `host_grants.*INSERT|host_grants.*UPDATE` 在 daemon 路径 (Go binary `packages/borgee-helper/`) 0 hit; 仅 server-go `internal/api/host_grants.go` 写.

2. **grant 字段跟 AP-1 user_permissions 概念分立** — host vs runtime 两层独立, **不复用 user_permissions schema**. user_permissions 是 platform-level (channel/message/admin perms); host_grants 是 OS-level (filesystem path / network domain / install / exec). 反约束: 反向 grep `host_grants.*JOIN.*user_permissions|grants.*INSERT.*user_permissions` 0 hit (字典分立, 跟 AL-1a vs HB-1 reason 分立同模式).

3. **audit log 5 字段 byte-identical 跟 HB-1 + HB-2 + BPP-4 dead-letter 同源** — `actor / action / target / when / scope` 五字段是跨 milestone audit schema (HB-4 §1.5 release gate 第 4 行 "审计日志格式锁定 JSON schema" 守门同源). HB-3 grant/revoke 走同 schema. **改 = 改四处**: HB-1 install audit + HB-2 host-IPC audit + BPP-4 dead-letter + HB-3 grant audit (跟 BPP-5 #501 锁链同模式延伸).

## 1. 拆段 (一 milestone 一 PR, 整段一次合 — 跟 BPP-2/3/4/5 协议同源)

| 段 | 文件 | 范围 |
|---|---|---|
| HB-3.1 schema + REST | `internal/migrations/hb_3_1_host_grants.go` (新, v=26) + 表 host_grants (id PK / user_id NOT NULL FK 逻辑 / agent_id NULL FK 逻辑 / grant_type CHECK enum / scope TEXT JSON / ttl_kind CHECK enum / granted_at NOT NULL / expires_at NULL / revoked_at NULL) + idx_user_id + idx_agent_id; `internal/api/host_grants.go` (新) GET/POST/DELETE `/api/v1/host-grants` (owner-only ACL 跟 anchor #360 同模式); 7 unit (schema 5 列反断 / grant_type 4 enum / ttl_kind 2 enum 'one_shot'/'always' / cross-user 403 reject / 撤销 < 100ms 真测 / audit log 5 字段 byte-identical / 反向断言 不复用 user_permissions schema) |
| HB-3.2 daemon 读路径合约 | `docs/implementation/modules/hb-2-spec.md` §3.2 cross-ref 锁 — daemon 读路径既有合约 byte-identical 跟 HB-3.1 schema (无 server-go 代码改, daemon 是 Go binary 真实施时跟 HB-2 同 PR (Go interface `GrantsConsumer` 替代 Rust trait); 本 milestone 只锁 contract); 跨 PR drift 守: server schema 改字段 = HB-2 daemon SELECT 改, 跟 DL-4 ↔ HB-1 drift anchor 8a35589 同模式 |
| HB-3.3 client SPA + e2e + closure | `packages/client/src/permissions/HostGrantsPanel.tsx` (新) 弹窗 UX 字面跟蓝图 §1.3 byte-identical (`[✗ 拒绝]    [✓ 仅这一次]    [✓ 始终允许]`); e2e: 弹窗触发 → 选 `仅这一次` → grants insert ttl_kind='one_shot' + expires_at=now+1h; 选 `始终允许` → ttl_kind='always' + expires_at NULL; 撤销 → revoked_at NOT NULL + daemon read 反断 < 100ms; REG-HB3-001..009 + acceptance + PROGRESS [x] |

## 2. 留账边界

- **cross-host federation** (留 v2) — 单 user 多 host 同步 grant, v1 单 host 1:1
- **grant 层级 inheritance** (留 v2) — `~/code` grant 是否覆盖 `~/code/sub`, v1 严格 path equality
- **multi-user host** (留 v2) — 单 host 多 user (e.g. shared Linux server), v1 单 user
- **2 grant_type install/exec 装机时弹窗 UX** (留 HB-1 install-butler 实施时落地) — HB-3.1 schema 已含 4 enum, 但 install/exec 类 grant insert 路径在 HB-1.5 install flow 真接, 本 PR 仅锁 schema

## 3. 反查 grep 锚 (Phase 5 验收 + HB-3 实施 PR 必跑)

```
git grep -nE 'host_grants\b' packages/server-go/internal/   # ≥ 1 hit (schema + handler 字面)
git grep -nE 'HostGrantType.*install|filesystem|network|exec' packages/server-go/internal/api/host_grants.go   # ≥ 1 hit (4 enum 字面)
# 反约束 (5 条 0 hit)
git grep -nE 'host_grants.*JOIN.*user_permissions|grants.*INSERT.*user_permissions' packages/server-go/   # 0 hit (字典分立)
git grep -nE 'host_grants.*INSERT|host_grants.*UPDATE' packages/host-bridge/   # 0 hit (daemon read-only, 待 HB-2 真实施)
git grep -nE 'admin.*host_grant|admin.*HostGrant' packages/server-go/internal/api/admin*.go   # 0 hit (admin 不撤销用户 grant, 用户唯一)
git grep -nE 'pendingGrants|grantQueue|deadLetterGrants' packages/server-go/internal/   # 0 hit (跟 BPP-4/BPP-5 best-effort 立场 AST scan 锁链延伸)
git grep -nE 'host_grants.*cache|cachedGrants' packages/server-go/internal/ packages/host-bridge/   # 0 hit (跟 HB-1 manifest 不缓存 + HB-2 §4.3 同模式)
```

## 4. 不在本轮范围 (反约束 deferred)

- ❌ cross-host federation / grant 层级 inheritance / multi-user host (跟 §2 留账同源, 留 v2)
- ❌ install/exec grant insert 路径 (留 HB-1.5 install flow 真接, 本 PR 仅锁 schema)
- ❌ filesystem grant 跟 IDE 集成 (`code .` 这种命令读权限) — v1 不涉, v2+ 看 IDE 协议演进
- ❌ network grant URL 通配符 (e.g. `*.example.com`) — v1 严格 domain equality, v2+ 加 wildcard
- ❌ admin god-mode 走 grant 路径 — admin 不撤销用户 grant (蓝图 §1.3 + ADM-0 §1.3 红线 字面承袭, "用户授权" 是用户主权)
- ❌ AP-1 user_permissions schema 复用 (字典分立反约束 — host vs runtime 两层独立)
