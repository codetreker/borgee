# CM-5 — agent-to-agent collaboration (X2 conflict path)

> Blueprint: `concept-model.md §1.3` (§185 "未来你会看到 agent 互相协作") + `agent-lifecycle.md §1` (Borgee is a collaboration platform, not an agent platform; agent collaboration goes through Borgee platform mechanisms + plugin runtime)
> Spec: `docs/_archive/implementation/modules/cm-5-spec.md` (v0, five design principles + three implementation slices + four-line deny-list grep)
> Acceptance: `docs/_archive/qa/acceptance-templates/cm-5.md` (§1 schema constraints + §2 server path validation + §3 client UI + §4 grep)
> Implementation entry (CM-5.1): **stub pending** — planned package path `packages/server-go/internal/api/cm5stance/cm_5_1_anti_constraints_test.go` (five constraint grep tests, same stub pattern as PR #469; real package lands with CM-5.1 implementation PR)

## 1. Design Principles (five byte-identical locks)

| Principle | Content | Constraint Source |
|---|---|---|
| ① reuse human collaboration path | agent-to-agent collaboration uses the same collaboration path as humans (DM-2 mention router + CV-1 artifact + AP-0/AP-2 permission); no `agent_messages` table / `ai_to_ai_channel` / `POST /agents/:id/notify-agent` bypass | TestCM51_NoBypassTable + TestCM51_NoBypassEndpoint |
| ② owner-first responsibility | `artifact_versions.committed_by` is always user.id (agents also use user.role='agent'); no separate `triggered_by_agent_id` column | TestCM51_NoOwnerBypassColumn |
| ③ reuse X2 conflict handling | reuse CV-1.2 single-doc lock 30s + CV-4.1 iterations state + CV-4 #380 ⑦ error code `artifact.locked_by_another_iteration` byte-identical; do not introduce new schema (artifact_locks / iteration_priority tables) | TestCM51_NoNewLockTable + TestCM51_X2ConflictLiteralReuse |
| ④ mentions use DM-2 | agent A → B mention uses DM-2.2 mention router (#372); MentionPushedFrame 8 fields remain byte-identical; no dedicated `agent_to_agent_mention` frame | TestCM51_NoBypassTable (includes frame name) |
| ⑤ owner-first visibility | agent A → agent B collaboration artifacts are visible to both owners, matching human collaboration visibility; no owner_visibility scope or "ai_only" hidden field | acceptance §3.1 client UI validation |

## 2. X2 Conflict Path (design ③)

> Scenario: the same artifact is committed by 2+ agents at nearly the same time (`?iteration_id=` query landing within < 200ms).

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

**Not added**: no schema (no v=N+ migration), no new endpoint, and no new frame.

## 3. agent A → B Mention Path (design ④)

> Scenario: agent A sends `Hi @agent_B, can you check this?` in channel C.

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

**Constraints**:
- agent.role='agent' does not change mention router dispatch; it uses the same path as human users.
- No dedicated `agent_to_agent_mention` frame (covered by BPP-1 #304 envelope CI reflection lint).
- MentionPushedFrame 8 fields remain byte-identical with the shared cursor sequence used by ArtifactUpdated 7 / AnchorCommentAdded 10 / IterationStateChanged 9.

## 4. 协作可见性 (设计 ⑤)

agent A → B 协作产物对两 owner 都可见:
- artifact iterate 链 (CV-4 既有 `GET /api/v1/artifacts/:id/iterations`) — owner_A + owner_B 都返
- anchor reply 链 (CV-2 既有 `GET /api/v1/artifacts/:id/anchors/:anchor_id/comments`) — owner_A + owner_B 都返
- mention thread (DM-2 既有) — owner_A + owner_B owner 视图都可见

**反向约束**: 不拆 `visibility_scope` 列, 不引入 `ai_only` 隐藏字段 (透明协作是产品设计字面 — 蓝图 §185).

## 5. CM-5 Implementation Slices (CM-5.1 / CM-5.2 / CM-5.3)

| 段 | 实施物 | 数据库改动 | PR |
|---|---|---|---|
| CM-5.1 schema constraint lock | `cm_5_1_anti_constraints_test.go` 5 cases (NoBypassTable / NoBypassEndpoint / NoOwnerBypassColumn / NoNewLockTable / X2ConflictLiteralReuse) + this document | **none** (① reuses the human collaboration path; no table split) | this PR |
| CM-5.2 server 路径验证 | `cm_5_2_agent_to_agent_test.go` 端到端 (TestCM52_AgentMentionsAgent / AgentCommitsAfterAgent409 / AgentIterateChainOwnerVisible) | 无 | 后续 PR |
| CM-5.3 client UI | `AgentManager.tsx` hover collaboration path + e2e where two agents commit the same artifact and trigger 409 + screenshot | none | follow-up PR |

## 6. 边界 (跟其他 milestone 关系)

| Milestone | 关系 |
|---|---|
| CM-4 ✅ | agent_invitations 邀请就位, CM-5 不动 |
| CV-1 ✅ | single-doc lock 30s 复用 (设计 ③) |
| CV-4 ✅ | iterate state 复用 + 409 错码 byte-identical (#380 ⑦) |
| DM-2 ✅ | mention dispatch 路径复用 (设计 ④) — MentionPushedFrame 8 字段 byte-identical |
| AP-3 (Phase 4) | agent acting-as-user 权限对接 |
| RT-3 ⭐ (Phase 4) | 多端全推 + 活物感 — 推 owner 双方 |

## 7. Constraint Grep Deny List

每 CM-5.* PR 必跑 — `go test ./internal/api/cm5stance/...` (5 cases):

```
- agent_messages\b / ai_to_ai_channel / agent_only_message / agent_to_agent_mention — 0 hit (设计 ①+④)
- POST /api/v1/agents/.*/notify-agent — 0 hit (设计 ①)
- triggered_by_agent_id / committed_by_agent — 0 hit (设计 ②)
- CREATE TABLE artifact_locks / iteration_priority — 0 hit (设计 ③)
- CM-5 自起 X2 错码 (cm5.x2_conflict / agent_collision / artifact.x2_conflict / x2_lock_held) — 0 hit (设计 ③ 复用 CV-4 #380 ⑦)
```

CI runs these checks continuously, following the same guard pattern as the BPP-1 envelope reflection lint.
