# HB-13 — 创建 agent + 配 channel 网页流程

> Wave: borgee-helper-v1-release
> Issue: gh#681
> Status: placeholder — 真起这 milestone 时由 4 角色完整 4-piece 替换

## 入口

HB-9 plugin 配好链 已合.

## 出口

网页"添加 agent"填名字 → 选 runtime (v1 仅 OpenClaw) → plugin connection 自动注册; 给已有 agent 配 channel 走 owner-only 鉴权; agent 列表 4 态 + 故障原因码字面跟 AL-1a #249 byte-identical.

## 蓝图引用

- `agent-lifecycle.md §2.1` 用户自填
- `agent-lifecycle.md §2.2` 默认路径
- gh#681 issue body 第 2 / 3 项
