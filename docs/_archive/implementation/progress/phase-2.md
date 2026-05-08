# Phase 2 — 协作闭环 (detail)

> 引自 [PROGRESS.md](../PROGRESS.md) 概览表 Phase 2 行 — milestone 翻牌点单源在此.

## Phase 2 — 协作闭环 ⭐ (2026-04-28 R3 立场冲突后重排)

> **R3 重排 (#188 merged)**: 6 条立场冲突落地后, Phase 2 解封顺序改为: ADM-0 (admin 拆表) + AP-0-bis (message.read 回归) + INFRA-2 (Playwright 提前) + RT-0 (/ws push 顶住 BPP) + CM-onboarding (Welcome channel) → 然后 CM-4.3b/4.4 → 闸 4 签字。
> **Phase 2 工期净增 +8-10 天** (战马 R3 实测), 但避免 CM-3 之后每个 endpoint 都要写 admin 特殊分支。

**Milestones (按 R3 解封顺序)**

### Phase 2 解封前置 (R3 新增)

- [x] **INFRA-2** Playwright scaffold (E2E) — 战马 / 飞马 / 烈马 (PR #195 merged)
  - 必须前置到 RT-0 之前 (烈马 R3: latency ≤ 3s 硬条件 vitest 跑不了)
  - 工期 2-3 天 (战马 R1: vite orchestrate + cookie fixtures + chromium CI)
- [x] **ADM-0** admin 拆表 (admins 独立表 + cookie 拆 + god-mode endpoint) — 战马 / 飞马 (主, 起草) / 烈马 / 野马
  - [x] **ADM-0.1** admins 独立表 + env bootstrap + 独立 cookie name (双轨并存) (PR #197 merged)
  - [x] **ADM-0.2** cookie 拆 + RequirePermission 去 admin 短路 + god-mode 白名单 (users.role='admin' 调 user-api 401) (PR #201 merged)
  - [x] **ADM-0.3** users.role enum 收 + backfill 旧 admin 行 → admins 表 + revoke session (users.role='admin' 行数 = 0) (PR #223 merged, v=10)
  - 工期 server 4-6 天 + client 1 天
  - 烈马一票否决: cookie 串扰反向断言
  - 详见 [`modules/admin-model.md`](modules/admin-model.md)
- [x] **AP-0-bis** message.read 默认 grant + backfill 迁移 — 战马 / 飞马 / 烈马 (PR #206 merged, v=8)
  - 工期 1 天
  - 必带 `testutil.SeedLegacyAgent` helper (烈马 R3, CM-3 也用)
  - **依赖 ADM-0.2 已 merge** (飞马 R1 P0 ②: AP-0-bis 加 RequirePermission("message.read") 必须在 admin 直通短路砍掉之后, 否则 admin 既被砍直通又没 message.read 而 401 中间态)
  - 详见 [`modules/auth-permissions.md`](modules/auth-permissions.md)
- [x] **CM-onboarding** Welcome channel + auto-join + system message — 战马 / 飞马 / 野马 (立场) / 烈马 (PR #203 merged, v=7)
  - 工期 0.5-1 天
  - **依赖野马 `00-foundation/onboarding-journey.md` (硬截止 2026-05-05)** — 飞马 R1 P1 ③: 防卡死风险
  - 野马 must-fix (实施时落地): Welcome system message 必带 quick action button "创建 agent" + backfill 失败的空状态降级文案 (§11 反约束) + 文案锁定位置在 onboarding-journey.md
  - 详见 [`modules/concept-model.md`](modules/concept-model.md) §10
- [x] **RT-0** /ws push 顶住 BPP (取代 60s polling) — 战马 / 飞马 / 烈马 / 野马 (PR #218 client + #237 server merged)
  - 工期 1.5-2 天
  - 依赖 INFRA-2 (latency E2E 验)
  - 蓝图 realtime §2.3: schema 必须等同未来 BPP frame, CI lint 强制
  - 野马硬条件: latency ≤ 3s (Playwright stopwatch 截屏)
  - 详见 [`modules/realtime.md`](modules/realtime.md)

### CM-4 完成 (前置就位后)

- [x] **CM-4.0** agent_invitations schema + 状态机单测 (#183 merged)
- [x] **CM-4.1** API handler POST/GET/PATCH (#185 merged)
- [x] **CM-4.2** client UI inbox + quick action (#186 merged) — **60s polling, RT-0 后切 ws push 自动升级**
- [ ] **CM-4.3b** 离线检测 + system message — ⏸️ Deferred Phase 4 (closure #284 phase-2-exit-announcement.md DEFERRED 2 项之一)
- [ ] **CM-4.4** 5 分钟节流 + E2E — ⏸️ Deferred Phase 4 (closure #284 DEFERRED 2 项之一) — 战马 / 烈马
- [x] **闸 4 独立流程**: 野马 demo 签字 ✅ #275 (4/6 接受条件闭, 2 deferred Phase 4) + 5 张关键截屏 + blueprint-sha.txt

### Phase 2 后置 (CM-4 闸 4 通过后)

- [x] **ADM-1** 用户侧隐私承诺页 (Phase 4 启动 milestone) — 战马B (实施) / 战马D (e2e + 截屏) / 野马 (文案 + G4.1 demo 签字 ⏸️) / 烈马 (验收)
  - 用户承诺页 3 条文案锁死 (admin-model §4.1) ✅ #455 PrivacyPromise.tsx PRIVACY_PROMISES + drift test (vite `?raw` import doc-as-truth) + PRIVACY_TABLE_ROWS 八行三色锁
  - 隐私承诺反查表 v0 ✅ 落 (PR #211, 野马) + 实施 spec ✅ #228 (1 组件 + 1 页面 + 5 反向断言)
  - 实施 PR ✅ #455 (PrivacyPromise + SettingsPage + drift + 14 vitest cases + ⚙️ 按钮 wiring)
  - e2e + G4.1 双截屏 ✅ #459 (3 cases PASS, `g4.1-adm1-{privacy-promise,privacy-table}.png` 入 git, 真 4901+5174 不 mock)
  - REG-ADM1-001..006 6 行 🟢 落 registry ✅ (drift / promise literal / table byte-identical / 三色锁 / details 反约束 / admin-user 路径分叉)
  - 联签项 (admin 写动作 system DM `admin_name` 非 UUID) ⏸️ deferred 给 ADM-2 真实施 (蓝图 §1.4 R3 字面已锁)
  - 详见 [`modules/admin-model.md`](modules/admin-model.md)

**配套 doc 工件 (Phase 2 已落)**:
- ADM-0 立场反查表 v0 ✅ (PR #205, 野马)
- G2.4 5 张截屏 spec + 野马 partial 签 ✅ (PR #199 计划 + 后续 spec)

**Gates**

- [x] G2.0 (新, R3) ADM-0 cookie 串扰反向断言 — 烈马 / 证据: PR #197+#201+#223 (closure #284 SIGNED) ✅
- [x] G2.1 邀请审批 E2E (Playwright + ws push) — 战马/烈马 / 证据: PR #237+#239 (closure #284 SIGNED) ✅
- [x] G2.2 离线 fallback E2E — 战马/烈马 / 证据: PR #237+#239 (closure #284 SIGNED) ✅
- [x] G2.3 节流不变量 (fake clock 单测) — 烈马 / 证据: PR #229 internal/throttle + #236 T1-T5 全过
- [x] G2.4 用户感知签字 — **野马** / 证据: PR #275 (4/6 接受闭, 2 deferred Phase 4); 截屏 `docs/evidence/cm-4/`; closure #284 SIGNED ✅
- [x] G2.5 presence 接口契约 (IsOnline + Sessions 锁死) — 飞马/战马 / 证据: PR #277 AL-3 contract.go (closure #284 SIGNED) ✅
- [x] G2.6 (新, R3) /ws → BPP schema 等同性 (CI lint byte-identical) — 飞马 / 证据: PR #304+#237 (closure #284 SIGNED) ✅
- [x] **G2.audit** v0 代码债 audit — 飞马 / 证据: PR #212+#231+#244+#251 (closure #284 SIGNED) ✅

**野马签字**: ___ (日期: ___) | 1 周 dogfood 反馈期截止: ___


## 更新日志归档 (历史 changelog 迁入)

| 日期 | 更新人 | 变化 |
|------|--------|------|
| 2026-04-28 | 飞马 | D1–D9 flip (audit #212 派活): Phase 1 改 ✅ DONE (G1.4 closed by #208 + #210, CM-3 closed by #208); Phase 2 解封前置 5/6 改 [x] (INFRA-2 #195, ADM-0.1 #197, ADM-0.2 #201, AP-0-bis #206, CM-onboarding #203); ADM-0 总括改 🔄 (ADM-0.3 #63 in progress); RT-0 留 ⏳; 配套 doc 工件 (#205 ADM-0 立场反查 / #211 ADM-1 隐私承诺 / #199 G2.4 截屏 plan) 落 Phase 2 后置区 |

---
