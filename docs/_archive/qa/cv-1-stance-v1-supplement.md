# CV-1 v1 立场补丁 (#295 v1 transition unlocked)

> **状态**: v1 (野马, 2026-04-28)
> **触发**: #295 §5 三条件全满足 → RT-1 (#290 RT-1.1 server cursor / #292 RT-1.2 client backfill / #296 RT-1.3 BPP session.resume) + AL-3 (#301/#302/#303/#305) + BPP-1 envelope CI lint (#304, G2.6 ⏸️→✅ DONE, commit 4724efa) 全 merged。
> **目的**: CV-1.x 实施 PR 直接吃此表为 v1 acceptance; 跟 #282 v0 表组合, v0 锁形态 + 本表锁字段/边界/反向断言细节。
> **关联**: `canvas-vision.md` §1.4 / §1.5; `realtime.md` §2.1 / §2.3; `admin-model.md` §1.4 红线; #282 v0 ②③⑤⑦ 四项锁延伸。
> **配套**: 飞马 CV-1 spec brief (架构) + 烈马 CV-1 acceptance template — 一起 review 一起 merge → CV-1 实施基线。

---

## 1. 4 项立场 v1 细化

### ② 单文档锁 30s TTL (lock-holder 字段 + 边界 + fallback)

| 项 | v1 立场细化 (字面锁) |
|---|---|
| 字段 | `artifacts.lock_holder_user_id` (nullable) + `lock_acquired_at` (timestamp); commit / release / TTL 三路写 |
| 边界 | T+0 acquire → T+29s 持有有效 (commit 必须在窗口内) → T+30s 起任意人 acquire 视为新锁 (last-writer-wins, 旧 holder 后写收 409 conflict + reload hint) |
| fallback | 锁过期 = 字段非空但 `now - lock_acquired_at > 30s`, 谁先抢 (UPDATE WHERE lock_acquired_at < now-30s) 谁拿; 不靠后台 GC (lazy expire) |
| 反约束 | ❌ 不上 CRDT (canvas §2 显式不做); ❌ 不上多锁 / range lock (单文档粒度); ❌ 不显示 lock-holder 头像走 AL-3 presence (留 v2, 跟 ⑥ 委员会决议范围一致) |
| 跟 v0 区别 | v0 (#282 ②) 锁形态 (last-writer-wins + 30s TTL + conflict 409); 本表锁**字段名 + 边界字面 + lazy expire 实现** |

引: AL-3 presence #303 ⑤ (5s 节流 + 60s 心跳) 时序仅 presence 用, 不复用到锁 TTL (锁 30s 比 presence 60s 更短 = 更激进释放).

### ③ 版本线性 + agent 默认无删 (PK 单调 + role check + audit)

| 项 | v1 立场细化 |
|---|---|
| 表 | `artifact_versions(id PK AUTOINCREMENT, artifact_id, version, committer_id, committer_kind, body, created_at, rolled_back_from_version)`; `version` 跟 PK 同向单调, 无 fork 列; UNIQUE(artifact_id, version) 锁线性 |
| agent role check | `DELETE FROM artifact_versions WHERE committer_id = ?` 路径加 `RequirePermission('artifact.delete_history')` 闸; agent 默认无该 perm (跟 concept-model §2 默认最小化) |
| audit | 任何 delete (即使 owner grant) 落 `audit_log` 行 (跟 ADM-2 #266 acceptance 同 schema, action='artifact.version.delete') |
| 反约束 | ❌ 不允许 agent runtime 自己 grant delete-history (owner UI 主动 grant 才行); ❌ 不允许版本图状 (no fork v1); ❌ 无限保留无 GC (留 v2 — 跟 #282 ③ 一致) |
| 跟 v0 区别 | v0 锁 "agent 默认无删" 立场; 本表锁**表结构 + RequirePermission 闸 + audit_log schema 同源**; owner grant UI 在本表内仍**未实施** (留下波 — 跟 ADM-2 #266 配套) |

### ⑤ ArtifactUpdated #237 envelope (字段顺序 + cursor + BPP-1 自动 lint)

| 项 | v1 立场细化 |
|---|---|
| 字段顺序 | `{type, cursor, artifact_id, version, channel_id, updated_at, kind}` 7 字段 byte-identical 跟 RT-1.1 #290 实施 (`internal/ws/cursor.go::ArtifactUpdatedFrame` + `cursor_test.go::TestArtifactUpdatedFrameFieldOrder` golden JSON) + cv-1-spec.md L13; `type='artifact_updated'` (下划线, `FrameTypeArtifactUpdated` const) |
| committer pull (反约束) | envelope **不带** `committer_id` / `committer_kind` — committer 信息走 GET /api/v1/artifacts/:id 拉; envelope 仅信号, 内容/作者全 pull (跟 ❌ 不在 envelope 内塞 body 内容 同准则) |
| kind 标签 | `kind` ∈ {`"commit"`, `"rollback"`} — client 据此区分新写 vs 回滚, 渲染版本侧栏 "v{N+1} (rollback from v{M})" 时用 |
| cursor | 跟 RT-1.1 共用 server cursor 单调 (单一 events 表), 不另设 artifact-only cursor |
| BPP-1 lint | #304 envelope CI lint 自动 enforce (`bpp/frame_schemas.go` reflect 比对 server-go 端 `WSEnvelope` 字段顺序) — 改字段顺序 = lint fail = PR 卡 |
| 反约束 | ❌ client 不能用 `updated_at` 排序 (RT-1 ① 反约束: server cursor 唯一可信序); ❌ 不另造 artifact-only push 通道 (走统一 /ws hub + BPP-1 frame schema); ❌ 不在 envelope 内塞 body 内容 (artifact 内容走 GET /artifacts/:id 拉, push 仅信号) |
| 跟 v0 区别 | v0 #282 ⑤ 仅锁 "envelope 套 #237 + 飞马人工 lint 闸位"; 本表锁**字段名 + 顺序 byte-identical + BPP-1 #304 自动 lint 接管** |

### ⑦ rollback owner-only (REST action + 反向断言 + 触发 push)

| 项 | v1 立场细化 |
|---|---|
| API | `POST /artifacts/:id/rollback {to_version: N}` (单独 action endpoint, 非 PATCH body 字段 — 防 client 误触); 服务端等价于 `INSERT artifact_versions ... body = (SELECT body FROM artifact_versions WHERE version=N)` 产新版本, 旧版本不删 |
| 元数据 | 新 row 加列 `rolled_back_from_version` (nullable int), 跟 #282 ⑦ v1 预留一致; UI 渲染 "v{N+1} (rollback from v{M})" |
| 反向断言 | admin cookie 调 → 401 (跟 #303 ⑦ admin god-mode 不主导 runtime 行为, rollback 是写动作, admin 不入); channel member 非 owner 调 → 403 (跟 channel-model §1.4 owner-only 同模式); 锁持有 = 别人时 → 409 (跟 ② 锁路径一致, rollback 也要拿锁) |
| 触发 push | rollback 成功 → 走 ⑤ 同 envelope 推 ArtifactUpdated frame; system message 不发 (rollback 是 owner 行为, 不污染 fanout) |
| 跟 v0 区别 | v0 #282 ⑦ 锁 "owner-only + 触发新 commit"; 本表锁**REST 路径 + 三反向断言 (401/403/409) + rolled_back_from_version 列 + 触发 ArtifactUpdated envelope** |

---

## 2. 反向 grep — CV-1.x v1 PR merge 后跑, 全部预期 0 命中

```bash
# ② lock-holder 不应被 sanitizer 漏 (lock_holder_user_id 应在 GET /artifacts/:id response 里, 但 god-mode endpoint 走元数据白名单不含 body)
grep -rnE "lock_holder_user_id" packages/server-go/internal/api/admin*.go | grep -v _test.go
# ③ agent 不应有自 grant delete-history 路径
grep -rnE "GrantPermission.*artifact\.delete_history.*FromAgent|self_grant" packages/server-go/internal/ | grep -v _test.go
# ⑤ artifact 不应自造 envelope (走 #237/#290 共用)
grep -rnE "type:.*'artifact\.updated'|ArtifactUpdated.*Envelope\{" packages/server-go/internal/ws/ | grep -v _test.go | grep -v frame_schemas
# ⑦ rollback 不应是 PATCH body 字段
grep -rnE "PATCH.*/artifacts.*rollback|body\.rollback_to" packages/server-go/internal/api/ | grep -v _test.go
```

---

## 3. 验收挂钩 (CV-1.x PR 必带, 配 v0 #282 表)

- CV-1.1 schema: ② `lock_holder_user_id` + `lock_acquired_at` 列 + ③ `artifact_versions` PK + `committer_kind` + `rolled_back_from_version` 列 (CV-1.1 PR # 待战马 spawn)
- CV-1.2 handler: ② 30s TTL lazy expire + 409 conflict; ⑦ POST rollback action + 三反向断言 (401/403/409); ③ RequirePermission 闸 + audit_log
- CV-1.3 sync: ⑤ ArtifactUpdated 字段顺序 byte-identical (#290 共序) + BPP-1 #304 lint 自动 enforce + GET /artifacts/:id 拉 body (push 仅信号)
- v1 解封追溯: 三条件 PR # 引: RT-1 #290 + #292 + #296 / AL-3 #301 + #302 + #303 + #305 / BPP-1 #304 (跟 #295 §5 一致)

---

## 4. 不在 v1 范围 (推 v2)

- ❌ 锁 holder 头像 + 在线状态 (跟 AL-3 presence 联动) — 留 v2
- ❌ owner grant delete-history UI — 留 ADM-2 #266 配套实施波
- ❌ 版本 GC 策略 — v2; ❌ 段落锚点对话 — canvas §2 v2; ❌ artifact 跨 channel 共享 — v2

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-28 | 野马 | v1, ②③⑤⑦ 四项 v1 立场细化 (字段名 + 边界 + REST + 反向断言 + audit + envelope 字段顺序), 跟 #295 §5 三条件 PR # 引追溯 (#290/#292/#296/#301/#302/#303/#304/#305) |
| 2026-04-28 | 野马 | v1.1 patch — 修 PR # 引: RT-1 三段全列 (#290/#292/#296), BPP-1 #292 → #304 (envelope CI lint 真落 commit 4724efa, G2.6 ⏸️→✅ DONE), AL-3 三轨 + 文案锁 (#301/#302/#303/#305 — #304 是 BPP-1 不是 AL-3) |
| 2026-04-29 | 野马 | v1.2 patch — 二轮反查 (post-#334) 抓出 3 处 drift, 跟 #334 实施 + #337 acceptance + #338 cross-grep 反模式 三源对齐: ② `workspace_files.lock_holder_user_id` → `artifacts.lock_holder_user_id` (表名); ③ `version_no` → `version` + `committer_user_id` → `committer_id` + 加 UNIQUE 约束 + `rolled_back_from_version` 列入 schema; ⑤ envelope 字段名同步; ⑦ `to_version_no` → `to_version` |
| 2026-04-29 | 烈马 | v1.3 patch — 三轮反查 (frame 顺序 align #290 实施): ⑤ envelope 8 字段 → 7 字段 (`{type, cursor, artifact_id, version, channel_id, updated_at, kind}`), `type` 字面 `artifact.updated` → `artifact_updated` (下划线, `FrameTypeArtifactUpdated` const), 砍 `committer_id` / `committer_kind` 列 (committer 信息 GET pull, 反约束 ⑤ envelope 仅信号), 加 `kind` ∈ {commit, rollback} 标签; 跟 `internal/ws/cursor.go` + `cursor_test.go::TestArtifactUpdatedFrameFieldOrder` golden JSON byte-identical; acceptance §2.5 同步 align (反向修原则跟 #337 一致) |
