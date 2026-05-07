# 684 — Agents 管理页 Manage 区排版重组 — 产品立场 brief

> Issue: gh#684 "Agents 页面里的详情排版混乱"
> 作者: yema (PM, 立场 brief)
> 实施分支: `fix/agents-detail-layout` (worktree `.worktrees/agents-detail-layout`, team-lead 起)
> 待实施: dev 按本 brief 起 design doc + 实施
> 关联: PR #706 (#698 form 重叠) 同模式 design — `docs/implementation/design/al-agent-form-overlap.md`

## §0 范围

修 `packages/client/src/components/AgentManager.tsx` 的 Manage 展开区 (line 231-410, expanded section). 用户拍 3 件事:
1. 页面排版混乱要重新组织
2. Prompt 输入框默认太小
3. "Show" 按钮多余 → 默认显 mask 后 key + 加复制按钮直接拷剪贴板

**不在范围**:
- AgentConfigPanel 内部 form 排版 (`packages/client/src/components/AgentConfigPanel.tsx`) — 那是 #698 / PR #706 在做的事, **不动**, 等 #698 合后再核对视觉一致
- Create Agent 模态 (`AgentManager.tsx` line 354+) — 用户没报, 不动
- 其它 form (Settings / Workspace / Invitations) — 不动

## §1 当前页面 section 顺序 (Manage 展开后真实层级)

按 `AgentManager.tsx:231-410` 现状, Manage 展开后从上到下:

