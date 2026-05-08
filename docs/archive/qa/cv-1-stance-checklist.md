# CV-1 立场反查表 (Phase 3 第二波 — canvas / artifact)

> **状态**: v0 (野马, 2026-04-28) · v1-transition 解封 (烈马, 2026-04-28)
> **目的**: Phase 3 第二波 milestone 立场锚点 — 战马 CV-1.x PR 直接吃此表为 acceptance; 飞马 / 野马 review 拿此表反查漂移。同 #263 模板但更深 (展开 7 项立场)。
> **关联**: `canvas-vision.md` §0/§1.1-§1.6/§2; `channel-model.md` §1.1/§1.3 双支柱; `realtime.md` §2.1 (BPP `artifact.commit/progress`) + §2.3 (envelope 等同); `blueprint/README.md` 14 立场 §5 (artifact 集合) + §11 (沉默胜于假 loading)。
> **依赖**: CHN-1 merged (channel 形状稳) + #237 envelope + #269 RT-1 spec + #280 BPP-1 envelope lint。

---

## 0. v1 transition 解封 ✅ (2026-04-28)

CV-1 三前置依赖**全部满足** → 战马A **CV-1.1 schema 解封信号**, 可以开始实施。

| 前置 | 闭合 PR | 验收锚 |
|---|---|---|
| **RT-1 三段全闭** ✅ | #290 (RT-1.1 server cursor) + #292 (RT-1.2 client backfill) + #296 (RT-1.3 agent BPP session.resume) | `acceptance-templates/rt-1.md` REG-RT1-001..010 全 🟢; ArtifactUpdated frame byte-identical 锁 (`cursor_test.go::TestArtifactUpdatedFrameFieldOrder`) — CV-1 `artifact.commit/progress` frame 直接套用 |
| **AL-3 接口契约** ✅ | #301 (飞马 spec) + #302 (烈马 acceptance) + #303 (野马 stance) | `acceptance-templates/al-3.md` 13 验收 + 4 反查锚; presence 三态 + `PresenceTracker.IsOnline` 接口锁定 — CV-1 channel 成员在线性查询不回退 |
| **BPP-1 envelope CI lint 真落** ✅ | **#304** merged 2026-04-28 13:35:02Z (commit `4724efa`) — G2.6 留账闸 ⏸️ → ✅ DONE | `acceptance-templates/al-2a.md` §蓝图行为对照 + REG-BPP1-001..008 全 🟢; CV-1 artifact frame 自动套 envelope CI lint 闸位 (whitelist + 方向锁 + 字段顺序 + RT-0/RT-1.3 dispatcher byte-identical) |

