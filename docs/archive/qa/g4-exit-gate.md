# Phase 4 退出 Gate — G4.* 5 闸 + audit 草稿 (飞马)

> 飞马 · 2026-04-29 · ≤200 行 · 跟 G3 closure `g3-exit-gate.md` 同模式 byte-identical 结构 / 草稿版 — Phase 4 收尾联签
> Trigger: Phase 4 6/9 milestone merged (AL-1b ✅ #482 / AL-2a ✅ #480 / AL-2b ✅ #481 / ADM-1 ✅ #483 / ADM-2 ✅ #484 / CM-5 ✅ #476) + 2 in-flight (BPP-2 #485 / RT-3 #488 phase 1) + 1 未起 (AP-1) → G4 草稿落地 (跟 BPP-3 实施同 PR 装, 不开独立 docs PR per 新协议)
> 关联: `docs/qa/signoffs/g3-exit-gate.md` (G3 closure 5 链锁源头) / `docs/implementation/PHASE-4-ENTRY-CHECKLIST.md` (#456 entry 8 milestone) / `docs/implementation/PROGRESS.md` (Phase 4 进度) / `docs/qa/regression-registry.md` (REG 数学对账)

---

## 1. G4.* 5 闸 + G4.audit 状态总览 (拟)

| 闸 | 主旨 | 依赖 milestone | 证据 PR (拟/实) | Status |
|---|---|---|---|---|
| **G4.1** | admin SPA + privacy promise E2E (8 行表三色锁 + vite ?raw drift CI 拦) | ADM-1 ✅ (#483 merged) | #459 G4.1 acceptance ✅ SIGNED | ✅ |
| **G4.2** | god-mode metadata audit + impersonation E2E (5 action audit + 5 system DM byte-identical + impersonation_grants v=23) | ADM-2 ✅ (#484 merged 54851b8) | 待 (drftd 在 #484 closure follow-up) | ⏳ |
| **G4.3** | BPP v2 协议 envelope (12-frame) + ack dispatcher + 8 处 reason 链 E2E | BPP-2 #485 in-flight + AL-2b ✅ #481 + AL-1b ✅ #482 | 待 #485 merged 后 closure | ⏳ |
| **G4.4** | agent↔agent 协作走人 path 不裂表 E2E (DM-2 router 复用 + CV-1 lock 复用 + 0 行新 server impl) | CM-5 ✅ (#476 merged 59a833f) | 待 (REG-CM5 7 行 + 5 端到端 case) | ⏳ |
| **G4.5** | runtime registry + agent_configs SSOT 全链 E2E (registry → SSOT blob 整体替换 → BPP push → ack) | AL-4 ✅ + AL-2a ✅ + AL-2b ✅ | 待 (端到端 4 段串联) | ⏳ |
| **G4.audit** (滚动) | Phase 4 跨 milestone 代码债 audit (滚动加 audit row, 跟 G3.audit #448 同模式) | 全 Phase 4 milestone + AP-1 (待启) | 飞马起草 closure announcement | 滚动 |

### G4.audit row — kindBadge 5 源补齐 (post-#485 verify 抓出)

- [ ] **kindBadge helper 5 源补齐** — G3.audit 标"5 源 byte-identical" 但 post-#485 reverse-grep 实为 **2 源** (ArtifactPanel + AnchorThreadPanel), 缺 DM-2 / CV-4 / CHN-4 渲染面复用. G4.audit row, 派战马 (CV-2 / DM-2 / CV-4 / CHN-4 任选) follow-up commit 进下个 milestone PR. 字面: `committer_kind === 'agent' ? '🤖' : '👤'`. 真补后 G4 closure §3 链 1 才闭.
- [ ] **链 4 rollback owner-only DOM gate regex 对齐** — 代码用 `channel?.created_by` (optional chain), spec regex 用 `channel.created_by`. 无语义 bug 但 reverse-grep 哨兵 miss. 二选一: 改 regex `channel\??\.created_by` 或代码去 `?.`.

**通过判据**: G4.1-G4.5 全 ✅ SIGNED + G4.audit 滚动闭 + AP-1 milestone 完成 + 跨 phase 留账锚 (Phase 5+/6+) 全明示 → Phase 4 closure announcement (跟 G3 closure 同模式无软留账).

---

## 2. Phase 4 章程严守 9 milestone 全闭判据 (跟 G3 同模式)

| # | milestone | spec brief | acceptance | 文案锁/stance | 实施 |
|---|---|---|---|---|---|
| 1 | AL-4 runtime registry | ✅ #379 v2 | ✅ | ✅ #387 stance | ✅ #398/#414/#417 (pre-Phase4) |
| 2 | AL-2a SSOT | ✅ | ✅ #264 | ✅ #454 stance + content-lock | ✅ #480 (7a0c69b) |
| 3 | AL-2b BPP frame | ✅ | ✅ #452 | ✅ stance via spec | ✅ #481 (225e739) |
| 4 | AL-1b 5-state | ✅ #453 | ✅ | ✅ #458 stance + content-lock | ✅ #482 (9be6197) |
| 5 | ADM-1 admin SPA | ✅ | ✅ #262 | ✅ content-lock | ✅ #483 (48deb9b) |
| 6 | ADM-2 god-mode | ✅ #475 | ✅ | ✅ stance + content-lock | ✅ #484 (54851b8) |
| 7 | BPP-2 协议 v2 | ✅ #460 | ✅ | ✅ stance + content-lock | 🟡 IN-FLIGHT #485 |
| 8 | AP-1 严格 403 | ⚪ 未起 | ⚪ | ⚪ | ⚪ |
| - | CM-5 (parallel) | ✅ #463 | ✅ | ✅ | ✅ #476 (59a833f) |

**8/9 spec 已闭**, 1 未起 (AP-1) — Phase 4 收尾前必补 4 件套 + 实施.

---

## 3. 跨 milestone byte-identical 链锁延续 (Phase 4 防漂移)

承袭 G3 closure 5 链 + Phase 4 加 4 链:

| 链 | 字面 | 源数 | 守在哪里 |
|---|---|---|---|
| 链 1-5 (G3 closure 承袭) | kindBadge 二元 / CONFLICT_TOAST / fanout / rollback / 5-frame envelope | 5+1+1+2+12 | Phase 4 不破链 (新组件无新源) |
| **链 6** runtime-only 字段反约束 | api_key / temperature / token_limit / retry_policy 不入 schema/frame/admin | 多源 (AL-2a #454 + BPP-2 #460 + ADM-2 #484 反约束 grep 全 0) | AL-2a / AL-2b / BPP-2 / ADM-2 立场 |
| **链 7** admin god-mode reject 四源 | admin endpoint 不挂 PATCH 业务路径, /admin-api/* 双轨 | 4 源 (AL-3 #303 + AL-4 #379 + AL-1b #458 + AL-2a #454) + ADM-2 #484 路径分叉守红线 | ADM-0 §1.3 红线 |
| **链 8** agent silent default 三源 | 状态/配置变化不发 system message broadcast / 不污染 channel chat 流 | 3 源 (AL-3 #305 + AL-1b #458 + AL-2a #454) | BPP push server→plugin 而非 channel chat |
| **链 9** SSOT blob 整体替换 PATCH 语义 | PK 单 agent_id 不裂 multi-row | 单源 (AL-2a schema PK) | 蓝图 §1.4 字面 |
| **链 10** reason 八处单测锁 | `agentpkg.Reason*` SSOT 6 const, 8 处引用 byte-identical | 8 源 (AL-1a state.go / AL-3 lib/agent-state.ts / CV-4 iterations.go / AL-4 runtimes.go switch+test / AL-2b dispatcher+ack) | `internal/agent/state.go::Reason*` |
| **链 11** 12-frame envelope 共序 | type/cursor 头位 byte-identical, hub.cursors.NextCursor() 单调发号 | 12 (6 control + 6 data) | BPP-2 #485 envelope.go + frame_schemas_test |
| **链 12** busy=task-level vs online=session-level 拆死 | data-presence (session) + data-task-state (task) 双 DOM attr 正交 | 单源 (AL-1b #482 PresenceDot.tsx) | 蓝图 §2.3 R3 决议 |
| **链 13** 走人 path 不裂表 (CM-5) | agent↔agent 走 DM-2 router + CV-1 lock 复用, 0 行新 server impl / 0 新表 | 单源 (CM-5 #476) | 立场 ① |

**反向 grep 命令** (G4 closure 跑):

```bash
# 链 6: runtime-only 字段不入 schema/frame
grep -rnE 'agent_configs.*api_key|agent_config_update.*api_key|admin_actions.*api_key' packages/server-go/internal/   # 0 hit

# 链 7: admin god-mode reject — admin 不挂 PATCH 业务路径
grep -rnE 'PATCH /api/v1/agents/.*/config|admin.*PATCH.*agent.*config' packages/server-go/internal/api/admin*.go   # 0 hit

# 链 10: reason SSOT 唯一 (字面 enum 不复制)
grep -nE '"api_key_invalid"|"quota_exceeded"|"network_unreachable"|"runtime_crashed"|"runtime_timeout"' packages/server-go/internal/api/ packages/server-go/internal/bpp/   # 0 hit (全走 const)

# 链 11: 12-frame envelope 共序
grep -nE 'Type\s+string\s+`json:"type"`' packages/server-go/internal/bpp/envelope.go   # 12 hit

# 链 13: CM-5 不裂表 / 走 DM-2 router
grep -rnE 'agent_messages|agent_to_agent_router' packages/server-go/internal/api/   # 0 hit
```

---

## 4. Phase 5+/6+ 跨 phase 留账锚 (G3.audit 承袭)

| 项 | Phase | 触发条件 | 锚 |
|---|---|---|---|
| **CHN-3 lazy 90d GC cron** | Phase 4+ | 作者删 group + 90 天后 cron | g3-audit.md A1 |
| **AL-4 hermes plugin** | Phase 5+ | BPP-2 ✅ (本 phase 闭) + Hermes runtime 真接通 | g3-audit.md A2 |
| **CV-4 iterate retry 永不实施** | 反约束 | 永不实施 | g3-audit.md A3 |
| **第 6 轮 remote-agent 安全** | Phase 6+ | binary 签名 + 沙箱 + 资源限制 + uninstall | g3-audit.md A6 |
| **DL-4 / RT-3 / CS-3 / HB-1 / AP-3** | Phase 4+ 后段 | DL-4 → HB-1/CS-3 排序锁 (飞马 R2) | execution-plan.md §Phase 4+ |
| **HB-2/3/4 信任五支柱** ⭐ | Phase 4+ 收尾 | RT-3 ✅ + agent runtime 信任元数据 | 蓝图 §4 信任 |
| **BPP-1/-3/-4 plugin lifecycle** | Phase 4+ | BPP-3 plugin read loop 真接管 (#481 deferred 锚) | AL-2b 451a3e8 |
| **AP-1 严格 403** ⚠️ | **Phase 4 收尾必补** | 4 件套 + 实施 + REG-CHN1-007 ⏸️→🟢 | g3-audit.md A5 |

---

## 5. 退出条件 (Phase 4 closure announcement)

- §1 G4.* 5 闸全 ✅ SIGNED + G4.audit 滚动闭
- §2 9 milestone 4 件套全闭 (含 AP-1 补完)
- §3 跨 milestone byte-identical 13 链 (G3 5 + Phase 4 8 新) 反向 grep 全 0 hit
- §4 跨 phase 留账锚字面延续 (Phase 5+/6+)
- 跟 G3 closure 同模式无软留账 (用户拍板章程严守再公告)

---

## 6. 关联 PR / 文件

- **G3 closure**: `docs/qa/signoffs/g3-exit-gate.md` (#451 飞马联签, 5 闸全签源头)
- **G3.audit**: `docs/implementation/00-foundation/g3-audit.md` (#448 飞马 v1 fill, 6 audit row)
- **PHASE-4-ENTRY-CHECKLIST**: `docs/implementation/PHASE-4-ENTRY-CHECKLIST.md` (#456 飞马, 8 milestone entry)
- **G4.1 (ADM-1)**: #459 acceptance signoff (烈马 ✅)
- **Phase 4 6/9 merged**: #482 / #480 / #481 / #483 / #484 / #476
- **In-flight**: #485 BPP-2 / #488 RT-3 phase 1
- **未起**: AP-1 4 件套 + 实施

---

## 7. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 飞马 | v0 草稿 — Phase 4 退出 gate G4.* 5 闸 + G4.audit 框架 (跟 G3 closure 同模式 byte-identical 结构). 6/9 milestone merged + 2 in-flight + 1 未起 (AP-1). 13 跨 milestone byte-identical 链锁 (G3 5 承袭 + Phase 4 8 新: runtime-only / admin reject / silent default / SSOT blob / reason 八处 / 12-frame / busy vs online / 走人 path). Phase 5+/6+ 留账锚字面延续 g3-audit.md §3. 跟 BPP-3 worktree 同 PR 装 (新协议: 不开独立 docs PR). |
