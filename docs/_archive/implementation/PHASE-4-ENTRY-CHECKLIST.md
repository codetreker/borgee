# Phase 4 Entry Checklist (飞马, Phase 3 closure 后接力)

> 飞马 · 2026-04-29 · ≤200 行 · Phase 3 退出 gate 全签 (#451 G3 closure 5 闸 ✅ 无软留账) 接 Phase 4 entry
> 关联: `docs/qa/signoffs/g3-exit-gate.md` (#451 飞马联签) / `docs/implementation/00-foundation/g3-audit.md` (#448 飞马 v1 fill 6 audit row 触发条件) / `docs/qa/phase-3-readiness-review.md` (#390 飞马 v2 patch IN-FLIGHT → SIGNED)
> 蓝图锚: execution-plan.md §Phase 4+ 剩余模块 + 依赖锁紧表 (DL-4 / BPP-1→AL-2→BPP-3 / CM-5 / HB-1 / CS-3) + G4.audit 滚动闸

---

## 1. Phase 4 8 项接力路径 entry 状态表

| # | milestone | entry 状态 | 责任战马 | 期望 PR (spec / impl / acceptance) | 蓝图锚 |
|---|---|---|---|---|---|
| 1 | **AL-4** runtime registry | ⏳ spec ✅ #379 v2 / impl 待派 | 战马待派 | spec ✅ #379 / impl AL-4.1 schema v=16 / AL-4.2 server / AL-4.3 SPA | agent-lifecycle.md §2.2 + Borgee 不带 runtime 立场 #7 |
| 2 | **AL-2a** agent_configs SSOT | 🔄 IN-FLIGHT — schema #447 (zhanma-a, 飞马 LGTM ✅) / server PATCH 排队 | zhanma-a | schema ✅ #447 (LGTM) / AL-2a.2 server REST PATCH (排队) | plugin-protocol.md §1.4 SSOT + agent-lifecycle.md §2.1 |
| 3 | **AL-2b** BPP frame agent_config_update | ⏳ acceptance #452 in-flight / impl 待 BPP-3 同合 | (待派) | acceptance #452 (LGTM 待) / impl 跟 BPP-3 串行 | plugin-protocol.md §1.5 热更新 frame 留 AL-2b |
| 4 | **AL-1b** agent 故障态 | 🔄 IN-FLIGHT — schema PR #453 (zhanma-c) | zhanma-c | impl AL-1b.1 schema v=21 / AL-1b.2 server / AL-1b.3 SPA | agent-lifecycle.md §2.3 故障态 + AL-1a #249 6 reason 枚举承袭 |
| 5 | **ADM-1** admin SPA UI | 🔄 IN-FLIGHT — acceptance #262 (zhanma-d Phase 4 已开) | zhanma-d | acceptance #262 / impl ADM-1.1/1.2/1.3 | (Phase 2 G2.4 #6 留账接力) |
| 6 | **BPP-2** plugin 协议 v2 | ⏳ spec drafting (zhanma-e) | zhanma-e | spec brief / acceptance / impl BPP-2.1+/2.2+/2.3+ | plugin-protocol.md §2 v2 协议升级 |
| 7 | **AP-1** access perms 严格 403 | ⏳ Phase 4 中段 | 待派 | impl AP-1 milestone (CHN-1 REG-CHN1-007 ⏸️→🟢 trigger flip) | g3-audit.md A5 + GitHub 私有 repo 同模式 |
| 8 | **ADM-2** admin god-mode 元数据扩 | ⏳ Phase 4 后段 (依赖 ADM-1) | 待派 | impl ADM-2 + AL-3.3 god-mode (REG-AL3-011 ⚪→🟢 trigger flip) | g3-audit.md A4 + ADM-0 §1.3 红线 |

---

## 2. Phase 4 后段 + Phase 5+/6+ 留账 (跨 phase 接力, g3-audit.md §3 同源)

| 项 | Phase | 触发条件 | 锚 |
|---|---|---|---|
| **CHN-3 lazy 90d GC cron** | Phase 4+ cron job milestone | 作者删 group + 90 天后 cron 跑 | g3-audit.md A1 (SQL 字面齐) |
| **AL-4 hermes plugin** | Phase 5+ | BPP-2 plugin 协议 v2 落地 + Hermes runtime 真接通 | g3-audit.md A2 |
| **CV-4 iterate retry 永不实施** | 反约束 | 永不实施 (#380 ⑦ + #365 ②) | g3-audit.md A3 (永久反约束三连) |
| **第 6 轮 remote-agent 安全** | Phase 6+ | binary 签名 / 沙箱 / 资源限制 / uninstall | g3-audit.md A6 + 蓝图 §4 |
| **DL-4 plugin manifest API** | Phase 4 后段 | HB-1 / CS-3 前置 (飞马 R2 锁排序) | execution-plan.md §Phase 4+ 依赖锁紧 |
| **CM-5 agent↔agent 协作** | Phase 4 后段 | CM-4 已闭 | execution-plan.md §Phase 4+ 依赖锁紧 |
| **HB-1 health/heartbeat** | Phase 4 后段 | DL-4 落地后 | execution-plan.md §Phase 4+ 依赖锁紧 |
| **CS-3 client web push** | Phase 4 后段 | DL-4 落地后 | execution-plan.md §Phase 4+ 依赖锁紧 |

---

## 3. Phase 4 退出 gate 框架 (G4.* 跟 G3 同模式拟)

> v0 提议 — 实施过程中按 milestone 完成节奏 refine; G4.audit 是滚动的 (跟 execution-plan.md §Phase 4+ 退出 gate 字面一致)

| 闸 | 主旨 (拟) | 依赖 milestone | Owner |
|---|---|---|---|
| **G4.1** | runtime registry + 启停 E2E (AL-4 真接通 plugin) | AL-4 + AL-2a + AL-2b 全闭 | 战马 + 烈马 |
| **G4.2** | admin SPA + god-mode 元数据扩 E2E | ADM-1 + ADM-2 + AL-3.3 god-mode | 战马 + 烈马 |
| **G4.3** | agent 故障态 UX + 4 态 badge demo | AL-1b 全闭 + AL-1a 三态 enum 串接 | 战马 + 野马 |
| **G4.4** | 严格 403 + admin 不入业务路径全 e2e | AP-1 + CHN-1 REG-CHN1-007 flip | 战马 + 烈马 |
| **G4.5** | BPP-2 plugin 协议 v2 envelope CI lint | BPP-2 全闭 + AL-2b BPP frame 同合 | 战马 + 飞马 |
| **G4.audit** (滚动) | Phase 4+ 跨 milestone 代码债 audit (实施时滚动加 audit row) | 全 Phase 4 milestone | 飞马 |

**通过判据 (拟)**: G4.1-G4.5 全 ✅ + G4.audit 滚动闭 + 跨 phase 留账锚 (Phase 5+/6+) 全明示 → Phase 4 closure announcement.

---

## 4. 跨 milestone byte-identical 链锁延续 (Phase 3 → Phase 4 防漂移)

Phase 3 closure #451 §3 锁定 5 链 byte-identical, **Phase 4 实施 PR review 时必跑反向 grep 验**:

1. **kindBadge 二元 🤖↔👤** — Phase 4 新组件 (AL-1b 故障态 / AL-2a config UI / ADM-1 admin SPA) 字面跟 5 源同源, 不裂第 6 源
2. **CONFLICT_TOAST `内容已更新, 请刷新查看`** — Phase 4 新 PATCH/POST endpoints (AL-2a config / AL-1b status / ADM-1 ops) 走相同 409 文案, 不开同义词
3. **fanout 文案 `{agent_name} 更新 {artifact_name} v{n}`** — CV-4 iterate completed 已复用, AL-2a/AL-2b config update 不发新 fanout (走轮询 reload, 立场 §1.5)
4. **rollback owner-only DOM gate** — AL-2a config UI / AL-1b 故障态 manual override / ADM-1 admin actions 全走 owner-only 双闸 (line 254 + line 57 同模式)
5. **5-frame envelope 共序** — Phase 4 新 frame (AL-2b agent_config_update / BPP-2 envelope schema) 必须 type/cursor 头位 byte-identical 跟 5-frame 同模式, 不裂 namespace

---

## 5. 实施 owner 链 (Phase 4 起步)

跟 Phase 3 实施 owner 链 (战马A CV 主线 / 战马C DM 主线 / 战马B Phase 4 ADM-1 提前) 接续:

- **战马A**: AL-4 runtime registry 接力 (Phase 4 入口位)
- **zhanma-a**: AL-2a agent_configs SSOT (in-flight #447 + AL-2a.2 server)
- **zhanma-c**: AL-1b agent 故障态 (in-flight #453)
- **zhanma-d**: ADM-1 admin SPA (in-flight #262 acceptance)
- **zhanma-e**: BPP-2 plugin 协议 v2 (spec drafting)
- **战马B**: ADM-1 已开 #262 acceptance template (Phase 2 G2.4 #6 留账接力承袭)
- **后段待派**: AP-1 / ADM-2 / CHN-3 GC cron / DL-4 / CM-5 / HB-1 / CS-3

---

## 6. 关联 PR / 文件

- **G3 closure**: #451 (本 PR 前置 — Phase 3 退出 gate 5 闸全签)
- **G3.audit**: #448 (飞马 v1 fill, 6 audit row 触发条件)
- **readiness review**: #390 (飞马 v2 patch IN-FLIGHT 状态)
- **G3 evidence bundle**: #442 (战马 evidence path 字面锁)
- **AL-2a schema**: #447 (zhanma-a, 飞马 LGTM ✅)
- **AL-2b acceptance**: #452 (in-flight, 飞马 review 排队)
- **AL-1b schema**: #453 (zhanma-c, 飞马 review 排队)
- **AL-4 spec v2**: #379 (飞马 merged 962fec7)
- **ADM-1 acceptance**: #262 (zhanma-d 已开)
- **BPP-2 spec drafting**: zhanma-e in-flight

---

## 7. 5 链 byte-identical 反向 grep 命令清单 (Phase 4 review 复用)

```bash
# 链 1: kindBadge 二元 5 源
git grep -nE "committer_kind.*===.*'agent'.*\?.*'🤖'.*:.*'👤'" packages/client/src/   # 预期 ≥5 源 (Phase 4 不增源数)

# 链 2: CONFLICT_TOAST 同源
git grep -nF "内容已更新, 请刷新查看" packages/client/src/   # 预期 ≥1 (CV-1.3 line 49 单源, Phase 4 PATCH/POST 复用)

# 链 3: fanout 文案
git grep -nE "%s 更新 %s v%d|fanout.*agent_name.*artifact_name" packages/server-go/internal/api/   # 预期 ≥1 (CV-1 artifacts.go:591 单源)

# 链 4: rollback owner-only DOM gate
git grep -nE "showRollbackBtn = isOwner && !isHead && !editing|isOwner = channel\.created_by === currentUser\.id" packages/client/src/   # 预期 ≥2 双闸

# 链 5: 5-frame envelope 共序 (type/cursor 头位)
git grep -nE "Type\s+string\s+\`json:\"type\"\`" packages/server-go/internal/ws/    # 预期 ≥5 (RT-1 / Anchor / Mention / Iteration / 第 5 个 Phase 4 新 frame 加进来时同源)
```

任一链漂移 (源数变化 / 字面不一致) → CI fail, 跟 G2 #338 cross-grep 反模式同精神.

---

## 8. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 飞马 | v0 — Phase 4 entry checklist (跟 #451 G3 closure §6 8 项接力路径同源 + g3-audit.md §3 6 audit row 触发条件 + Phase 5+/6+ 留账接力锚字面延续). 8 milestone entry 状态表 (3 IN-FLIGHT + 5 ⏳) + Phase 4+ 后段 8 项跨 phase 留账 + G4.* 退出 gate 框架 5 闸 (拟) + 5 跨 milestone byte-identical 链锁延续 + 实施 owner 链 + 5 链反向 grep 命令清单. 跟 execution-plan.md §Phase 4+ 字面对齐 (DL-4 → HB-1/CS-3 排序锁 / BPP-1→AL-2→BPP-3 串行 / CM-5 依赖 CM-4 / G4.audit 滚动). |
