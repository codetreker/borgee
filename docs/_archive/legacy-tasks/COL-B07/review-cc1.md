# B07 Task Breakdown Review — CC Round 1

> 审阅对象：`task-breakdown.md` vs `design.md` + `prd.md` + `docs/ui/slash-commands.md`
>
> 日期：2026-04-24 | 审阅者：CC

---

## CRITICAL

### C1. 多参数命令完全缺失

**来源**：PRD 需求 4、线框图 10d

PRD 明确要求支持多参数命令（branch、environment、message），线框图 10d 展示了多字段表单（含必填 `*` 标记、Cancel / Execute 按钮）。但 `AgentCommand` 接口只有一个 `paramType` 字段，无法描述多个参数。task-breakdown 中没有任何 task 处理多参数 schema 或多参数表单 UI。

**影响**：命令参数模型从根上不匹配 PRD，涉及 T1（存储 schema）、T4a（协议）、T5（前端 registry）、以及一个缺失的参数表单 task。

**建议**：要么 v1 scope down 为"单参数 text"并在 PRD 中标注降级，要么扩展 AgentCommand 为 `params: Array<{ name, type, required, placeholder }>` 并新增参数表单 task。

---

### C2. 频道维度命令可见性缺失

**来源**：PRD 需求 2 验收标准

> "只显示当前频道内 Agent 的命令（Agent 不在频道则命令不显示）"

design 和 task-breakdown 中 CommandStore 按 agentId/connectionId 存储，完全没有 channel 维度。`GET /api/v1/commands` 返回全局命令列表，不接受 channelId 参数。前端也无过滤逻辑。

**影响**：用户在 A 频道会看到 B 频道 Agent 的命令并可尝试执行，违反 PRD。

**建议**：要么 `GET /api/v1/commands?channelId=xxx` 服务端过滤（需要 Agent→channel 映射），要么前端根据频道成员列表过滤（需知道哪些 agentId 在当前频道）。

---

### C3. 命令消息持久化 vs PRD "不持久化"矛盾

**来源**：PRD 需求 3

> "命令调用记录不作为普通消息持久化（仅结果消息持久化）"

design T4b 将 command 消息通过 `Q.createMessage()` 写入 DB，与 PRD 直接矛盾。用户重新加载页面后会看到 command 消息但可能没有对应结果。

**建议**：明确决策——要么 PRD 让步允许持久化（推荐，简单），要么 command 消息走 ephemeral 通道不落库。无论哪种需更新文档一致性。

---

### C4. 参数输入表单 UI 无对应 task

**来源**：线框图 10d

10d 展示了选中命令后的参数输入面板（表单字段 + placeholder + Cancel/Execute 按钮），这是核心交互流程的一环。task-breakdown 中没有任何 task 覆盖此 UI 组件。

T5 只处理 SlashCommandPicker（命令列表/搜索/同名选择），T4c 只处理 pending/timeout。参数表单是一个独立组件。

**建议**：新增 task（如 T5b）实现 `CommandParamForm.tsx`，依赖 T5。

---

## HIGH

### H1. Agent 离线预检缺失

**来源**：PRD 需求 3 验收标准

> "Agent 不在线时提示'Agent 离线，无法执行命令'"

当前设计仅有 30s 超时事后提示，没有发送前的在线状态预检。用户选择一个刚断开的 Agent 命令后要等 30s 才知道 Agent 离线。

**建议**：由于命令在 WS 断开时自动清除，理论上 picker 中不会出现离线 Agent 命令。但存在 race condition（Agent 刚断开、commands_updated 尚未到达前端）。建议在 T4c 中补充说明此 race 场景的处理策略，或在 PRD 中将预检降级为"best-effort"。

---

### H2. /nick 线框图要求专用面板，task 未覆盖

**来源**：线框图 10h

10h 展示 `/nick` 有独立输入面板：显示当前昵称（`Current: Alice`）、输入框、Cancel / Change 按钮。T7 将 /nick 实现为简单 text 参数 + ephemeral 消息，无专用 UI。

**建议**：T7 的 /nick 实现需包含 `NickChangePanel.tsx`（或复用 C4 的参数表单 + 额外"当前昵称"显示）。

---

### H3. T8 测试缺少前端测试

**来源**：design.md 测试策略

design.md 测试策略明确包含"CommandRegistry（前端）：resolve/search 覆盖内置+远程+冲突+ambiguous"，但 T8 只有 3 个服务端测试文件，零前端测试。

**建议**：T8 新增 `packages/client/src/__tests__/registry.test.ts`，覆盖 resolve 三态 + search 分组 + 内置优先级。

---

### H4. command_id 追踪机制不明确

**来源**：T4b + T4c

T4b 将 `command_id` 存入消息 metadata（JSON），T4c 用 `reply_to_id === command_id` 匹配。但 `reply_to_id` 通常指向消息 ID 而非 metadata 中的值。Agent 如何得知 command_id 并设置 reply_to_id？

design.md 说"Agent 回复普通文本消息，附带 reply_to_id = command_id"，但 command_id 在 metadata 中，Agent 需从收到的消息里解析 metadata JSON 才能拿到。

**建议**：明确 command_id 是否就是消息 ID 本身（更简单），还是 metadata 中的独立字段（需文档化 Agent 解析方式）。

---

### H5. PRD 验收标准 "返回命令 ID" + "删除单条命令" 未覆盖

**来源**：PRD 需求 1 验收标准

> - "Agent 通过 API 注册命令成功，返回命令 ID"
> - "Agent 可以删除已注册的命令"
> - "同一 Agent 注册重复命令名返回冲突错误"

design 使用 snapshot 全量替换语义（无命令 ID、无单条删除、无冲突错误）。这是 PRD 和 design 之间的分歧，task-breakdown 跟随了 design。

**建议**：确认 PRD 已被 design 决策 supersede（snapshot 语义取代 CRUD），并更新 PRD 验收标准以匹配。否则需在 T1/T2 中实现单条 CRUD。

---

### H6. 线框图 10a System 组包含 help/invite/leave/topic/dm 但 task-breakdown 未提及迁移

**来源**：线框图 10a

线框图 10a 的 System 组只展示 status/clear/nick，但 PRD 和 design 都提到 8 个内置命令。现有 5 个内置命令（help/invite/leave/topic/dm）需要在新的分组 UI 中正确渲染。T5 重写 SlashCommandPicker 时需确保现有命令不回归。

**建议**：T5 验证标准中明确"现有 5 个内置命令在分组 UI 中正常显示"。

---

*End of review.*