1. **Header 行**: agent 名 + 在线状态 + ID + Created 日期 + Collapse + Delete 按钮
2. **API Key 段**: "API key is hidden." + Show 按钮 → 点 Show 拉明文 → 显 + 复制图标 + Hide 按钮 + Rotate API Key 按钮
3. **Runtime 卡片** (RuntimeCard, 仅 owner): 跑状态 / start / stop
4. **Config (SSOT)** 段: 标题 + AgentConfigPanel (那个 form 是 #698 在修)
5. **Permissions** 段: KNOWN_PERMISSIONS 复选框
6. **Add to Channel** 段: channel 下拉 + Add 按钮

视觉问题 (按用户截屏判断):
- Header 跟 API Key 之间没明显分组, 视觉上跟 Config / Permissions 平级
- API Key 段 Show 按钮 + Rotate 按钮分两行, 没必要
- Config (SSOT) 跟 Agent 配置 重复标题, 多此一举
- Permissions 跟 Add to Channel 没有分组结构, 平铺
- 整页没有"读"vs"写"的视觉分层

## §2 产品方向 (PM 立场, dev 按这条写 design doc)

### §2.1 Section 序 (重新组织, 按"身份 → 凭证 → 配置 → 权限 → 加入"语义)

新顺序 (上到下):

1. **Identity (身份卡)**: 头像 + 名 + ID + Created 日期 + 在线状态 + 顶部按钮 (Collapse / Delete)
   - **当前 Header 行重组**, 加头像缩略 (跟 channel 列表 channel 头像一致大小, 32px)
   - 状态 dot 跟在线/离线/故障三态色环对齐 (按 `agent-lifecycle.md §2.3` 三态色环)

2. **Credentials (凭证卡)**: API Key + Rotate 同段
   - 默认显 **mask key** (例: `bgr_...abc1` 形式, 末 4 位露)
   - 复制按钮 (📋) 在 key 右内嵌, 点击直接 `navigator.clipboard.writeText` 完整明文 + 短 toast "已复制" (1.5s)
   - **不再有 "Show" 按钮**: 用户原话 "Show 按钮多余, 默认 mask + 加复制" — 完整明文从不渲染到 DOM, 安全更高 (跟 host-bridge §1.2 安全四件套精神一致)
   - Rotate API Key 按钮放 mask 行的右侧 / 或下面紧挨, 不分两行

3. **Runtime (运行时卡)**: 沿用 RuntimeCard, 不动 (owner-only DOM gate)

4. **Config (配置卡)**: 删 "Config (SSOT)" 标题, 直接渲染 AgentConfigPanel
   - "Agent 配置" 标题在 AgentConfigPanel 内部已经有了 (`<h3>Agent 配置</h3>`, AgentConfigPanel.tsx:112)
   - 外层 `<strong>Config (SSOT)</strong>` 是冗余, 删掉
   - 注: form 排版本身是 #698 / PR #706 在修, 不动

5. **Permissions (权限卡)**: 现有 KNOWN_PERMISSIONS 复选框, 不动

6. **Channels (加入频道卡)**: 现有 Add to Channel 段, 不动

每个卡之间用 `<section>` + 上下 16px margin + 浅色 border 分隔 (跟 channel 列表分组的视觉规矩一致), 让"读"vs"写"区分明显.

### §2.2 Prompt 输入框尺寸

用户原话 "Prompt 输入框默认太小". 当前 AgentConfigPanel 内 prompt textarea 默认 1 行 (浏览器默认):
- **新规矩**: prompt textarea `rows={8}` (8 行默认高), 字体保持 `var(--text-base, 14px)` (不强制 monospace, 跟 borgee 既有 textarea 一致 — 这是写 prompt 的, 不是写代码)
- 用户拖拽 resize (CSS `resize: vertical`) 允许扩到 20 行
- 反约束: 不强制 monospace 字体 (prompt 是自然语言指令, 不是代码; monospace 反 Borgee 整体视觉风格)
- 注: 这条改的是 AgentConfigPanel.tsx 内部 prompt textarea 属性, 跟 #698 / PR #706 排版改动是同一文件, 实施时 dev 协调先后 (建议本 PR rebase #706 合后再做, 防 git 冲突)

### §2.3 API Key 显示策略 (重点)

> **Security 立场已并入** (heima 2026-05-08 pre-design 4 点立场): mask 末 4 位露 / clipboard same-origin only / **auto-clear 60s** / 不引第三方 clipboard 库. 详见 §2.3 各项已标 "Sec" 来源.

- **默认显示** (Sec 立场 1): `bgr_...{last4}` 模式 (前 4 字符 `bgr_` + 中间 `...` + 末 4 字符), 例: `bgr_...abc1`
  - **前缀 `bgr_` 按 borgee 真值** (zhanma 反查 `server-go/internal/store/queries_phase2b.go:440` `GenerateAPIKey()` 返 `"bgr_" + hex.EncodeToString(b)`, 格式是 `bgr_<64 hex>`, 不是 OpenAI `sk-` — 之前 brief v1 我抄行业模板没核 borgee 真值, zhanma 抓得准)
  - 末 4 位露**够认人不够暴破解**, 思路跟 GitHub PAT / Stripe / OpenAI 行业一致, 前缀按 borgee 真值 `bgr_`
  - 反约束 (Sec 立场 1): 末 4 位**不能用来做认证 / lookup**, server 端拒任何 "按末 4 位查 key" 的 endpoint
- **复制按钮**: `📋` 图标按钮, 紧贴 mask key 右侧 (内嵌), aria-label="复制 API Key", title="复制完整 API Key 到剪贴板"
- **复制行为**: `navigator.clipboard.writeText(完整 key)` + toast "API Key 已复制, 60 秒后自动清空" (Sec 立场 3, 较长 toast 持续到清空动作前 — 不是 1.5s 普通短 toast)
- **剪贴板 auto-clear** (Sec 立场 3, 重点新加): 复制后启动 `setTimeout(60_000)` → 60 秒后 `navigator.clipboard.writeText('')` 清空剪贴板
  - 理由: 防剪贴板残留被别 app / 输入法 / 截图 OCR / macOS Universal Clipboard 跨设备同步等抓走
  - 60s 是 1Password / Bitwarden 行业默认值, 用户语境一致
  - 边界: 用户在 60s 内已经粘贴去别处 (例如填进 OpenAI dashboard) — 那次粘贴成功了, auto-clear 只是清浏览器剪贴板, 不影响已粘贴值
  - 边界: 用户在 60s 内主动复制别的内容 — 我们的 setTimeout 还会触发, 但 `writeText('')` 只清当前剪贴板, 用户的新复制内容是后写的会被清掉. 修法: 复制时存当前 key 字符串到 closure, 60s 后**先 readText 比对**, 只在剪贴板里还是这把 key 时才清 (比对失败说明用户已经主动改了剪贴板, 不动)
- **失败兜底**: 浏览器不支持 clipboard API 时 (例如非 https / 旧浏览器), fallback 到 `document.execCommand('copy')`; 都失败 → toast "复制失败, 请手动选择 mask 后的 key 复制片段"
- **Show / Hide / 明文 DOM 永远不渲染** (Sec 立场 by-construction) — 完整明文从 server `GET /api/v1/agents/:id/api_key` 拉到内存后只走 clipboard, 不进 DOM. 关键安全提升.
  - 反约束 (Sec 立场 4): mask 切回真值如果未来要做 (admin "Show full key" 按钮) **必须经过 server 二次校验当前 session**, 不能 client 缓存 key 字符串后展示 (防 IDOR, 跟 #687 同源 by-construction 防御)
- **不引第三方 clipboard 库** (Sec 立场 4): 保持 navigator.clipboard 原生, 不引 clipboard.js / copy-to-clipboard 等. 现状已经是原生, 不退化
- **Rotate API Key 按钮**: 位置在 mask key 那一行的最右侧 (跟复制按钮同行) 或紧贴下方, 不分两行
- 边界:
  - **没 key 状态** (新 agent 还没生成): 显 "尚未生成 API Key" + 显式 "Generate API Key" 按钮 (跟 Rotate 复用同 endpoint)
  - **加载中**: mask 位置显 spinner 或 "加载中..." 文字, 复制按钮 disabled
  - **agent 不在线 / API key 失效**: 不影响 mask 显示 (mask 只是把 server 已存的 key 走客户端展示; 失效是 runtime 层问题, 跟 mask 无关)
  - **clipboard same-origin / user gesture** (Sec 立场 2): `navigator.clipboard.writeText` 浏览器原生防 cross-origin + 必须 user gesture (按钮点击触发), 不需要额外 client 防御. 同 page 别 component 监听 `paste` 事件能拿到, 但 paste 必须用户主动粘贴, 不是 background sniff, 不需防

### §2.4 反约束 (硬性, 写给 dev 当 grep 守卫)

```
# Show 按钮永不再出现 (用户原话明文禁)
grep -rnE "['\"]Show['\"]|reveal.*key" packages/client/src/components/AgentManager.tsx | grep -v _test  # 应 0 hit

# 完整明文 API key 永不进 DOM (反向核安全, Sec 立场 by-construction)
grep -rnE "innerText.*api_key|innerHTML.*api_key|{.*api_key.*}" packages/client/src/components/AgentManager.tsx | grep -v _test
# 应该只在 clipboard.writeText 调用里有, 不在 JSX render 里

# 第三方 clipboard 库不许引 (Sec 立场 4)
grep -rE "from\s+['\"]clipboard['\"]|copy-to-clipboard|clipboard\.js" packages/client/src/  # 应 0 hit

# auto-clear setTimeout 必须真接 writeText('') (Sec 立场 3)
grep -rn "setTimeout.*60.*writeText\|writeText\\(''\\)" packages/client/src/components/AgentManager.tsx | grep -v _test  # 应 ≥1 hit

# 反 OpenAI sk- 前缀误用 (zhanma 反查 server 真值是 bgr_ 不是 sk-, 防有人复制粘贴 OpenAI 文档误抄)
grep -rnE "['\"]sk-\.\.\." packages/client/src/components/AgentManager.tsx | grep -v _test  # 应 0 hit

# Rotate API Key 按钮文案不变
grep -rn "Rotate API Key" packages/client/src/components/AgentManager.tsx  # 字面保留 byte-identical (跟 al-2a content lock 不撞 — 那个锁的是 form SSOT 字段, 不是 API Key 段文案)
```

## §3 文案锁 (新加, 给 dev 写 design doc 时锁)

#684 引入新用户可见文案, byte-identical 锁住 (改 = 改三处: design + AgentManager.tsx + 单元测试断言):

| 位置 | 文案 byte-identical | 备注 |
|---|---|---|
| Mask 模式 | `bgr_...{last4}` | 前 4 char `bgr_` + `...` + 末 4 (Sec 立场 1: 露末 4 位够认人不够暴破解; 前缀 `bgr_` 按 borgee `GenerateAPIKey()` 真值, 不是 OpenAI `sk-`) |
| 复制按钮 aria-label | `复制 API Key` | screen reader 用 |
| 复制按钮 title (hover) | `复制完整 API Key 到剪贴板` | 鼠标悬停提示 |
| 复制成功 toast | `API Key 已复制, 60 秒后自动清空` | 持续到 60s auto-clear 触发, 不是 1.5s 短 toast (Sec 立场 3 — 用户必须知道剪贴板会被清) |
| auto-clear 后 toast (可选) | `剪贴板已清空 (安全保护)` | 60s 触发后短 toast 1.5s 提示用户已清, 反 dev 实施时漏告诉用户 |
| 复制失败 toast | `复制失败, 请手动选择 mask 后的 key 复制片段` | 浏览器不支持时降级 |
| 没 key 占位 | `尚未生成 API Key` | 跟 borgee 既有空状态文案精神一致 |
| Generate API Key 按钮 | `Generate API Key` | 沿用 Rotate API Key 命名风格 |
| 加载中占位 | `加载中...` | 跟 borgee 既有 loading 占位一致 |

反向 grep 反近义词漂移 (写给 dev):
```
grep -rnE "['\"]显示\\s*key['\"]|['\"]reveal.*api['\"]|['\"]copy\\s*key['\"]|['\"]Show.*Key['\"]" packages/client/src/  # 应 0 hit
grep -rnE "['\"]复制成功['\"]|['\"]Copied['\"]" packages/client/src/components/AgentManager.tsx  # 应 0 hit (统一用 "已复制")
grep -rnE "['\"]sk-" packages/client/src/components/AgentManager.tsx  # 应 0 hit (反 OpenAI 前缀误用, borgee 真值是 bgr_)
```

## §4 蓝图覆盖比对

蓝图反向核 (查"agent detail / agent manage / api key 显示策略 / mask"):

- `client-shape.md`: 没锚 agent detail 页面排版规矩 (沉默, 实施层可拍)
- `client-shape.md §1.3`: 锚了 agent 故障态 UX "API key 已失效, 需要重新填写", 但这是错误文案规矩, 跟 mask 显示规矩不冲突
- `agent-lifecycle.md §2.3`: 故障原因码 6 个含 `api_key_invalid`, 但这是状态枚举, 跟 mask UI 不冲突
- `host-bridge.md §1.2`: 安全四件套 (白名单双签 / 进程沙箱 / 更新策略 / 一键卸载) 精神跟 "默认 mask + 复制不渲染明文" 方向一致
- `plugin-protocol.md §1.4`: agent_config SSOT 字段划界 — `api_key` 在 **runtime 管** 列 (不在 Borgee SSOT form), 但 **API Key 段不是 config form**, 是独立段 (`AgentManager.tsx:234-254`), 不冲突
- `al-2a-content-lock.md ②`: 锁 SSOT form **不准** 出现 `api_key` 字段, 跟本 brief 不冲突 — 本 brief 改的是 API Key **独立段** 的显示策略, 不动 SSOT form

**蓝图层沉默 / 既有规矩兼容**, 实施可走.

## §5 测试策略 (给 dev 起手)

| 层 | 覆盖 |
|---|---|
| **vitest unit** | AgentManager.tsx 现有 unit test 锚 ID/标题文案, 加: (a) mask 模式 `bgr_...{last4}` 字面渲染 (b) 复制按钮 aria-label / title byte-identical (c) Show 按钮 DOM 不再出现 (反向断言) (d) 复制成功 toast 文案 byte-identical (e) 反 OpenAI 前缀 `sk-` 不出现 |
| **e2e** | 加 1 个新 spec (或扩 `al-2a-acceptance-followup.spec.ts`): 锚点 1280 viewport / login / 进 Agents 页 / Manage 展开 → mask key 显示 / 点复制按钮 → toast 出现 + clipboard 被写 (Playwright 有 `page.evaluate(() => navigator.clipboard.readText())` 但需 grant context.grantPermissions). |
| **手工真验 (liema 6 步)** | (1) testing 1280 / login / Agents / Manage 展开 → 看 section 6 个卡 (Identity / Credentials / Runtime / Config / Permissions / Channels) 顺序 + 视觉分层 (2) Credentials 段默认显 mask, 没 Show 按钮 (3) 点复制按钮 → toast "已复制" + 浏览器剪贴板真有完整 key (paste 到记事本验证) (4) Prompt textarea 默认 8 行, 可拖拽扩到 20 行 (5) 480px mobile viewport / 同样路径 → 6 个卡纵向 stack 不溢出 (6) 反向核: 整个 Manage 区 DOM 里全文搜 "Show" 按钮 / 完整明文 key — 都应该 0 hit |

## §6 §10 留账 (给 dev 写 design doc 时落)

- **clipboard API fallback**: 浏览器不支持 navigator.clipboard 时 (例如非 https / 旧浏览器), 走 document.execCommand fallback. 实施时反向 grep `navigator.clipboard|execCommand` 在 packages/client/ 看是否有既有 helper 复用.
- **mask 字面拼接抽 helper**: 如果项目其它地方未来也要 mask secret (admin 那边 admin api key 可能), 抽 `lib/mask.ts::maskSecret(key, prefix=3, suffix=4)` helper. 不在 #684 范围, 但 design 提一句给 dev 留 followup issue.
- **Generate API Key (空状态)**: 现有代码可能没有"没 key" 路径 (假设 agent 创建时一定生成 key). 实施时 dev 反向 grep 验证 — 如果 always 有 key, 这条边界可不实现, design §2.3 改成 "agent 必有 key, 不需要空状态" + 反向 grep 锚 0 hit "尚未生成 API Key".
- **server-side mask + reveal endpoint 拆分** (team-lead + heima + yema 拍, follow-up): 当前 `sanitizeAgent` (`packages/server-go/internal/api/agents.go:86-103`) 返完整 plaintext, owner-gated by-design (heima 反查 L240 `agent.OwnerID != user.ID → 403`). #684 client-only mask 不改 server. server-side mask 需要: (a) 改 `sanitizeAgent` 默认返 mask (b) 加 `POST /api/v1/agents/:id/reveal_api_key` 显式 reveal endpoint (c) reveal 加 audit log 记 user + ts + IP. 范围扩到 server + 涉及 audit / admin god-mode 决策, 走独立 brainstorm + 蓝图迭代. **不在 #684 范围, 留 follow-up issue** (跟 audit log 那条一起讨论).
- **audit log 拉 key 事件** (heima 提, follow-up): 加 server audit log 记 GET api_key + reveal 动作 (user + ts + IP). 跟 "server-side mask + reveal endpoint" 是同一波 server side 重设计, 单独决定容易 stance 不齐, 一起讨论 stance 后再开实施 issue.

## §7 不在范围

- 不动 AgentConfigPanel 内部 form 排版 (#698 / PR #706 在做)
- 不动 Create Agent 模态 (用户没报)
- 不动 admin SPA agent 管理路径 (不是同一 SPA)
- 不抽通用 mask helper (留 followup, 见 §6)
- 不引入第三方 clipboard 库 (跟 #691 design 反第三方 UI 库精神一致)
- 不动 API Key server 端 endpoint (`GET /api/v1/agents/:id/api_key`, `POST /api/v1/agents/:id/rotate_key`) — 端点不变, 只动 client 显示

## §8 实施步骤建议给 dev (4 签后)

1. 等 PR #706 (#698 form 重叠) 合到 main, 防 AgentManager.tsx 文本冲突
2. 改 `AgentManager.tsx`:
   - 重组 expanded section 顺序 (6 卡)
   - 删 "Show" 按钮 + setVisibleKey state + handleShowKey 函数 (用户拍 "Show 多余")
   - 加 mask 模式 helper (`maskApiKey(key)` 内联或 `lib/mask.ts`)
   - 加复制按钮 + clipboard 写入 + toast
   - 加 "Identity" / "Credentials" / "Channels" 等卡片 wrapper
3. 改 `AgentConfigPanel.tsx` 内部 prompt textarea 加 `rows={8}` + `style={{ resize: 'vertical' }}`
4. 加 vitest 单元测试 (按 §5)
5. 加 e2e spec (按 §5)
6. testing 真验路径
7. team-lead 开 PR (Closes gh#684)
8. 4 角色 review 后 merge

## 4 角色 review 待签 (本 brief 是 PM 立场, dev 真实施 design doc 4 签走 design doc)

- [x] **PM (yema)**: 立场 brief (本文档) — section 序 / mask 策略 / Prompt 尺寸 / 文案锁 + byte-identical 反 grep 守卫
- [ ] **Architect (feima)**: 实施 design doc 时签 (架构合理性 + AgentManager.tsx 重组的最小手术 + 跟 #698 / PR #706 协调先后)
- [ ] **QA (liema)**: 实施 design doc 时签 (testing 真验 6 步 + 单元测试覆盖 + 反向 grep 守卫)
- [ ] **Security (heima)**: 实施 design doc 时签 (mask + clipboard 安全提升的核对 + 完整明文不进 DOM 的真验)

---

**给真做 #684 的 dev**: 本 brief 是 PM 立场输入, 你按本 brief 写完整 design doc (跟 #686 / #687 / #691 / #698 同模式 9-10 段) 然后走 4 签流程. 任何反对 / 需要改 PM 立场的, SendMessage yema 讨论, 不擅自改 brief 字面.
