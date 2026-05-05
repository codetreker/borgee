# QA Acceptance Templates — 烈马交付

> 作者: 烈马 (QA) · 2026-04-28 · R3 派活后预备
> 用途: 4 个 R3 新 milestone 的 acceptance checklist 模板, 派活时直接交付实施人。
> 更新: 任何 milestone 落地 PR 后, 烈马把"实施证据"列回填 (PR # / 单测路径 / 截屏路径)。

## 模板规范

每个 milestone 一个文件, 表头固定 4 列:

| 验收项 | 实施方式 | Owner | 实施证据 |

**实施方式枚举** (与 onboarding-journey.md §6 对齐):
- `E2E` — Playwright (INFRA-2 落地后)
- `unit` — Go server test / vitest client test
- `CI grep` — lint job (forbidden-strings / source pattern check)
- `人眼` — PR review / demo signoff (流程级)

**Owner 枚举**: 飞马 / 战马 / 野马 / 烈马 (单一 owner; 跨人协作用 `/` 分隔, 主导在前)

## 文件清单 (派活顺序)

| Milestone | 文件 | 状态 |
|---|---|---|
| INFRA-2 (Playwright scaffold) | `infra-2.md` | ✅ (基础设施验收: config / smoke test / CI / 命令入口) |
| ADM-0 (admin 拆表 3 段 PR) | `adm-0.md` | ✅ |
| AP-0-bis (message.read 默认) | `ap-0-bis.md` | ✅ |
| RT-0 (/ws push 顶 BPP) | `rt-0.md` | ✅ |
| CM-onboarding (Welcome channel) | `cm-onboarding.md` | ✅ |

## 引用蓝图 / R3 决议

- 蓝图固化: PR #188 (merged)
- implementation 重排: PR #189 (merged)
- onboarding journey: PR #190
- 立场冲突对照表: `docs/_archive/conflicts/b29-vs-blueprint.md`
