# CV-3 D-lite 文案锁 (野马 G3.4 demo 预备)

> **状态**: v0 (野马, 2026-04-29)
> **目的**: CV-3.x client UI 实施前锁 D-lite (artifact kind 扩展) 文案 + DOM 字面 + URL/XSS 反约束 — 跟 AL-3 #305 / DM-2 #314 / AL-4 #321 / CHN-2 #354 / CV-2 #355 同模式 (用户感知签字 + 文案 byte-identical), 防 CV-3 实施时把 D-lite 退化成 Miro/CRDT 同义词 / XSS 漏开。**4 件套并行**, 跟飞马 #363 spec / 烈马 acceptance template (待派) / 战马A 实施同步起。
> **关联**: `canvas-vision.md` §1.2 (D-lite, 不是 Miro) + §1.4 (artifact 集合: Markdown / 代码片段带语言标注 / 设计稿图片或链接 / 看板 v2+) + §2 v1 不做清单 (❌ 无限画布 / ❌ 多 artifact 关联 / ❌ CRDT / ❌ PDF / ❌ 看板); 飞马 CV-3 spec brief #363 §0 立场 ① enum 扩 / §0 ② 三 renderer / §0 ③ mention preview kind 三模式; CV-1 #346 ArtifactPanel.tsx + #347 kindBadge byte-identical 同源; CV-2 #355 ⑤ agent 不能开 thread 反约束三连同精神。
> **#338 cross-grep 反模式遵守**: 既有实施 `lib/markdown.ts:14-28` hljs 已映射 8 语言 (js/ts/py/css/json/sh/html/sql/md), CV-3 spec 用 `prism-react-renderer` 是**新组件路径** — 字面池干净, 但语言白名单字面**必须**跟 spec #363 §0 ① 11 项 byte-identical 同源, 不抄 markdown.ts 子集 (markdown.ts 是 markdown 内代码块路径, CV-3 是 artifact-kind=code 独立路径, 两路径并存不冲突)。

---

## 1. 7 处文案 + DOM 字面锁

