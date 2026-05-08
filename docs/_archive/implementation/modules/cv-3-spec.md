# CV-3 spec brief — D-lite 画布渲染 (artifact 类型扩展: code + image/link)

> 飞马 · 2026-04-29 · ≤80 行 spec lock (实施视角 3 段拆 PR 由战马A 落)
> **蓝图锚**: [`canvas-vision.md`](../../blueprint/current/canvas-vision.md) §1.2 (D-lite, 不是 Miro) + §1.4 (artifact 集合: Markdown / 代码片段带语言标注 / 设计稿图片或链接 / 看板待办 v2+) + §2 v1 不做清单 (❌ 无限画布 / ❌ 多 artifact 关联视图 / ❌ CRDT / ❌ PDF / ❌ 看板)
> **关联**: CV-1 三段四件全闭 ✅ Markdown ONLY (#334+#342+#346+#348) — artifact_versions schema + RT-1 envelope + Markdown 渲染就位; CV-2 三段进行中 (#359 schema + #360 server, #361 锚点对话 envelope 复用); CHN-1 #276 channel 权限继承; RT-1 #290 cursor envelope 复用
> **章程闸**: G3.4 协作场骨架 demo 价值 — D-lite "画布" 视觉差异化 (代码高亮 + 图片嵌入) 是退出公告 demo 必要素材

> ⚠️ 锚说明: CV-1 v1 故意 Markdown ONLY (CV-2 spec §4 反约束); CV-3 升 D-lite 加 code (带语言标注) + image/link 两类, **不做** 无限画布 / 多 artifact 关联 / CRDT / 看板 (蓝图 §2 v1 不做字面禁守住)

## 0. 关键约束 (3 条立场, 蓝图字面 + CV-1/CV-2 边界对齐)

1. **artifact_type enum 扩展, 不裂表** (蓝图 §1.4 字面 "artifact 集合"): `artifacts.type` 枚举从 `'markdown'` 扩 +`'code'` +`'image_link'` (CV-1.1 #334 既有列名 byte-identical — `artifacts.type TEXT NOT NULL CHECK (type = 'markdown')` cv_1_1_artifacts.go:72; 列名是 `type` 不是 `kind`, RT-1 frame.Kind json:"kind" 是 frame 字段独立约定; client DOM `data-artifact-kind` 也独立); `artifact_versions.body` 复用 — code 存源码 string + 头部 JSON metadata `{language: "go|ts|py|..."}`, image_link 存 URL string + metadata `{kind: "image"|"link", thumbnail_url}`; **反约束**: 不开 `artifact_code` / `artifact_images` 拆表路径 (避免 schema 爆炸 + RT-1 envelope cursor 失同步)
2. **client 渲染按 type 分支, 单组件三 renderer** (D-lite 字面禁多 view 视图): `<ArtifactPanel>` 加 `switch(artifact.type)` 三分支 → `<MarkdownRenderer>` (CV-1.3 已就位) / `<CodeRenderer>` (新, syntax highlight 走 `prism-react-renderer`) / `<ImageLinkRenderer>` (新, `<img>` for image / `<a>` for link); **反约束**: 不开 "拖拽连线" / "多 artifact 关联视图" 路径 (蓝图 §2 字面禁 — Miro 那个坑); 不开 CRDT 多人实时编辑 (一人一锁 + RT-1 push 已够)
3. **mention 引用展开 preview 跟 type 一致** (蓝图 §1.4 字面 "@ 引用时自动展开预览"): message 流里 `<artifact:abc123>` token 渲染时按 `artifact.type` 调对应 renderer 缩略版 (markdown 头 80 字 / code 头 5 行 + 语言徽标 / image 缩略图 192px); **反约束**: PDF / PR diff / 看板 / 思维导图 0 hit (蓝图 §2 字面禁守住, 留 v3+)

## 1. 拆段实施 (CV-3.1 / 3.2 / 3.3, ≤ 3 PR)

| 段 | 范围 | 闭锁 | owner |
|---|---|---|---|
| **CV-3.1** schema enum 扩 + server validation | `artifacts.type` enum CHECK 加 `'code'` / `'image_link'` (migration **v=17**, AL-4.1 顺延 v=16 后); **SQLite 12-step 重建路径** (SQLite 不支持 ALTER CHECK 直接改): (1) `CREATE TABLE artifacts_new` 新 CHECK `type IN ('markdown','code','image_link')` 全列字面齐 cv_1_1_artifacts.go:72 → (2) `INSERT INTO artifacts_new SELECT * FROM artifacts` (transaction 包住, WAL 模式) → (3) `DROP TABLE artifacts` → (4) `ALTER TABLE artifacts_new RENAME TO artifacts` → (5) 重建 `idx_artifacts_channel_id` 索引; CHECK 反约束 reject `'pdf'` / `'kanban'` / `'mindmap'` (蓝图 §2 字面); `POST /artifacts` body 字段名仍是 `type` (跟 createArtifactRequest.Type artifacts.go:208 byte-identical, **不引入 `kind` 字段名**), 立场 ④ 旧锁 `400 "type must be 'markdown' (v1)"` (artifacts.go:253) **删**, 改 `type=='code'` 必含 `metadata.language` ∈ 白名单 (`go|ts|js|py|md|sh|sql|yaml|json|html|css`, 11 项 + `text` fallback) 或 400 `artifact.invalid_language`; `type=='image_link'` 必含 `metadata.kind` ∈ ('image','link') + URL 校验 (https only, 反约束 javascript: / data: 0 hit) | 待 PR (战马C) | 战马C |
| **CV-3.2** client renderer 三分支 + mention preview | `<ArtifactPanel>` 加 type switch (CV-1.3 #346 入口 byte-identical); `<CodeRenderer>` 套 prism-react-renderer + 行号 + 复制按钮; `<ImageLinkRenderer>` image 走 `<img loading="lazy" src>` + max-height: 480px; link 走 `<a target="_blank" rel="noopener noreferrer">`; mention `<artifact:id>` 流内 preview 缩略 (markdown/code/image 各 80字/5行/192px); 反约束: 不渲染 raw HTML (XSS 红线, body 字符串原样不 dangerouslySetInnerHTML) | 待 PR (战马C) | 战马C |
| **CV-3.3** e2e + 章程退出公告 demo 截屏 | e2e: 创 code artifact (Go) + 渲染高亮验 prism class hit + 创 image_link + `<img>` DOM 验; mention 引用 code/image artifact 流内 preview 渲 (跟 CV-1.3 #348 §3.3 e2e 模式 byte-identical); G3.4 协作场骨架 demo 截屏 3 张 (markdown / code 高亮 / image 嵌入) byte-identical 入 `docs/qa/signoffs/` (撑章程退出公告) | 待 PR (战马C) | 战马C |

## 2. 与 CV-1 / CV-2 / RT-1 / CHN-1 留账冲突点

- **CV-1 artifact_versions 复用** (非冲突): 老 markdown artifact 仍走 `type='markdown'` 路径不破; CV-1.3 markdown renderer 不动, 仅 ArtifactPanel switch 多两个分支; **立场 ④ 文案删**: CV-1.2 既有 `400 "type must be 'markdown' (v1)"` (artifacts.go:253) 在 CV-3.1 实施时**删**, 改放开 'code'/'image_link' 三态接受
- **CV-2 锚点对话 markdown ONLY** (CV-2 §4 反约束守住): 锚点 v2 仅挂 markdown artifact (CV-2 §4 字面 "❌ 锚点挂 PDF / 图片 / 代码 (CV-1 Markdown ONLY 锁同源, 留 CV-3 D-lite 后)") — CV-3 落地后是否对 code/image 加锚, **留 v3+ 不在本轮**; CV-2 server `POST /artifacts/:id/anchors` 加前置校验 `artifact.type=='markdown' or 403 anchor.unsupported_artifact_kind` (跟 CV-2 立场 ① 同源拒绝模式)
- **RT-1 ArtifactUpdated frame**: frame.Kind json:"kind" 字段已在 (cursor.go:56), 是 frame 字段独立约定 (跟 server schema 列名 `type` 不混); 复用同 envelope 不另起; client `wsClient.ts` switch frame.kind 分发到对应 renderer (现有 'markdown' case 可能 hard-coded, CV-3.2 实施时扩三 case 或 fallback default)
- **CHN-1 channel 权限继承**: code/image artifact 创/读权限 = artifact 所属 channel 成员权限; 跟 markdown 同源不另立 ACL
- **AL-4.1 v 号顺延 patch** (并行): AL-4 spec brief v=15 → v=16 (DM-2.1 #361 已抢 v=15); CV-3.1 拿 v=17 (AL-4.1 后顺延); 顺延锁字面写入 §3 grep 反查
- **v=15/16/17 sequencing 锁** (字面延续 #356 v3 + #361 兑现): CV-2.1 v=14 ✅ / DM-2.1 v=15 ✅ / AL-4.1 v=16 待 (战马待派) / CV-3.1 v=17 (本 spec)

## 3. 反查 grep 锚 (Phase 3 续作 / Phase 4 验收)

```
git grep -nE "type.*CHECK.*'code'.*'image_link'|type IN \('markdown','code','image_link'\)" packages/server-go/internal/migrations/   # ≥ 1 hit (CV-3.1, 列名是 type 不是 kind)
git grep -nE 'CodeRenderer|ImageLinkRenderer'                  packages/client/src/components/   # ≥ 1 hit (CV-3.2)
git grep -nE 'prism-react-renderer'                            packages/client/package.json     # ≥ 1 hit (语法高亮锁)
git grep -nE 'artifact\.type.*===.*\x27code\x27|type.*===.*\x27image_link\x27' packages/client/src/   # ≥ 1 hit (type switch 锁)
# 反约束 (6 条 0 hit, v1 加 1 条)
git grep -nE 'CREATE TABLE.*artifact_code|CREATE TABLE.*artifact_images' packages/server-go/internal/migrations/   # 0 hit (立场 ① 不裂表)
git grep -nE "type.*CHECK.*\x27pdf\x27|type.*CHECK.*\x27kanban\x27|type.*CHECK.*\x27mindmap\x27" packages/server-go/internal/migrations/   # 0 hit (蓝图 §2 v1 不做)
git grep -nE 'dangerouslySetInnerHTML.*body|innerHTML.*=.*body' packages/client/src/components/   # 0 hit (XSS 红线)
git grep -rnE 'CRDT|yjs|automerge'                              packages/client/ packages/server-go/   # 0 hit (蓝图 §2 字面 CRDT 巨坑不踩)
git grep -nE 'href.*=.*javascript:|src.*=.*data:image'         packages/client/src/components/   # 0 hit (image_link URL 白名单 https only)
git grep -nF "type must be 'markdown' (v1)"                   packages/server-go/internal/api/artifacts.go   # 0 hit (立场 ④ 旧锁删, CV-3 放开三态)
```

任一 0 hit (除反约束行) → CI fail.

## 4. 不在本轮范围 (反约束)

- ❌ 无限画布 / 拖拽布局 (蓝图 §2 字面 "Miro 那个坑")
- ❌ 多 artifact 关联视图 / 拖拽连线 (蓝图 §2 字面)
- ❌ CRDT 多人实时编辑 (蓝图 §2 字面 "CRDT 巨坑 v1 不踩"; 一人一锁 + RT-1 push 同源)
- ❌ PDF / PR diff 渲染 (蓝图 §2 字面)
- ❌ 看板 / 思维导图 / 白板 (蓝图 §2 字面 "非 markdown 形态")
- ❌ code artifact 上挂锚点 (CV-2 §4 字面留 v3+, 跟 image_link 同)
- ❌ image upload 走 server 存储 (image_link 走外链 URL only, server 存储本地图片留 Phase 5+)
- ❌ admin 看 code/image body (走 god-mode endpoint 不返回 body, ADM-0 §1.3 红线同源)

## 5. Test plan (实施 PR 各自带, 此 spec 不带)

- CV-3.1: migration v=16 → v=17 双向 + **SQLite 12-step 重建表** (CREATE artifacts_new + INSERT SELECT + DROP + RENAME + 重建 idx, transaction 包住) + type enum CHECK reject 'pdf'/'kanban'/'mindmap' + POST /artifacts type='code' 缺 metadata.language → 400 `artifact.invalid_language` + URL javascript:/data: reject + **反向断言**旧立场 ④ 文案 `"type must be 'markdown' (v1)"` 已删 (artifacts.go:253 改 multi-type accept)
- CV-3.2: vitest CodeRenderer prism class hit (Go/TS) + ImageLinkRenderer `<img loading="lazy">` + 反向 grep dangerouslySetInnerHTML 0 hit + mention preview type switch 三分支
- CV-3.3: e2e 创 code/image artifact 渲染 + mention 流内 preview + G3.4 demo 3 截屏入 signoffs/
