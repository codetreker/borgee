# Blueprint / Implementation / docs/current 审计轮换

> 飞马 · 2026-04-28 · 防止 R3 重排后文档与代码再脱节
> 实例: #212 审计一次性抓出 PROGRESS.md 落后 9 行 + migrations.md §7 4 个 v 缺 → 说明没有固定节律就会脱节。

## 1. 三层审计节律

| 触发时机 | 谁来做 | 看什么 | 命令 / grep |
|---|---|---|---|
| **每周一 (固定)** | 飞马 | PROGRESS.md vs `gh pr list --state merged --search "merged:>=$(date -d '7 days ago' +%F)"` | diff 已 merged 的 PR # 是否在 PROGRESS log 出现 |
| **每 milestone ✅** | 该 milestone owner (战马/野马) | docs/current/server/{migrations.md, data-model.md, http-api.md} | `grep -n 'v=[0-9]' registry.go` 对照 migrations.md §7 行数; `git diff main -- internal/store/models.go` 对照 data-model.md |
| **每 Phase 退出关卡** | 飞马 + 烈马 | 三层全量对账 (blueprint § ↔ implementation/modules/*.md ↔ docs/current/) | 落到 `docs/implementation/00-foundation/phase-N-N+1-transition-audit.md` (#212 模板) |
| **每 PR merge 前** | 评审人 (飞马) | PR body `## Current 同步` 段是否填 (规则 6 lint) | CI lint 已盯, 评审人复核非 N/A 时点开 docs/current/ diff |

## 2. 审计核对项 (Phase 退出关卡用)

- [ ] PROGRESS.md 概览行 ↔ 实际 merged milestone PR # (per Phase)
- [ ] blueprint §X.Y ↔ implementation/modules/<m>.md milestone 状态行 (Status marker)
- [ ] docs/current/server/migrations.md §7 v 行数 == registry.go `All` 长度
- [ ] docs/current/server/data-model.md 表/列 == `internal/store/models.go` GORM struct
- [ ] docs/current/server/http-api.md 路由 == `cmd/server/main.go` mux 注册
- [ ] **docs/current 字面常量核验** (见 §2.1) — 旧常量残留 → 🔴 P0
- [ ] 已废弃 milestone (如 ADM-3) 标 `obsolete` 不删行 (评审可追溯)

### 2.1 docs/current vs main 代码 字面常量核验 (PR #242 教训)

**触发**: 任何改 cookie 名 / migration v / handler 函数名 / 中间件名 / 路由前缀 / 文件名引用 的 PR merge 后, 下次 Phase 关卡审计 **必跑**。

**命令模板** (复制即用):

```bash
# 1. docs/current 残留旧常量 → 🔴 P0
grep -rn "<old_const>" docs/current/ && echo "🔴 P0 脱节" || echo "✅ 干净"
# 2. main 代码确认新常量落地 (反向断言)
grep -rn "<new_const>" packages/server-go/ packages/client/src/ | head

# 例 (PR #242 实测 4 处脱节, ADM-0 系列触发):
grep -rn "borgee_admin_token" docs/current/    # 旧 cookie → 应 0
grep -rn "/api/v1/admin/"     docs/current/    # ADM-0.2 删 god-mode → 仅历史标注
grep -rn "admin_auth.go"      docs/current/    # ADM-0.2 删文件 → 应 0
grep -rn "users.role *= *['\"]admin"  docs/current/  # ADM-0.3 enum 收 → 应 0
```

**判定**: docs/current 含旧常量任意一处 → 审计这一行标 🔴 P0, 派飞马修 PR 后才放行 Phase 退出。

## 3. 红线

❌ Phase 退出无审计文档 · ❌ PROGRESS log 跳周 · ❌ docs/current diff 空但 PR 改 schema · ❌ milestone PR merge 后 owner 不补 docs/current
