# 2. 主界面（桌面端）

```
+────────────────────+─────────────────────────────────────────────────────────+
│ COLLAB             │  [Chat]  [Workspace]  [Remote]            ← Tab 切换    │
│                    ├─────────────────────────────────────────────────────────┤
│ ▾ CHANNELS         │  # general                                    [⚙]  [📌] │
│   # general        ├─────────────────────────────────────────────────────────┤
│   # dev            │                                                         │
│   # design         │  ┌──┐  Alice           10:30 AM                        │
│                    │  │AV│  Hey team, the new build is ready!                │
│ ▾ DIRECT MESSAGES  │  └──┘                                                   │
│   🟢 Bob           │                                                         │
│   🟡 Carol         │  ┌──┐  🤖 AgentX       10:31 AM                        │
│   ⚫ Dave           │  │AV│  Build #142 passed. Coverage: 94.2%.             │
│                    │  └──┘  ```                                              │
│                    │        test/auth.spec.ts  ✓ 24 passed                   │
│                    │        test/chat.spec.ts  ✓ 18 passed                   │
│                    │        ```                                              │
│                    │                                                         │
│                    │  ┌──┐  Bob              10:33 AM                        │
│                    │  │AV│  @Alice nice! merging now 🚀                      │
│                    │  └──┘                                                   │
│                    │                                                         │
│                    │                                                         │
│                    │                                                         │
│                    ├─────────────────────────────────────────────────────────┤
│ [⚙][🤖][📁][🌐]   │  ┌─────────────────────────────────────────┐  [Send]    │
│ Settings Agents    │  │  Type a message...                       │            │
│ Workspace Remote   │  └─────────────────────────────────────────┘            │
+────────────────────+─────────────────────────────────────────────────────────+
```

- **左侧 Sidebar**（固定宽度 ~220px）
  - 频道列表：分组折叠，`#` 前缀
  - Direct Messages：显示在线状态（🟢 在线 / 🟡 离开 / ⚫ 离线）
  - 底部按钮行：Settings / Agents / Workspace / Remote
- **右侧主区域**
  - Tab 栏：Chat / Workspace / Remote 三个 Tab
  - 频道 Header：频道名 + 设置齿轮 + 置顶
  - 消息列表：头像 + 用户名 + 时间 + 内容
  - 输入框 + Send 按钮
