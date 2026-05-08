# Phase 1 → Phase 2 过渡对账 audit

> 作者: 飞马 · 2026-04-28 · R3 重排后第一次系统对账
> 范围: 蓝图 / implementation / docs/current 三方对照本周 (3 天) PR 落地, 找 drift 出 out-of-date 清单 + 派活。

## 1. 本周 PR 落地 (3 天 / git log origin/main, 倒序)

#210 G1 全签 / #208 CM-3 / #209 registry / #206 AP-0-bis / #207 registry / #203 CM-onboarding / #205 ADM-0 反查表 / #204 CI fix / #202 registry / #201 ADM-0.2 / #200 CM-3 反查表 / #199 G2.4 截屏计划 / #198 bug-029 / #197 ADM-0.1 / #195 INFRA-2 / #193 acceptance 模板 / #192 CODEOWNERS / #191 ADM-0 checklist / #190 onboarding-journey v1 / #189 R3 重排 / #188 R3 蓝图。

**Phase 1**: 闭环, G1.1–G1.5 + G1.audit 全签 (#210). **Phase 2 前置**: INFRA-2 / ADM-0.1 / ADM-0.2 / AP-0-bis / CM-onboarding ✅ 5/6 merged; ADM-0.3 🔄 (#63, 战马); RT-0 可解锁 (INFRA-2 就绪)。

## 2. Out-of-date 清单 (drift)

### 🟥 P0 — PROGRESS.md (单一进度真相落后, 9 行待 flip)

| # | 位置 | 现状 | 应为 |
|---|------|------|------|
| D1 | Phase 概览 Phase 1 行 | `🔄 4/5 + audit ✅, G1.4 待 CM-3` | `✅ DONE` (#210) |
| D2 | Phase 1 CM-3 行 | `[ ]` + 子 `[ ]`×2 | `[x] (#208)` 合并 |
| D3 | Phase 1 G1.4 行 | `[ ] ⏸ 延后...` | `[x]` 证据 g1-audit.md §2.* + #208/#210 |
| D4 | Phase 2 INFRA-2 行 | `[ ]` | `[x] (#195)` |
| D5 | Phase 2 ADM-0.1 行 | `[ ]` | `[x] (#197)` |
| D6 | Phase 2 ADM-0.2 行 | `[ ]` | `[x] (#201)` |
| D7 | Phase 2 ADM-0.3 行 | `[ ]` | `🔄 (task #63, v=10)` |
| D8 | Phase 2 AP-0-bis 行 | `[ ]` | `[x] (#206, v=8)` |
| D9 | Phase 2 CM-onboarding 行 | `[ ]` | `[x] (#203, v=7)` |

派给 **team-lead**, 单 PR docs-only, ≤ 0.5 天。

### 🟥 P0 — docs/current/server/migrations.md §7 (落后 4 个 v)

| # | 项 | 应补 | 派谁 |
|---|----|------|------|
| D10a | §7 表追加 v=7 cm_onboarding_welcome 行 + 小节 | + messages.quick_action 列 / system user / #welcome 频道说明 | 战马B (CM-onboarding 作者) |
| D10b | §7 表追加 v=8 ap_0_bis_message_read 行 + 小节 | message.read 默认 grant + 老 agent backfill | 战马 (AP-0-bis 作者, 可与 ADM-0.3 PR 合 docs) |
| D10c | §7 表追加 v=9 cm_3_org_id_backfill 行 + 小节 | 4 表 org_id 反向回填 + PRAGMA gating + v=6 跳号说明 | 战马A (CM-3 作者) |
| D10d | §7 表追加 v=10 adm_0_3_users_role_collapse 行 + 小节 | users.role enum 收 + admin 行迁 admins 表 | 战马 (合 ADM-0.3 PR 内) |

### 🟡 P1 — docs/current/server/data-model.md (随 ADM-0.3 / CM-onboarding 同步)

- D11 §1 `users.role (member / admin / agent)` → ADM-0.3 后改 `(member / agent)` + 加 `admins` / `admin_sessions` 行 → **战马合 ADM-0.3 PR 顺手改**。
- D12 §1 messages 行追加 `quick_action TEXT` 列 → **战马B**。

### 🟡 P1 — architecture/admin-model.md

- D13 §2 Milestones ADM-0.1 / ADM-0.2 加 ✅ marker → **本 PR 已落** (飞马)。

### 🟢 P2 — 蓝图 / onboarding-journey 无 drift

blueprint admin-model / concept-model / auth-permissions 全部锁 R3 #188 立场, 本周 PR 行为一致, **无需改**。onboarding-journey #190 已 v1 + CM-onboarding #203 落地反向印证一致。

## 3. 派活汇总

| 派给 | 任务 | 形式 | 工期 |
|------|------|------|------|
| team-lead | PROGRESS.md D1–D9 (9 行 flip) | 单 PR docs-only | ≤ 0.5 天 |
| 战马B | migrations.md v=7 + data-model.md quick_action (D10a + D12) | 单 PR 或 follow-up | ≤ 0.3 天 |
| 战马A | migrations.md v=9 (D10c) | 单 PR docs-only | ≤ 0.3 天 |
| 战马 | migrations.md v=8 + v=10 (D10b + D10d) + data-model.md users.role (D11) | **合 ADM-0.3 PR 内** (其 PR 改 internal/ 必同步 docs/current) | 0 (随 PR) |
| 飞马 | admin-model.md D13 + 本 audit doc | 本 PR | ✅ 已落 |

## 4. Phase 2 闸 4 前瞻

5/6 解封前置已 merged; ADM-0.3 + RT-0 是剩余阻塞。CM-4.3b / CM-4.4 等 RT-0; 闸 4 demo 反查表 #205 已就位。

**预警**: 若 D1–D9 不及时 flip, 闸 4 签字时野马台账会见 5 个 `[ ]` 假数据淹没。**建议 team-lead 优先解决 D1–D9, 与 ADM-0.3 解耦**。
