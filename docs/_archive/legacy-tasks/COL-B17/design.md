# COL-B17: @mention 输入过滤 — 技术设计

日期：2026-04-22 | 状态：Draft

## 1. 概述

输入框输入 `@` 后弹出成员列表，支持键盘输入过滤（ID 和名字都能匹配）。选择后插入 mention 标记。

## 2. 前端实现

### 2.1 触发逻辑

在 `MessageInput` 组件中监听输入：
- 检测到 `@` 字符（且前面是空格或行首）→ 激活 mention 模式
- 继续输入的字符作为过滤关键词
- 按 `Esc` / 删除 `@` / 点击外部 → 退出 mention 模式

### 2.2 过滤逻辑

```typescript
const filtered = members.filter(m =>
  m.display_name.toLowerCase().includes(query) ||
  m.id.toLowerCase().includes(query)
);
```

- 大小写不敏感
- 空 query 显示全部成员
- 最多显示 10 个结果

### 2.3 UI 组件：MentionPicker

```
┌──────────────────┐
│ 🟢 Alice (alice) │  ← 高亮选中
│ 🤖 Bot-1 (bot1)  │
│ 👤 Bob (bob123)  │
└──────────────────┘
```

- 位置：输入框上方弹出（桌面端），底部弹出（移动端，复用 B16 bottom-sheet）
- 键盘导航：↑↓ 选择，Enter 确认，Esc 取消
- 显示：头像/状态图标 + display_name + (id)
- Agent 和 User 都显示

### 2.4 插入 Mention

选择成员后：
- 输入框中插入 `@display_name `（带尾随空格）
- 消息发送时替换为 `<@userId>` 格式
- 渲染时 `<@userId>` 显示为高亮的 `@display_name`

### 2.5 数据源

从当前频道成员列表获取（已有 `GET /api/v1/channels/:id/members`）。前端缓存，切换频道时刷新。

## 3. 改动文件

| 文件 | 改动 |
|------|------|
| `components/MentionPicker.tsx` | 新建：mention 下拉列表组件 |
| `components/MessageInput.tsx` | 加 `@` 检测 + mention 模式 state |
| `components/MessageItem.tsx` | 渲染 `<@userId>` 为高亮 mention |
| `hooks/useMention.ts` | 新建：mention 逻辑 hook |

## 4. Task Breakdown

### T1: useMention hook + MentionPicker 组件
- `@` 检测 + 过滤逻辑
- 下拉列表 UI（桌面端上方弹出）
- 键盘导航

### T2: 插入 + 发送 + 渲染
- 选择后插入 `@name`
- 发送时转 `<@userId>`
- 渲染时高亮显示

### T3: 移动端适配
- 底部弹出（复用 B16 bottom-sheet 模式）
- 触摸选择

## 5. 验收标准

- [ ] 输入 `@` 弹出成员列表
- [ ] 输入过滤（名字 + ID）
- [ ] 键盘 ↑↓ 选择 + Enter 确认
- [ ] 发送后 mention 高亮显示
- [ ] 移动端底部弹出
