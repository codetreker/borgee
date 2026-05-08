# Phase 1 — 身份闭环 (detail)

> 引自 [PROGRESS.md](../PROGRESS.md) 概览表 Phase 1 行 — milestone 翻牌点单源在此.

## Phase 1 — 身份闭环

**Milestones**

- [x] **CM-1** organizations 表落地 — 战马 / 飞马 / 野马 / 烈马
  - [x] PR CM-1.1 schema (organizations 表 + users.org_id 列 + 索引) (PR #176)
  - [x] PR CM-1.2 注册自动建 org (PR #178)
  - [x] PR CM-1.3 admin stats GROUP BY org_id (PR #179)
  - [x] PR CM-1.4 admin 调试页 (visibility checkpoint, 非 acceptance) (PR #180)
- [x] **AP-0** 默认权限注册回填 (与 CM-1 并行) — 战马 / 飞马 / 野马 / 烈马
  - [x] PR AP-0.1 注册时写默认权限 (human=`*`, agent=`message.send`) (PR #177)
- [x] **CM-3** 资源归属 org_id 直查 (CM-4 之后) — 战马 / 飞马 / 野马 / 烈马 (PR #208)
  - [x] PR CM-3.1 写路径 (4 张表 stamp org_id at INSERT) (PR #208)
  - [x] PR CM-3.2 读路径 (跨 org 403 + JOIN owner_id 全删) (PR #208)

**Gates**

- [x] G1.1 数据层 org_id 落地 — 战马/烈马 / 证据: 烈马本地 fresh DB SQL 直查 — schema_migrations v2 cm_1_1_organizations / organizations DDL / users·channels·messages·workspace_files·remote_nodes 5 表 org_id TEXT NOT NULL DEFAULT '' / 5 索引 idx_*_org_id 全部确认
- [x] G1.2 注册自动建 org E2E — 战马/烈马 / 证据: 真实 POST /api/v1/auth/register 路径, users.org_id 非空 + organizations 行 name="<DisplayName>'s org" (烈马本地 in-mem sqlite acceptance run, 2026-04-28)
- [x] G1.3 agent 继承 owner org — 战马/烈马 / 证据: member 创 agent → agent.OrgID == owner.OrgID; admin 创 human → 自动建 org (同上 run)
- [x] G1.4 读路径直查 (SQL EXPLAIN + grep 黑名单) — 飞马/烈马 / 证据: closed by #208 (CM-3 写/读路径 stamp + 跨 org 403) + #210 (g1-audit.md §2.1 黑名单 grep count==0 / §2.2 三类 403 PASS / §2.3 EXPLAIN 走 idx_*_org_id)
- [x] G1.5 UI 不泄漏 org_id (合约测试) — 烈马/野马 / 证据: 用户面 6 端点 + 2 响应体 leak-scan 全部不含 `org_id` (GET /api/v1/users/me, /admin-api/v1/auth/me, /admin-api/v1/users, /api/v1/agents; POST /api/v1/auth/register, /api/v1/agents 响应); /admin-api/v1/stats by_org 是 admin-only 故意暴露, 白名单
- [x] **G1.audit** v0 代码债 audit 行已登记 (organizations 删库 / users.org_id 加列) — 飞马 (PR #182)


## 更新日志归档 (历史 changelog 迁入)

| 日期 | 更新人 | 变化 |
|------|--------|------|
| 2026-04-28 | 烈马 | Phase 1 收口: CM-1 (PR #176/#178/#179/#180) + AP-0 (#177) + G1.audit (#182) 全 merged; Gates G1.1/G1.2/G1.3/G1.5 ✅ (烈马本地 fresh DB SQL 直查 + 真实 HTTP register/agent E2E + 6 端点 leak-scan); G1.4 ⏸ 待 CM-3 写路径完成后补; Phase 概览改 🔄 4/5 + audit ✅ |

---
