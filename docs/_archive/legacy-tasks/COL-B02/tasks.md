Now I have the full picture. Here's the task breakdown:

---

## COL-B02 Task Breakdown

### Task 1: ConfirmDeleteModal 组件
| 项目 | 详情 |
|---|---|
| **改动文件** | 新建 `packages/client/src/components/ConfirmDeleteModal.tsx`, 修改 `index.css` |
| **预估行数** | ~60 行 TSX + ~30 行 CSS |
| **内容** | Props: `channelName, onConfirm, onCancel, loading`。模态遮罩 + 标题 "删除频道" + "确定删除 **#频道名**？此操作不可恢复。" + 取消(secondary)/删除(danger, 不 autoFocus) 按钮。loading 时禁用按钮。 |
| **验证** | 手动：弹窗渲染、loading 禁用、取消关闭、ESC 关闭 |
| **依赖** | 无 |

---

### Task 2: 频道设置页添加删除按钮 + 权限判断
| 项目 | 详情 |
|---|---|
| **改动文件** | `packages/client/src/components/ChannelMembersModal.tsx`, `index.css` |
| **预估行数** | ~40 行 TSX + ~20 行 CSS |
| **内容** | 底部"危险区域"红色删除按钮。显示条件：`channel.type !== 'dm' && !channel.isDefault && (user.role === 'admin' \|\| hasPermission('channel.delete'))`。`#general` 和 DM 永不显示。 |
| **验证** | 手动：admin 看到按钮、普通有权限用户看到、无权限用户不可见、`#general`/DM 不显示 |
| **依赖** | Task 1（引用 ConfirmDeleteModal） |

---

### Task 3: 删除 API 调用 + 成功跳转 + Toast
| 项目 | 详情 |
|---|---|
| **改动文件** | `packages/client/src/components/ChannelMembersModal.tsx` |
| **预估行数** | ~30 行 |
| **内容** | 确认后 `DELETE /api/v1/channels/:id`，loading 状态管理。成功 → 关闭弹窗 → `dispatch REMOVE_CHANNEL`(reducer 已自动回退到 general) → `showToast("频道已删除")`。失败 → `showToast(error)` + 弹窗保持。 |
| **验证** | 手动：删除成功跳转 general + toast；网络错误时 toast 报错弹窗不关；loading 期间按钮不可重复点击 |
| **依赖** | Task 1, Task 2 |

---

### Task 4: WS `channel_deleted` 跳转逻辑（其他用户）
| 项目 | 详情 |
|---|---|
| **改动文件** | `packages/client/src/hooks/useWebSocket.ts` |
| **预估行数** | ~15 行 |
| **内容** | 在现有 `channel_deleted` handler 中补充：若 `state.currentChannelId === deletedChannelId` → `showToast("#频道名 已被删除")`。跳转已由 `REMOVE_CHANNEL` reducer 自动处理。不在该频道时静默（已有）。 |
| **验证** | 多浏览器测试：用户 A 删除频道，用户 B 在该频道 → 自动跳转 + toast；用户 C 不在该频道 → 侧边栏静默移除 |
| **依赖** | 无（独立于 T1-T3，可并行开发） |

---

### Task 5: Admin 频道管理面板同步
| 项目 | 详情 |
|---|---|
| **改动文件** | `packages/client/src/components/admin/ChannelsTab.tsx` |
| **预估行数** | ~10 行 |
| **内容** | 确认 admin 面板的删除逻辑与 T3 一致（已有 deleteChannel 调用），补加 ConfirmDeleteModal 替代 `window.confirm`，对 `#general` 隐藏删除按钮。 |
| **验证** | admin 面板：general 无删除按钮；删除其他频道弹确认框 |
| **依赖** | Task 1 |

---

### 依赖关系图

```
T1 (ConfirmDeleteModal)
├── T2 (删除按钮 + 权限) → T3 (API 调用 + 跳转)
├── T5 (Admin 面板)
T4 (WS 跳转) ← 独立，可并行
```

**建议执行顺序**: T1 → T2+T4 并行 → T3 → T5

**总预估**: ~205 行改动，涉及 4 个现有文件 + 1 个新文件。
