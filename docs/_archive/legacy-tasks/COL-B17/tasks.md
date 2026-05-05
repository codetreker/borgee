# COL-B17: @mention 输入过滤 — Task Breakdown

## 现状分析

已有实现（基本框架已就绪）：
- `MentionPicker.tsx` — 下拉列表组件（avatar + name + Bot 标记）
- `MessageInput.tsx` — `@` 触发检测、mentionQuery/mentionVisible state、键盘导航(↑↓/Enter/Esc)、insertMention 函数
- `markdown.ts` — `<@userId>` → 高亮 `@displayName` 渲染
- `MessageItem.tsx` — 调用 renderMarkdown 传入 mentions + userMap
- 发送时已提取 `<@userId>` 并通过 WS 传递 `mentions[]`

---

## Task 列表

### T1: MentionPicker 过滤支持 user ID 匹配
**状态**: 需改动
**描述**: 当前过滤只匹配 `display_name`，需同时匹配 `user.id`（设计要求 ID+名字都能匹配）
**文件**:
| 文件 | 改动 |
|------|------|
| `packages/client/src/components/MentionPicker.tsx:16-18` | filter 条件加 `u.id.toLowerCase().includes(query)` |
| `packages/client/src/components/MessageInput.tsx:66-68` | 同步修改 `filteredUsers` 的 filter 逻辑 |

**预估行数**: ~4 行改动
**验证**: 输入 `@ali` 匹配 display_name 含 "ali" 的用户；输入 `@bot1` 匹配 id 含 "bot1" 的用户
**依赖**: 无

---

### T2: 过滤结果限制 10 条
**状态**: 需改动
**描述**: 设计要求最多显示 10 个结果，当前无上限
**文件**:
| 文件 | 改动 |
|------|------|
| `packages/client/src/components/MentionPicker.tsx:16` | `.slice(0, 10)` |
| `packages/client/src/components/MessageInput.tsx:66` | `filteredUsers` 加 `.slice(0, 10)` |

**预估行数**: ~2 行改动
**验证**: 频道有 >10 成员时，空 query 只显示前 10 个
**依赖**: 无

---

### T3: MentionPicker 显示 user ID
**状态**: 需改动
**描述**: 设计要求显示 `display_name (id)` 格式，当前只显示 name
**文件**:
| 文件 | 改动 |
|------|------|
| `packages/client/src/components/MentionPicker.tsx:38` | 在 `mention-name` span 后加 `<span className="mention-id">({user.id})</span>` |

**预估行数**: ~3 行改动
**验证**: 下拉列表中每项显示 "Alice (alice123)" 格式
**依赖**: 无

---

### T4: MentionPicker 显示在线状态图标
**状态**: 需改动
**描述**: 设计要求显示 🟢 / 🤖 / 👤 状态图标
**文件**:
| 文件 | 改动 |
|------|------|
| `packages/client/src/components/MentionPicker.tsx:35-39` | avatar 区域加 role-based 图标 (agent→🤖, 其他→👤/🟢 based on online status) |

**预估行数**: ~8 行改动
**验证**: Agent 显示 🤖，在线用户显示 🟢，离线用户显示 👤
**依赖**: 需确认是否有 online status 数据源（可能需要 AppContext 中的 presence 信息）

---

### T5: 移动端 MentionPicker 底部弹出
**状态**: 需改动
**描述**: 移动端复用 B16 的 bottom-sheet 模式，mention picker 从底部弹出
**文件**:
| 文件 | 改动 |
|------|------|
| `packages/client/src/components/MentionPicker.tsx` | 检测移动端 → 使用 bottom-sheet 布局 |
| `packages/client/src/components/MessageInput.tsx` | 移动端 mention 可能需要调整 position context |
| CSS 文件（待确认） | `.mention-picker` 移动端样式：bottom-sheet 动画 |

**预估行数**: ~30 行改动
**验证**: 移动端视口下 `@` 触发后列表从底部弹出，可触摸选择
**依赖**: B16 bottom-sheet 实现

---

### T6: useMention hook 提取（可选重构）
**状态**: 设计建议但非必须
**描述**: 设计文档提到新建 `hooks/useMention.ts`，将 MessageInput 中的 mention 逻辑抽出。当前逻辑内联在 MessageInput 中（~50 行 state + handlers）。
**文件**:
| 文件 | 改动 |
|------|------|
| `packages/client/src/hooks/useMention.ts` | 新建：提取 mentionQuery/mentionVisible/mentionIndex/mentionStart + handleChange 中的 mention 检测 + insertMention |
| `packages/client/src/components/MessageInput.tsx` | 替换内联逻辑为 hook 调用 |

**预估行数**: ~80 行（新文件 60 + MessageInput 改动 20）
**验证**: 功能与重构前一致，无回归
**依赖**: T1-T4 先完成后再重构更合理

---

## 优先级排序

| 优先级 | Task | 理由 |
|--------|------|------|
| P0 | T1 | 核心功能：ID 过滤是 PRD 核心需求 |
| P0 | T2 | 核心功能：结果数限制 |
| P1 | T3 | UI 完善：显示 ID 帮助用户区分同名用户 |
| P1 | T4 | UI 完善：状态图标（依赖 presence 数据） |
| P2 | T5 | 移动端适配（依赖 B16） |
| P2 | T6 | 代码质量重构（可选） |

## 依赖关系图

```
T1 (ID过滤) ──┐
T2 (限10条)  ──┼──> T6 (useMention hook 重构)
T3 (显示ID)  ──┤
T4 (状态图标) ─┘
                    T5 (移动端) ──> 依赖 B16 bottom-sheet
```
