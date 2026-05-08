# HB-4 ⭐ 4.2 demo 签字 — 野马 (placeholder, release 前真补)

> **Status**: ⏸️ deferred — release 前真补. 本 doc 是 placeholder 锁字面要求 + 截屏路径锚 (跟 HB-4 spec brief §2 留账 + acceptance §3.3 同源).
> **Owner**: 野马 (主, demo 签字) — "作为用户我敢装这个 daemon"
> **关联**: spec `docs/implementation/modules/hb-4-spec.md` + acceptance `docs/qa/acceptance-templates/hb-4.md` §3.3 + release gate `docs/release/host-bridge-release-gate.md` §4

## 签字字面 (蓝图 §HB-4 字面要求)

野马在 release 前 demo 完整跑一遍 Borgee Helper, 提交以下 3 张截屏 + 一句签字:

### 截屏 1: 五支柱状态页

- **路径**: 设置页 → "Helper 信任" tab
- **预期 DOM**: 5 行状态指示 (开源 / 签名 / 可审计日志 / 可吊销 / 限定能力)
- **截屏存**: `docs/evidence/hb-4/01-five-pillars-status.png`

### 截屏 2: 情境授权弹窗

- **触发**: 装 plugin 后, agent 第一次想读 ~/code (filesystem) 时
- **预期 DOM**: HB-3 HostGrantsPanel 三按钮 (拒绝 / 仅这一次 / 始终允许) + title `"DevAgent 想读取你的 ~/code"` byte-identical 跟蓝图 §1.3
- **截屏存**: `docs/evidence/hb-4/02-permission-popup.png`

### 截屏 3: 撤销后行为

- **场景**: 用户在设置页撤销 grant → agent 立即不能读
- **预期**: ≤ 100ms 内 daemon 拒绝 (跟 HB-4 release gate 第 5 行 byte-identical)
- **截屏存**: `docs/evidence/hb-4/03-revoke-effective.png`

## 签字字面 (release 前 野马填)

```
野马 [姓名缩写] 于 [日期] 完整跑过 Borgee Helper v1 demo, 上述 3 张截屏
真实有效, 作为用户我敢装这个 daemon. 五支柱可见 + 情境授权 + 撤销有效
三件事用户可读. ⭐ 标志性 milestone HB-4 4.2 demo 签字 ✅.
```

## 反约束 (跟 spec §0 立场 ④ + acceptance §4.2 同源)

- 4.2 demo 签字走本 doc 路径, **不混入** `release-gate.yml` 4.1 行为不变量数字化清单
- 截屏路径锚 byte-identical 跟 acceptance §3.3 + spec §2 留账
- 签字仅由野马提交, 不允许其他角色代签 (用户视角立场)
