# COL-B02: 频道删除 UX — 技术设计

日期：2026-04-21 | 状态：Draft

## 1. 背景

后端已有频道软删除（`DELETE /api/v1/channels/:channelId`）和 WS 广播（`channel_deleted`）。本次只做前端。

## 2. 现有后端 API

- `DELETE /api/v1/channels/:channelId` → 204（软删除，设 `deleted_at`）
- WS 广播 `channel_deleted { channel_id }`（broadcastToChannel）
- 保护：#general 不可删、DM 不可删、权限检查

## 3. 前端设计

### 3.1 删除按钮

在频道设置（ChannelSettings 或类似组件）底部「危险区域」显示红色删除按钮。

显示条件：
```typescript
const canDelete = 
  channel.type !== 'dm' &&
  !channel.isDefault &&
  (currentUser.role === 'admin' || userPermissions.includes('channel.delete'));
```

### 3.2 确认弹窗（ConfirmDeleteModal）

新建 `ConfirmDeleteModal.tsx`：
- Props: `{ channelName, onConfirm, onCancel, loading }`
- 标题："删除频道"
- 正文："确定删除 **#频道名**？此操作不可恢复。"
- 取消按钮（secondary）+ 删除按钮（danger，不 autoFocus）
- loading 时禁用按钮

### 3.3 删除流程

1. 点击删除 → 打开 ConfirmDeleteModal
2. 确认 → `DELETE /api/v1/channels/:channelId`（loading 状态）
3. 成功 → 关闭弹窗 → dispatch `REMOVE_CHANNEL` → navigate to #general → Toast "频道已删除"
4. 失败 → Toast 错误 → 弹窗保持

### 3.4 WS 事件处理（其他用户）

`useWebSocket` 已有 `channel_deleted` handler（P1 实现的 `REMOVE_CHANNEL`）。需补充：
- 如果 `currentChannelId === deletedChannelId` → navigate to #general + Toast "# 频道名 已被删除"
- 否则 → 静默移除（侧边栏更新，已有）

### 3.5 Toast 通知

使用简单的 inline 通知（和消息发送失败的错误提示同模式），或 window.alert 作为 v1 简化方案。

## 4. AppContext 变更

- `REMOVE_CHANNEL` reducer 已有（P1）
- 需新增：删除成功后 navigate 到 #general 的逻辑（在组件层处理，不在 reducer 里）

## 5. Task Breakdown

### T1: 删除按钮 + 确认弹窗

**改动文件**：新建 `ConfirmDeleteModal.tsx`、修改频道设置组件、`index.css`

**内容**：
1. ConfirmDeleteModal 组件
2. 频道设置页底部"危险区域"+ 删除按钮（条件渲染）
3. 点击删除 → 弹窗 → 确认 → 调用 DELETE API → 跳转 #general

**验收标准**：
- [ ] 有权限时显示删除按钮
- [ ] 确认弹窗正确显示频道名
- [ ] 删除成功跳转 #general
- [ ] loading 状态 + 错误提示
- [ ] #general 和 DM 不显示删除按钮

### T2: WS channel_deleted 跳转逻辑

**改动文件**：`useWebSocket.ts` 或 `AppContext.tsx`

**内容**：
1. channel_deleted 事件处理补充：当前在被删频道 → navigate #general + Toast
2. 不在被删频道 → 静默（已有 REMOVE_CHANNEL）

**验收标准**：
- [ ] 其他用户在被删频道时自动跳转 + 提示
- [ ] 不在被删频道时静默移除

## 6. 不在范围

- 频道 header 右键菜单
- 归档/恢复
