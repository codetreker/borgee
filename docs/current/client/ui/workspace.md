# 6. Workspace 页面

```
+────────────────────+─────────────────────────────────────────────────────────+
│  📁 Workspace      │  docs/architecture.md                          [Raw]    │
│                    ├─────────────────────────────────────────────────────────┤
│  ▾ 📁 docs/        │                                                         │
│    📄 README.md    │  # Architecture                                         │
│    📄 architecture │                                                         │
│    📁 api/         │  ## Overview                                            │
│      📄 spec.yaml  │                                                         │
│  ▾ 📁 src/         │  The system uses a client-server architecture           │
│    📄 index.ts     │  with WebSocket for real-time messaging.                │
│    📄 auth.ts      │                                                         │
│  ▾ 📁 assets/      │  ## Components                                          │
│    🖼 logo.png     │                                                         │
│                    │  - **API Server**: Express + Socket.IO                  │
│                    │  - **Client**: React SPA                                │
│                    │  - **Database**: SQLite                                 │
│                    │                                                         │
│                    │  ```typescript                                           │
│                    │  interface Message {                                     │
│                    │    id: string;                                           │
│                    │    content: string;                                      │
│                    │    author: User;                                         │
│                    │  }                                                       │
│                    │  ```                                                     │
│                    │                                                         │
+────────────────────+─────────────────────────────────────────────────────────+
```

## 6a. 右键菜单

```
│    📄 README.md    │
│    📄 architecture │       ┌──────────────────┐
│    📁 api/         │       │  📝 Rename        │
│      📄 spec.yaml ←[右键]  │  📋 Copy path     │
│  ▾ 📁 src/         │       │  📁 Move to...    │
│    📄 index.ts     │       │  ──────────────── │
│                    │       │  🗑  Delete        │
│                    │       └──────────────────┘
```

- **左侧文件树**：可折叠目录，图标区分文件夹 📁 / 文件 📄 / 图片 🖼
- **右键菜单**：Rename / Copy path / Move to / Delete
- **右侧 FileViewer**：
  - Markdown → 渲染预览（默认）+ Raw 切换
  - 代码 → 语法高亮
  - 图片 → 内联预览
  - 其他文本 → 等宽纯文本
