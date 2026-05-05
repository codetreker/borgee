# COL-B20: Channel Workspace — 方向文档

日期：2026-04-22 | 状态：Direction (讨论确认)

## 1. 概述

每个 Channel 可以创建一个 Workspace——频道级的文件存储空间。存放不适合放 git 的项目文档、截图、会议纪要等。与消息流的区别：消息是时间线，Workspace 是持久化文件。

## 2. 核心设计决策

| 决策项 | 结论 | 决策人 |
|--------|------|--------|
| 文件类型 | 所有类型 | 建军 |
| 大小限制 | 单文件 ≤ 10MB | 建军 |
| 存储 | v1 本地磁盘 | 建军 |
| 目录结构 | 支持文件夹，按 user 隔离 | 建军 |
| 冲突处理 | 同名不覆盖，自动加后缀 `file (1).md`（后缀名前） | 建军 |
| 在线编辑 | 支持 Markdown 编辑 | 建军 |
| 消息联动 | 聊天可引用/嵌入 Workspace 文件 | 建军 |
| 附件关联 | 消息附件自动存入 Workspace | 建军 |
| 管理页面 | 统一页面查看所有 channel 的 workspace | 建军 |

## 3. 架构

### 3.1 存储结构

```
/data/workspaces/{userId}/{channelId}/
  ├── meeting-notes/
  │   ├── 2026-04-21.md
  │   └── 2026-04-22.md
  ├── screenshots/
  │   └── ui-mockup.png
  └── design-draft.md
```

按 userId 隔离——多用户系统，每个用户有自己的文件空间。

### 3.2 数据库

```sql
workspace_files (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id),
  channel_id TEXT NOT NULL REFERENCES channels(id),
  path TEXT NOT NULL,            -- 相对路径（含文件夹）
  filename TEXT NOT NULL,
  mime_type TEXT,
  size_bytes INTEGER NOT NULL,
  source TEXT DEFAULT 'upload',  -- upload | message_attachment
  source_message_id TEXT,        -- 如果来自消息附件
  created_at TEXT DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);
```

### 3.3 API

- `GET /api/v1/channels/:channelId/workspace` — 列出文件
- `GET /api/v1/channels/:channelId/workspace/files/:fileId` — 下载/查看文件
- `POST /api/v1/channels/:channelId/workspace/upload` — 上传文件
- `PUT /api/v1/channels/:channelId/workspace/files/:fileId` — 更新（编辑 Markdown）
- `DELETE /api/v1/channels/:channelId/workspace/files/:fileId` — 删除
- `POST /api/v1/channels/:channelId/workspace/mkdir` — 创建文件夹
- `GET /api/v1/workspaces` — 统一管理页面（所有 channel 的 workspace 汇总）

### 3.4 冲突处理逻辑

上传 `design.md` 到已有 `design.md` 的目录：
1. 如果请求带 `replace=true` → 覆盖
2. 否则 → 自动重命名为 `design (1).md`
3. 后缀加在扩展名前：`file (1).md`，不是 `file.md (1)`

### 3.5 消息附件自动入库

消息发送时如果携带附件：
1. 文件存入 Workspace `/data/workspaces/{userId}/{channelId}/attachments/`
2. `workspace_files` 记录 `source='message_attachment'` + `source_message_id`
3. 消息中显示附件缩略图 + "在 Workspace 中查看" 链接

### 3.6 前端

- 侧边栏 "Workspace" tab（与 Remote Explorer 并列）
- 文件树 + FileViewer（共享组件）
- Markdown 文件点击 → 渲染模式；编辑按钮 → 编辑模式
- 统一管理页面：左侧 channel 列表，右侧文件树

## 4. 未来扩展（不在 v1）

- S3 存储后端
- 文件版本历史
- 协同编辑（多人同时编辑 Markdown）
- 更大文件限制

## 5. 依赖

- 共享 FileViewer 组件（与 B19 Remote Explorer 共用）
- B18 富文本/Markdown 编辑器（Workspace 的 Markdown 在线编辑可复用）
