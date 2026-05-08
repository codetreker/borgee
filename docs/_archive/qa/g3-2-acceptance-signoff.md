# G3.2 Acceptance Signoff — CV-2 锚点对话 E2E (烈马)

> **状态**: ✅ **SIGNED** (烈马, 2026-04-30)
> **闸**: Phase 3 G3.2 退出闸 — CV-2.3 anchor comments e2e PASS + 烈马 QA acceptance signoff
> **关联**: 详细 acceptance log 锚 [`docs/qa/signoffs/g3.2-cv2-liema-signoff.md`](signoffs/g3.2-cv2-liema-signoff.md) (5/5 一签 2026-04-29) + evidence bundle [`docs/_archive/evidence/g3-exit/README.md`](../evidence/g3-exit/README.md) §2

---

## §1 G3.2 退出条件

| # | 条件 | 状态 |
|---|------|------|
| ① | CV-2.3 anchor comments e2e PASS (`cv-2-anchor-comments.spec.ts` 真 server-go 4901 + vite 5174) | ✅ |
| ② | 烈马 QA acceptance signoff (本文档) | ✅ |

两条件全闭 → G3.2 闸通过.

---

## §2 证据链

- **PR #404** — CV-2.3 client SPA 选区→锚点 entry + thread side panel + WS push 实时刷, 4 文案字面 byte-identical (跟 #355 文案锁 ①②③④ 同源), e2e §3.1+§3.2+§3.5+§3.6 PASS (merged 693e70c)
- **PR #421** — CV-2 closure: REG-CV2-001..005 ⚪→🟢 stack + AnchorCommentAdded 10 字段 byte-identical 三源闭环 (spec #368 v3 ↔ schema #359 ↔ frame #360) (merged)
- **e2e**: `packages/e2e/tests/cv-2-3-anchor-client.spec.ts` (实施名, 任务别名 `cv-2-anchor-comments.spec.ts`) 跑过 §3.1 选区→锚 / §3.2 thread side panel / §3.5 WS push ≤3s / §3.6 agent 视角 DOM 无入口
- 上游闭环: CV-2.1 #359 schema v=14 + CV-2.2 #360 server 4 endpoints 全 merged

---

## §3 Acceptance template 引用

锚 [`docs/qa/acceptance-templates/cv-2.md`](acceptance-templates/cv-2.md) 验收清单全 ✅:

- §0 关键约束 ①..⑦ (锚=人机界面 / 段落 range / 评论独立 entity / anchor 不改 body / version-pin / mention 复用 DM-2 / channel-scoped) — 全锁
- §1 schema (CV-2.1) ✅ — REG-CV2-001
- §2 server (CV-2.2) ✅ — REG-CV2-002 + REG-CV2-003 (10 字段 frame)
- §3 client (CV-2.3) ✅ — REG-CV2-004 + REG-CV2-005

5/5 通过 (跟 g3.2-cv2-liema-signoff.md §1 表 byte-identical 同源).

---

## §4 反向断言 (regression-registry §3 CV-2 路径)

既有 channel-level comments (普通 message 流) 行为 **不变** — anchor + comment 数据层与 `messages` 表拆死, comment 不进 channel WS fanout (反向 fanout, 仅推订阅 anchor 的客户端). 反约束三连永久锁:

- (a) client DOM agent 视角无 hover 入口 ✅ (`§3.6` count==0)
- (b) server agent role POST anchor → 403 `anchor.create_owner_only` ✅ (`TestCV22_AgentCannotCreateAnchor`)
- (c) cross-channel POST anchor → 403 ✅ (`TestCV22_CrossChannel403`)

锚 [`docs/qa/regression-registry.md`](regression-registry.md) §3 CV-2 路径, REG-CV2-001..005 全 🟢.

---

## §5 Signoff

| 角色 | 姓名 | 日期 | 一句话 |
|------|------|------|--------|
| QA | 烈马 | 2026-04-30 | CV-2.3 anchor comments e2e PASS, 立场无漂移, G3.2 ready to sign |

---

## §6 PROGRESS.md 翻牌

PROGRESS.md Phase 3 概览 G3.2 行: ⏸️ → ✅ (烈马 acceptance signoff doc 由 ⏸️ 翻为 ✅, 本 PR 同步落).
