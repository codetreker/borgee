# COL-B20: Channel Workspace — Task Breakdown

日期：2026-04-22

---

## T1: 数据库 + 基础 CRUD API

**目标**：建表 `workspace_files`，实现文件上传/列表/下载/删除 API。

**改动文件**：
| 文件 | 改动 | 预估行数 |
|------|------|----------|
| `packages/server/package.json` | 无需改动（`@fastify/multipart` 已注册） | 0 |
| `packages/server/src/db.ts` | 新增 `workspace_files` 表迁移 + 索引 | ~30 |
| `packages/server/src/queries.ts` | 新增 workspace 相关查询函数（insertFile, listFiles, getFile, deleteFile） | ~80 |
| `packages/server/src/routes/workspace.ts` | **新建**：GET list / POST upload / GET download / DELETE | ~150 |
| `packages/server/src/index.ts` | import + 注册 workspace routes | ~3 |
| `packages/server/src/types.ts` | WorkspaceFile 类型定义 | ~15 |

**验证方式**：
- 单元测试 `__tests__/workspace.test.ts`：上传文件 → 列表 → 下载 → 删除 → 确认 404
- curl 手动验证 multipart upload、10MB 限制、auth 校验

**依赖**：无（基础 task）

---

## T2: 文件夹 + 冲突处理 + 移动

**目标**：mkdir API、parent_id 嵌套、同名冲突自动加后缀、move API。

**改动文件**：
| 文件 | 改动 | 预估行数 |
|------|------|----------|
| `packages/server/src/routes/workspace.ts` | 新增 POST mkdir / POST move 路由 | ~80 |
| `packages/server/src/queries.ts` | 新增 mkdir、move、resolveConflict、getSiblingNames 查询 | ~60 |

**验证方式**：
- 测试：创建文件夹 → 上传到子文件夹 → list with parentId → 同名上传验证后缀 `file (1).md`
- 测试：move 文件到另一个文件夹 → 目标有同名时自动加后缀
- 测试：UNIQUE 约束确认（user_id + channel_id + parent_id + name）

**依赖**：T1

---

## T3: FileViewer 共享组件

**目标**：通用文件预览组件，按 MIME 分发渲染（Markdown / 代码高亮 / 图片 / 文本 / 二进制提示）。

**改动文件**：
| 文件 | 改动 | 预估行数 |
|------|------|----------|
| `packages/client/src/components/FileViewer.tsx` | **新建**：根据 mime/扩展名分发到子查看器 | ~120 |
| `packages/client/src/components/viewers/ImageViewer.tsx` | **新建**：图片查看（img 标签 + 缩放） | ~30 |
| `packages/client/src/components/viewers/MarkdownViewer.tsx` | **新建**：复用 `lib/markdown.ts` 渲染 | ~40 |
| `packages/client/src/components/viewers/CodeViewer.tsx` | **新建**：highlight.js 语法高亮（已有依赖） | ~50 |
| `packages/client/src/components/viewers/TextViewer.tsx` | **新建**：纯文本 pre 显示 | ~15 |
| `packages/client/src/index.css` | FileViewer 相关样式 | ~40 |

**验证方式**：
- 浏览器手动测试：上传 .md / .ts / .png / .txt / .zip → 各自正确渲染
- 二进制文件显示"不支持预览"提示

**依赖**：T1（需要下载 API 获取文件内容）

---

## T4: 前端文件树侧边栏

**目标**：Sidebar 增加 "Workspace" tab，树形展示当前频道的 workspace 文件。

**改动文件**：
| 文件 | 改动 | 预估行数 |
|------|------|----------|
| `packages/client/src/components/WorkspacePanel.tsx` | **新建**：文件树 + 右键菜单 + 拖拽上传 | ~250 |
| `packages/client/src/components/ChannelView.tsx` | 增加 Workspace tab/panel 切换 | ~30 |
| `packages/client/src/lib/api.ts` | 新增 workspace API 调用函数（listFiles, uploadFile, mkdir, deleteFile, moveFile, downloadFile） | ~60 |
| `packages/client/src/types.ts` | WorkspaceFile 类型 | ~15 |
| `packages/client/src/index.css` | 文件树样式、右键菜单样式 | ~60 |

**验证方式**：
- 浏览器测试：切换到 Workspace tab → 看到文件列表 → 展开文件夹 → 点击文件打开 FileViewer
- 右键菜单：新建文件夹、删除、重命名
- 拖拽文件到面板 → 上传成功
- 空 workspace 显示占位提示

