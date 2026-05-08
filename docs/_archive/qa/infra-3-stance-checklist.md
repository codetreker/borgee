# INFRA-3 PROGRESS split — PM stance checklist v1

> Anchor: `docs/implementation/modules/infra-3-spec.md` v1 §0-§5
> Mode: 真 infra refactor milestone — 0 server / 0 schema / 0 endpoint, 但含 release-gate.yml CI step + 5 子文件实际拆分 + .gitattributes merge=union 扩, 不是 docs-only.

## 1. 立场 (5 项)

1. **主 PROGRESS.md 瘦身 ≤100 行 + 行 ≤200 字符** — 战马 commit detail 长 1500+ 字符不入主文件; CI step `progress-line-budget` 双锁 (awk length>200 + wc -l ≤100). 反向: 长 milestone detail → `progress/phase-N.md` 子文件.
2. **5 子文件每 Phase 一文件 (用户拍板 byte-identical)** — `progress/phase-{0,1,2,3,4}.md` 5 子文件; 不裂第二维度子目录 (反 by-module/changelog/ 拆分); phase-4.md 装 Phase 4+ 9 模块组 + 历史 changelog 归档.
3. **翻牌机制 byte-identical 不变 (单一真相, 仅切路径)** — milestone PR 翻牌走 `progress/phase-N.md` 子文件 `[ ]→[x]+PR#`; 概览表 (主 PROGRESS.md) 同步翻 ✅/🔄. 反向: 主 PROGRESS.md `^- \[ \]|^- \[x\]` 0 hit (CI 守); 子文件 ≥1 hit (真翻牌点).
4. **`.gitattributes merge=union` 扩 5 子文件** — 跟 regression-registry #560 + PROGRESS.md 既有同精神, 解并发翻牌 conflict; 战马同期翻 phase-4.md 不撞.
5. **CI 守门链第 5 处 `progress-line-budget`** — 跟 BPP-4 AST scan / HB-3 dict-isolation / AP-4-enum / HB-4 audit-schema 同模式; release-gate.yml 加 step 自动校验, 不依赖人工.

## 2. 黑名单 grep (反向断言)

| Pattern | Where | 立场 |
|---|---|---|
| `^- \[ \]\|^- \[x\]` 在 主 PROGRESS.md | `docs/implementation/PROGRESS.md` | §1 ③ 翻牌点单源在子文件 |
| 单行 > 200 chars 在 主 PROGRESS.md | 同上 | §1 ① 行长度限 |
| 总行 > 100 在 主 PROGRESS.md | 同上 | §1 ① 主文件瘦身 |
| `progress/phase-[0-4]\.md` 跳转锚 | 主 PROGRESS.md | §1 ② 5 跳转锚必齐 |
| `changelog/<milestone>.md` 目录 | 任意 | §1 ② 不分 changelog |

## 3. 不在范围 (留账, v3+)

- ❌ `changelog/<milestone>.md` 外迁长 commit 历史 (用户拍板"不分 changelog")
- ❌ 翻牌机制改 (单一真相, milestone PR 同步翻子文件)
- ❌ regression-registry 拆 (已 merge=union 解, v3+)
- ❌ acceptance-templates / phase-N-readiness-review 改 (已分 / 历史快照不动)
- ❌ 模块组目录 (progress/by-module/) — 用户拒绝, 维持 by-Phase

## 4. 验收挂钩

- §1 ① + ② → REG-INFRA3-001 / 002 (CI step `progress-line-budget` 5 sub-check 全 PASS)
- §1 ③ → REG-INFRA3-003 (主 PROGRESS.md checkbox 0 hit + 子文件 ≥1 hit)
- §1 ④ → REG-INFRA3-004 (.gitattributes 5 子文件加 merge=union)
- §1 ⑤ → REG-INFRA3-005 (release-gate.yml progress-line-budget step 真挂, CI 守门链第 5 处)

## 5. 既有外锚不破 (兼容性反查)

- `phase-2-exit-announcement.md` 仍引用 (Phase 2 closure 锚)
- `acceptance-templates/<milestone>.md` 仍引用 (PR # 引用 PROGRESS.md anchored line)
- `signoffs/g3.*-yema-signoff.md` 仍引用 (G3 闸签字)
- gh issue / PR # 引用 PROGRESS.md anchored line: 用户应在 PR review 时 verify 既有 `#L24-25` 风格 anchor 不破 (新 layout 行号变 → anchor stale, 但 PR# 描述不依赖行号)
