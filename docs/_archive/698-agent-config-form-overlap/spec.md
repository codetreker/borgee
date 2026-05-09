# 698 — Agent Manage form 排版修

> Issue: https://github.com/codetreker/borgee/issues/698
> 优先级: P2 / current-iteration

## 要做什么

`packages/client/src/components/AgentConfigPanel.tsx` 6 个 `<label>` 加 `display: block`, 让 label 单独占行, input 在下方. 跟 CreateAgentModal form 排版一致.

## 不做什么

- 不动 `.agent-page` 容器宽度 (PR #694 已修)
- 不动 CreateAgentModal form (现有排版好的)

## 修法

方案 B (CSS class): 加 `.agent-config-form` class, AgentConfigPanel 加 wrapper. 详见 `design.md` (zhanma-d 已做 design pre-work).
