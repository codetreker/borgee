# G3 截屏归档 audit 报告

> **状态**: v0 (战马C, 2026-04-29)
> **目的**: Phase 3 退出闸 G3.4 demo screenshot 归档前置 audit — 列截屏目录现状, 找命名 drift / 缺失 / 实际文件 vs README 锁路径不一致, 输出修补清单.
> **关联**: `docs/qa/screenshots/README.md` (野马 v0, 22 张 G3 路径锁) + 各 milestone e2e PR 实际 page.screenshot() path.
> **范围**: 仅 G3.* (Phase 3) — G2.* (Phase 2) 已 closed, 命名稳定不在本 audit.

---

## 1. 现状盘点 (`docs/qa/screenshots/` 目录实际文件)

| 文件 | 类型 | 出 PR | README 行 | 状态 |
|------|------|-------|----------|------|
| `g3.4-cv3-markdown.png` | G3.4 baseline | #408 e32d44a | line 32 | 🟢 in-spec |
| `g3.4-cv4-iterate-pending.png` | G3.4 | #422 4940e24 | _未列_ | ⚠️ **README 缺失** (实际产出 vs 锁不齐 — README §1 line 35 锁的是 `g3.4-cv4-iterate-trigger.png` 不同名) |
| `g3.x-chn3-sidebar-reorder.png` | G3.x | #422 4940e24 | line 42 | 🟢 in-spec |
| `g2.4-realtime-latency.png` | G2.4 (Phase 2) | #239 | _Phase 2 closed_ | 🟢 stable, 不在本 audit |
| `g2.7-runtime-agent-settings.png` | G2.7 (Phase 2 ext) | #427 1a7f6e3 | _未在 G3 README_ | 🟢 AL-4 acceptance §3 锚 |
| `chn-4-followup-cross-org-isolation.png` | CHN-4 followup | #423 3da88e7 | _未列_ | ⚠️ **命名 drift** — 无 G3.x 前缀, 命名规则跟 README 不一致 |
| `chn-4-followup-dm-no-handle.png` | CHN-4 followup | #423 3da88e7 | _未列_ | ⚠️ **命名 drift** — 同上 |

---

## 2. README §1 锁路径 vs 实际产出 (drift 列表)

> 锚: `docs/qa/screenshots/README.md` §1 line 26-46 22 张路径锁

### 2.1 已产出 (3/22)

