# DEFERRED-UNWIND spec brief — fixme/skip 5+ 项 unwind (≤80 行)

> 飞马 · 2026-05-01 · post-Phase 4+ closure deferred 大扫除 (5+ ⏸️/skip/fixme 票一波兑现)
> **关联**: cv-4 ⏸️ + g2.4-adm-0 ⏸️ + cv-3-3-deferred + g2.4-demo-screenshots + AL-2 wrapper 4.2 + HB-4 4.2 deferred
> **命名**: DEFERRED-UNWIND = post-closure deferred 票批量兑现 (跟 NAMING-1 #614 / REFACTOR-2 #613 一次做干净铁律承袭)

> ⚠️ Server / client / e2e / screenshots mixed milestone — **0 schema 改 / 0 endpoint URL 改** + 一波清干净 deferred 票.
> 用户铁律 "一次做干净不留尾" — 后 Phase 4+ closure 残留 deferred 票批量兑现.

## 0. 关键约束 (3 条立场)

1. **deferred 票一次兑现, 不挑 top-N** (用户 2026-04-30 铁律): 5+ ⏸️/skip/fixme 票全清:
   - **D1 cv-4 ⏸️ deferred** — CV-4 v2 iteration retry server queue / iteration-level comment
   - **D2 g2.4-adm-0 ⏸️ deferred** — ADM-0 §1.3 admin path 立场 demo signoff yema 留账票
   - **D3 cv-3-3-deferred** — CV-3.3 deferred test reverse-grep / e2e 漏件
   - **D4 g2.4-demo-screenshots** — Phase 2 g2.4 demo 截屏单 yema 签字 deferred (留 release 前补 — 但本 PR 也清)
   - **D5 AL-2 wrapper 4.2** — release gate 4.2 demo 3 截屏 (5-state UI / error→online / busy/idle BPP frame)
   - **D6 HB-4 4.2** — release gate 4.2 demo 3 截屏 (五支柱状态页 / 情境授权弹窗 / 撤销后行为)
   反约束: 反向 grep `⏸️|deferred|fixme|TODO` in regression-registry.md / acceptance-templates / progress phase-4.md count==0 (post-PR merge).

2. **0 schema / 0 endpoint URL / 0 routes.go 改 + 0 production 行为改**: deferred unwind 是验证 + 截屏 + e2e 补件性质, 不动 schema/endpoint/routes. 反约束: 0 migration v 号 + git diff `internal/api/` `internal/migrations/` `server.go` 0 行 production 改.

3. **截屏走 yema 签字 + e2e 真过 PR body 示输出 + REG ⏸️→🟢 真翻**: 9+ 截屏 (D4-D6 各 3+) yema 真签 (反 zhanma 自签 yema 路径), e2e PR body 示 PASS 输出, REG-* ⏸️ 真翻 🟢. 反约束: yema PR body comment 必示 sign-off 字面.

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 范围 |
|---|---|
| **DU.1 D1-D3 e2e + verify** | D1 cv-4 deferred test 真补 (~3 case) / D2 ADM-0 §1.3 admin path stance signoff yema verify / D3 cv-3-3 deferred reverse-grep test 真补; ~50 行 e2e |
| **DU.2 D4-D6 demo 截屏 (yema 签字)** | g2.4-demo-screenshots / AL-2 wrapper 4.2 (5-state UI / error→online / busy/idle BPP) / HB-4 4.2 (五支柱状态页 / 情境授权弹窗 / 撤销后行为) — 9 截屏 docs/qa/screenshots/{g2.4-*,al-2-wrapper-*,hb-4-*}.png + yema PR comment 真签 |
| **DU.3 closure** | REG-CV4-* / REG-G2-* / REG-AL2W-* / REG-HB4-* 全 ⏸️→🟢 + 6 反向 grep + 0 production 改 + post-#619 haystack 三轨过 + 4 件套 spec 第一件 |

## 2. 反向 grep 锚 (6 反约束)

```bash
# 1) deferred 票全清 (反向断言 ⏸️ 0 hit, 除 v2+ 永久留账行)
grep -cE '⏸️ deferred|⏸️ DEFERRED' docs/qa/regression-registry.md docs/qa/acceptance-templates/  # ≤2 hit (允许 v2+ 永久留, 本 milestone target ≤ baseline-5)

# 2) 9 截屏真补
ls docs/qa/screenshots/{g2.4-demo-,al-2-wrapper-,hb-4-}*.png 2>/dev/null | wc -l  # ≥9

# 3) e2e 真过 (cv-4 + cv-3-3 + ADM-0 stance verify)
grep -lE 'cv-4-.*deferred|cv-3-3-.*deferred|adm-0-stance' packages/e2e/tests/*.spec.ts  | wc -l  # ≥3 hit

# 4) 0 production 改
git diff origin/main -- packages/server-go/internal/ packages/client/src/ | grep -cE '^\+|^-' | head -1  # ≤10 hit (允许 import 微调)
git diff origin/main -- packages/server-go/internal/migrations/ | grep -cE '^\+\s*Version:'  # 0 hit

# 5) yema PR comment sign-off 真签 (反 zhanma 自签 yema 路径)
gh pr view <N> --comments | grep -iE 'yema.*sign.?off|野马.*签字'  # ≥3 hit (D4-D6 各 1)

# 6) post-#619 haystack gate + 既有 test
THRESHOLD_FUNC=50 THRESHOLD_PACKAGE=70 THRESHOLD_TOTAL=85 BUILD_TAGS="sqlite_fts5" go run ./scripts/lib/coverage/  # ALL ≥阈值
go test -tags 'sqlite_fts5' -timeout=300s ./... && pnpm vitest run && pnpm exec playwright test -g 'cv-4|cv-3-3|adm-0'  # ALL PASS
```

## 3. 不在范围 (留账)

- ❌ **真 v2+ 永久留账** (Tauri 桌面壳 / NATS / PG / 对象存储) — 蓝图明示 v2+ 不动
- ❌ **新 milestone scope** — 本 PR 仅 deferred 兑现, 不引新功能

## 4. 跨 milestone byte-identical 锁

- 跨 milestone 既有 production code byte-identical 不破
- yema PR sign-off 模式 (跟 G3.audit / phase-3-exit-announcement signoff 同模式承袭)
- RT-3 #616 截屏 + e2e 模式承袭
- AL-2 wrapper / HB-4 release-gate ≥10 硬条件 byte-identical 不破
