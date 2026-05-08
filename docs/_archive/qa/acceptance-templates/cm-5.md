# Acceptance Template — CM-5: agent↔agent 独立协作 + X2 冲突裁决

> 蓝图: `concept-model.md §1.3` (§185 "未来你会看到 agent 互相协作") + `agent-lifecycle.md §1` (Borgee 是协作平台)
> Spec: `docs/implementation/modules/cm-5-spec.md` (战马A v0, 5 立场 + 3 拆段 + 7 行黑名单 grep)
> 拆 PR (拟): **CM-5.1** schema 反约束锁 + 反向 grep 黑名单测试 + **CM-5.2** server 协作路径验证 + **CM-5.3** client UI 透明协作可见性 + 文案锁
> 依赖: CM-4 ✅ (#220+#222+#243+...) + AP-3 (Phase 4, 暂不阻塞 CM-5.1 schema)
> Owner: 战马A 实施 / 烈马 验收

## 验收清单

### §1 schema (CM-5.1) — 反约束锁 + 黑名单 grep (无新表立场)

> 锚: spec §1.1 + 立场 ①②③ — agent↔agent 走人协作 path, 不裂表/不开旁路.

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 立场 ① 物理拆死 — 无新 schema 改动; 反向 grep `agent_messages\b` / `ai_to_ai_channel` / `agent_only_message` 在 `internal/migrations/` count==0 | CI grep | 战马A / 烈马 | ✅ 三段全闭 #463 + #473 + #476 (CM-5.3 stacked) — 详见 REG-CM5-001..005 |
| 1.2 立场 ② 责任旁路反约束 — `triggered_by_agent_id` / `committed_by_agent` 列在 `cv_1_1_artifacts.go` / `cv_4_1_artifact_iterations.go` count==0 | unit + grep | 战马A / 烈马 | ✅ 三段全闭 #463 + #473 + #476 (CM-5.3 stacked) — 详见 REG-CM5-001..005 |
| 1.3 立场 ③ 新锁表反约束 — `artifact_locks\s+TABLE` / `iteration_priority\s+TABLE` 在 migrations/ count==0; X2 冲突复用 CV-1.2 既有 single-doc lock 30s | grep + unit | 战马A / 烈马 | ✅ 三段全闭 #463 + #473 + #476 (CM-5.3 stacked) — 详见 REG-CM5-001..005 |
| 1.4 立场 ④ mention 旁路反约束 — `agent_to_agent_mention` / `POST /api/v1/agents/:id/notify-agent` 在 internal/ count==0 | CI grep | 飞马 / 烈马 | ✅ 三段全闭 #463 + #473 + #476 (CM-5.3 stacked) — 详见 REG-CM5-001..005 |
| 1.5 文档同步 — `docs/current/server/data-model.md` 加 "agent↔agent 协作 schema 反约束" 段 (跟 chn-2 #353 同模式) | review | 战马A / 烈马 | ✅ 三段全闭 #463 + #473 + #476 (CM-5.3 stacked) — 详见 REG-CM5-001..005 |

### §2 server (CM-5.2) — 协作路径验证 (复用 path 不开新代码)

> 锚: spec §1.2 + 立场 ④⑤ — agent A → B mention 走 DM-2 router; iterate 链 owner-first 可见.

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 agent A → agent B mention 走 DM-2.2 router (#372 既有 path); MentionPushedFrame 8 字段 byte-identical 推 agent B; system DM 到 B's owner (跟人同模式) | unit + e2e | 战马A / 烈马 | ✅ 三段全闭 #463 + #473 + #476 (CM-5.3 stacked) — 详见 REG-CM5-001..005 |
| 2.2 X2 冲突 — agent A `commit?iteration_id=X` + agent B 同 artifact `commit?iteration_id=Y` < 200ms → 第二写者 409 with code `artifact.locked_by_another_iteration` byte-identical (跟 CV-4 #380 ⑦ 同源) | unit | 战马A / 烈马 | ✅ 三段全闭 #463 + #473 + #476 (CM-5.3 stacked) — 详见 REG-CM5-001..005 |
| 2.3 立场 ⑤ — agent iterate 链 owner-first 可见 (`GET /artifacts/:id/iterations` returns 全链, owner_A + owner_B 都能查) | unit + e2e | 战马A / 烈马 | ✅ 三段全闭 #463 + #473 + #476 (CM-5.3 stacked) — 详见 REG-CM5-001..005 |
| 2.4 反约束 — server 不开 `POST /api/v1/agents/:id/notify-agent` 旁路 endpoint (CI grep + 路由表反向断言) | grep | 飞马 / 烈马 | ✅ 三段全闭 #463 + #473 + #476 (CM-5.3 stacked) — 详见 REG-CM5-001..005 |

### §3 client UI (CM-5.3) — 透明协作可见性 + 文案锁

> 锚: spec §1.3 + 立场 ⑤ — owner-first 透明协作 (蓝图 §185 "agent 互相协作" 用户感知).

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 AgentManager.tsx hover agent 显示 "正在协作: {agentName}" 链路 (反 owner 的 agent 离线/忙碌时不显示协作状态) | vitest + e2e | 战马A / 烈马 | ✅ 三段全闭 #463 + #473 + #476 (CM-5.3 stacked) — 详见 REG-CM5-001..005 |
| 3.2 X2 冲突 toast 文案锁 byte-identical — "正在被 agent {name} 处理" (跟 CV-4 #380 ⑦ + #365 反约束 ② 三源同源 byte-identical) | vitest content-lock | 战马A / 野马 | ✅ 三段全闭 #463 + #473 + #476 (CM-5.3 stacked) — 详见 REG-CM5-001..005 |
| 3.3 e2e 双 agent commit 同 artifact 触发 409 + screenshot 入 `docs/qa/screenshots/cm-5-x2-conflict.png` | e2e + screenshot | 战马A / 烈马 | ✅ 三段全闭 #463 + #473 + #476 (CM-5.3 stacked) — 详见 REG-CM5-001..005 |
| 3.4 反约束 — client 不订阅 agent_only frame (`grep -nE 'agent_only|agent_to_agent' packages/client/src/` count==0) | CI grep | 飞马 / 烈马 | ✅ 三段全闭 #463 + #473 + #476 (CM-5.3 stacked) — 详见 REG-CM5-001..005 |

### §4 反约束 grep 黑名单 (跟野马 #366 7 行黑名单同模式)

> 锚: spec §2 — 7 行 byte-identical 同根, 每 CM-5.* PR 必跑 0 命中.

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 立场 ① 旁路表 — `agent_messages\b` / `ai_to_ai_channel` / `agent_only_message` count==0 | CI grep | 飞马 / 烈马 | _(每 CM-5.* PR 必跑)_ |
| 4.2 立场 ① 旁路 endpoint — `POST /api/v1/agents/.*/notify-agent` count==0 | CI grep | 飞马 / 烈马 | _(每 CM-5.* PR 必跑)_ |
| 4.3 立场 ② 责任旁路 — `triggered_by_agent_id` / `committed_by_agent` count==0 | CI grep | 飞马 / 烈马 | _(每 CM-5.* PR 必跑)_ |
| 4.4 立场 ③ 新锁表 — `artifact_locks\s+TABLE` / `iteration_priority\s+TABLE` count==0 | CI grep | 飞马 / 烈马 | _(每 CM-5.* PR 必跑)_ |

## 边界 (跟其他 milestone 关系)

| Milestone | 关系 | 字面承袭 |
|---|---|---|
| CM-4 ✅ | agent_invitations 邀请就位, CM-5 不动 | agent_invitations 表 byte-identical 不破 |
| CV-1 ✅ | single-doc lock 30s 复用 (立场 ③) | artifacts.locked_by + 409 toast 字面 |
| CV-4 ✅ | iterate state 复用; X2 冲突 409 错码同源 #380 ⑦ | `artifact.locked_by_another_iteration` byte-identical |
| DM-2 ✅ | mention dispatch 路径复用 (立场 ④) | MentionPushedFrame 8 字段 byte-identical |
| AP-3 (Phase 4) | agent acting-as-user 权限对接 | AP-3 落 → CM-5.2 follow-up 加 owner_A acting-as agent_A 验 |
| RT-3 ⭐ (Phase 4) | 多端全推 + 活物感 — 推 owner 双方 | 复用 RT-1 fanout, 不开新 frame |

## 退出条件

- §1 schema 反约束 5 项 + §2 server 4 项 + §3 client 4 项 + §4 grep 4 行**全绿** (一票否决)
- 登记 `docs/qa/regression-registry.md` REG-CM5-001..005
- 跟 CV-4 #380 ⑦ + DM-2.2 #372 + CV-1.2 #342 既有 path byte-identical 不破
- agent↔agent 协作走人 path 立场 ① 守住 (反约束 grep 7 行 0 命中)
