# AP-2 stance checklist — UI bundle (无角色名 capability 单源, 蓝图 §1.3)

> 7 立场 byte-identical 跟 ap-2-spec.md §0+§2 (飞马 v0 待 commit). **真有 prod code (capability bundle UI 组件 + bundle ↔ capability 映射 SSOT) + client UI 字面 (content-lock 真锁) 但 0 schema 改 / 0 endpoint 行为改 / 复用 AP-4-enum #591 14-cap**. 跟 AP-4-enum #591 capability enum + AP-1 #493 / AP-3 #521 既有 ACL + DL-1 #609 4 interface 同模式承袭. content-lock 必备 (UI bundle 命名 + 蓝图 §1.3 角色无名化字面).

## 1. 角色无名化 (蓝图 §1.3 byte-identical)
- [ ] **0 hardcoded role name** — 反向 grep `"admin"|"member"|"owner"|"guest"|"viewer"` 在 packages/client/ + UI 路径 user-visible 0 hit (反角色名漂入 UI 字面)
- [ ] capability 通过 bundle 命名暴露 (反 role.name 暴露给用户)
- [ ] 用户视角仅看 bundle 名 (例: `读取频道` / `管理消息` 待 content-lock 真定字面), 不看 admin/member 角色字面

## 2. capability 复用 AP-4-enum #591 14-cap (反 cap 漂入)
- [ ] AP-4-enum 14-cap byte-identical 不破 (count==14 反向 grep 锚守)
- [ ] bundle ↔ capability 映射 SSOT (反多处散布)
- [ ] 反向 grep 反 capability 第 15 项漂入 (跟 AL-1a 6-dict + HB-1 7-dict + HB-2 8-dict + AP-4-enum 14-cap + DL-2 retention 3-enum + DL-3 阈值 3-const 字典分立第 7 处承袭)
- [ ] reflect-lint 单源 (跟 AP-4-enum #591 反向 grep CI 守门承袭)

## 3. bundle 单源 SSOT (反 magic mapping 散布)
- [ ] **bundle const 单源** — 反向 grep `bundles_v2|capabilityGroupAlt|roleBundle` 0 hit
- [ ] bundle ↔ capability 映射在 server-go 单 helper SSOT (反 client + server 双源 drift)
- [ ] bundle 命名锁 (待 content-lock 真定) byte-identical, 反 `RoleGroup` / `CapBundle` / `PermissionPack` 同义词漂

## 4. 0 schema 改 (复用 AP-1 + AP-3 既有)
- [ ] 反向 grep `migrations/ap_2_` 在 packages/server-go/ 0 hit
- [ ] `currentSchemaVersion` 不动 (反向断 0 行改)
- [ ] 复用 AP-1 #493 + AP-3 #521 既有 ACL helper + AP-4-enum #591 capability enum
- [ ] 反 ALTER 既有 schema (反加 bundle 字段漂入 admin_actions)

## 5. 0 endpoint 行为改 (UI 层组合)
- [ ] 0 endpoint shape 改 — `git diff origin/main -- internal/api/server.go | grep -E '\\+.*Method|\\+.*Register'` 0 hit
- [ ] 0 response body / 0 error code 字面改 — 既有错码 byte-identical
- [ ] AP-2 仅 UI 层 capability bundle 组件, 反"加 /api/v1/bundles endpoint" 漂 (留 v2)
- [ ] 既有 unit + e2e 全 PASS byte-identical (反 race-flake)

## 6. agent ↔ human 同源 (PR #568 §4 端点延伸)
- [ ] bundle 不分 sender 类型 — agent 跟 human 同 bundle 视角 byte-identical
- [ ] 反向 grep `agent_bundle|human_bundle|sender_specific_bundle` 0 hit
- [ ] 跟 DL-1 #609 4 interface 同精神 (data layer 不分 sender 类型立场延伸)

## 7. admin god-mode 不挂 bundle (ADM-0 §1.3 红线)
- [ ] 反向 grep `admin.*bundle|admin.*capability_bundle` 在 packages/server-go/ 0 hit
- [ ] 反向 grep `/admin-api.*bundle` 0 hit
- [ ] bundle 走 user-rail (反 admin override, anchor #360 owner-only ACL 锁链 22+ PRs 立场延伸)

## 反约束 — 真不在范围
- ❌ 加 /api/v1/bundles endpoint (留 v2 server 真启)
- ❌ 加 schema bundle 字段 (留 v2 持久化)
- ❌ 引入新 capability 第 15 项 (反 14-cap byte-identical 锁)
- ❌ role name 字面暴露 user UI (蓝图 §1.3 红线)
- ❌ 加新 CI step (跟 AP-4-enum + DL-1/2/3 + REFACTOR-1/2 + INFRA-3 同精神)
- ❌ admin god-mode 加挂 bundle (永久不挂, ADM-0 §1.3 红线)

## 跨 milestone byte-identical 锁链 (5 链)
- **AP-4-enum #591 14-cap** — bundle ↔ capability 映射真兑现, 反 cap 漂入第 15 项 (字典分立第 7 处承袭)
- **AP-1 #493 + AP-3 #521 既有 ACL helper 复用** — 不另起授权路径 (跟 DM-9/10/11/12 + DL-1 #609 同模式)
- **AL-1a 6-dict + HB-1 7-dict + HB-2 8-dict + AP-4-enum 14-cap + DL-2 3-enum + DL-3 3-const + AP-2 bundle 单源** — 字典分立锁链第 7 处
- **DL-1 #609 4 interface** — bundle helper 走 server-side internal/api/, 跟 handler baseline N=108 + factory 单源承袭
- **anchor #360 owner-only ACL 锁链 22+ PRs** + REG-INV-002 fail-closed + ADM-0 §1.3 红线

## PM 拆死决策 (3 段)
- **bundle 命名 vs role name 暴露拆死** — bundle 命名 (本 PR 选, 蓝图 §1.3 红线), 反 role.name 字面暴露 UI
- **bundle 单源 SSOT vs magic mapping 散布拆死** — server-go 单 helper (本 PR), 反 client + server 双源 drift
- **复用 14-cap vs 引入 15+ cap 拆死** — AP-4-enum byte-identical (本 PR), 反第 15 cap 漂入 (字典分立锁链)

## 用户主权红线 (5 项)
- ✅ 角色无名化 (用户仅看 bundle 不看 admin/member, 蓝图 §1.3 红线)
- ✅ 既有 ACL gate 字面 + 行为 byte-identical (anchor #360 + REG-INV-002 守)
- ✅ 0 schema / 0 endpoint shape / 0 既有 ACL 改
- ✅ agent ↔ human 同源 (反 sender_specific_bundle 漂)
- ✅ admin god-mode 不挂 bundle (ADM-0 §1.3 红线)

## PR 出来 5 核对疑点
1. role name 反向 grep `"admin"|"member"|"owner"|"guest"|"viewer"` 在 user-visible UI 字面 0 hit
2. AP-4-enum 14-cap byte-identical (count==14 反向 grep 锚) + bundle ↔ cap mapping 单源
3. 0 schema / 0 endpoint shape 改 (`git diff` 反向断言)
4. bundle 单源 SSOT (反向 grep `bundles_v2|capabilityGroupAlt|roleBundle` 0 hit)
5. cov ≥85% (#613 gate) + admin god-mode grep 0 hit
