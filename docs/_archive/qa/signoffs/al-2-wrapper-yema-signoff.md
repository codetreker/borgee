# AL-2 wrapper ⭐ 4.2 demo 签字 — 野马 (placeholder, release 前真补)

> **Status**: ⏸️ deferred — release 前真补. 跟 HB-4 yema-signoff 同模式 (4.2 demo 签字独立路径, 不混入 4.1 行为不变量数字化清单).
> **Owner**: 野马 (主, demo 签字) — "作为 owner 我看得懂 agent 的状态变化"
> **关联**: spec `docs/implementation/modules/al-2-wrapper-spec.md` + acceptance `docs/qa/acceptance-templates/al-2-wrapper.md` §3.3 + release gate `docs/release/agent-lifecycle-release-gate.md`

## 签字字面 (蓝图 §2.3 字面要求)

野马在 release 前 demo 完整跑一遍 agent lifecycle, 提交以下 3 张截屏 + 一句签字:

### 截屏 1: 5-state UI 渲染

- **路径**: agent 列表 → 任一 agent → 状态指示
- **预期 DOM**: PresenceDot 5-state describeAgentState (online/busy/idle/error/offline)
- **截屏存**: `docs/evidence/al-2-wrapper/01-5-state-ui.png`

### 截屏 2: error → online 反向链 (BPP-5 reconnect)

- **触发**: kill plugin → 30s 后 agent 状态 = error/network_unreachable → restart plugin → reconnect_handshake → state 反向回 online
- **预期**: UI 显示 "重连中…" 文案 (蓝图 §1.6 故障 UX 区分表第 1 行) → online dot 恢复
- **截屏存**: `docs/evidence/al-2-wrapper/02-error-to-online.png`

### 截屏 3: busy/idle BPP frame 触发

- **场景**: agent 收 task → BPP-2.2 task_started frame → state 翻 busy → finish → task_finished frame → state 翻 idle
- **预期**: 状态变化由 BPP frame 驱动 (反向断言: 不通过任何 REST PATCH 路径)
- **截屏存**: `docs/evidence/al-2-wrapper/03-busy-idle-bpp.png`

## 签字字面 (release 前 野马填)

```
野马 [姓名缩写] 于 [日期] 完整跑过 agent lifecycle v1 demo, 上述 3 张
截屏真实有效, 作为 owner 我看得懂 agent 的状态变化. 5-state 可读 +
error→online 反向链可见 + busy/idle BPP frame 触发三件事 owner 可读.
⭐ 标志性 milestone AL-2 wrapper 4.2 demo 签字 ✅.
```

## 反约束

- 4.2 demo 签字走本 doc 路径, **不混入** `al-release-gate.yml` 4.1 行为不变量数字化清单
- 截屏路径锚 byte-identical 跟 acceptance §3.3 + spec §2 留账
- 签字仅由野马提交, 不允许其他角色代签 (用户视角立场)
- 跟 HB-4 yema-signoff 拆独立路径 (host 层 vs runtime 层 demo 拆死)
