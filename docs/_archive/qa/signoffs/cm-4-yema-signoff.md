# CM-4 闸 4 demo 用户感知签字 — 野马 (PM)

> **状态**: ✅ **SIGNED** (野马, 2026-04-28, post-bug-029 fix #198)
> **历史**: 2026-04-28 初审 → not-signed-pending-fix → 战马 PR #198 修 P0 → 野马补签 ✅
> **任务**: #46 (闸 4 demo 用户感知签字, 4 项 ✅/❌ 验收 + 3 张截图)
> **范围**: CM-4.0 + CM-4.1 + CM-4.2 + bug-029 fix (PR #198)
> **方法**: AI 团队无 GUI sandbox, 不录视频, 改静态代码审 + 文案/立场反查 (闸 3 反查表延伸)。截屏由 INFRA-2 Playwright 后置补归档 (G2.4 demo)。
> **解封**: Phase 1 #30 退出 gate 本签解封, 还差 audit + CM-3。

---

## 1. 验收清单 (野马 R1 锁定 4 项)

| # | 验收项 | 立场锚 | 结果 | 证据 |
|---|--------|--------|------|------|
| ① | inbox 列表对**业主语言友好**, 不暴露 org_id / agent_id 原始 UUID | §1.1 (UI 永不暴露 org_id) + §1.2 (agent=同事) | ✅ **pass (post bug-029)** | PR #198: server `sanitizeAgentInvitation(s, inv)` JOIN users+channels 加 `agent_name` / `channel_name` / `requester_name`; client `InvitationsInbox.tsx:175-198` name 优先, raw UUID 留 `title=` hover; 空串 fallback 防 deleted FK。`TestAgentInvitations_SanitizerKeys` 反向断言 raw-UUID guard + `channel_name == "priv-sanitizer"` |
| ② | quick action [同意/拒绝] 体感即时, error 态有解释 (§11) | README §核心 11 沉默胜于假 loading | ✅ pass | `:81-89` ApiError 409 → "该邀请已被处理或状态已变更, 请刷新"; 其他 err → 显式 errorMsg 而非沉默 |
| ③ | empty 态有 CTA / 解释 (不准空白屏) | §1.4 团队感知 + README §核心 11 | 🟡 partial (v0 接受) | `:132-135` "暂无待处理邀请" / "暂无邀请记录" 显式空文案 (✅ §11), 但**无 CTA** 引导 — 接受 v0, v1 加 CTA (跨 milestone 留账, 见 §4) |
| ④ | bell badge ≥1 时业主一进 app 就能看到 (§1.4 第一眼) | §1.4 团队感知主体验 | ✅ pass | `Sidebar.tsx:35,309-315` pendingInvitations badge 在 sidebar 顶部, 99+ 截断, aria-label 带计数 — 业主视野命中 |

**总体**: 4/4 通过 (③ partial 接受为 v0) → ✅ **签字**, Phase 1 #30 退出 gate 本签解封。

---

## 2. P0 红线 — raw UUID 暴露 (✅ resolved by PR #198)

### 2.1 原现场

`packages/client/src/components/InvitationsInbox.tsx:176-178` (修复前):

```jsx
<strong>邀请 agent</strong> <code>{invitation.agent_id}</code>{' '}
加入 channel <code>{invitation.channel_id}</code>
```

server sanitizer 当时只返回 ID, 不带 name。

### 2.2 立场冲突 (历史记录, 已解)

- **§1.1** "UI 永不暴露 org_id / 内部 ID"
- **§1.2** "agent 是同事不是工具"
- **onboarding-journey §3 步骤 5** 文案锁: agent 显示是 `🤖 {name}`, 不是 ID

### 2.3 修复落点 (PR #198, 战马, ~+108/-17 LOC)

1. **server**: `sanitizeAgentInvitation` 改签 `(*store.Store, *AgentInvitation)`, JOIN users+channels 加 3 个 name 字段; lookup miss → 空串 fallback (FK 删除防御)。4 个 call site 全转。
2. **client**: name 优先渲染, raw UUID 仅 `title=` hover (a11y/调试); `AgentInvitation` interface 加 3 optional name 字段。
3. **反向断言双轨**:
   - `TestAgentInvitations_SanitizerKeys`: white-list 加 3 name key + `channel_name == "priv-sanitizer"` seed 锁 + raw-UUID guard (任何 name 字段 ≠ 对应 ID)。
   - `TestSanitizerOmitsNilOptionals`: nil-store 也保 3 key 在场且为空串, schema 稳定。

### 2.4 业主感知 (修后)

> 邀请你的 agent **助手** 加入 channel **#design**

✅ §1.1 + §1.2 双立场对齐, onboarding-journey §3 步骤 5 文案锁不冲突。

---

## 3. 截屏挂账 (3 张)

> **AI 团队无 GUI sandbox**, 截屏由 INFRA-2 Playwright 后置生成 (G2.4 demo) — 本签字不卡截屏。修红线后再截 ✅。

| # | 内容 | 状态 | 备注 |
|---|------|------|------|
| 1 | inbox 空态 (业主无邀请) | ⚪ Playwright 后置 | INFRA-2 已 merge, G2.4 demo 跑时补 |
| 2 | inbox 列表 + 同意按钮 (name 显示) | ⚪ Playwright 后置 | 修后 agent name + channel name 渲染 |
| 3 | sidebar bell badge ≥1 | ⚪ Playwright 后置 | 同周期截 |

---

## 4. 跨 PR 立场延伸提醒 (留账, 不阻塞本签)

1. **CM-4.3b (邀请 system message DM)** — system message 内引用 agent / channel 必须用 name, 不要 fall back 到 ID, 跟 ① 同根。
2. **CM-onboarding** — step 2 [创建 agent] CTA 跳 AgentManager, AgentManager 列表也要审一遍是否 raw UUID 暴露。
3. **AL-1b** — sidebar agent subject 文案锁已在 onboarding-journey §3 步骤 5。
4. **③ empty 态 CTA v1 留账** — onboarding-journey 立场: empty 必有下一步; 当前 v0 仅文案无 CTA, v1 (Phase 3?) 加 CTA "邀请你的 agent 加入 channel"。

---

## 5. 签字状态机

```
[init]   not-signed-pending-fix (2026-04-28 初审)
   ↓ (战马 PR #198: server sanitizer JOIN + client name 渲染 + 反向断言)
[当前]   ✅ SIGNED (2026-04-28 补签)
   ↓ (Phase 1 #30 退出 gate audit + CM-3 后)
[exit]   Phase 1 关闭, Phase 2 全速
```

---

## 6. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-28 | 野马 | 初次审, 1 P0 红线, not-signed-pending-fix |
| 2026-04-28 | 野马 | PR #198 修 P0 (raw UUID → name), 静态 re-review 通过, ✅ SIGNED |
