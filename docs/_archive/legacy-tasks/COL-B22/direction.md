# COL-B22: 消息路径文件链接 — 方向文档

日期：2026-04-22 | 状态：Discussion | 依赖：B21

## 背景
Agent 发消息时经常提到本地文件路径（如 `/workspace/collab/src/server.ts`）。希望自动把路径变成可点击链接，点击后在浏览器预览文件内容。

## 决策记录（2026-04-22 讨论）

### 核心流程
1. Agent 发消息，包含本地文件路径
2. 正则匹配检测路径（绝对路径 `/xxx/yyy.ext`）
3. 路径渲染成可点击链接
4. 点击后通过该 Agent 的 Plugin WS 连接读取文件
5. FileViewer 渲染（代码高亮 / Markdown / 图片 / 文本 fallback）

### 关键决策
- **走 Agent 的 Plugin 连接**：谁发的消息就走谁的 Plugin WS 读（不新建连接）
- **只对 owner 可见**：其他成员看到纯文本路径
- **路径误判可接受**：顶多是一个打不开的链接，自己的机器只有自己看
- **安全控制**：机器上 `~/.config/collab/` 配置文件控制允许访问的目录白名单
- **Agent 离线**：链接点击后提示"Agent 离线"

### 和 Remote Explorer 的关系
- Remote Explorer（B19）：用户主动浏览文件树，独立节点 + 独立 WS
- 文件链接（B22）：被动触发，Agent 提到路径自动链接化，走 Plugin WS
- 两者共享 FileViewer 组件和文件读取协议

### 共享 FileViewer 组件
- `.md` → Markdown 渲染
- `.ts/.js/.py/...` → 代码高亮（Shiki）
- `.png/.jpg/.gif` → 图片查看器
- 文本检测 → 纯文本 fallback
- 二进制 → 提示"不支持预览"