> **解封结论**: CV-1.1 (artifacts schema + commit/progress frame) 的所有跨模块契约都已锁死, 战马A 可直接落 schema PR; CV-1.2 (BPP frame) 走已落地的 envelope CI lint 闸位 (#304), 不再需要飞马人工 lint。RT-1 ArtifactUpdated frame 已是参考样本 (字段顺序 `type/cursor/artifact_id/version/channel_id/updated_at/kind`)。

---

## 1. 7 项立场 — 锚 §X.Y + 反约束 + v0/v1

| # | 立场锚 | 一句话立场 | 反约束 (X 是, Y 不是) | v0 / v1 |
|---|--------|----------|----------------------|---------|
| ① | canvas-vision §1.3 + §1.4 + 14 立场 §5 | **artifact 归属 = channel**, 跟 channel 走 (channel 软删 → artifact 软删随; 成员离开 → 失访问); 非 author 主权 | **是** workspace per channel, `artifacts.channel_id NOT NULL`, 无 `owner_id` 主权语义; **不是** 归属 author (author 离开不带走), **不是** 跨 channel 共享 (v1 不做多 artifact 视图) | v0: `channel_id` 唯一归属键; v1 同 |
| ② | canvas-vision §2 不做 + 14 立场 §2 | **多人编辑 = 单文档锁 (last-writer-wins + locked_by_user_id 30s TTL)**, 不上 CRDT | **是** 串行编辑, agent 写入 = 一次新版本, 锁过 30s 自动释放; **不是** realtime CRDT (§2 显式不做, 巨坑); **不是** 无锁覆盖 (后写者收 conflict 提示 → reload) | v0: TTL 锁 + conflict UI hint; v1 同, 加 lock-holder 头像显示 |
| ③ | canvas-vision §1.4 + §1.5 表 | **版本历史线性保留, agent 默认无权删历史** (write/edit ✅ 默认, delete-history ❌ 默认 grant) | **是** 每 commit 一新版本行, 可回滚到前一版; agent 写内容 ✅, 删历史 ❌ (§1.5 表锁); **不是** 版本图状 (no fork v1); **不是** 无限保留无 GC (v2 加策略, v1 不限期) | v0: 线性 + agent 无删权; v1 加 owner grant UI |
| ④ | canvas-vision §1.4 + §2 v1 做 | **artifact 类型白名单 = Markdown ONLY** (v0/v1) | **是** 单一 markdown 形态 (canvas-vision §2 "v1 做" 锁); **不是** 代码片段 / 图片 / PDF / 看板 (§2 v1 不做, v2+); 上传非 markdown → 走老 `workspace_files` 附件路径 (与 artifact 分轨) | v0: artifact.type='markdown' 唯一枚举; v1 同, v2+ 加 `code` / `image` 类型 |
| ⑤ | realtime §2.1 + §2.3 + 飞马 #269 RT-1 spec | **artifact 同步 = `artifact.commit` / `artifact.progress` frame 套 #237 envelope** (字段名/顺序 byte-identical, server cursor 单调) | **是** Phase 2 `/ws` hub 发 `ArtifactUpdated{cursor, artifact_id, version, channel_id, updated_at, kind}` 跟 BPP-1 frame schema 同序 (烈马 #280 envelope CI lint); **不是** 自造 envelope, **不是** client timestamp 排序 (RT-1 立场 ① 反约束) | v0: ws hub + 飞马人工 lint 闸位; v1: BPP-1 CI lint 接管 |
| ⑥ | canvas-vision §1.6 + 14 立场 §2 | **协作场景区分: agent commit / human commit, 锚点对话仅人审 agent 产物** (人机界面, 非 agent 间通信) | **是** version row 必带 `committer_kind` (agent / human), 通知 fanout 时锚点评论仅 owner 审 agent 产物路径 (canvas-vision §1.6); **不是** agent 之间用锚点互通 (§1.6 显式打掉, agent 间走 channel message + artifact 引用); **不是** 无差别通知 (区分 agent vs human commit, 避免噪音) | v0: `committer_kind` 列 + agent commit fanout system message `"{agent_name} 更新 {artifact_name} v{n}"`; v1: 锚点对话 (留 v2 §2 不做) |
| ⑦ | canvas-vision §1.5 表 + §1.4 | **rollback = owner only** (channel 创建者), UI = 版本列表点 "回滚到此版本" → 触发新 commit (不删旧) | **是** owner 触发 rollback → 等价于以旧版本内容产新 commit (线性版本不破坏, agent 仍可继续 iterate); **不是** 任意成员可 rollback (作者控大局, channel-model §1.4 同模式); **不是** 删除中间版本 (§1.5 删历史 grant 才行); **不是** admin 可 rollback (admin 不入 channel, ADM-0 红线) | v0: owner-only + 触发新 commit; v1 同, 加 `rolled_back_from_version` 元数据 |

---

## 2. 黑名单 grep — Phase 3 第二波反查 (PR merge 后跑, 全部预期 0 命中)

```bash
# CV-1 ①: artifact 表不应有 owner_id 主权列 (channel_id 唯一归属)
grep -rnE "artifacts.*owner_id|workspace_artifacts.*owner_id" packages/server-go/internal/store/ | grep -v _test.go
# CV-1 ②: 不应引入 CRDT 库
grep -rnE "yjs|automerge|y-protocols" packages/client/ packages/server-go/ | grep -v _test
# CV-1 ④: artifact.type 不应出现非 markdown 枚举 (v0/v1)
grep -rnE "artifact\.type.*=.*\"(code|image|pdf|kanban)\"" packages/server-go/internal/ | grep -v _test.go
# CV-1 ⑤: ArtifactUpdated frame 不应自造 envelope (字段名/顺序须套 #237)
grep -rnE "ArtifactUpdated.*timestamp|sort.*ArtifactUpdated.*time" packages/server-go/internal/ws/ | grep -v _test.go
# CV-1 ⑦: rollback 路径不应允许非 owner
grep -rnE "rollback.*RequirePermission.*[^o]wner" packages/server-go/internal/server/ | grep -v _test.go
```

---

## 3. 不在 CV-1 范围 (避免 PR 膨胀)

- ❌ 段落锚点对话 (canvas-vision §2 v1 不做, v2 加 — ⑥ 仅锁 v2 形态立场)
- ❌ 多 artifact 关联视图 / 拖拽连线 (§2 v2+); ❌ realtime CRDT (§2 不做)
- ❌ artifact 跨 channel 共享 / 引用图 (v2)
- ❌ 代码 / 图片 / PDF / 看板 artifact (§2 v2+, ④ 锁 markdown 唯一)
- ❌ 删除中间版本 / GC 策略 (③ v0 不限期, v2 加)
- ❌ admin 看 artifact 内容 (走 god-mode endpoint 不返回 body, ADM-0 §1.3 已锁)

---

## 4. 验收挂钩

- CV-1.1 (schema): ① `artifacts.channel_id NOT NULL` + 无 `owner_id` 主权; ④ `type` 枚举 = `'markdown'` 唯一 CHECK; ③ `artifact_versions` 线性 + `committer_kind` ⑥
- CV-1.2 (handler): ② 单文档锁 30s TTL + conflict 409; ⑦ rollback owner-only 403 反断言; ⑥ agent commit fanout system message 文案锁 `"{agent_name} 更新 {artifact_name} v{n}"`
- CV-1.3 (sync): ⑤ ArtifactUpdated envelope byte-identical 反向 grep + #237 同序 + 飞马人工 lint 闸位 (BPP-1 CI lint 接管前)
- Phase 3 第二波闸 (野马): §1 7 项全锚 + §2 grep 0 + §3 不在范围 6 条对得上 → ✅ CV-1 解封

---

## 5. v0 → v1 切换条件 (立场补丁前置)

> v1 立场补丁 PR **不可早开** — 三条件齐全才解封, 跟 RT-1 + BPP-1 + CV-1.1 实施 PR # 锁。

| 项 | v0 当前 (本表锁) | v1 切换触发 (三条件 AND) | v1 立场补丁内容预留 |
|----|----------------|-------------------------|------|
| 锁 ② | last-writer-wins + 30s TTL, conflict 409 | RT-1.1+1.2+1.3 全 merged (#269 spec 实施落地) | 加 lock-holder 头像 + 在线状态 (走 AL-3 presence) |
| 删历史 ③ | agent 默认无删权, owner grant 走 ad-hoc | CV-1.1 schema PR merged (`artifact_versions` 表稳) | owner grant UI + version GC 策略草案 |
| envelope ⑤ | /ws hub + 飞马人工 lint 闸位 | BPP-1 envelope CI lint 真落 (#280 merged, 不是占号) | 升级 frame schema 注释锁 → CI 自动 lint |
| rollback ⑦ | owner-only, 触发新 commit, 无 metadata | 同 ② 三条件 | 加 `rolled_back_from_version` 元数据列 |

**反约束**: v1 补丁 PR title 必须引 RT-1.3 + BPP-1 (#280) + CV-1.1 三 PR # (规则 6 留账闸编号锁同模式); 任一未落, v1 PR 不开 (避免立场漂移到未实施的下一阶段)。

---

## 6. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-28 | 野马 | v0, 7 项立场 (①-⑦ 比 #263 立场 ①②③ 展开) + 5 行黑名单 grep + 6 条不在范围 + 验收挂钩 |
| 2026-04-28 | 野马 | v0.1, 加 §5 v0/v1 切换条件 (锁 ②③⑤⑦ 四项 v1 补丁前置 — RT-1.3 + BPP-1 #280 + CV-1.1 三 PR # AND 触发) |