**依赖**：T1, T2, T3

---

## T5: 消息附件自动存入 Workspace

**目标**：发消息带附件时，自动将文件写入 workspace（source=message_attachment），放入 `attachments/` 虚拟文件夹。

**改动文件**：
| 文件 | 改动 | 预估行数 |
|------|------|----------|
| `packages/server/src/routes/messages.ts` | 发消息逻辑中增加：检测附件 → 调用 workspace 存入 | ~40 |
| `packages/server/src/queries.ts` | 新增 ensureAttachmentsFolder、insertFromAttachment | ~30 |
| `packages/server/src/routes/workspace.ts` | 提取文件写入逻辑为可复用函数 | ~20 |

**验证方式**：
- 测试：发一条带图片附件的消息 → workspace 中 `attachments/` 文件夹下出现该文件
- 验证 source_message_id 正确关联
- 验证不影响现有消息发送流程

**依赖**：T1, T2

---

## T6: Markdown 在线编辑

**目标**：FileViewer 中 .md 文件增加"编辑"按钮，切换到 Tiptap 编辑器，保存调用 PUT API。

**改动文件**：
| 文件 | 改动 | 预估行数 |
|------|------|----------|
| `packages/client/src/components/viewers/MarkdownViewer.tsx` | 增加编辑模式切换 | ~20 |
| `packages/client/src/components/MarkdownEditor.tsx` | **新建**：复用 Tiptap（参考 MessageInput/EditEditor） | ~100 |
| `packages/server/src/routes/workspace.ts` | PUT /files/:id 路由（更新文件内容） | ~30 |
| `packages/server/src/queries.ts` | updateFileContent 查询 | ~10 |
| `packages/client/src/lib/api.ts` | updateFile API 函数 | ~10 |

**验证方式**：
- 浏览器：点击 .md 文件 → 预览模式 → 点"编辑" → Tiptap 编辑器 → 修改 → 保存 → 刷新确认内容更新
- 验证非 .md 文件不显示编辑按钮

**依赖**：T3, T4

---

## T7: 聊天引用 + 统一管理页面

**目标**：消息中引用 workspace 文件（预览卡片）；`/workspaces` 全局管理页面。

**改动文件**：
| 文件 | 改动 | 预估行数 |
|------|------|----------|
| `packages/client/src/components/WorkspaceManager.tsx` | **新建**：左侧频道列表 + 右侧文件树 | ~150 |
| `packages/client/src/components/MessageItem.tsx` | 识别 workspace 文件引用 → 渲染预览卡片 | ~40 |
| `packages/client/src/App.tsx` | 增加 `/workspaces` 路由/视图切换 | ~15 |
| `packages/server/src/routes/workspace.ts` | GET /api/v1/workspaces 汇总 API | ~30 |
| `packages/server/src/queries.ts` | getAllWorkspaceFiles(userId) 跨频道查询 | ~20 |
| `packages/client/src/lib/api.ts` | getAllWorkspaces API 函数 | ~10 |
| `packages/client/src/index.css` | 管理页面 + 引用卡片样式 | ~40 |

**验证方式**：
- 浏览器：点击顶栏 "Workspaces" → 管理页面 → 按频道分组 → 点击文件打开 FileViewer
- 消息中粘贴 workspace 文件链接 → 显示预览卡片（文件名、类型、大小）
- 验证只显示当前用户有权限的频道

**依赖**：T3, T4, T5

---

## 依赖关系图

```
T1 (DB + API)
├── T2 (文件夹/冲突) ──┐
├── T3 (FileViewer)     ├── T4 (文件树) ── T6 (Markdown编辑)
│                       │
├── T5 (附件存入) ──────┴── T7 (引用/管理页)
```

## 行数汇总

| Task | Server 预估 | Client 预估 | 合计 |
|------|------------|------------|------|
| T1 | ~280 | 0 | ~280 |
| T2 | ~140 | 0 | ~140 |
| T3 | 0 | ~295 | ~295 |
| T4 | 0 | ~415 | ~415 |
| T5 | ~90 | 0 | ~90 |
| T6 | ~40 | ~130 | ~170 |
| T7 | ~50 | ~255 | ~305 |
| **合计** | **~600** | **~1095** | **~1695** |
