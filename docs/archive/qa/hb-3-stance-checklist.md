# HB-3 立场反查清单 (战马A v0)

> 战马A · 2026-04-29 · 立场 review checklist (跟 HB-1 #491 + HB-2 #491 + BPP-4 #499 stance 同模式)
> **目的**: HB-3 三段实施 (HB-3.1 schema+REST / 3.2 daemon 读路径合约 / 3.3 client SPA + e2e + closure) PR review 时, 飞马/野马/烈马按此清单逐立场 sign-off.
> **关联**: spec `docs/implementation/modules/hb-3-spec.md` (战马A v0 9b97809) + acceptance `docs/qa/acceptance-templates/hb-3.md` (战马A v0) + content-lock `docs/qa/hb-3-content-lock.md` (战马A v0, 弹窗 UX 字面)

## §0 立场总表 (3 立场 + 4 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | host_grants schema **单源** — HB-3 持 ownership; HB-2 daemon + install-butler **read-only consumer** | host-bridge.md §1.3 + HB-2 spec §3.2 read-only consumer 已锁 | 反向 grep `host_grants.*INSERT\|host_grants.*UPDATE` 在 `packages/host-bridge/` (Rust crate, 待 HB-2 真实施) count==0; server-go `internal/api/host_grants.go` 唯一写路径 |
| ② | host_grants 字段 **跟 AP-1 user_permissions 概念分立** — host vs runtime 字典分立, 不复用 schema | concept-model.md §1.4 字段划界 + HB-1/HB-2 reason 字典分立同模式 | 反向 grep `host_grants.*JOIN.*user_permissions\|grants.*INSERT.*user_permissions` count==0; schema CHECK 守 grant_type ∈ 4-enum (install/exec/filesystem/network) |
| ③ | audit log **5 字段 byte-identical 跟 HB-1 + HB-2 + BPP-4 dead-letter 同源** (`actor/action/target/when/scope`) | host-bridge.md §2 信任五支柱第 3 条 + HB-4 §1.5 release gate 第 4 行 "审计日志格式锁定 JSON schema" | reflect 锁字段名 + JSON tag (跟 BPP-4 #499 dead_letter_test::TestBPP4_DeadLetter_AuditSchema5FieldsByteIdentical 同模式); 改 = 改**四处单测锁链** (HB-1 install + HB-2 host-IPC + BPP-4 dead-letter + HB-3 grant audit) |
| ④ (边界) | 4 grant_type CHECK enum 字面 byte-identical 跟蓝图 §1.3 弹窗 UX | host-bridge.md §1.3 字面 (install / exec / filesystem / network) | DB CHECK constraint + Go enum const 字面同源; 改 = 改三处 (migration + handler + content-lock §1.①) |
| ⑤ (边界) | 2 ttl_kind CHECK enum 字面 byte-identical 跟蓝图 §1.3 UX 选项 | host-bridge.md §1.3 弹窗 "[仅这一次][始终允许]" 字面 | DB CHECK constraint + content-lock §1.② 字面 (one_shot / always); 弹窗按钮 data-action 跟 enum 字面映射 byte-identical |
| ⑥ (边界) | 撤销 grant **< 100ms** daemon 立即拒绝 (HB-4 §1.5 release gate 第 5 行) | host-bridge.md §1.5 release gate | server REST DELETE → revoked_at NOT NULL; daemon 每次 IPC 重查 (反向 grep `cachedGrants\|grantsCache` 0 hit) — 跟 HB-1 manifest 不缓存 + HB-2 §4.3 同模式 |
| ⑦ (边界) | admin god-mode **不入** grant 路径 — 用户授权是用户主权, admin 不撤销用户 grant | admin-model.md ADM-0 §1.3 红线 + 蓝图 §1.3 字面承袭 | 反向 grep `admin.*host_grant\|admin.*HostGrant` 在 `internal/api/admin*.go` count==0 |
| ⑧ (边界) | best-effort 立场承袭 BPP-4 #499 + BPP-5 #501 — 不挂 grant retry queue / persistent state | BPP-4 §0.3 字面承袭 + #501 锁链延伸 | AST scan `pendingGrants\|grantQueue\|deadLetterGrants` 在 `internal/api/host_grants*.go` 非 _test.go 源 count==0 (跟 BPP-4 dead_letter_test + BPP-5 reconnect_handler_test 锁链延伸第 3 处) |

## §1 立场 ① schema SSOT (HB-3.1 守)

**蓝图字面源**: `host-bridge.md` §1.3 (4 类授权) + HB-2 spec §3.2 read-only consumer 合约.

**反约束清单**:

- [ ] migration `hb_3_1_host_grants.go` v=26 是 host_grants 表唯一定义 (CREATE TABLE 单源)
- [ ] server-go `internal/api/host_grants.go` 是唯一 INSERT/UPDATE/DELETE 路径
- [ ] HB-2 daemon (Rust crate `packages/host-bridge/`) 反向 grep `host_grants.*INSERT\|host_grants.*UPDATE` 0 hit (待 HB-2 真实施时强制)
- [ ] daemon SELECT 走 HB-2 spec §3.2 既有合约, 不另起 ORM mapper

## §2 立场 ② 字典分立 (HB-3.1 守)

**蓝图字面源**: `concept-model.md` §1.4 字段划界 + HB-1/HB-2 reason 字典分立同模式 (install vs host-IPC vs runtime 三层独立).

**反约束清单**:

- [ ] host_grants 表无 FK / JOIN 到 user_permissions
- [ ] 反向 grep `host_grants.*JOIN.*user_permissions\|grants.*INSERT.*user_permissions` count==0
- [ ] grant_type CHECK enum 4 项 (install/exec/filesystem/network) 跟 user_permissions.permission (channel.read / message.send 等) 字面集不交
- [ ] handler 路径不导入 user_permissions (反向 grep `host_grants.go.*user_permissions` count==0)

## §3 立场 ③ audit log 5 字段同源 (HB-3.1 守)

**蓝图字面源**: `host-bridge.md` §2 信任五支柱 + HB-4 §1.5 release gate 第 4 行 "审计日志格式锁定 JSON schema (含 actor / action / target / when / scope)".

**反约束清单**:

- [ ] HB-3 audit struct 5 字段 byte-identical 跟 BPP-4 #499 DeadLetterAuditEntry struct (改 = 改四处单测锁链承袭)
- [ ] reflect 锁: field name + JSON tag 顺序 byte-identical (跟 BPP-4 dead_letter_test::TestBPP4_DeadLetter_AuditSchema5FieldsByteIdentical 同模式)
- [ ] grant 操作 (insert/revoke) 真写 audit log; 反向断言 audit row count == grant op count
- [ ] log key 字面: `host_grants.granted` + `host_grants.revoked` (跟 BPP-4 `bpp.frame_dropped_plugin_offline` + HB-1/HB-2 audit log key 同 prefix 模式)

## §4 蓝图边界 ④⑤⑥⑦⑧ — 跟 4 enum / 2 ttl / 撤销 < 100ms / admin red line / best-effort 不漂

**反约束清单**:

- [ ] grant_type 4-enum 字面跟蓝图 §1.3 byte-identical (install / exec / filesystem / network); 改 = 改三处 (migration CHECK + Go const + content-lock §1.①)
- [ ] ttl_kind 2-enum 字面 (one_shot / always); 跟弹窗按钮 data-action 字面映射
- [ ] 撤销 grant → daemon read 反断 < 100ms (HB-4 release gate 第 5 行真测; v1 是 server 端 DELETE + daemon 每次 IPC 重查实现)
- [ ] admin god-mode 0 hit (`internal/api/admin*.go` 反向 grep)
- [ ] AST scan `pendingGrants\|grantQueue\|deadLetterGrants` 0 hit (BPP-4 + BPP-5 best-effort 锁链延伸第 3 处)

## §5 退出条件

- §1+§2+§3 (4+4+4) + §4 (5) 全 ✅
- 反向 grep / AST scan 8 项全 0 hit (daemon 写 + JOIN user_permissions + admin god-mode + best-effort + grants 不缓存)
- audit schema 跨四 milestone (HB-1 / HB-2 / BPP-4 / HB-3) byte-identical 不漂 (改 = 改四处单测锁)
- 蓝图 §1.3 4-enum + 2 ttl 字面 byte-identical (跟 content-lock §1.①+§1.② + spec §1+§3 三处单测锁)
