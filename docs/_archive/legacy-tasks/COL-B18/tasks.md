# COL-B18: Rich Text / Markdown Input — Task Breakdown

## T1: Tiptap 安装 + 基础编辑器替换

**目标**：将 `<textarea>` 替换为 Tiptap 编辑器，支持基础 Markdown 输入和实时预览。

**改动文件**：
| 文件 | 改动说明 | 预估行数 |
|------|----------|----------|
| `packages/client/package.json` | 添加 `@tiptap/react` `@tiptap/starter-kit` `@tiptap/pm` `tiptap-markdown` | ~5 |
| `packages/client/src/components/MessageInput.tsx` | textarea → `EditorContent`；重写 handleSend 从 editor 导出 Markdown；Enter 换行 / Ctrl+Enter 发送；保留 emoji picker 和图片上传 | ~120 |
| `packages/client/src/components/MessageInput.css` (或 styles) | Tiptap 编辑器样式：`.ProseMirror` 基础样式、最小/最大高度、placeholder | ~40 |

**依赖**：无（第一个 task）

**验证**：
- [ ] 输入 `**bold**` 实时渲染为粗体
- [ ] 输入 `` `code` `` 渲染为行内代码
- [ ] 输入 ` ``` ` 后换行进入代码块
- [ ] Ctrl+Enter 发送消息，Enter 换行
- [ ] 发送后消息内容在 MessageList 中正确显示（Markdown 保留）
- [ ] 编辑器内容发送后清空
- [ ] 粘贴图片 / 拖拽图片仍可用

---

## T2: 格式工具栏

**目标**：在编辑器上方添加格式工具栏按钮（B / I / Code / List / Quote / Link）。

**改动文件**：
| 文件 | 改动说明 | 预估行数 |
|------|----------|----------|
| `packages/client/src/components/Toolbar.tsx` | **新建**：工具栏组件，接收 `editor` 实例，按钮调用 `editor.chain().focus().toggleBold()` 等命令；高亮当前激活格式 | ~100 |
| `packages/client/src/components/Toolbar.css` | 工具栏样式：flex 布局、按钮 hover/active 状态、移动端水平滚动 | ~50 |
| `packages/client/src/components/MessageInput.tsx` | 引入 `<Toolbar editor={editor} />` 放在 EditorContent 上方 | ~5 |

**依赖**：T1（需要 Tiptap editor 实例）

**验证**：
- [ ] 点击 B 按钮切换粗体，按钮高亮反映当前状态
- [ ] 点击 I 按钮切换斜体
- [ ] Code 按钮切换行内代码
- [ ] List 按钮切换无序列表
- [ ] Quote 按钮切换引用块
- [ ] 移动端工具栏可水平滚动，不溢出屏幕
- [ ] Ctrl+B / Ctrl+I 快捷键与工具栏状态同步

---

## T3: @mention 迁移到 Tiptap Extension

**目标**：将 B17 的自定义 mention 实现迁移为 Tiptap Mention extension，mention 作为不可编辑的 inline node 存在于编辑器中。

**改动文件**：
| 文件 | 改动说明 | 预估行数 |
|------|----------|----------|
| `packages/client/package.json` | 添加 `@tiptap/extension-mention` `@tiptap/suggestion` | ~2 |
| `packages/client/src/extensions/mention.ts` | **新建**：自定义 Mention extension 配置 + suggestion 配置（触发字符 `@`、过滤逻辑、渲染 popup） | ~80 |
| `packages/client/src/components/MentionPicker.tsx` | 适配 Tiptap suggestion API：接收 `SuggestionProps` 而非自定义 props；改为 `forwardRef` 以支持 suggestion 的 `ref` | ~40 改动 |
| `packages/client/src/hooks/useMention.ts` | **删除或大幅精简**：trigger 检测和插入逻辑由 Tiptap suggestion 接管 | -80 |
| `packages/client/src/components/MessageInput.tsx` | 移除旧 mention 逻辑（useMention 调用、手动 mention 事件）；在 editor extensions 中注册 mention extension；handleSend 从 editor JSON 中提取 mention nodes 构建 mentions 数组 | ~30 改动 |

**依赖**：T1（需要 Tiptap 编辑器）

**验证**：
- [ ] 输入 `@` 弹出用户选择面板
- [ ] 输入 `@abc` 过滤匹配用户
- [ ] 键盘上下选择 + Enter 确认插入 mention node
- [ ] mention node 在编辑器中显示为高亮 pill，不可部分编辑
- [ ] 发送后消息中 mention 正确显示（`<@userId>` 格式传输）
- [ ] Backspace 整体删除 mention node
- [ ] 移动端 mention picker 正常显示

---

## T4: 消息渲染增强

**目标**：确保消息列表正确渲染所有 Markdown 格式，代码块语法高亮。

**改动文件**：
| 文件 | 改动说明 | 预估行数 |
|------|----------|----------|
| `packages/client/package.json` | 添加 `highlight.js`（或已有则跳过） | ~1 |
| `packages/client/src/lib/markdown.ts` | 配置 `marked` 使用 highlight.js 做代码块高亮；确保列表、引用、链接正确渲染 | ~20 |
| `packages/client/src/components/MessageItem.tsx` | 若需要：调整渲染逻辑以兼容新格式（通常无需改动） | ~5 |
| 全局 CSS / highlight theme | 引入 highlight.js 主题 CSS；微调 blockquote / list / code 在消息气泡中的样式 | ~30 |

**依赖**：T1（发送的消息需要是 Markdown 格式才能验证渲染）

**验证**：
- [ ] 粗体、斜体、行内代码在消息气泡中正确渲染
- [ ] 代码块有语法高亮（至少 JS/TS/Python）
- [ ] 有序/无序列表正确缩进渲染
- [ ] 引用块有左边框样式
- [ ] 链接可点击，新窗口打开
- [ ] 旧消息（纯文本）仍然正常显示

---

## T5: 消息编辑适配

**目标**：消息编辑模式也使用 Tiptap 编辑器（当前是 textarea）。

**改动文件**：
| 文件 | 改动说明 | 预估行数 |
|------|----------|----------|
| `packages/client/src/components/MessageItem.tsx` | 编辑模式从 textarea 切换为 Tiptap mini editor，加载现有 Markdown content 到编辑器 | ~60 |

**依赖**：T1, T3（需要 Tiptap + mention extension）

**验证**：
- [ ] 点击编辑后，消息内容在 Tiptap 编辑器中正确加载（格式保留）
- [ ] 编辑后保存，消息正确更新
- [ ] 编辑时 mention 显示为 pill 且可保留

---

## 依赖关系图

```
T1 (基础编辑器)
├── T2 (工具栏)
├── T3 (Mention 迁移)
│   └── T5 (编辑适配)
└── T4 (渲染增强)
```

## 建议执行顺序

T1 → T3 → T2 → T4 → T5

- T1 先行，所有后续 task 依赖它
- T3 紧跟，因为 mention 是核心功能，早迁移减少冲突
- T2 工具栏独立性强，可穿插
- T4 渲染侧独立，可与 T2/T3 并行
- T5 最后，需要前面都就绪
