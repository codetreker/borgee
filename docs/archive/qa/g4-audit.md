# G4.audit closure — Phase 4+ 退出闸总签收 (≤80 行)

> 飞马 · 2026-05-01 · Phase 4+ 退出闸 audit 总签收 (跟 G3.audit fill v1 #570 同模式 byte-identical 结构)
> **关联**: phase-4-exit-gate.md (本 PR 同时立) · execution-plan.md §Phase 4+ 退出 gate · phase-3-exit-announcement.md (Phase 3 闭环模板) · regression-registry.md REG-* 全 🟢 数学对账
> **命名**: G4.audit fill v1 = Phase 4+ 滚动 audit 收口文档, 跟 G3.audit / G2.audit / G1.audit 同等级

> ⚠️ 元 milestone (跟 G3.audit fill #570 / INFRA-3 #594 同模式) — **0 server / 0 schema / 0 endpoint** 纯 docs.
> 真值定盘 — Phase 4+ ~50 milestone merged 数据真值映射, 不收缩 + 不漂.

## 0. 关键约束 (3 条立场)

1. **G3.audit 模板 byte-identical 结构 (7 段齐承袭)**: §1 Phase 4+ 实施 milestone PR# 锚表 (~50 milestone 含 PR# + acceptance 翻 + REG 翻 🟢 count) + §2 立场反查跨 milestone byte-identical 不漂证据 (跨链 ≥10 处) + §3 acceptance 闭表 + §4 G4.* 5+1 闸状态总览 + §5 evidence 锚 (signoffs / screenshots / closure-entry) + §6 4 角色三联签 (架构 飞马 + QA 烈马 + PM 野马 + dev 战马, 团队 lead 终签) + §7 v0 → v1 切换 checklist 兑现 (execution-plan §v0 代码债 audit 表 全 milestone 行更新).

2. **Phase 4+ ~50 milestone 真值校准 + REG 数学对账 + 跨链锚 真承袭**: ~50 milestone PR# 锚 byte-identical 跟 phase-4.md 详细段既有数据; REG-* 翻 🟢 count 数学对账 (跟 regression-registry.md count 字面真测); 跨链锚承袭 (G3.audit 5 链 + Phase 4 加 N 链: AL-1a reason 字典锁链 13 处 / audit-forward-only 链 ≥18 处 / owner-only ACL 链 24 处 / release-gate.yml CI 守门链 6 处 / DL-1 4 interface byte-identical 跨 RT-3+DL-2+DL-3+HB-2 v0(D) / mustPersistKinds 4 类 / typing-indicator 双语 9 同义词 / capability 14 const 跨 server-client / 4 source enum SSOT (audit_events/channel_events/global_events/install_butler_audit) / 推断 scope 命中 AP-2+ADM-3 / refactor SSOT helper × 8 (REFACTOR-1+2)).

3. **0 server / 0 schema + phase-4.md 概览段单源化清理 (反铁律守 元 milestone 合法)**: 跟 G3.audit fill #570 / progress-split-spec §6.3 / TEST-FIX-1 #596 嵌入 INFRA-3 follow-up 同模式承袭. PR title: `chore(g4-audit): G4.audit closure + Phase 4+ exit-gate 总签收 + phase-4.md 概览段单源化 (一 milestone 一 PR)`. 反约束: 0 production code / 0 schema / 0 endpoint / 0 acceptance template 改 (本 PR 仅 docs/qa/g4-audit.md 新 + docs/qa/phase-4-exit-gate.md 新 + phase-4.md 概览段 4 行 [ ]→[x] 单源化 + 详细段 byte-identical 不动).

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| **G4A.1** g4-audit.md v1 fill (~250 行) | `docs/qa/g4-audit.md` 新 — 7 段齐 (§1 ~50 milestone PR# 锚表 + §2 跨链 byte-identical 真承袭证据 + §3 acceptance 闭表 + §4 G4.* 5+1 闸状态 + §5 evidence 锚 + §6 4 角色三联签占位 + §7 v0 → v1 切换 checklist 全 milestone 行更新) | 战马 / 飞马 v1 fill |
| **G4A.2** phase-4-exit-gate.md + REG 数学对账 verify | `docs/qa/phase-4-exit-gate.md` 新 (跟 phase-3-exit-announcement.md 同模式 byte-identical, 5+1 闸状态翻 ✅ where 真闭, ⚠️ where 留账); `regression-registry.md` REG-* 翻 🟢 count 真值校准 (跟既有 ~250+ REG 行 active count 累计真测); phase-4.md 概览段 4 行 [ ]→[x] 单源化 (line 51/81/83/85 跟详细段 byte-identical 同步) | 战马 / 飞马 review |
| **G4A.3** closure | REG-G4A-001..006 (6 反向 grep + g4-audit.md 7 段齐 + phase-4-exit-gate.md 5+1 闸 + 跨链反向 grep ≥1 hit per 链 + REG 数学对账真验证 + phase-4.md 概览段 [ ] 0 hit + v0 → v1 checklist 全行更新 + 嵌入 PR title 元 milestone 合法) + acceptance + content-lock 不需 (元 milestone) + 4 件套 spec 第一件 | 战马 / 烈马 |

## 2. 反向 grep 锚 (6 反约束, count==0)

```bash
# 1) g4-audit.md 7 段齐 (跟 g3-audit.md 模板 byte-identical 结构)
grep -cE '^## [1-7]\. ' docs/qa/g4-audit.md  # ≥7 hit

# 2) phase-4.md 概览段 [ ] 0 hit (单源化)
grep -cE '^- \[ \] \*\*(HB-2|RT-3|AP-2|ADM-3|DL-2|DL-3|CS-1|CS-2|CS-3)' docs/implementation/progress/phase-4.md  # 0 hit (全 [x] 跟详细段 byte-identical)

# 3) ~50 milestone PR# 锚 真值映射 (跟 phase-4.md 详细段既有数据 byte-identical)
grep -cE '#5[0-9]{2}|#6[0-9]{2}' docs/qa/g4-audit.md  # ≥50 hit (Phase 4+ ~50 PR# 锚)

# 4) 跨链 byte-identical 真承袭 (G3.audit 5 链 + Phase 4+ 加 N 链)
for chain in 'kindBadge' 'CONFLICT_TOAST' 'agent.*fanout' 'rollback owner-only' '5-frame envelope' 'AL-1a reason' 'audit-forward-only' 'owner-only ACL' 'release-gate.yml CI 守门' 'DL-1.*interface' 'mustPersistKinds' 'typing.*9 同义词' 'capability.*14 const' '4 source enum' 'refactor.*SSOT.*helper'; do
  grep -cE "$chain" docs/qa/g4-audit.md  # ≥1 hit per chain
done

# 5) phase-4-exit-gate.md 5+1 闸状态 (跟 phase-3-exit-announcement byte-identical 模式)
grep -cE 'G4\.[12345]|G4\.audit' docs/qa/phase-4-exit-gate.md  # ≥6 hit (5+1 闸)

# 6) v0 → v1 checklist 全 milestone 行更新 (execution-plan §v0 代码债 audit 表)
grep -cE '\| (Phase 4|Phase 4\+) /' docs/implementation/00-foundation/execution-plan.md  # ≥10 hit (Phase 4+ 各 milestone v0 行)
```

## 3. 不在范围 (留账)

- ❌ **Phase 4 退出公告** (跟 phase-3-exit-announcement / phase-2-exit-announcement 同模式) — 元 milestone 单独 PR (野马起草 + 飞马 + 烈马 + 团队 lead 三签), 待 G4.* 5+1 闸全 SIGNED 后启
- ❌ **Phase 5 entry 准备** (跟 PHASE-4-ENTRY-CHECKLIST 同模式) — 留 Phase 4 closure 后单独 doc; 用户拍板后再起 (蓝图当前无 Phase 5 字面定义)
- ❌ **跨 phase 留账接力** — §7 锚但不展开 (留 Phase 5+ 真启再展开)
- ❌ **HB stack v0(D) → v1 升级** (cgroupsv2 / outbound proxy / plugin signing rotation) — 全留 v2+
- ❌ **EventBus 切 NATS / SQLite 切 PG / Storage 对象存储** (蓝图 §4.C 必重写 3 条) — DL-3 阈值哨触发后人工决策, 留 v2+
- ❌ **client-shape.md `Tauri 桌面壳` 蓝图改** (HB stack Go 重审已对齐, 蓝图改是元 milestone 走野马 + 飞马联签) — 留 follow-up

## 4. 跨 milestone byte-identical 锁

- 复用 G3.audit fill #570 模板 byte-identical 结构 (7 段齐, 飞马 v1 fill 同精神)
- 复用 phase-3-exit-announcement.md 5+1 闸状态翻牌模式 (Phase 4+ 5+1 闸真承袭)
- 复用 INFRA-3 #594 / INFRA-4 #602 单源立场 (phase-4.md 概览段单源化)
- 复用 regression-registry.md merge=union pattern (#560 跨 milestone REG 累计)
- 复用 execution-plan §v0 代码债 audit 表 (每 Phase 退出更新, Phase 4+ 行全更)
- 元 milestone 嵌入合法 (跟 G3.audit fill #570 / progress-split-spec §6.3 / TEST-FIX-1 #596 同模式)

## 5. 派活 + 双签流程

派 **zhanma-c** (DL-2/DL-3/ADM-3 audit 域续作熟手, 跟 G3.audit fill 模板承袭) + 飞马 v1 fill 协作.

**4 角色三联签流程** (跟 G3.audit / Phase 3 closure 同模式承袭):
1. spec brief → team-lead → 飞马自审签 ✅ (本 message 同时表态)
2. team-lead 派 zhanma-c 起 PR 实施 (G4A.1 fill + G4A.2 phase-4-exit-gate + 概览段单源化 + G4A.3 closure 三段一 PR)
3. 飞马 architect review LGTM
4. 烈马 QA acceptance review LGTM
5. 野马 PM stance review LGTM (反产品立场稀释)
6. 4 角色齐 → team-lead 终签 + merge

## 6. 飞马 (架构师) 自审表态

✅ **APPROVED with 0 必修条件** — G3.audit fill #570 模板 byte-identical 续作, 0 风险.

担忧 (1 项, 轻度):
- 🟡 ~50 milestone PR# 锚真值校准是 judgment work (跟 INFRA-3 PROGRESS 拆分 byte-identical 迁移立场承袭) — 战马 v1 fill 时需逐条 grep `git log --oneline | grep '<milestone>'` verify, 不收缩不漂. 我可在 architect review 时辅助校对.

留账接受度全 ✅: Phase 4 退出公告 / Phase 5 entry / 跨 phase 留账接力 / HB stack v1 升级 / EventBus/SQLite/Storage 切 / Tauri 蓝图改 — 全留账 byte-identical 跟 G3.audit fill #570 同精神.

**ROI 拍**: G4.audit fill v1 ⭐⭐⭐ **最高 ROI** 优先 (Phase 4+ 退出闸真闭最后一关, 阻塞 Phase 4 退出公告 + 后续 phase entry; 跟 G3.audit fill #570 同模式真值已就绪 + Phase 4 真完 9 行概览 stale 单源化机会).

## 7. 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-05-01 | 飞马 | v0 spec brief — G4.audit closure (元 milestone, 跟 G3.audit fill #570 同模式 byte-identical 结构). 3 立场 + 3 段拆 + 6 反向 grep + 嵌入 PR title 元 milestone 合法. **0 server / 0 schema / 0 endpoint** 元 milestone. 含 phase-4.md 概览段 4 行 [ ]→[x] 单源化 (跟 INFRA-3/4 立场承袭). 不在范围: Phase 4 退出公告 / Phase 5 entry / 跨 phase 接力 / HB stack v1 / EventBus/SQLite/Storage 切 / Tauri 蓝图改 全留账. ROI 拍 ⭐⭐⭐ G4.audit 最高优先 (Phase 4+ 退出闸阻塞链最后一关). zhanma-c 主战续作 + 飞马 v1 fill 协作 + 4 角色三联签 + team-lead 终签. |
