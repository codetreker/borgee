# CAPABILITY-DOT stance checklist — capability snake_case → dot-notation byte-identical 跟蓝图 §3

> 7 立场 byte-identical 跟 capability-dot-spec.md (飞马待 commit). **真兑现 G4.audit 交叉核验项 2 (第三方抓的 capability spec drift)** — 14 const snake_case → dot-notation byte-identical 跟蓝图 auth-permissions.md §3 + 补全跟蓝图 cap 数量一致. 真有 prod code (rename + 字面 lint reflect 重新对齐源头) 但 0 schema / 0 endpoint shape 改. content-lock 必备 (capability 字面影响真 user-visible 错码 + DM body 模板).

## 1. 14 const snake_case → dot-notation byte-identical 跟蓝图 §3
- [ ] `read_channel` → `channel.read` / `write_channel` → `channel.write` / `delete_channel` → `channel.delete` / `read_artifact` → `artifact.read` / `write_artifact` → `artifact.edit_content` (蓝图字面) / `commit_artifact` → `artifact.commit` / 等 14 const 全 byte-identical
- [ ] **黑名单 grep 真测**: 反向 grep `"[a-z_]+"` 在 capabilities.go 0 hit (snake_case literal 真清)
- [ ] **白名单 grep**: `"[a-z]+\\.[a-z_]+"` ≥14 hit (dot-notation 真兑现)

## 2. 补全跟蓝图 cap 数量一致 (蓝图 §3 ~16 cap)
- [ ] 蓝图 §3 全列项真兑现: messaging (5: send/read/edit_own/delete_own + mention.user) + workspace (4: read/artifact.create/edit_content/modify_structure) + channel (6: create/invite_user/invite_agent/manage_members/set_topic/delete) + org (2: agent.manage / `*` admin) ≈ 17 cap
- [ ] 反 cap 漂入第 18 项 (反 5/3 偷工减料, 跟字典分立锁链承袭)

## 3. AP-4-enum #591 reflect-lint 升级守源头 (G4.audit 项 2 真因)
- [ ] reflect-lint 守 capability 字面 byte-identical **跟蓝图** (而非仅守代码 const, 第三方抓的"守错源头"真修)
- [ ] 加 `release-gate.yml` `capability-blueprint-mirror-lint` step 反向 grep 蓝图 §3 字面跟代码 const byte-identical (改一处 = 改两处)
- [ ] 字典分立锁链第 4 处真兑现 (跟字面值 byte-identical 跟蓝图)

## 4. AP-1 #493 + BPP-3.1 #495 + BPP-3.2 既有 capability 字面承袭真兑现
- [ ] AP-1 abac.go HasCapability(ctx, perm, scope) 14 const 字面真改 dot-notation byte-identical
- [ ] BPP-3.1 PermissionDeniedFrame 8 字段 `required_capability` + `current_scope` 字面跟新 dot-notation byte-identical
- [ ] BPP-3.2 system DM body 模板 `{agent_name} 想 {attempted_action} 但缺权限 {required_capability}` 真兑现 dot-notation 字面

## 5. 0 schema / 0 endpoint shape 改 (server-side rename)
- [ ] 反向 grep `migrations/capability_dot_` 0 hit + `currentSchemaVersion` 不动
- [ ] 0 endpoint shape 改 (capability 字面在 BPP frame body + error code 内, 不改 endpoint path / method)
- [ ] 既有 unit 全 PASS byte-identical (反 race-flake)

## 6. user-visible 错码字面 byte-identical (反 user 体验漂)
- [ ] DM body 模板 `{required_capability}` 渲染 dot-notation 字面 (跟蓝图 §3 byte-identical)
- [ ] 错码 + permission_denied frame 字面对锁跨 spec/code/REG/e2e/content-lock 五处
- [ ] 反向 grep snake_case 字面在 user-visible UI / DM body 0 hit (字面真清)

## 7. admin god-mode 不挂 capability rename (ADM-0 §1.3 红线)
- [ ] 反向 grep `admin.*capability|admin.*HasCapability` 在 packages/server-go/ 0 hit
- [ ] capability `*` (admin only) 字面 byte-identical 跟蓝图 §3 不动 (admin scope 唯一)

## 反约束 — 真不在范围
- ❌ 加新 capability 第 18 项 (反 蓝图 §3 ~17 cap byte-identical 锁)
- ❌ 加 schema / endpoint shape / 加新 CI step (反 0 行为改)
- ❌ admin god-mode 加挂 capability override (永久不挂)
- ❌ 改 capability 决策逻辑 (仅 rename 字面 byte-identical 跟蓝图)

## 跨 milestone byte-identical 锁链 (5 链)
- AP-1 #493 abac.go HasCapability + AP-4-enum #591 reflect-lint (升级守源头)
- BPP-3.1 #495 PermissionDeniedFrame + BPP-3.2 system DM body 模板字面承袭
- 蓝图 auth-permissions.md §3 字面单源 (`<domain>.<verb>` 风格 byte-identical)
- 字典分立锁链第 4 处真兑现 (字面值跟蓝图 byte-identical, 反"假锁链")
- anchor #360 owner-only ACL 22+ PRs + ADM-0 §1.3 红线

## PM 拆死决策 (3 段)
- **dot-notation vs snake_case 拆死** — 蓝图 §3 字面 byte-identical (本 PR), 反 spec drift 真因
- **守源头 vs 守代码拆死** — reflect-lint 升级守蓝图字面 (本 PR), 反"守代码 const 而非蓝图" (第三方抓的真因)
- **scope 全清 vs top-N 拆死** — 14→17 cap 全清一次合 (用户铁律, 反 REFACTOR-1 留尾教训)

## 用户主权红线 (5 项)
- ✅ 蓝图 §3 字面是 capability 真值锚源头 (用户主权红线)
- ✅ 既有 ACL gate 行为 byte-identical 不破 (仅字面 rename)
- ✅ user-visible 错码 / DM body 字面跟蓝图 byte-identical
- ✅ 0 schema / 0 endpoint shape 改
- ✅ admin god-mode `*` cap 蓝图字面 byte-identical 不动

## PR 出来 5 核对疑点
1. 反向 grep snake_case literal 在 capabilities.go 0 hit + dot-notation literal ≥14 hit
2. cap 数量补全 ~17 跟蓝图 §3 byte-identical (count 反向断言)
3. AP-1 + BPP-3.1 + BPP-3.2 既有路径 byte-identical 真改 dot-notation
4. reflect-lint 升级守蓝图字面 (capability-blueprint-mirror-lint CI step 真启)
5. cov ≥85% (#613 gate) + admin grep 0 hit
