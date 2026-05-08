# CS-1 三栏 + Artifact 分级 — PM stance checklist

> Anchor: `docs/implementation/modules/cs-1-spec.md` v0 §0-§5
> Mode: client wrapper milestone (0 server / 0 schema / 0 endpoint).

## 1. 立场 (5 项)

1. **三栏 byte-identical 跟蓝图 §1.2** (`240px / 1fr / 380px` + 768px breakpoint) — `<AppShell>` CSS grid 单源, 反向 grep `styled-components|emotion|tailwind` 在 AppShell.tsx 0 hit
2. **Artifact 4 态 state machine 单源** (`useArtifactPanel` hook, 4 态 closed/drawer/split/fullscreen) — 反向: 不允许多 state 表达 mode
3. **closed → split 直接 reject** (蓝图 §1.2 "避免自动劈开屏幕" 字面立场) — `useArtifactPanel.promoteToSplit()` 在 closed 状态返回 false (反向断言单测覆盖)
4. **0 server / 0 schema / 0 endpoint** — git diff origin/main -- packages/server-go/ count==0; 反向 grep `cs_1.*api|cs1.*server|migrations/cs_1` 0 hit
5. **Wrapper milestone 不破既有组件** — 复用 ChannelList / ChannelView / ArtifactPanel byte-identical (CS-1 仅加 layout container + drawer wrap, 不动既有内部渲染)

## 2. 黑名单 grep (反向断言, count==0)

| Pattern | Where | 立场 |
|---|---|---|
| git diff origin/main -- packages/server-go/ | server prod | §1 ④ 0 server |
| `cs_1.*api\|cs1.*server\|migrations/cs_1` | 全仓 | §1 ④ 0 endpoint |
| `styled-components\|emotion\|tailwind` 在 AppShell.tsx | client | §1 ① 复用 既有 CSS grid |
| `SplitView.*directOpen\|artifact.*autoSplit` | client | §1 ③ 反自动劈开屏幕 |
| 多 state 表达 mode (`useState<.*Mode\|useReducer.*Mode>`) 在 AppShell/ChannelView | client | §1 ② state 单源 |

## 3. 不在范围 (留账)

- ❌ 顶部团队栏故障 UX (CS-2 #595)
- ❌ PWA install + Web Push (CS-3 #598)
- ❌ Tauri 壳 + host-bridge daemon (HB-2)
- ❌ IndexedDB 乐观缓存 (CS-4 留 v3+)
- ❌ Artifact split 比例自定义 (50/50 → 60/40 等, v3+)
- ❌ 多 artifact 并存 split (≥2 artifact, v3+)

## 4. 验收挂钩

- §1 ① → REG-CS1-001 (computeGridColumns 4 desktop + 4 mobile mode byte-identical)
- §1 ② → REG-CS1-002 (useArtifactPanel 9 case state machine 全 PASS)
- §1 ③ → REG-CS1-003 (closed → split promoteToSplit 返回 false 单测)
- §1 ④ → REG-CS1-004 (反向 grep 0 hit + git diff server-go 0)
- §1 ⑤ → REG-CS1-005 (既有 ChannelView/ArtifactPanel byte-identical 不破, 现有 558 vitest 全 PASS)
- 文案锁 → REG-CS1-006 (data-testid + aria-label 字面 byte-identical 跟 content-lock §1)
