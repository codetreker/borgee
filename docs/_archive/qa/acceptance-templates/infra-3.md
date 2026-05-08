# INFRA-3 PROGRESS split — acceptance template v1

> Anchor: `docs/implementation/modules/infra-3-spec.md` v1
> Stance: `docs/qa/infra-3-stance-checklist.md` §1-§2

## §1 主 PROGRESS.md 瘦身锁 (spec §0 立场 ①)

- **§1.1** 总行 ≤100 (CI step `progress-line-budget` `wc -l`)
- **§1.2** 单行 ≤200 字符 (CI step `awk 'length > 200'`)
- **§1.3** 主文件不含 milestone 翻牌 checkbox (CI step `^- \[ \]|^- \[x\]` count==0)

## §2 5 子文件就位 (spec §0 立场 ②)

- **§2.1** `progress/phase-{0,1,2,3,4}.md` 5 文件存在 (CI step `for p in 0..4; test -f`)
- **§2.2** 主 PROGRESS.md ≥5 跳转锚 `progress/phase-[0-4]\.md` (CI step `grep -cE`)
- **§2.3** 子文件 ≥1 翻牌 checkbox (本 PR 迁移既有 50+ checkbox 入子文件; 反向断言主翻牌点不再是主 PROGRESS.md)
- **§2.4** Phase 4+ 历史 changelog 归档入 `progress/phase-4.md` §更新日志归档 (用户拍板"不分 changelog/")

## §3 既有外锚不破 (兼容性反查)

- **§3.1** `phase-2-exit-announcement.md` 仍可引用 (Phase 2 closure 锚)
- **§3.2** `acceptance-templates/<milestone>.md` 引用不破
- **§3.3** `signoffs/g*-yema-signoff.md` 引用不破

## §4 .gitattributes merge=union 扩 (spec §0 立场 ④)

- **§4.1** 5 子文件加 `merge=union` (跟 regression-registry / PROGRESS.md 既有 2 行同精神扩 5 行)

## §5 CI 守门链第 5 处 (spec §0 立场 ⑤)

- **§5.1** release-gate.yml 加 step `progress-line-budget` (5 sub-check: 行长 / 总行 / 子文件 / checkbox / 跳转锚)
- **§5.2** 跟 BPP-4 AST scan / HB-3 dict-isolation / AP-4-enum / HB-4 audit-schema CI 守门链同模式

## REG (本 PR closure 翻 🟢)

| Reg ID | Source | Test path / grep | Owner | Status |
|---|---|---|---|---|
| REG-INFRA3-001 | infra-3-spec §0 立场 ① — 主 PROGRESS.md ≤100 行 + 行 ≤200 字符 | release-gate.yml::progress-line-budget step ① + ② sub-check | 战马C / 飞马 | 🟢 active |
| REG-INFRA3-002 | infra-3-spec §0 立场 ② — 5 子文件就位 + 跳转锚 ≥5 | release-gate.yml::progress-line-budget step ③ + ⑤ sub-check | 战马C / 飞马 | 🟢 active |
| REG-INFRA3-003 | infra-3-spec §0 立场 ③ — 主 PROGRESS.md checkbox 0 hit (翻牌点单源在子文件) | release-gate.yml::progress-line-budget step ④ sub-check | 战马C / 烈马 | 🟢 active |
| REG-INFRA3-004 | infra-3-spec §0 立场 ④ — .gitattributes merge=union 扩 5 子文件 | `git check-attr merge docs/implementation/progress/phase-{0,1,2,3,4}.md` 全 union | 战马C / 飞马 | 🟢 active |
| REG-INFRA3-005 | infra-3-spec §0 立场 ⑤ — CI 守门链第 5 处 progress-line-budget step 真挂 | release-gate.yml grep 'name: progress-line-budget' ≥1 hit | 战马C / 飞马 | 🟢 active |

## 退出条件

- §1-§5 全 PASS (CI step `progress-line-budget` 全绿 + 5 子文件就位 + 既有外锚不破)
- REG-INFRA3-001..005 全 🟢 active (本 PR closure 翻牌)
