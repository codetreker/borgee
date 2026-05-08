# Acceptance Template — CV-3: D-lite 画布渲染 (artifact kind 扩展: code + image_link)

> 蓝图: `canvas-vision.md` §1.2 (D-lite, 不是 Miro) + §1.4 (artifact 集合: Markdown / 代码片段带语言标注 / 设计稿图片或链接 / 看板待办 v2+) + §2 v1 不做清单 (❌ 无限画布 / ❌ 多 artifact 关联 / ❌ CRDT / ❌ PDF / ❌ 看板)
> Spec: `docs/implementation/modules/cv-3-spec.md` (飞马 #363, 3 立场 + 3 拆段 + 9 grep 反查 (5 反约束))
> 文案锁: `docs/qa/cv-3-content-lock.md` (野马 #370, 7 处字面 + XSS 红线两道闸 + 11 行反向 grep)
> 拆 PR (拟): **CV-3.1** schema enum 扩 v=17 + server validation + **CV-3.2** client 三 renderer + mention preview + **CV-3.3** e2e + G3.4 demo 3 截屏归档
> Owner: 战马A 实施 (CV-3 顺位 CV-2 后) / 烈马 验收

## 验收清单

### §1 schema (CV-3.1) — artifacts.kind enum 扩 + server validation

> 锚: 飞马 #363 spec §1 CV-3.1 + CV-1.1 #334 schema 三轴 + #370 ② 11 项语言白名单字面

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 `artifacts.kind` enum CHECK 加 `'code'` / `'image_link'` (CV-1.1 #334 既有 `'markdown'` 不动); migration v=17 (CV-3.1) → v=18 (CV-4.1) sequencing 字面延续 v=14/15/16/17 | migration drift test | 战马C / 烈马 | `internal/migrations/cv_3_1_artifact_kinds_test.go::TestCV31_AcceptsCodeAndImageLinkKinds` PASS (#396 dc7144c) + registry.go v=17 字面锁 |
| 1.2 反约束 — kind CHECK reject `'pdf'` / `'kanban'` / `'mindmap'` (蓝图 §2 v1 不做字面禁) | migration drift test | 飞马 / 烈马 | `cv_3_1_artifact_kinds_test.go::TestCV31_RejectsPdfKanbanMindmap` (6 reject 子断言含 'pdf'/'kanban'/'mindmap'/'doc'/'video'/空) PASS (#396 dc7144c) |
| 1.3 server `POST /artifacts` body validation: `kind=='code'` 必含 `metadata.language` ∈ 11 项白名单 (`go|ts|js|py|md|sh|sql|yaml|json|html|css` + `text` fallback); 缺/外白名单值 → HTTP 400 `artifact.invalid_language` | unit | 战马C / 烈马 | `internal/api/cv_3_2_artifact_validation_test.go::TestIsValidCodeLanguage_11WhitelistPlusText` (12 项接受) + `TestIsValidCodeLanguage_RejectsFullNameSynonyms` (#370 §1 ② 短码唯一防御 — golang/typescript/python/shell/bash/plaintext 全名 + GO/TS/Py/MD 大小写漂移 + rust/c/cpp/java/yml spec 外语言 全 reject) + `TestValidateArtifactMetadata_Code_RequiresLanguage` (7 子: 缺/空 reject + go/text 通过 + golang/typescript/rust 全名/外白名单 reject) PASS (#400 df0b7da) |
| 1.4 server `POST /artifacts` body validation: `kind=='image_link'` 必含 `metadata.kind` ∈ `('image','link')` + URL 校验 https only (反约束 javascript: / data: / http: reject) | unit | 战马C / 烈马 | `cv_3_2_artifact_validation_test.go::TestValidateImageLinkURL_AcceptsHttpsAbsolute` (5 https 变体含 case 不敏感) + `TestValidateImageLinkURL_RejectsNonHttpsSchemes` (XSS 红线第一道, 11 reject — javascript:/data:/data:image/http:/file:/chrome:/chrome-extension:/ftp:) + `TestValidateImageLinkURL_RejectsMalformed` (空/scheme-relative/无 host) + `TestValidateArtifactMetadata_ImageLink_RequiresHttpsOnly` (10 子含 thumbnail XSS reject) PASS (#400 df0b7da) |
| 1.5 反约束 — 不裂表 `artifact_code` / `artifact_images` (立场 ① enum 扩不裂表) | grep | 飞马 / 烈马 | `grep -nE 'CREATE TABLE.*artifact_code\|CREATE TABLE.*artifact_images' packages/server-go/internal/migrations/` count==0 ✅ + `cv_3_1_artifact_kinds_test.go::TestCV31_NoSeparateKindTables` sqlite_master 反向断言 (artifact_code/artifact_images/artifact_image_links 全不存在) PASS (#396 dc7144c) |

### §2 client SPA (CV-3.2) — kind switch 三 renderer + mention preview

> 锚: 飞马 #363 spec §1 CV-3.2 + 野马 #370 文案锁 7 处字面 + DOM 锁

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 `<ArtifactPanel>` kind switch 三分支 — DOM `data-artifact-kind="markdown\|code\|image_link"` byte-identical 跟 #370 ① 同源 (反 camelCase `imageLink` 漂移) | vitest + e2e | 战马D / 烈马 | `packages/client/src/__tests__/ArtifactPanel-kind-switch.test.tsx` PASS (#408 e32d44a, vitest 三 enum DOM 反断) |
| 2.2 `<CodeRenderer>` 11 项语言徽标 byte-identical 跟 #370 ② 同源 — `<span class="code-lang-badge" data-lang="{lang}">{LANG_LABEL[lang]}</span>` 跟 spec §0 ① 11 项白名单 byte-identical (LANG_LABEL = lang 大写) | vitest table-driven | 战马D / 烈马 | `__tests__/CodeRenderer.test.tsx` PASS (#408 e32d44a, table-driven 12 项白名单 + 反向 `golang`/`typescript`/`python` 全名同义词 reject) |
| 2.3 `<CodeRenderer>` 复制按钮 — `<button class="code-copy-btn" title="复制代码" aria-label="复制代码">📋</button>` byte-identical (icon 锁 📋 + title/aria 双绑); 点后 toast `"已复制"` byte-identical 1.5s 自动消 | e2e | 战马D / 烈马 | `__tests__/CodeRenderer.test.tsx` 复制按钮 + toast 单测 PASS (#408 e32d44a); e2e clipboard 真路径走 vitest jsdom 模拟, deferred 真 e2e 留 CV-5+ list endpoint 后切 (#424 cv-3-3-deferred 已锚 §3.1 同模式) |
| 2.4 `<ImageLinkRenderer>` image 分支 — `<img loading="lazy" src="{https_url}" alt="{title}" class="artifact-image">` byte-identical, max-height 480px CSS 锁; **XSS 红线第一道**: src 反向 grep `javascript:\|data:image\|http:` count==0 | vitest + e2e | 战马D / 烈马 | `__tests__/ImageLinkRenderer.test.tsx` PASS (#408 e32d44a, strictly assert loading="lazy" + https only) + e2e `cv-3-3-renderers.spec.ts::§3.2 image_link 协议反向 reject — javascript:/data:/http: 400 (XSS 红线第一道)` PASS |
| 2.5 `<ImageLinkRenderer>` link 分支 — `<a href="{https_url}" target="_blank" rel="noopener noreferrer" class="artifact-link">{title}</a>` byte-identical; **XSS 红线第二道**: vitest 必须 **strictly assert** rel attr 字串原样 (字串 `"noopener noreferrer"` 而非 `includes`, 漏 = reverse-tab XSS leak) | vitest | 战马D / 烈马 | `__tests__/ImageLinkRenderer.test.tsx::link rel byte-identical strict assert` (`expect(rel).toBe("noopener noreferrer")` 字串原样) PASS (#408 e32d44a) |
| 2.6 mention 引用 preview kind 三模式 — markdown 头 80 字符 + ellipsis `…` / code 头 5 行 + 语言徽标 (跟 §2.2 byte-identical) / image 缩略图 `<img loading="lazy" style="max-width: 192px">` byte-identical 跟 #370 ⑥ 同源; 容器 DOM `<span class="artifact-preview" data-artifact-kind="{kind}">` 包裹 | vitest + e2e | 战马D / 烈马 | `__tests__/MentionArtifactPreview.test.tsx` PASS (#408 e32d44a, 三模式 byte-identical); e2e mention 引用 round-trip ⏸️ deferred CV-5+ list endpoint (`cv-3-3-deferred.spec.ts::§3.3` test.fixme 锚, #424 75ad22b) |
| 2.7 kind 兜底文案 (CHECK 外的旧/未来 kind) — `<div class="artifact-kind-unsupported">此 artifact 类型 ({kind}) 暂不支持渲染</div>` byte-identical (中文字面占位 `{kind}` 仅展示原 kind 字串, 不渲染 body, 优雅降级) | vitest | 战马D / 烈马 | `__tests__/ArtifactPanel-kind-switch.test.tsx` 兜底文案 PASS (#408 e32d44a, 反向断言不 throw + 不渲染 body) |
| 2.8 反约束 — 不渲染 raw HTML (XSS 红线): `dangerouslySetInnerHTML.*body` 反向 grep count==0 | grep | 飞马 / 烈马 | `grep -nE 'dangerouslySetInnerHTML.*body\|innerHTML.*=.*body' packages/client/src/components/Artifact*.tsx` count==0 ✅ (#408 e32d44a 全 client renderer 走 React JSX, 0 raw HTML 注入) |

### §3 e2e + G3.4 demo (CV-3.3) — 全流验证 + 截屏归档

> 锚: 飞马 #363 spec §1 CV-3.3 + 章程 G3.4 协作场骨架 demo 退出公告价值

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 e2e 创 code artifact (Go) → 渲染 prism syntax highlight class hit (`prism-token` / `prism-code` 等); 跟 CV-1.3 #348 markdown render e2e 模式 byte-identical | e2e | 战马A / 烈马 | ⏸️ **deferred CV-5+ list endpoint** (ArtifactPanel v1 仅 setArtifact via handleCreate UI, default type='markdown'; code-kind 走 REST 直接创但 panel 不渲染. `packages/e2e/tests/cv-3-3-deferred.spec.ts::§3.1 code artifact (Go) prism syntax highlight class hit` test.fixme 占位锚, closure path body comment 已写 — list endpoint 落地后切真路径) |
| 3.2 e2e 创 image_link artifact (https URL) → `<img>` DOM 验 + `loading="lazy"` 实测; 反向断言 javascript:/data:/http: URL reject 400 | e2e | 战马A / 烈马 | `packages/e2e/tests/cv-3-3-renderers.spec.ts::§3.2 image_link 协议反向 reject — javascript:/data:/http: 400 (XSS 红线第一道)` PASS (#408, REST 协议反断 server-side; UI 渲染 `<img loading="lazy">` 走 vitest ImageLinkRenderer.test.tsx 单测 byte-identical) |
| 3.3 e2e mention 引用 code/image artifact → 流内 preview 缩略 (跟 §2.6 byte-identical, 跟 CV-1.3 #348 §3.3 模式同) | e2e | 战马A / 烈马 | ⏸️ **deferred CV-5+ list endpoint** (mention `<artifact:id>` token preview 走 GET /artifacts/:id 已通, 但 3 kinds round-trip e2e 需 list view 验证创建; vitest `MentionArtifactPreview.test.tsx` 单测 3 kinds DOM 字面锁 byte-identical 已闭. `cv-3-3-deferred.spec.ts::§3.3 mention preview kind 三模式` test.fixme 占位锚) |
| 3.4 G3.4 demo 截屏 3 张 byte-identical 入 `docs/qa/screenshots/g3.4-cv3-{markdown,code-go-highlight,image-embed}.png` (撑章程退出公告, 跟 G2.4#5 / G2.5 / G2.6 / G3.x 同模式) | Playwright `page.screenshot()` | 战马A / 野马 / 烈马 | 🟢 1/3: `g3.4-cv3-markdown.png` PASS (#408 `cv-3-3-renderers.spec.ts::§3.4 G3.4 demo markdown 截屏`) ;  ⏸️ 2/3 deferred CV-5+ list endpoint: `g3.4-cv3-code-go-highlight.png` + `g3.4-cv3-image-embed.png` (`cv-3-3-deferred.spec.ts::§3.4 ... code/image_link` test.fixme 占位锚, list endpoint 落地后切真路径 + page.screenshot path-locked) |

### §4 反向 grep / e2e 兜底 (跨 CV-3.x 反约束)

> 锚: 飞马 #363 spec §3 5 反约束 grep + 野马 #370 §2 11 行反向 grep (含 XSS 红线两道闸)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 反约束 — CRDT 0 hit: `grep -rnE 'CRDT\|yjs\|automerge' packages/client/ packages/server-go/` count==0 (蓝图 §2 字面 "CRDT 巨坑不踩") | CI grep | 飞马 / 烈马 | _(每 CV-3.* PR 必跑)_ |
| 4.2 反约束 — javascript:/data:/http: image src 0 hit: `grep -rnE 'src=\{?["'"'"']?(javascript:\|data:image\|http:)' packages/client/src/components/ImageLinkRenderer.tsx` count==0 | CI grep | 飞马 / 烈马 | _(每 CV-3.2/3.3 PR 必跑)_ |
| 4.3 反约束 — kind 同义词 / camelCase 漂移 0 hit: `grep -rnE "['\"]( imageLink\|code_image\|pdf\|kanban\|mindmap)['\"]" packages/client/src/components/Artifact*.tsx` count==0 | CI grep | 飞马 / 烈马 | _(每 CV-3.2 PR 必跑)_ |
| 4.4 反约束 — 全名语言同义词 0 hit (短码唯一): `grep -rnE "['\"]( golang\|typescript\|python\|shell\|bash\|plaintext)['\"]" packages/client/src/components/CodeRenderer.tsx` count==0 | CI grep | 飞马 / 烈马 | _(每 CV-3.2 PR 必跑)_ |
| 4.5 反约束 — DOM `[data-artifact-kind="markdown\|code\|image_link"]` ≥ 3 (三 enum 各 ≥1, 漏 = 视觉混淆) | CI grep | 飞马 / 烈马 | `grep -rnE 'data-artifact-kind=["'"'"'](markdown\|code\|image_link)["'"'"']' packages/client/src/components/ArtifactPanel.tsx` count≥3 |

## 边界 (跟其他 milestone 关系)

| Milestone | 关系 | 字面承袭 |
|---|---|---|
| CV-1 ✅ | markdown 路径不破 (kind='markdown' 老 artifact 渲染保持 #346 既有) | CV-1.3 ArtifactPanel kind switch 入口 byte-identical |
| CV-2 #356/#360 | anchor 仅挂 markdown artifact (server 前置 `anchor.unsupported_artifact_kind` 403) — code/image 加锚留 v3+ | CV-2 §4 反约束 字面承袭 |
| CV-4 #365 | iterate 适用所有 kind (intent 文本同适用); diff view code 走 prism + jsdiff 行级, image_link 走前后缩略图并排 fallback | CV-4 立场 ③ client diff 字面承袭 |
| RT-1 ✅ | ArtifactUpdated frame `kind` 字段已存在, 复用同 envelope 不另起 | cursor.go::ArtifactUpdatedFrame.Kind byte-identical |
| BPP-1 ✅ #304 | envelope CI lint 自动覆盖 frame 字段顺序 | reflect 比对 server-go 端字段顺序 |

## 退出条件

- §1 schema 5 项 + §2 client 8 项 + §3 e2e + demo 4 项 + §4 反向 grep 5 项**全绿** (一票否决)
- XSS 红线两道闸全 0 hit (image src 协议 + link rel strictly assert byte-identical)
- 登记 `docs/qa/regression-registry.md` REG-CV3-001..017 (5 schema + 8 client + 3 e2e + 1 demo 截屏归档)
- G3.4 demo 截屏 3 张归档 (撑章程 Phase 3 退出公告)
- v=14-17 sequencing 字面延续 (CV-2.1 ✅ / DM-2.1 ✅ / AL-4.1 v=16 / **CV-3.1 v=17**)
