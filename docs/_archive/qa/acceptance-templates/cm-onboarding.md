# Acceptance Template — CM-onboarding: Welcome channel + auto-join + system message

> 蓝图: `docs/blueprint/current/concept-model.md` §10 (R3 引用 onboarding-journey)
> Journey doc: `docs/implementation/00-foundation/onboarding-journey.md` (野马 PR #190)
> R3 决议: 4 人 review 盲点 B1 + 立场 §1.4 + §11 (2026-04-28)
> 依赖: PR #190 (野马 onboarding-journey) merged + 文案锁定

## 验收清单

### 数据契约 (步骤 1)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 注册成功事务: org + user + #welcome channel (`kind=system`) + channel_member + system message **同事务**写入 | unit | 战马 / 烈马 | _(待填)_ |
| host-bridge / push 调用**不在**注册事务内 | unit (注入失败 mock 验证不影响注册成功) | 战马 / 烈马 | _(待填)_ |
| `kind='system'` 的 system message 支持 quick action button 字段 (CM-onboarding +0.5 天范围) | unit | 战马 / 烈马 | _(待填)_ |

### 行为不变量 — happy path E2E (步骤 1 → 5)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 步骤 1 success: 注册后 `selectedChannelId === welcomeChannelId`, URL 不显示 "👈 选择频道" | E2E | 烈马 | _(待填, INFRA-2 后)_ |
| 步骤 2 success: DOM 含 "**欢迎来到 Borgee 👋**" + 按钮 "创建 agent" 可点 | E2E (DOM 字面文案) | 烈马 | _(待填)_ |
| 步骤 3-4 success: AgentManager 3 步内, 创建成功 toast "🎉 {name} 已加入你的团队" | E2E | 烈马 | _(待填)_ |
| 步骤 4 success system message: "@{name} 上线了, 试试和它打招呼 →" | E2E + unit | 战马 / 烈马 | _(待填)_ |
| 步骤 5 success: 左栏 agent 行 + subject "正在熟悉环境…" (≤2min 默认) | E2E | 烈马 | _(待填, AL-1b 同期)_ |

### 行为不变量 — error 路径 (§11 反约束)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 步骤 1 ❌ register 500: DOM 含 "正在准备你的工作区, 稍候刷新…" + [重试] 按钮 | E2E + fault injection | 烈马 | _(待填)_ |
| 步骤 2 ❌ system message 写入失败: channel 标题位 "⚠️ 欢迎消息加载失败, [重试]"; 不渲染空 channel | E2E + server stub | 烈马 | _(待填)_ |
| 步骤 3 ❌ 名字重复: inline error "这个名字已经有人用了, 换一个吧" | E2E + seed | 烈马 | _(待填)_ |
| 步骤 3 ❌ runtime 503: inline error 文案 + 创建按钮**仍可点** + 创建后 agent 行显示 "故障 (runtime_unreachable)" | E2E + mock runtime | 烈马 | _(待填)_ |

### §11 反约束 grep (CI lint)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 维护 `docs/qa/forbidden-strings.txt` 含 "👈 选择频道" / 单字 "loading…" | CI grep | 飞马 / 野马 | _(待填)_ |
| `grep -r <forbidden> packages/client/src` 必须 0 行 | CI lint job | 飞马 | _(待填)_ |

### G2.4 用户感知签字 (人眼)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| G2.4 截屏 1 "Welcome 第一眼非空屏" (步骤 2) | 人眼 + 截屏 hash | 野马 / 烈马 | _(待填, 入 docs/evidence/cm-onboarding/)_ |
| G2.4 截屏 2 "左栏团队感知" (步骤 5) | 人眼 + 截屏 hash | 野马 / 烈马 | _(待填)_ |
| 步骤 6 §1.3 体感断档口播 (野马 demo 时口头执行) | 人眼 (野马) | 野马 | _(待填, demo 录像)_ |

### 流程级 (CODEOWNERS)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| `.github/CODEOWNERS` 钉 `onboarding-journey.md @yema` | CI (GitHub native) | 烈马 (PR #192) | _PR #192_ |
| 文案改动 PR 列表必含 @yema review | 人眼 (流程) | 飞马 (review 流程) | _(每次 PR 验)_ |

### 退出条件

- 上表 17 项全绿 (E2E 12 + unit 3 + CI grep 1 + 人眼 3 + 流程 1)
- onboarding-journey.md §6 验收挂钩表所有引用项闭合
- 野马 G2.4 demo 签字
