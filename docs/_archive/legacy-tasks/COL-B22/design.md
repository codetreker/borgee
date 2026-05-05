# COL-B22: 消息路径文件链接 — 技术设计

日期：2026-04-22 | 状态：Draft | 依赖：B21（Plugin WS）

## 1. 概述

Agent 消息中的文件路径自动变成可点击链接，点击后通过 Plugin WS 读取文件并用 FileViewer 预览。

## 2. 路径检测

### 2.1 正则

```typescript
const FILE_PATH_RE = /(?:^|\s)(\/(?:[a-zA-Z0-9._-]+\/)*[a-zA-Z0-9._-]+\.[a-zA-Z0-9]+)(?=\s|$|[,.)}\]])/g;
```

匹配 `/` 开头的绝对路径，必须有扩展名（减少误判）。

### 2.2 检测时机

**前端渲染时**——不修改消息内容，只在 `MessageItem` 渲染 Agent 消息时做路径替换。

```typescript
function renderWithFileLinks(content: string, agentId: string, isOwner: boolean): ReactNode {
  if (!isOwner) return renderMarkdown(content);
  // 用正则匹配路径，替换为 <FileLink> 组件
}
```

### 2.3 条件

- 只对 Agent 发的消息
- 只对该 Agent 的 owner 渲染为链接
- 其他人看到纯文本

## 3. 文件读取

### 3.1 流程

```
用户点击链接
  → 前端 GET /api/v1/agents/:agentId/files?path=/xxx
    → Server 检查：用户是否是 agent owner
    → Server 通过 PluginManager.request(agentId, { action: 'read_file', path })
      → Plugin 收到请求，检查白名单，读文件
      → Plugin 返回 { content, mime_type, size }
    → Server 返回文件内容
  → 前端 FileViewer 渲染
```

### 3.2 Server API

```
GET /api/v1/agents/:agentId/files?path=/workspace/foo.ts
```

**权限**：只有 agent owner 可调用。

**实现**：

```typescript
fastify.get('/api/v1/agents/:agentId/files', async (req) => {
  const { agentId } = req.params;
  const { path } = req.query;
  
  // 1. 校验 owner
  const agent = Q.getAgent(db, agentId);
  if (agent.owner_id !== req.userId) throw 403;
  
  // 2. 检查 Plugin 在线
  if (!pluginManager.isConnected(agentId)) throw 503 'Agent offline';
  
  // 3. 发请求给 Plugin
  const result = await pluginManager.request(agentId, {
    action: 'read_file',
    path,
  }, 10000); // 10s timeout
  
  return result;
});
```

### 3.3 Plugin 端处理

```typescript
wsClient.onRequest(async (data) => {
  if (data.action === 'read_file') {
    // 检查白名单
    if (!isAllowed(data.path)) return { error: 'path_not_allowed' };
    // 读文件
    const content = await fs.readFile(data.path, 'utf-8');
    const stat = await fs.stat(data.path);
    return { content, size: stat.size, mime_type: getMime(data.path) };
  }
});
```

### 3.4 白名单

`~/.config/collab/file-access.json`：

```json
{
  "allowedPaths": ["/workspace", "/home/user/projects"],
  "maxFileSize": 1048576
}
```

默认没有配置 = 不允许任何路径。

## 4. 前端组件

### 4.1 FileLink

```typescript
function FileLink({ path, agentId }: Props) {
  const [loading, setLoading] = useState(false);
  const [file, setFile] = useState(null);
  
  const handleClick = async () => {
    setLoading(true);
    const res = await api.getAgentFile(agentId, path);
    if (res.error === 'agent_offline') showToast('Agent 离线');
    else setFile(res);
    setLoading(false);
  };
  
  return (
    <>
      <span className="file-link" onClick={handleClick}>
        📄 {path}
      </span>
      {file && <FileViewer file={file} onClose={() => setFile(null)} />}
    </>
  );
}
```

### 4.2 Agent 离线状态

- Plugin 未连接 → API 返回 503
- 前端显示 toast "Agent 离线，无法读取文件"
- 链接样式变灰（disabled）

## 5. 改动文件

### Server
| 文件 | 改动 |
|------|------|
| `src/routes/agents.ts` | 加 `GET /agents/:agentId/files` endpoint |

### Client
| 文件 | 改动 |
|------|------|
| `components/FileLink.tsx` | 新建：可点击文件路径链接 |
| `components/MessageItem.tsx` | Agent 消息渲染时路径替换 |
| `lib/api.ts` | 加 `getAgentFile()` |

### Plugin
| 文件 | 改动 |
|------|------|
| `src/ws-client.ts` | onRequest handler 加 `read_file` |
| `src/file-access.ts` | 新建：白名单检查 + 文件读取 |

## 6. Task Breakdown

### T1: Server API + Plugin 文件读取
- `GET /agents/:agentId/files` endpoint
- PluginManager.request 调用
- Plugin onRequest `read_file` handler
- 白名单配置

### T2: 前端路径检测 + FileLink
- 正则匹配
- FileLink 组件
- MessageItem 渲染集成

### T3: 离线处理 + 错误 UI
- Agent 离线提示
- 读取超时提示
- 白名单拒绝提示

## 7. 验收标准

- [ ] Agent 消息中 `/xxx/yyy.ts` 路径变成可点击链接
- [ ] 点击后 FileViewer 显示文件内容
- [ ] Agent 离线时提示
- [ ] 非 owner 看到纯文本
- [ ] 白名单外路径拒绝
