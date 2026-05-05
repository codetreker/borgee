# CS-1 spec brief — 三栏布局 + Artifact 分级展开 (≤80 行)

> 飞马 · 2026-04-30 · Phase 4+ Client Shape 主体框架 (蓝图 client-shape.md §1.2)
> **蓝图锚**: [`client-shape.md`](../../blueprint/current/client-shape.md) §1.2 (主界面: 三栏 + 顶部团队栏 + Artifact 分级展开) + §0 (Web SPA 是协作主战场) + §1.4 (本地持久化乐观缓存)
> **关联**: CHN-9 #553 visibility 三态 (private/public/creator_only) + CV-1..14 artifact panel 既有渲染 + RT-4 #562 channel presence (顶部团队栏数据源) + AL-1b 5-state (头像状态色环)
> **命名**: CS-1 = Client Shape 第一段 — 三栏主体框架 (CS-2 留: 顶部团队栏故障 UX, CS-3 留: PWA install + Web Push)

> ⚠️ Wrapper milestone — 复用既有 component 全部 (Sidebar / ChannelList / MessageTimeline / ArtifactPanel),
> 仅落 layout container 三栏 + Artifact 分级展开状态机 + 移动 drawer 降级.
> **0 server 改 + 0 schema 改** — 纯 client refactor.

## 0. 关键约束 (3 条立场, 蓝图字面承袭)

