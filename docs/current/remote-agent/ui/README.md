# Remote Agent UI Sketch

This is an older combined Remote Explorer sketch retained as Interaction And Layout Reference. It maps to two user SPA surfaces: the Remote nodes sidepane for node/binding management and the Channel remote tab for browsing a channel's bound remote tree.

It does not define product behavior, setup flow, protocol authority, or proof that Remote Agent has a complete standalone UI page. Current protocol caveats remain defined by `../protocol.md`, and filesystem boundary behavior remains defined by `../filesystem-boundary.md`.

## Combined Remote Explorer Sketch

```
+──────────────────────────────────────────────────────────────────────────────+
│  🌐 Remote Explorer                                                          │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─ Connected Nodes ─────────────────────────────────────────────────────┐  │
│  │  Name            Status       Last Seen            Actions            │  │
│  │  ─────────────────────────────────────────────────────────────────    │  │
│  │  🟢 dev-server   online       just now              [🗑]              │  │
│  │  🟢 staging      online       2 min ago             [🗑]              │  │
│  │  ⚫ prod-backup  offline      3 days ago            [🗑]              │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│  ┌─ Directory Bindings (dev-server) ─────────────────────────────────────┐  │
│  │  Local Alias        Remote Path                  Actions              │  │
│  │  ─────────────────────────────────────────────────────────────────    │  │
│  │  project-src        /home/user/project/src       [Edit] [🗑]         │  │
│  │  logs               /var/log/app                 [Edit] [🗑]         │  │
│  │                                                                       │  │
│  │  [+ Add Binding]                                                      │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│  ┌─ Remote Files: dev-server:/home/user/project/src ─────────────────────┐  │
│  │  ▾ 📁 src/                                                            │  │
│  │    📄 index.ts                                                        │  │
│  │    📄 server.ts                                                       │  │
│  │    📁 routes/                                                         │  │
│  │      📄 auth.ts                                                       │  │
│  │      📄 chat.ts                                                       │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│  ┌─ Setup ───────────────────────────────────────────────────────────────┐  │
│  │  Token: collab_rt_****************************  [👁] [📋 Copy]       │  │
│  │                                                                       │  │
│  │  Run on your remote machine:                                          │  │
│  │  ┌───────────────────────────────────────────────────────────────┐    │  │
│  │  │ curl -fsSL https://collab.app/install | sh -s -- \           │    │  │
│  │  │   --token collab_rt_****                                     │    │  │
│  │  └───────────────────────────────────────────────────────────────┘    │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
+──────────────────────────────────────────────────────────────────────────────+
```

- **Node 列表**：名称 + 在线状态（🟢/⚫）+ 最后在线时间 + 删除按钮
- **目录绑定**：本地别名 ↔ 远程路径映射，可编辑/删除/新增
- **远程文件树**：选中 Node 后浏览远程目录结构
- **Token 区域**：默认遮挡，👁 切换显示，📋 一键复制
- **启动命令**：旧图保留的 setup affordance；实际连接和请求协议以 `../protocol.md` 为准。

## Architecture Notes

- Node list and token affordances map to the user SPA Remote nodes sidepane.
- Remote file tree browsing maps to the channel Remote tab.
- The Remote Agent module owns protocol and filesystem boundary documentation, not a separate browser application shell.
- The install/setup text in the sketch is illustrative and should not be treated as a stable installation contract.

## Related Docs

- `../README.md`
- `../protocol.md`
- `../filesystem-boundary.md`
- `../../client/feature-surfaces.md`