| # | 场景 | 字面锁 (byte-identical) | 反约束 |
|---|------|-----|------|
| ① | **kind switch 三 renderer 入口** (ArtifactPanel) | DOM: `<div data-artifact-kind="markdown|code|image_link">` byte-identical (跟 #363 §0 ① 三 enum 字面同源, `image_link` 单字串带下划线锁); switch 顺序 markdown → code → image_link | ❌ 不准 `data-artifact-kind` attr 缺失 (e2e DOM grep 命中); ❌ 不准混入 `'pdf'/'kanban'/'mindmap'/'code_image'/'imageLink'` 同义词 (蓝图 §2 v1 不做 + camelCase 漂移防御) |
| ② | **代码 artifact 语言徽标** (CodeRenderer 头部) | DOM: `<span class="code-lang-badge" data-lang="{lang}">{LANG_LABEL[lang]}</span>` 11 项白名单 byte-identical 跟 #363 §0 ① 同源: `go|ts|js|py|md|sh|sql|yaml|json|html|css` + `text` (fallback 12 项); LANG_LABEL 字面 = lang 大写 (`go→GO`, `ts→TS`, `text→TEXT`) | ❌ 不准 `'golang'/'typescript'/'python'/'shell'/'bash'/'plaintext'` 全名同义词 (跟 spec 11 项 byte-identical 锁, 短码唯一); ❌ 不准添加 spec 外语言 (#363 §3 反向 grep `kind.*CHECK.*'pdf'/'kanban'/'mindmap'` count==0 同精神 — 白名单收窄) |
| ③ | **代码块复制按钮** (CodeRenderer 右上角) | DOM: `<button class="code-copy-btn" title="复制代码" aria-label="复制代码">📋</button>` byte-identical (icon 锁 📋 + title/aria 中文双绑); 点后 toast `"已复制"` byte-identical 1.5s 自动消 | ❌ 不准 "Copy" / "Copy to clipboard" / "复制到剪贴板" / "拷贝" 同义词漂移; ❌ 不准复制按钮在非 code kind 上渲染 (DOM 反约束: `data-artifact-kind="markdown|image_link"` 子树无 `.code-copy-btn`) |
| ④ | **图片 artifact 渲染锁** (ImageLinkRenderer image 分支) | DOM: `<img loading="lazy" src="{https_url}" alt="{title}" class="artifact-image">` byte-identical (跟 #363 §1 CV-3.2 字面); max-height: 480px CSS 锁; src 必 https 协议 (反约束: `javascript:|data:image|http:` count==0) | ❌ 不准 `src` 含 `javascript:` / `data:image` / `http:` (XSS + 混合内容红线, #363 §3 同源 grep); ❌ 不准 `loading="eager"` (移动端流量保护); ❌ 不准 `<img>` 在 link 分支渲染 (kind 二元拆死) |
| ⑤ | **链接 artifact 渲染锁** (ImageLinkRenderer link 分支) | DOM: `<a href="{https_url}" target="_blank" rel="noopener noreferrer" class="artifact-link">{title}</a>` byte-identical (rel 三联锁 noopener+noreferrer 防 reverse-tab XSS) | ❌ 不准缺 `rel="noopener noreferrer"` (实施 vitest 必须 strictly assert rel 字串原样, 漏 = reverse-tab XSS leak); ❌ 不准 `target="_self"` (跳走 SPA 上下文坏体验); ❌ 不准 `href` 含 `javascript:|data:` |
| ⑥ | **mention 引用 preview kind 三模式** (message 流内 `<artifact:id>` token 渲染) | markdown: 头 80 字符 + ellipsis `…`; code: 头 5 行 + 语言徽标 (跟 ② byte-identical); image: 缩略图 `<img loading="lazy" style="max-width: 192px">` byte-identical (跟 #363 §0 ③ 同源); preview 容器 DOM `<span class="artifact-preview" data-artifact-kind="{kind}">` 包裹 | ❌ 不准 markdown preview > 80 字 (隐私 + 流内噪声防御); ❌ 不准 code preview > 5 行 (噪声防御); ❌ 不准 image preview > 192px (流内布局保护); ❌ 不准 link preview 渲染 image (二元拆死) |
| ⑦ | **kind 不支持的兜底文案** (CHECK 外的旧/未来 kind) | DOM: `<div class="artifact-kind-unsupported">此 artifact 类型 ({kind}) 暂不支持渲染</div>` byte-identical (中文字面, 占位 `{kind}` 仅展示原 kind 字串, 不渲染 body) | ❌ 不准 throw error (优雅降级); ❌ 不准 fallback 渲染 markdown (kind 跨界污染); ❌ 不准 "Unsupported" / "未支持" / "类型错误" 同义词漂移 |

---

## 2. 反向 grep — CV-3.x PR merge 后跑, 全部预期 0 命中 (除 ≥1 标记)

```bash
# ① data-artifact-kind 必有 + 三 enum 字面
grep -rnE 'data-artifact-kind=["'"'"'](markdown|code|image_link)["'"'"']' packages/client/src/components/ArtifactPanel.tsx | grep -v _test  # 预期 ≥3 (三 enum 各 ≥1)
# ① camelCase / 同义词漂移防御
grep -rnE "['\"](imageLink|code_image|pdf|kanban|mindmap)['\"]" packages/client/src/components/Artifact*.tsx 2>/dev/null | grep -v _test
# ② 语言白名单 11 项 byte-identical (短码唯一)
grep -rnE "['\"](golang|typescript|python|shell|bash|plaintext)['\"]" packages/client/src/components/CodeRenderer.tsx 2>/dev/null | grep -v _test
# ③ 复制按钮文案 byte-identical
grep -rnE "['\"](Copy|Copy to clipboard|复制到剪贴板|拷贝)['\"]" packages/client/src/components/CodeRenderer.tsx 2>/dev/null | grep -v _test
# ④ image src URL 协议白名单 (XSS 红线第一道)
grep -rnE 'src=\{?["'"'"']?(javascript:|data:image|http:)' packages/client/src/components/ImageLinkRenderer.tsx 2>/dev/null | grep -v _test
# ④ loading="eager" 流量防御
grep -rnE 'loading=["'"'"']eager["'"'"']' packages/client/src/components/ImageLinkRenderer.tsx 2>/dev/null | grep -v _test
# ⑤ rel 三联锁 byte-identical (reverse-tab XSS 第二道)
grep -rnE 'rel=["'"'"']noopener noreferrer["'"'"']' packages/client/src/components/ImageLinkRenderer.tsx 2>/dev/null | grep -v _test  # 预期 ≥1
# ⑤ target="_self" 防御
grep -rnE 'target=["'"'"']_self["'"'"']' packages/client/src/components/ImageLinkRenderer.tsx 2>/dev/null | grep -v _test
# ⑥ mention preview 容器锁
grep -rnE 'class=["'"'"']artifact-preview["'"'"']' packages/client/src/components/ 2>/dev/null | grep -v _test  # 预期 ≥1
# ⑦ 兜底文案漂移防御
grep -rnE "['\"](Unsupported|未支持|类型错误|Unknown kind)['\"]" packages/client/src/components/Artifact*.tsx 2>/dev/null | grep -v _test
```

---

## 3. 验收挂钩 (CV-3.x PR 必带)

- ① ArtifactPanel kind switch e2e: 创三 kind artifact → DOM `data-artifact-kind` 三态 byte-identical (跟 #363 §0 ① 同源)
- ② CodeRenderer 11 项语言徽标 vitest table-driven (`go|ts|js|py|md|sh|sql|yaml|json|html|css` + `text` fallback 12 项, 大写 LANG_LABEL byte-identical)
- ③ 复制按钮 e2e: 点击 → clipboard.writeText 触发 + toast `"已复制"` 1.5s + 反向 grep 同义词 0 hit
- ④ ImageLinkRenderer image 分支 e2e + vitest: `<img loading="lazy" src="https://...">` byte-identical + URL 协议反约束反向断言 (javascript:/data:/http: reject)
- ⑤ ImageLinkRenderer link 分支 vitest **strictly assert** `rel="noopener noreferrer"` 字串原样 (reverse-tab XSS 单测) + `target="_blank"` byte-identical
- ⑥ mention preview kind 三模式 e2e: `<artifact:id>` token 流内渲染 — markdown 80字截断 / code 5行+语言徽标 / image 192px 缩略 byte-identical
- ⑦ 兜底文案 e2e: 模拟未来 kind='kanban' (mock CHECK 跳过) → DOM `artifact-kind-unsupported` + 文案 byte-identical, 不渲染 body
- G3.4 demo 截屏 3 张归档 (跟 G2.4#5 / G2.5 / G2.6 / G3.x 同模式): `docs/qa/screenshots/g3.4-cv3-{markdown,code-go-highlight,image-embed}.png` (CI Playwright `page.screenshot()`, 撑章程退出公告)

---

## 4. 不在范围

- ❌ Miro 无限画布 / 拖拽连线 / 多 artifact 关联视图 (蓝图 §2 字面禁; #363 §0 ② 反约束)
- ❌ CRDT 多人实时编辑 (蓝图 §2 字面 "CRDT 巨坑"); ❌ PDF / 看板 / 思维导图 (蓝图 §2 v1 不做)
- ❌ code artifact 锚点对话 (CV-2 §4 反约束留 v3+; #363 §2 字面对齐)
- ❌ image upload 走 server 存储 (image_link 走外链 https URL only, server 存留 Phase 5+)
- ❌ admin SPA 看 code/image body (admin god-mode 字段白名单不含 body, ADM-0 §1.3 红线; 跟 CV-1/CV-2/DM-2 同源)
- ❌ 代码 artifact diff view (CV-4 锁, 不在 CV-3 范围); ❌ image 注释 / 锚点 (留 v3+)
- ❌ 自定义代码主题 / 行号开关 (留 v3+); ❌ 复制按钮键盘快捷键 (a11y v3+)

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 野马 | v0, 7 处文案锁 (kind switch DOM 三 enum + 语言徽标 11 项 byte-identical 跟 #363 §0 ① 同源 + 复制按钮 📋 "复制代码"/"已复制" + image `loading="lazy"` https only + link `rel="noopener noreferrer"` 三联锁 + mention preview kind 三模式 + kind 兜底 "暂不支持渲染") + 11 行反向 grep (含 XSS 红线两道闸 + reverse-tab 防御 + 同义词漂移防御) + G3.4 demo 截屏 3 张预备. #338 cross-grep 反模式遵守: `lib/markdown.ts` hljs 8 语言是 markdown 内代码块路径, CV-3 prism CodeRenderer 是 artifact-kind=code 独立路径, 两路径并存; 语言白名单字面跟 #363 §0 ① 11 项 byte-identical, 不抄 markdown.ts 子集 |
