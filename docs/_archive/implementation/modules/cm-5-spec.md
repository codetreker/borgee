# CM-5 Spec — agent↔agent 独立协作 + X2 冲突裁决

> 蓝图: `concept-model.md §1.3` (§185 "未来你会看到 agent 互相协作" — agent↔agent 在 CM-5/Phase 4, 不能让用户感觉 agent 是单兵木偶) + §1.4 (人和 agent 协作语义平等, 默认权力不对称)
> 蓝图: `agent-lifecycle.md §1` (Borgee 是协作平台, 不是 agent 平台 — agent 之间协作走 Borgee 平台机制 + plugin runtime)
> 依赖: CM-4 ✅ (#220+#222+#243+...) agent_invitations 邀请 + AP-3 (Phase 4, 暂不阻塞 CM-5 schema 段)
> Owner: 战马A 实施 / 烈马 验收 / 野马 立场 / 飞马 spec review
> 状态: 🟡 v0 DRAFT

---

## 0. 立场 (5 条 — 借 CM-1 / CM-4 / AP-3 同模式)

> ⚠️ 立场 byte-identical 锁字面源, 改一处必同步 acceptance + content-lock + grep 反约束.

1. **立场 ① — agent↔agent 同 channel 协作走人协作语义同 path**: agent A → agent B 的 mention / message / artifact commit 走跟人完全相同的 path (DM-2 mention router + CV-1 artifact + AP-0/AP-2 permission), **不裂** "agent_only_message" / "ai_to_ai_channel" 旁路. 反约束: schema 不裂 `agent_messages` 表, server 不开 `POST /api/v1/agents/:id/notify-agent` 旁路 endpoint.

2. **立场 ② — 责任归属不变 (owner-first)**: agent A commit artifact 到 channel C, 即便另一 agent B 提了 iterate request, **commit 责任仍归 agent A.owner** (跟 CV-1 立场 ② commit 单源同根). 反约束: `artifact_versions.committed_by` 永远是 user 行 (agent 也是 user.role='agent', 走 user.id), 不裂 `triggered_by_agent_id` 列.

3. **立场 ③ — X2 冲突裁决 last-writer-wins + 显式提示**: 同一 artifact 被 2+ agent 同时 commit (`?iteration_id=` query 落地间隔 < 200ms), server 走 CV-1 既有锁机制 (single-doc lock 30s) — **第二写者收 409 conflict** (`code: 'artifact.locked_by_another_iteration'`), client SPA UI 显示 "正在被 agent {ownerName} 处理" 字面 + retry 入口. 反约束: 不引入新 schema (artifact_locks / iteration_priority 表), 复用 CV-1.2 既有 single-doc lock + CV-4.1 iterations state 机制.

4. **立场 ④ — agent A → agent B mention 走 DM-2 router 不旁路**: agent A 在 channel 里 @agent B, 走 DM-2.2 mention dispatch (#372 既有路径) — MentionPushedFrame 8 字段 byte-identical, B's owner 收 system DM (跟人同模式). 反约束: agent.role='agent' 不影响 mention router 路径分流, 不开 `agent_to_agent_mention` 专属 frame.

5. **立场 ⑤ — 协作可见性 owner-first**: agent A → agent B 协作产物 (artifact iterate 链, anchor reply 链) 对**两 owner 都可见** (跟人协作产物 owner 可见同模式). 反约束: 不裂 owner_visibility scope, 不引入 "ai_only" 隐藏字段 — 透明协作是产品立场字面 (蓝图 §185).

---

## 1. 三段拆 (CM-5.1 / CM-5.2 / CM-5.3)

### CM-5.1 — schema 反约束锁 + 反向 grep 黑名单

**Schema 改动: 无新表** (立场 ① — agent↔agent 走人协作 path, 不裂表). 仅落 反约束 grep 锚到 `internal/migrations/registry.go` 注释 + 测试. 跟 CHN-2 #353 acceptance §0 立场 ⑤ 同模式 (DM 反约束不开新表, 走既有 channels.type='dm').

实施物:
- 反向 grep 黑名单测试 `cm_5_1_anti_constraints_test.go`:
  - `grep -nE 'agent_messages\b' migrations/` count==0 (反 立场 ① 旁路表)
  - `grep -nE 'ai_to_ai_channel|agent_only_message|agent_to_agent_mention' internal/` count==0
  - `grep -nE 'artifact_locks|iteration_priority' migrations/` count==0 (反 立场 ③ 新锁表)
  - `grep -nE 'triggered_by_agent_id' migrations/cv_1_1_artifacts.go cv_4_1_artifact_iterations.go` count==0 (反 立场 ② 责任旁路)
- 文档锚: `docs/current/server/data-model.md` "agent↔agent 协作 schema 反约束" 段 (跟 chn-2 同模式)

### CM-5.2 — server 协作场景验证 + agent A → B mention 路径

实施物:
- `internal/api/cm_5_2_agent_to_agent_test.go`: 端到端验证 server 路径
  - `TestCM52_AgentMentionsAgent` — agent A 发 message @agent B, MentionPushedFrame 推 agent B + system DM 到 B's owner (跟 DM-2.2 #372 同模式)
  - `TestCM52_AgentCommitsAfterAgent409` — agent A `commit?iteration_id=X` + agent B 同 artifact `commit?iteration_id=Y` < 200ms → B 收 409 `artifact.locked_by_another_iteration` (CV-1.2 single-doc lock 复用)
  - `TestCM52_AgentIterateChainOwnerVisible` — agent A iterate → agent B iterate 同 artifact, owner_A + owner_B 都能 GET /artifacts/:id/iterations 列出全链 (立场 ⑤)
- 不动 server 实施代码 (立场 ① 复用既有 path), 仅加 test + 字面文档锚

### CM-5.3 — client UI 协作可见性 + 文案锁

实施物:
- `client/src/components/AgentManager.tsx` (修): hover agent 时显示 "正在协作: {agentName}" 链路 (立场 ⑤ 透明协作)
- 文案锁 byte-identical: "正在被 agent {name} 处理" (CV-4 iterate conflict toast 同源 #380 ⑦)
- vitest content-lock test (跟 chn-3-content-lock #402 同模式)
- e2e: 双 agent commit 同 artifact 触发 409 截屏

---

## 2. 反约束 grep 黑名单 (跟 CHN-3 #366 / CHN-2 #357 同模式)

每 CM-5.* PR 必跑 (CI grep 锚):
- `grep -rnE 'agent_messages\b|ai_to_ai_channel|agent_only_message' internal/ packages/client/src/` count==0
- `grep -nE 'POST /api/v1/agents/.*/notify-agent' internal/api/` count==0 (立场 ① 旁路 endpoint)
- `grep -nE 'triggered_by_agent_id|committed_by_agent' migrations/ internal/store/` count==0 (立场 ② 责任旁路)
- `grep -nE 'artifact_locks\s+TABLE|iteration_priority\s+TABLE' migrations/` count==0 (立场 ③ 新锁表)

---

## 3. 边界 (跟其他 milestone 关系)

| Milestone | 关系 | 字面承袭 |
|---|---|---|
| CM-4 ✅ | agent_invitations 邀请机制就位, CM-5 完全不动 | agent_invitations 表 byte-identical 不破 |
| CV-1 ✅ | single-doc lock 30s 复用 (立场 ③ X2 冲突走既有锁) | artifacts.locked_by + 409 toast 字面 |
| CV-4 ✅ | iterate state machine 4 态复用; agent commit 走 ?iteration_id= 同源 | 409 `artifact.locked_by_another_iteration` 跟 #380 ⑦ 同字面 |
| DM-2 ✅ | mention dispatch 路径复用 (立场 ④ — agent mention agent 走人 path) | MentionPushedFrame 8 字段 byte-identical |
| AP-3 (Phase 4) | agent acting-as-user 权限模型 (CM-5.2 server 验 owner_A acting-as agent_A) | AP-3 落地后 CM-5.2 test 对接, 暂用 CM-1 既有 user.id ACL |
| RT-3 ⭐ (Phase 4) | 多端全推 + 活物感 — agent↔agent 协作的 frame 推送给两 owner | 复用 RT-1 fanout, 不开新 frame |

---

## 4. 退出条件

- §1.1 + §1.2 + §1.3 全绿 (一票否决)
- §2 反约束 grep 7 条每 PR 必跑 0 命中
- 登记 `docs/qa/regression-registry.md` REG-CM5-001..005
- acceptance template `cm-5.md` ✅ (本 spec PR 同期落)