| README 锁路径 | 实际文件 | 状态 |
|--------------|---------|------|
| `g3.4-cv3-markdown.png` | ✅ 存在 | PASS (#408 baseline) |
| `g3.x-chn3-sidebar-reorder.png` | ✅ 存在 | PASS (#422) |
| `g3.4-cv4-iterate-pending.png` | ⚠️ README 锁的是 `g3.4-cv4-iterate-trigger.png`; 实际产出 `iterate-pending` | **drift A** (语义同, 命名漂) |

### 2.2 deferred (4/22, 已锚 #424)

| README 锁路径 | deferred 锚 | 状态 |
|--------------|-------------|------|
| `g3.4-cv3-code-go-highlight.png` | `cv-3-3-deferred.spec.ts::§3.4 code` | ⏸️ deferred CV-5+ list endpoint |
| `g3.4-cv3-image-embed.png` | `cv-3-3-deferred.spec.ts::§3.4 image_link` | ⏸️ deferred CV-5+ list endpoint |
| (CHN-2 三 / CV-2 四 / CV-4 三 / DM-2 五 等) | _尚无 deferred 锚_ | ⚠️ **silent miss** — 既未产出, 也没标 ⏸️ deferred (跟 #424 双轨锚模式不齐) |

### 2.3 实际产出但 README 未列 (3 张)

| 实际文件 | 出 PR | 修补建议 |
|---------|-------|---------|
| `g3.4-cv4-iterate-pending.png` | #422 4940e24 | drift A — README §1 line 35 `g3.4-cv4-iterate-trigger.png` 改为 `g3.4-cv4-iterate-pending.png` 或反过来重命名文件 (跟 #380 文案锁字面对齐, 见 §4.1) |
| `chn-4-followup-cross-org-isolation.png` | #423 3da88e7 | **命名 drift** — 加 `g3.x-` 前缀: `g3.x-chn4-followup-cross-org-isolation.png`; README §1 加行 |
| `chn-4-followup-dm-no-handle.png` | #423 3da88e7 | 同上: `g3.x-chn4-followup-dm-no-handle.png` |

### 2.4 缺失 (15/22, 既未产出也未 deferred 锚)

| README 锁路径 | milestone | 文案锁源 | 备注 |
|--------------|-----------|---------|------|
| `g3.3-cv1-markdown-render.png` | CV-1 G3.3 signoff | CV-1 #346/#347 | G3.3 单独 signoff 闸; 既无 e2e 产出也无 deferred 锚 |
| `g3.3-cv1-commit-dropdown.png` | 同 | 同 | 同 |
| `g3.3-cv1-rollback-flow.png` | 同 | 同 | 同 |
| `g3.4-chn4-chat.png` | CHN-4 | #382 ⑥ | CHN-4 #411/#423 e2e 真路径未触 chat tab 截屏 |
| `g3.4-chn4-workspace.png` | CHN-4 | #382 ⑥ | 同 |
| `g3.4-cv2-anchor-entry.png` | CV-2 | #355 ① | CV-2.3 #404 e2e 跑 §3.1+§3.2 但未截屏 |
| `g3.4-cv2-thread-bubble.png` | CV-2 | #355 ② | 同 |
| `g3.4-cv2-agent-reply.png` | CV-2 | #355 ④ | 同 |
| `g3.4-cv2-resolved.png` | CV-2 | #355 ⑥ | 同 |
| `g3.4-cv4-running-state.png` | CV-4 | #380 ③ | CV-4.3 #416/#422 截屏锁 `iterate-pending` 单张 |
| `g3.4-cv4-completed-newversion.png` | CV-4 | #380 ④ | 同 |
| `g3.4-cv4-diff-view.png` | CV-4 | #380 ⑤ | 同 |
| `g3.x-dm-sidebar-section.png` | CHN-2 | #354 ① | CHN-2.3 #413 e2e 跑 5 测试但未截屏 |
| `g3.x-dm-view-no-workspace.png` | CHN-2 | #354 ④ | 同 |
| `g3.x-dm-mention-third-blocked.png` | CHN-2 | #354 ⑤ | 同 |

---

## 3. drift 修补 (本 PR 立即修)

> 范围: 仅修 README §1 锁文 + 实际文件命名一致 (drift A + chn-4-followup 双); 缺失 15 张走 follow-up explicit deferred 锚 (跟 #424 同模式) 留 PR 闸位明示, 不在本 audit PR 范围.

### 3.1 drift A — README 锁 `iterate-trigger.png` vs 实际 `iterate-pending.png`

**决策**: 改 README 锁路径跟实际文件一致 (#422 已落, 重命名文件会让 PR 反查链断). README §1 line 35 字面 `g3.4-cv4-iterate-trigger.png` → `g3.4-cv4-iterate-pending.png`. 验内容描述同步: `iterate 按钮 🔄 owner-only DOM omit + 输入框 placeholder + state=pending 触发刚提交` (语义跟原 trigger 一致, 文件名跟 #380 文案锁 ① state 命名同源).

### 3.2 chn-4-followup 双截屏命名 drift

**决策**: 重命名 `.png` 文件加 `g3.x-` 前缀 (跟 README §1 命名规则 byte-identical), 并加 README §1 两行锁路径. 同步改 `packages/e2e/tests/chn-4-followup-*.spec.ts` 内 `page.screenshot({path:...})` 字面.

### 3.3 缺失 15 张 — 留 explicit deferred 锚 (本 PR 不修, 留 follow-up)

跟 #424 cv-3-3-deferred.spec.ts 同模式 — 加 explicit fixme + `TODO("e2e/截屏前置 work")` 锚; 各 milestone owner (战马A/D/E) 后续 PR 真截屏归档时切真路径. 本 audit PR 仅出报告, 不抢这 15 张活.

---

## 4. 命名规则建议 (固化, 跟 README §1 对齐)

| 段 | 前缀 | 例 |
|----|------|-----|
| Phase 3 退出闸 demo (G3.4) | `g3.4-` | `g3.4-cv3-markdown.png` / `g3.4-chn4-chat.png` |
| Phase 3 各 milestone 单签 (G3.x) | `g3.x-` | `g3.x-chn3-sidebar-reorder.png` / `g3.x-dm-sidebar-section.png` |
| Milestone followup (非闸位 demo) | `g3.x-{milestone}-followup-` | `g3.x-chn4-followup-dm-no-handle.png` (修后) |
| G3.3 单 milestone signoff | `g3.3-{milestone}-` | `g3.3-cv1-markdown-render.png` |

**反约束**: 不准命名漂移 (`chn-4-followup-*.png` 无 G 前缀 = 反约束, 此 audit 修).

---

## 5. 修补 checklist (本 PR 闭环)

- [x] 现状 audit (§1) — 7 张实际 + 22 张 README 锁
- [x] drift 列表 (§2) — 3 in-spec / 4 deferred / 3 drift / 15 缺失
- [x] drift 修 (§3.1 README 字面 + §3.2 文件重命名 + e2e spec 引用同步)
- [ ] 缺失 15 张 explicit deferred 锚 — 留 follow-up PR (跟 #424 同模式, 不抢战马 A/D/E 活)
- [x] 命名规则 (§4) 固化, 反约束 grep 防漂移

---

## 6. 后续 (留账)

1. **G3.3 CV-1 signoff 三张** (野马签字依据): `cv-1-3-canvas.spec.ts` 加 page.screenshot 真路径 — 留 CV-1 G3.3 signoff PR
2. **CHN-4 demo 双张** (`g3.4-chn4-{chat,workspace}.png`): CHN-4 e2e #411 已跑但未截屏, 加 page.screenshot 即可
3. **CV-2 demo 四张** (`g3.4-cv2-{anchor-entry,thread-bubble,agent-reply,resolved}.png`): CV-2.3 #404 e2e 已跑 §3.1+§3.2+§3.5+§3.6, 加 page.screenshot 即可
4. **CV-4 demo 三张** (`g3.4-cv4-{running-state,completed-newversion,diff-view}.png`): CV-4.3 #416/#422 已落 iterate-pending, 补 3 态截屏
5. **CHN-2 demo 三张** (`g3.x-dm-*.png`): CHN-2.3 #413 e2e 已跑 5 测试, 加 page.screenshot 即可

后续 audit (Phase 3 退出闸前): 跑全 22 张实际产出 ≥ 17/22 (3 deferred 不计) → G3.4 demo signoff 闸位 ✅.
