# COL-B18: 富文本/Markdown 输入 — 技术设计

日期：2026-04-22 | 状态：Draft

## 1. 概述

将消息输入框从纯文本 `<textarea>` 升级为富文本编辑器，支持 Markdown 语法（**粗体**、*斜体*、`代码`、```代码块```、列表等）。输入时实时预览，发送时保留 Markdown 格式。

## 2. 技术选型

**Tiptap**（基于 ProseMirror）：
- React 生态成熟，有 `@tiptap/react`
- 内置 Markdown 支持（`@tiptap/extension-markdown`）
- 可扩展：@mention 可以做成 Tiptap extension（和 B17 融合）
- 轻量，按需加载 extensions

## 3. 实现方案

### 3.1 编辑器

替换 `MessageInput` 中的 `<textarea>` 为 Tiptap editor：

```typescript
import { useEditor, EditorContent } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';

const editor = useEditor({
  extensions: [StarterKit, Markdown],
  content: '',
});
```

**支持的格式**：
- **粗体**（Ctrl+B / `**text**`）
- *斜体*（Ctrl+I / `*text*`）
- `行内代码`（`` `code` ``）
- 代码块（` ``` `）
- 有序/无序列表
- 引用（`>`）
- 链接

### 3.2 发送流程

1. 用户在 Tiptap 编辑器输入（所见即所得）
2. 发送时导出为 Markdown 字符串
3. 存储/传输用 Markdown 格式（和现有 `content` 字段兼容）
4. 渲染时用 Markdown renderer（已有基础）

### 3.3 与 B17 @mention 融合

Tiptap 支持自定义 node extension。将 B17 的 `@mention` 做成 Tiptap Mention extension：
- 输入 `@` 触发 suggestion（Tiptap 内置 `@tiptap/suggestion`）
- 选择后插入 mention node（不是纯文本）
- 导出时转为 `<@userId>`

### 3.4 工具栏

**桌面端**：输入框上方紧凑工具栏（B / I / Code / List / Quote）
**移动端**：输入框上方滑动工具栏（水平滚动，节省空间）

### 3.5 快捷键

| 快捷键 | 功能 |
|--------|------|
| Ctrl+B | 粗体 |
| Ctrl+I | 斜体 |
| Ctrl+E | 行内代码 |
| Ctrl+Shift+8 | 无序列表 |
| Ctrl+Shift+9 | 有序列表 |
| Ctrl+Enter | 发送消息 |
| Enter | 换行（不发送） |
| Shift+Enter | 换行 |

注意：**Enter 行为改变**——从"发送"改为"换行"。发送改为 Ctrl+Enter。需要在 UI 提示。

## 4. 改动文件

| 文件 | 改动 |
|------|------|
| `package.json` | 加 `@tiptap/react` `@tiptap/starter-kit` `@tiptap/extension-markdown` |
| `components/MessageInput.tsx` | textarea → Tiptap EditorContent |
| `components/Toolbar.tsx` | 新建：格式工具栏 |
| `components/MentionPicker.tsx` | 适配 Tiptap suggestion API |
| `hooks/useMention.ts` | 迁移到 Tiptap extension |

## 5. Task Breakdown

### T1: Tiptap 集成 + 基础格式
- 安装依赖
- 替换 textarea 为 Tiptap
- 基础 Markdown 格式支持
- Ctrl+Enter 发送

### T2: 工具栏
- 桌面端工具栏
- 移动端滑动工具栏
- 快捷键

### T3: Mention 迁移
- B17 mention 从自定义实现迁移到 Tiptap extension
- suggestion 插件适配

### T4: 渲染侧
- 确保消息列表正确渲染 Markdown
- 代码块语法高亮

## 6. 验收标准

- [ ] 输入 `**bold**` 实时显示粗体
- [ ] 工具栏按钮功能正常
- [ ] Ctrl+Enter 发送，Enter 换行
- [ ] @mention 在富文本编辑器中正常工作
- [ ] 代码块正确渲染
- [ ] 移动端工具栏可用
