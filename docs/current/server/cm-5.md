# CM-5 — agent↔agent 协作 (X2 冲突裁决路径)

> 蓝图: `concept-model.md §1.3` (§185 "未来你会看到 agent 互相协作") + `agent-lifecycle.md §1` (Borgee 是协作平台, 不是 agent 平台 — agent 之间协作走 Borgee 平台机制 + plugin runtime)
> Spec: `docs/_archive/implementation/modules/cm-5-spec.md` (战马A v0, 5 设计原则 + 3 拆段 + 4 行黑名单 grep)
> Acceptance: `docs/_archive/qa/acceptance-templates/cm-5.md` (§1 schema 反向约束 + §2 server 路径验证 + §3 client UI + §4 grep)
> Implementation entry (CM-5.1): **stub 待落** — 计划包路径 `packages/server-go/internal/api/cm5stance/cm_5_1_anti_constraints_test.go` (5 反向约束 grep test, 跟 PR #469 stub 同精神, 真包等 CM-5.1 实施 PR 落地)

## 1. 设计原则总览 (5 条 byte-identical 锁定)

| 原则 | 内容 | 反向约束出处 |
|---|---|---|
| ① 走人 path 不拆表 | agent↔agent 协作走人协作 path (DM-2 mention router + CV-1 artifact + AP-0/AP-2 permission); 反向约束: 不拆 `agent_messages` 表 / 不开 `ai_to_ai_channel` / 不开 `POST /agents/:id/notify-agent` 旁路 | TestCM51_NoBypassTable + TestCM51_NoBypassEndpoint |
| ② 责任 owner-first | `artifact_versions.committed_by` 永远是 user.id (agent 也是 user.role='agent'); 反向约束: 不拆 `triggered_by_agent_id` 列 | TestCM51_NoOwnerBypassColumn |
| ③ X2 冲突复用 | 复用 CV-1.2 single-doc lock 30s + CV-4.1 iterations state + CV-4 #380 ⑦ 错码 `artifact.locked_by_another_iteration` byte-identical; 反向约束: 不引入新 schema (artifact_locks / iteration_priority 表) | TestCM51_NoNewLockTable + TestCM51_X2ConflictLiteralReuse |
| ④ mention 走 DM-2 | agent A → B mention 走 DM-2.2 mention router (#372); MentionPushedFrame 8 字段 byte-identical; 反向约束: 不开 `agent_to_agent_mention` 专属 frame | TestCM51_NoBypassTable (含 frame name) |
| ⑤ 透明 owner-first 可见 | agent A → agent B 协作产物对两 owner 都可见 (跟人协作产物 owner 可见同模式); 反向约束: 不拆 owner_visibility scope, 不引入 "ai_only" 隐藏字段 | acceptance §3.1 client UI 验 |

## 2. X2 冲突裁决路径 (设计 ③ 详解)

> 场景: 同一 artifact 被 2+ agent 同时 commit (`?iteration_id=` query 落地间隔 < 200ms).

```
agent A (owner_A's agent) ──┐
                            ├── 同 artifact_id=X commit?iteration_id=YA
agent B (owner_B's agent) ──┘                                     ↓
                                                       CV-1.2 single-doc lock (30s)
                                                                  ↓
                                  第二写者 → 409 with code `artifact.locked_by_another_iteration`
                                                                  ↓
                                  client SPA UI toast: "正在被 agent {ownerName} 处理"
                                  + retry 入口 (跟 CV-4 #380 ⑦ 同字面)
```

**复用机制**:
- CV-1.2 既有 single-doc lock 30s (`artifacts.locked_by` 列 — channel 内仅一把锁)
- CV-4.1 既有 iterations state machine (4 态: pending/running/completed/failed)
- CV-4 #380 ⑦ 既有 409 错码字面 `artifact.locked_by_another_iteration`
- CV-4.3 既有 client UI toast 文案锁定 byte-identical

**不复用**: 不引入 schema (无 v=N+ migration), 不开新 endpoint, 不加新 frame.

## 3. agent A → B mention 路径 (设计 ④ 详解)

> 场景: agent A 在 channel C 里发 `Hi @agent_B, can you check this?` message.

```
agent A POST /api/v1/channels/C/messages (body 含 @agent_B token)
            ↓
DM-2.2 mention parser (#372 既有路径) — 解析 @ token → agent_B.user_id
            ↓
INSERT message_mentions (message_id, target_user_id=agent_B.id)
            ↓
DM-2.2 mention dispatch (#372 既有路径):
  - online: WS push MentionPushedFrame 8 字段 byte-identical 给 agent_B
  - offline: system DM 给 agent_B's owner (owner-first 责任语义)
```

**反向约束**:
- agent.role='agent' **不**影响 mention router 路径分流 (走人路径同源)
- 不开 `agent_to_agent_mention` 专属 frame (BPP-1 #304 envelope CI lint reflect 自动覆盖)
- MentionPushedFrame 8 字段 byte-identical 跟 ArtifactUpdated 7 / AnchorCommentAdded 10 / IterationStateChanged 9 共 cursor sequence

## 4. 协作可见性 (设计 ⑤)

agent A → B 协作产物对两 owner 都可见:
- artifact iterate 链 (CV-4 既有 `GET /api/v1/artifacts/:id/iterations`) — owner_A + owner_B 都返
- anchor reply 链 (CV-2 既有 `GET /api/v1/artifacts/:id/anchors/:anchor_id/comments`) — owner_A + owner_B 都返
- mention thread (DM-2 既有) — owner_A + owner_B owner 视图都可见

**反向约束**: 不拆 `visibility_scope` 列, 不引入 `ai_only` 隐藏字段 (透明协作是产品设计字面 — 蓝图 §185).

## 5. CM-5 三段拆 (CM-5.1 / CM-5.2 / CM-5.3)

| 段 | 实施物 | 数据库改动 | PR |
|---|---|---|---|
| CM-5.1 schema 反向约束锁定 | `cm_5_1_anti_constraints_test.go` 5 cases (NoBypassTable / NoBypassEndpoint / NoOwnerBypassColumn / NoNewLockTable / X2ConflictLiteralReuse) + 本文档 | **无** (① 走人 path 不拆表) | 本 PR |
| CM-5.2 server 路径验证 | `cm_5_2_agent_to_agent_test.go` 端到端 (TestCM52_AgentMentionsAgent / AgentCommitsAfterAgent409 / AgentIterateChainOwnerVisible) | 无 | 后续 PR |
| CM-5.3 client UI | `AgentManager.tsx` hover 协作链路 + e2e 双 agent commit 同 artifact 触发 409 + screenshot | 无 | 后续 PR |

## 6. 边界 (跟其他 milestone 关系)

| Milestone | 关系 |
|---|---|
| CM-4 ✅ | agent_invitations 邀请就位, CM-5 不动 |
| CV-1 ✅ | single-doc lock 30s 复用 (设计 ③) |
| CV-4 ✅ | iterate state 复用 + 409 错码 byte-identical (#380 ⑦) |
| DM-2 ✅ | mention dispatch 路径复用 (设计 ④) — MentionPushedFrame 8 字段 byte-identical |
| AP-3 (Phase 4) | agent acting-as-user 权限对接 |
| RT-3 ⭐ (Phase 4) | 多端全推 + 活物感 — 推 owner 双方 |

## 7. 反向约束 grep 黑名单 (跟野马 #366 同模式)

每 CM-5.* PR 必跑 — `go test ./internal/api/cm5stance/...` (5 cases):

```
- agent_messages\b / ai_to_ai_channel / agent_only_message / agent_to_agent_mention — 0 hit (设计 ①+④)
- POST /api/v1/agents/.*/notify-agent — 0 hit (设计 ①)
- triggered_by_agent_id / committed_by_agent — 0 hit (设计 ②)
- CREATE TABLE artifact_locks / iteration_priority — 0 hit (设计 ③)
- CM-5 自起 X2 错码 (cm5.x2_conflict / agent_collision / artifact.x2_conflict / x2_lock_held) — 0 hit (设计 ③ 复用 CV-4 #380 ⑦)
```

CI 自动跑, 反向约束守持续 — 跟 BPP-1 envelope reflect lint 同精神.
