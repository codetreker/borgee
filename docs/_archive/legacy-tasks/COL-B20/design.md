# COL-B20: Channel Workspace — 技术设计

日期：2026-04-22 | 状态：Draft

## 1. 概述

频道级文件存储。每个 Channel 的每个用户有独立的 Workspace，支持上传、文件夹、预览、编辑。

## 2. 数据库

```sql
CREATE TABLE workspace_files (
  id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
  user_id TEXT NOT NULL REFERENCES users(id),
  channel_id TEXT NOT NULL REFERENCES channels(id),
  parent_id TEXT REFERENCES workspace_files(id),  -- 文件夹嵌套
  name TEXT NOT NULL,
  is_directory INTEGER NOT NULL DEFAULT 0,
  mime_type TEXT,
  size_bytes INTEGER DEFAULT 0,
  source TEXT DEFAULT 'upload',  -- upload | message_attachment
  source_message_id TEXT,
  created_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now')),
  UNIQUE(user_id, channel_id, parent_id, name)
);
```

UNIQUE 约束防止同级同名。冲突时自动加后缀。

## 3. 存储

```
/data/workspaces/{userId}/{channelId}/{fileId}.dat
```

文件名用 fileId，不用原始文件名（防路径注入）。元数据在 SQLite。

## 4. API

### 文件操作

```
GET    /api/v1/channels/:channelId/workspace             -- 列出文件（?parentId=xxx）
POST   /api/v1/channels/:channelId/workspace/upload      -- 上传文件（multipart）
POST   /api/v1/channels/:channelId/workspace/mkdir       -- 创建文件夹
GET    /api/v1/channels/:channelId/workspace/files/:id   -- 下载/预览文件内容
PUT    /api/v1/channels/:channelId/workspace/files/:id   -- 更新（编辑 Markdown）
DELETE /api/v1/channels/:channelId/workspace/files/:id   -- 删除
POST   /api/v1/channels/:channelId/workspace/files/:id/move -- 移动文件
```

### 统一管理

```
GET    /api/v1/workspaces   -- 当前用户所有频道的 workspace 汇总
```

### 权限

- 所有 API 校验 `req.userId`
- 用户只能操作自己的文件
- 频道成员才能访问该频道的 workspace

## 5. 文件上传

### multipart 处理

用 `@fastify/multipart`：

```typescript
fastify.post('/api/v1/channels/:channelId/workspace/upload', async (req) => {
  const data = await req.file();
  // 检查 size <= 10MB
  // 检查同名冲突 → 加后缀
  // 写文件 + 写 DB
});
```

### 冲突处理

```typescript
function resolveConflict(name: string, existing: string[]): string {
  if (!existing.includes(name)) return name;
  const ext = path.extname(name);
  const base = path.basename(name, ext);
  let i = 1;
  while (existing.includes(`${base} (${i})${ext}`)) i++;
  return `${base} (${i})${ext}`;
}
```

## 6. 消息附件自动存入

消息发送时如果有附件：
1. 文件存入 `/data/workspaces/{userId}/{channelId}/{fileId}.dat`
2. 写 workspace_files 记录：`source='message_attachment'`, `source_message_id`
3. 放入 `attachments/` 虚拟文件夹

## 7. 前端

### 7.1 文件树（侧边栏）

侧边栏 "Workspace" tab：
- 树形文件列表
- 右键菜单：新建文件夹、上传、删除、重命名、移动
- 拖拽上传

### 7.2 FileViewer（共享组件）

```typescript
function FileViewer({ file }: { file: WorkspaceFile }) {
  if (file.mime_type?.startsWith('image/')) return <ImageViewer />;
  if (file.name.endsWith('.md')) return <MarkdownViewer />;
  if (isCode(file.name)) return <CodeViewer />;  // Shiki
  if (isText(file.mime_type)) return <TextViewer />;
  return <BinaryNotice />;
}
```

### 7.3 Markdown 编辑

点击 `.md` 文件 → FileViewer 渲染模式。点 "编辑" → Tiptap 编辑器（复用 B18）。保存 → `PUT /workspace/files/:id`。

### 7.4 统一管理页面

`/workspaces` 路由：左侧频道列表，右侧文件树。

## 8. 改动文件

### Server
| 文件 | 改动 |
|------|------|
| `package.json` | 加 `@fastify/multipart` |
| `src/db.ts` | workspace_files 表迁移 |
| `src/routes/workspace.ts` | 新建：所有 workspace API |
| `src/routes/messages.ts` | 附件自动存入 workspace |

### Client
| 文件 | 改动 |
|------|------|
| `components/WorkspacePanel.tsx` | 新建：文件树侧边栏 |
| `components/FileViewer.tsx` | 新建：通用文件查看器 |
| `components/FileUpload.tsx` | 新建：上传组件 |
| `components/WorkspaceManager.tsx` | 新建：统一管理页面 |
| `App.tsx` | 路由 + 侧边栏 tab |

## 9. Task Breakdown

### T1: 数据库 + 基础 API
- workspace_files 表迁移
- CRUD API（列出、上传、下载、删除）
- @fastify/multipart 集成

### T2: 文件夹 + 冲突处理
- mkdir API
- 嵌套文件夹（parent_id）
- 同名冲突自动后缀
- 移动文件

### T3: FileViewer 组件
- Markdown 渲染
- 代码高亮（Shiki/highlight.js）
- 图片查看
- 文本 fallback + 二进制提示

### T4: 前端文件树
- WorkspacePanel 侧边栏
- 树形列表 + 右键菜单
- 拖拽上传

### T5: 消息附件自动存入
- 发消息时附件写入 workspace
- 关联 source_message_id

### T6: Markdown 编辑
- FileViewer 编辑模式
- Tiptap 编辑器复用
- PUT API 保存

### T7: 聊天引用 + 统一管理
- 消息中引用 workspace 文件（预览卡片）
- `/workspaces` 管理页面

## 10. 验收标准

- [ ] 上传文件到频道 workspace
- [ ] 创建文件夹、嵌套
- [ ] 同名文件自动加后缀
- [ ] FileViewer 正确渲染各类型
- [ ] Markdown 在线编辑
- [ ] 消息附件自动存入
- [ ] 统一管理页面
- [ ] 单文件 ≤ 10MB 限制
