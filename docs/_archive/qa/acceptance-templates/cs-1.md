# CS-1 三栏 + Artifact 分级 — acceptance template v0

> Anchor: `docs/implementation/modules/cs-1-spec.md` v0
> Stance: `docs/qa/cs-1-stance-checklist.md` §1-§2

## §1 三栏 layout (spec §0 立场 ①)

- **§1.1** computeGridColumns 4 desktop mode byte-identical 跟蓝图: closed=`240px 1fr` / drawer=`240px 1fr 380px` / split=`240px 1fr 1fr` / fullscreen=`240px 1fr`
- **§1.2** Mobile (≤768px breakpoint): grid `1fr` 单列, sidebar 走 overlay (sidebarOpen=true), artifact split 走 fullscreen modal
- **§1.3** APP_SHELL_DESKTOP_SIDEBAR=240 / APP_SHELL_DESKTOP_DRAWER=380 / APP_SHELL_MOBILE_BREAKPOINT=768 字面 byte-identical 跟蓝图 §1.2

## §2 Artifact 4 态 state machine (spec §0 立场 ②+③)

- **§2.1** 4 态: closed / drawer / split / fullscreen byte-identical
- **§2.2** open(id) closed → drawer 允许; drawer/split/fullscreen 仅切 artifactId
- **§2.3** promoteToSplit drawer → split 返回 true; **closed → split 直接 reject 返回 false** ⭐ 立场 ③ 反向断言
- **§2.4** demoteToDrawer split → drawer; close any → closed (artifactId 清)
- **§2.5** setFullscreen(true) drawer → fullscreen, setFullscreen(false) → drawer; closed 状态保持 closed

## §3 0-server-no-schema (spec §0 立场 ④)

- **§3.1** git diff origin/main -- packages/server-go/ count==0
- **§3.2** 反向 grep `cs_1.*api|cs1.*server|migrations/cs_1` 0 hit
- **§3.3** 反向 grep `styled-components|emotion|tailwind` 在 AppShell.tsx 0 hit (复用既有 CSS grid)
- **§3.4** 反向 grep `SplitView.*directOpen|artifact.*autoSplit` 在 client/src 0 hit (反自动劈开屏幕)

## §4 Wrapper milestone 不破 (spec §0 立场 ⑤)

- **§4.1** 既有 ChannelList / ChannelView / ArtifactPanel byte-identical 不动
- **§4.2** 既有 558 vitest 全 PASS (Wrapper 加 22 新 vitest, total 580 PASS)
- **§4.3** typecheck clean (tsc --noEmit 0 error)

## REG (本 PR closure 翻 🟢)

| Reg ID | Source | Test path / grep | Owner | Status |
|---|---|---|---|---|
| REG-CS1-001 | spec §0 立场 ① — 三栏 byte-identical 蓝图 §1.2 | `AppShell.test.tsx::computeGridColumns: 4 desktop modes byte-identical` + `mobile single column` 2 case PASS | 战马C / 飞马 | 🟢 active |
| REG-CS1-002 | spec §0 立场 ② — Artifact 4 态 state machine 单源 | `useArtifactPanel.test.tsx` 9 case PASS (initial / open / promote / demote / close / setFullscreen) | 战马C / 烈马 | 🟢 active |
| REG-CS1-003 | spec §0 立场 ③ — closed → split 直接 reject (反 自动劈开屏幕) | `useArtifactPanel.test.tsx::promoteToSplit: closed → no-op returns false` PASS | 战马C / 飞马 / 烈马 | 🟢 active |
| REG-CS1-004 | spec §0 立场 ④ — 0 server / 0 schema / 0 endpoint | `git diff origin/main -- packages/server-go/` 0 lines + 反向 grep `cs_1.*api\|cs1.*server\|migrations/cs_1` 0 hit | 战马C / 飞马 | 🟢 active |
| REG-CS1-005 | spec §0 立场 ⑤ — Wrapper 不破既有 component (CV-1 ArtifactPanel + CHN-* ChannelView byte-identical) | full client vitest 79 files / 558 tests 全 PASS (post-CS-1: 580 tests, 22 新加 0 既有破) | 战马C / 烈马 | 🟢 active |
| REG-CS1-006 | content-lock §1 — DOM 锚 byte-identical (`data-testid="app-shell" / data-artifact-mode / data-mobile / data-testid="artifact-drawer" data-mode / data-testid="artifact-drawer-close"` 5 字面) | `AppShell.test.tsx` + `ArtifactDrawer.test.tsx` 13 case PASS (含 DOM data-testid + aria-label literal) | 战马C / 野马 / 烈马 | 🟢 active |

## 退出条件

- §1-§4 全 PASS
- REG-CS1-001..006 全 🟢 active
- 22 新 vitest 全 PASS + 既有 558 不破 + typecheck clean