1. **三栏布局 byte-identical 跟蓝图 §1.2 ASCII 图** (Sidebar 频道列表 + 主区聊天 + Artifact 抽屉/split): `<AppShell>` container 三 grid columns (\`grid-template-columns: 240px 1fr 380px\`); 反约束: 不另起 layout 系统 (复用既有 CSS grid + 跟 ChannelList / MessageTimeline / ArtifactPanel 既有组件 byte-identical 接入); 移动 (≤768px) 降级: 顶部团队栏 → drawer + Sidebar → drawer + 主区单栏 + Artifact split → 全屏 modal.

2. **Artifact 分级展开状态机** (蓝图 §1.2 表格 byte-identical): 4 态 \`closed\` (无 artifact 引用) / \`drawer\` (首次点击 artifact 引用 → 右侧抽屉轻量预览 380px) / \`split\` (显式动作 = 拖拽 artifact panel 边界 OR 二次点击 → 主区聊天 + artifact 并存 50/50 split) / \`fullscreen\` (mobile 降级时 modal 全屏). 反约束: 不允许 \`split\` 直接打开 (必先经 \`drawer\` 一次点击, 跟蓝图 §1.2 "避免自动劈开屏幕" 字面立场); state 单源 \`useArtifactPanel()\` hook (不另起多 state); 反向 grep \`SplitView.*directOpen\|artifact.*autoSplit\` count==0.

3. **0 server 改 + 0 schema 改 + 0 新 endpoint** (Wrapper milestone 立场, 跟 CV-9..14 / DM-5..6 / DM-9 / CHN-11..12 系列 0-server-prod 同模式承袭): 反向 grep \`packages/server-go/internal/\` git diff count==0; 不引入 cs_1 命名 server file; 反向 grep \`migrations/cs_1\|cs1.*api\|cs1.*server\` count==0. **真符合三选项决策树选项 C** (0-server-no-schema). 用户偏好 (column 宽度 / drawer width) 用 localStorage (蓝图 §1.4) — 不上 server.

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| **CS-1.1** AppShell + 三栏 layout | `packages/client/src/components/AppShell.tsx` (新 ≤120 行) — CSS grid 三栏 + responsive media query (≤768px → drawer 降级); `lib/use_artifact_panel.ts` (新, ≤60 行) — 4 态 state machine + transition guards (closed → drawer → split → fullscreen, 反向 closed → split 直接 reject); `App.tsx` 改: 用 `<AppShell>` 包既有 Sidebar/MessageTimeline/ArtifactPanel (≤20 行 wiring) | 战马C |
| **CS-1.2** Artifact 分级展开 + drawer | `components/ArtifactDrawer.tsx` (新, ≤80 行) — drawer 380px 右侧 slide-in + close button + drag-handle (拖到中间 → split 升级); `ArtifactPanel.tsx` (改 ≤10 行) — accept `mode` prop (drawer / split / fullscreen) 渲染 byte-identical 既有内容; 移动 drawer 降级测试 + 拖拽手势 (touch event 触发 split 升级) | 战马C |
| **CS-1.3** closure | REG-CS1-001..006 + acceptance + content-lock + PROGRESS [x] CS-1 + 4 件套 + docs/current sync (`docs/current/client/app-shell.md` ≤80 行 — 三栏 layout 字面 + 4 态 state 字面 + 移动降级断点 + drawer/split 触发 UX) + e2e (`packages/e2e/tests/cs-1-three-pane-layout.spec.ts` 5 case: 三栏渲染 / 点 artifact 引用 → drawer / 拖动 → split / 移动 ≤768px → drawer 降级 / 反向断 closed → split 不允许) | 战马C / 烈马 |

## 2. 反向 grep 锚 (5 反约束, count==0)

```bash
# 1) 0 server 改 (Wrapper milestone 立场 ③)
git diff origin/main -- packages/server-go/  | grep -c '^\+'  # 0 production lines

# 2) 不另起 layout 系统 (复用 既有 CSS grid)
git grep -nE 'styled-components|emotion|tailwind\..*layout' packages/client/src/components/AppShell.tsx  # 0 hit

# 3) artifact 自动 split 反向 (蓝图 §1.2 "避免自动劈开屏幕")
git grep -nE 'SplitView.*directOpen|artifact.*autoSplit|setMode\("split"\)' packages/client/src/  # 0 hit (除 ArtifactDrawer drag handler)

# 4) 三栏字面 byte-identical 跟蓝图 (240px / 380px / 768px breakpoint)
git grep -nE '240px|380px|768px' packages/client/src/components/AppShell.tsx  # ≥3 hit

# 5) 文案 byte-identical (mobile drawer 切换 button '团队' / '频道' 中文 + 同义词反向)
git grep -nE 'sidebar|channels-toggle|menu-icon' packages/client/src/__tests__/AppShell.test.tsx  # 用 data-testid 锚, 不漂同义词
```

## 3. 不在范围 (留账)

- ❌ 顶部团队栏故障 UX 4 层呈现 (蓝图 §1.3) — 留 **CS-2** 单 milestone (含 头像角标 + inline 修复 + banner + 故障中心)
- ❌ PWA install + Web Push (蓝图 §1.1 PWA 范围) — 留 **CS-3** 单 milestone (依赖 DL-4 push gateway #485 ✅ merged)
- ❌ Tauri 壳 + host-bridge daemon — 留 HB-2 (依赖 HB-1)
- ❌ IndexedDB 乐观缓存 (蓝图 §1.4) — 留 **CS-4** 单 milestone (跟 RT-1 cursor 协议同期)
- ❌ Artifact split 拖拽 50/50 → 60/40 等比例自定义 — v3+
- ❌ 多 artifact 并存 split (≥2 artifact) — v3+ (v1 仅 1 artifact 同时)

## 4. 跨 milestone byte-identical 锁

- 复用 ChannelList (CHN-3 / CHN-13 既有) byte-identical 不动 (CS-1 仅 layout 包裹)
- 复用 MessageTimeline (CV-1..14 + DM-3..7 既有) byte-identical 不动
- 复用 ArtifactPanel (CV-1 #347 既有 byte-identical, 仅加 mode prop ≤10 行)
- 复用 RT-4 #562 ChannelPresenceList 顶部团队栏数据 (本 spec 不实施团队栏 UI, 留 CS-2)
- AL-1b 5-state 头像色环 (CS-2 实施时复用既有 PresenceDot)
- 0-server-prod 系列模式承袭 (CV-9..14 / DM-5..6 / DM-9 / CHN-11..12 / CS-1 第 13 处)

## 5. 验收挂钩

- REG-CS1-001..006 (5 反向 grep + 4 态 state machine 单测 + e2e 3 viewport)
- 既有 ChannelList / MessageTimeline / ArtifactPanel unit tests 全 PASS (Wrapper 不破)
- vitest AppShell.test.tsx (5 case: 三栏渲染 / drawer 触发 / split 升级 / mobile 降级 / closed → split 反向 reject)
