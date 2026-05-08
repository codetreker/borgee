# INFRA-3 PROGRESS-Split spec brief v1 — 每 Phase 一文件 (≤80 行)

> 飞马 · 2026-04-30 · 用户拍板简化版 (team-lead 转): PROGRESS.md (355 行) → 主 ≤100 行 + `progress/phase-{0,1,2,3,4}.md` 5 子文件 (一 Phase 一文件)
> **关联**: PROGRESS.md 既有 Phase 概览 (line 17-25) + Phase 0..3 闸 detail + Phase 4+ 5 模块组 detail
> **命名**: PROGRESS-Split (元 milestone, 不占 PR# 序列, 嵌入下一战马 milestone PR 落地)

> ⚠️ Wrapper milestone — 0 server / 0 schema / 0 endpoint, 纯 docs refactor.
> 既有锚 (#284 closure / phase-2-exit-announcement.md / acceptance-templates 引用) byte-identical 不破.
> 翻牌走法不变 — 单一进度真相, milestone PR 同步翻 (改的是子文件路径不是机制).

## 0. 关键约束 (3 条立场, 跟用户拍板版 byte-identical)

1. **主 PROGRESS.md 瘦身 ≤100 行 + 跳转链接** (子文件分 Phase): 主 doc 保留 (a) Phase 概览 5 行表 byte-identical 跟 line 17-25 + (b) 当前 in-flight summary ≤10 行 (列 in-flight PR# + 一句话状态) + (c) 子文件跳转表 (`Phase 0 detail → progress/phase-0.md` 5 行) + (d) 更新日志 ≤20 行. 反约束: 主表行 ≤200 字符 (反战马 commit 塞 1500+ 字符到 milestone 行); CI 守 `awk 'length > 200' PROGRESS.md` count==0.

2. **5 子文件每 Phase 一文件 (用户简化拍板, byte-identical 跟拍板)**: 
   - `docs/implementation/progress/phase-0.md` — Phase 0 闸 + INFRA-1a/1b + G0.*
   - `docs/implementation/progress/phase-1.md` — Phase 1 闸 + CM-1/AP-0/CM-3 + G1.*
   - `docs/implementation/progress/phase-2.md` — Phase 2 闸 + 解封前置 + CM-4 + G2.*
   - `docs/implementation/progress/phase-3.md` — Phase 3 闸 + 11 milestone + G3.*
   - `docs/implementation/progress/phase-4.md` — Phase 4+ 全装一文件 (含 AL/BPP/HB/RT/AP/CM/ADM/DL/CS 9 模块组)
   
   反约束: 不裂第二维度子目录 (反"拆得太细"); 不另起 changelog/ 或 modules/ (用户拍板"不分 changelog/modules", 1500 字符 commit detail 留 phase-4.md 里允许长 — 已分 Phase 文件后单文件长度可接受).

3. **翻牌机制 byte-identical 不变 (单一真相, 仅切路径)**: milestone PR 翻牌走 `progress/phase-N.md` 子文件 (`[ ]` → `[x]` + PR# 锚 + closure summary), 跟既有翻牌路径 line-by-line 同精神; 概览表 (主 PROGRESS.md) 同步翻 ✅ DONE / 🔄 IN PROGRESS (跟 #590 yema 翻牌路径承袭). 反约束: 反向 grep `\[ \]|\[x\]` 在 `docs/implementation/progress/` ≥1 hit (子文件是真翻牌点); 在 `docs/implementation/changelog/` 0 hit (本 spec 不创 changelog/, 留账).

## 1. 拆段实施 (3 段, 一 milestone 一 PR — 嵌入下一战马 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| **PS.1** 主 PROGRESS 瘦身 + 5 子文件拆 | `docs/implementation/PROGRESS.md` 改 (≤100 行, Phase 概览 + in-flight + 跳转表 + 更新日志) + 5 子文件新 (`progress/phase-{0,1,2,3,4}.md`); 既有内容 byte-identical 迁移 (不改字面, 仅切文件); 跳转锚 `[Phase 0 detail](progress/phase-0.md)` 5 行 byte-identical | 战马 (主) / 飞马 review |
| **PS.2** `.gitattributes` merge=union 5 子文件加 + CI 单行 budget 守门 | `.gitattributes` 5 子文件加 `merge=union` (跟 regression-registry #560 同精神, 解并发翻牌 conflict); `.github/workflows/release-gate.yml` 加 step `progress-line-budget` 反向 grep `awk 'length > 200' docs/implementation/PROGRESS.md` count==0 + `wc -l docs/implementation/PROGRESS.md ≤ 100` 双锁 (跟 BPP-4 AST scan / HB-3 dict-isolation / AP-4-enum #591 / HB-4 audit-schema CI 守门链**第 5 处**) | 战马 / 飞马 review |
| **PS.3** closure | REG-PS-001..005 (5 反向 grep + 单行 budget + 子文件存在 + 既有外锚不破 + 翻牌锚 ≥1) + acceptance + content-lock 不需 (纯 docs 拆) + 4 件套 spec 第一件; PROGRESS 子文件不需 docs/current sync (是 PROGRESS 本身) | 战马 / 烈马 |

## 2. 反向 grep 锚 (5 反约束, count==0)

```bash
# 1) 主 PROGRESS.md 单行 ≤200 字符 (反战马塞 commit 历史)
awk '{ if (length > 200) print NR":"length }' docs/implementation/PROGRESS.md  # 0 行

# 2) 主 PROGRESS.md 总行 ≤100 (瘦身真兑现)
[ $(wc -l < docs/implementation/PROGRESS.md) -le 100 ] || exit 1

# 3) 5 子文件存在 + 跳转锚 byte-identical
for p in 0 1 2 3 4; do test -f docs/implementation/progress/phase-$p.md || exit 1; done
git grep -nE 'progress/phase-[0-4]\.md' docs/implementation/PROGRESS.md  # ≥5 hit (跳转表)

# 4) 既有外锚 (#284 closure / phase-2-exit-announcement / acceptance-templates) 不破
git grep -nE 'phase-2-exit-announcement|acceptance-templates|signoffs/' docs/  # 仍 ≥1 hit per anchor

# 5) 翻牌锚 真在子文件不再在主 PROGRESS (单源)
git grep -cE '^- \[ \]|^- \[x\]' docs/implementation/PROGRESS.md  # 0 hit (主仅概览表 + 跳转, 不放 checkbox)
git grep -cE '^- \[ \]|^- \[x\]' docs/implementation/progress/phase-*.md  # ≥50 hit (真翻牌点)
```

## 3. 不在范围 (留账)

- ❌ `changelog/<milestone>.md` 外迁长 commit 历史 (用户拍板"不分 changelog", 1500 字符留 phase-4.md 里允许长) — 留 v3+ 若 phase-4.md 真过 ≥1500 行再起
- ❌ 翻牌机制改 (milestone PR 同步翻子文件不变, 单一真相)
- ❌ regression-registry 拆 (已 merge=union 解, v3+)
- ❌ acceptance-templates / phase-N-readiness-review 改 (已分 / 历史快照不动)
- ❌ 模块组目录 (progress/by-module/) — 用户拍板拒绝, 维持 by-Phase

## 4. 跨 milestone byte-identical 锁

- 复用 `.gitattributes merge=union` 模式 (子文件 5 个全加, 跟 regression-registry #560 同精神, 解并发翻牌 conflict)
- 复用 release-gate.yml CI 守门链 (跟 BPP-4 / HB-3 / AP-4-enum #591 / HB-4 同模式) — `progress-line-budget` step **第 5 处链**
- **0-server-no-schema 第 15 处 Wrapper milestone 候选** (跟 CV-15 #592 / AP-4-enum #591 / CHN-11..12 / DM-9 系列同源)
- 跳转锚 `[Phase 0 detail](progress/phase-0.md)` 风格跟 `acceptance-templates/cv-1.md` / `signoffs/g3.4-chn4-liema-signoff.md` 子目录引用同模式

## 5. 验收挂钩

- REG-PS-001..005 (5 反向 grep + CI step `progress-line-budget` 双锁: 单行 ≤200 + 总行 ≤100)
- 既有外锚不破 (gh issue / PR # 引用 PROGRESS.md anchored line 全 verify)
- merge gate verify — 战马并发翻牌 (3 战马同时 merge milestone PR) 不撞 conflict (merge=union 真兑现)

## 6. 派活 + 嵌入路径

**派 zhanma-c** (跟 PR #560 `.gitattributes merge=union` / PR #574 naming-map 同 docs refactor 风格同源; 非 zhanma-e 因 RT-3 #588 in-flight). 嵌入合法路径 (反开独立 docs PR 铁律):
1. 战马起下一 milestone worktree 时 cherry-pick `progress-split-spec.md` 进 `docs/implementation/modules/`
2. PS.1+PS.2+PS.3 三段同 PR 整闭 (跟 CV-15 #592 / HB-1 #589 一 milestone 一 PR 同模式)
3. PR title 双义: `feat(<milestone>): ... + chore(progress-split)` — 元 milestone 不占 PR# 序列, 嵌入合法
4. 翻牌: PROGRESS-split 自身 [x] 进 `progress/phase-4.md` (本 spec 是 Phase 4+ 元 milestone)
