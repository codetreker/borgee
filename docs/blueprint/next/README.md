# Next Blueprint Discussion

> 状态: 草拟 / 讨论入口, 不是 frozen blueprint。
> 来源: backlog selection 后进入 `next-iteration` 的 11 条核心 issue。
> 规则: 本目录只讨论下一版产品形状; 草拟期不修改 `_meta/`; `current/` 仍是实施 PR 的唯一冻结依据。freeze / cutover 时才写 source-issues / traceability metadata。

## 0. 一句话定位

下一版蓝图讨论的中心不是重写 Borgee 定位, 而是补齐 v1 使用中暴露出的六类空白: Helper onboarding、mention 粒度、channel authority、client truthfulness、privacy scope guard、sidebar/account IA。

默认版本判断: **非反转集群按 minor bump 讨论**。但 gh#681 no-sandbox 与 current host-bridge 的 v1 process sandbox / sandbox trust pillar 冲突, 是 **major-trigger / open major decision**; freeze 前必须决定是保留 sandbox、重写当前信任支柱, 还是按 major 处理。

---

## 1. 选入集群

| 集群 | Source issues | 讨论主题 |
|---|---|---|
| Host bridge / OpenClaw onboarding | gh#681, gh#659 | 网页配 OpenClaw、创建 / 配置 agent、host-bridge 开机自启 |
| Mention routing 粒度 | gh#674, gh#693 | per-channel `requireMention` 覆盖、`@Everyone` broadcast |
| Channel authority / 管理 | gh#685, gh#688, gh#690 | 用户侧 channel 管理页、owner 不能 leave、私有 channel 锁角标 |
| Client truthfulness | gh#724 | 已写组件必须真 mount; forbidden state 必须可见; e2e 不能假绿 |
| Privacy scope guard | gh#654 | 不扩 privacy/compliance 产品范围; 不撤安全 / 隐私边界 |
| Sidebar / account IA | gh#669, gh#670 | footer 只留关键入口; 头像打开个人面板; logout 进个人面板 |

## 2. Source issue list

### 2.1 Core selected

- gh#681 — 扩展 Remote-Agent, 支持自动配置 OpenClaw
- gh#659 — remote-agent 重启系统后自动启动
- gh#674 — Agent `requireMention` 支持按 channel 区分设置
- gh#693 — Channel `@` 增加 `@Everyone`
- gh#685 — 用户侧缺 Channel 管理页面
- gh#688 — `离开频道` 规则让 owner 困惑
- gh#690 — 私有 Channel 小锁图标太大
- gh#724 — client 实施缺失补完: ArtifactComments mount + ACL forbidden state UX
- gh#654 — 现阶段不要做隐私 / 合规
- gh#669 — 左下角按钮太多
- gh#670 — 左下角头像点击

### 2.2 Stay backlog / conditional

- gh#702 — 只在下一版打开 agent config / onboarding 文案时带入。
- gh#707, gh#697 — quality gate / a11y 完善留 backlog, 不随本次自动扩大范围。
- gh#607 — 文件命名维护项留 backlog, 不进入产品形状讨论。
- gh#675 — 像素风视觉重设仅在用户明确要求时单独开 visual redesign 讨论。

## 3. 明确非目标

- 不冻结下一版蓝图; 本目录只承载四角色 + 用户讨论。
- 不修改 `docs/blueprint/current/` 或 `_meta/`。
- 不改变 issue label / milestone; label 切换由 Teamlead 在流程点处理。
- 不把 Borgee 改成 runtime 平台; OpenClaw 仍是外部 runtime, Borgee 只做接入 / Helper / 配置面。
- gh#681 v1 只讨论 OpenClaw、Mac/Linux、本机 setup; 不做 remote-host setup; 直连 / power-user plugin 路径继续有效, 不被网页配置替代。
- 不新增完整 host command bridge; 命令执行仍走 runtime 能力, 不是 Borgee 平台直接执行。
- 不做 user-facing privacy/compliance 产品扩张; 隐私作为内部安全问题处理, backend / security 控制保留。
- 不把 gh#724 扩成全项目质量平台重建; 只讨论已发现的真实 client surface / forbidden state / e2e 真值问题。
- 不做全产品视觉 redesign; gh#675 不随 sidebar 小修进入本轮。

## 4. Decisions / open decisions

### 4.1 Host bridge

- 网页配 OpenClaw 的 v1 最小闭环是什么: install plugin / create agent / configure channel 是否必须同批交付?
- 已决: 开机自启 + crash restart 都需要, 但只属于长生命周期、非 sudo 的 helper / agent service; `install-butler` 不自启、不常驻、不监督重启。
- 已决: gh#681 host bridge v1 不提供 / 不承诺 sandbox; trust boundary 来自 Borgee 不是 runtime owner、没有 Borgee command channel、显式用户授权、file / network allowlists、非 sudo 常驻服务。Open major: 这反转 current host-bridge 的 sandbox trust pillar, 除非 freeze 时同步重写 / 移除该支柱; sandbox 若重开, 属 backlog / security hardening, 不是 v1 acceptance。
- 已决: Autostart 红线是 bounded restart / backoff、用户可见 status / logs、卸载必须 disable service、不缓存 sudo、不保留持久 privileged installer。
- Helper UI 是否需要把 autostart、卸载、版本提示放进同一个信任说明面板?

### 4.2 Mention routing

- `requireMention` 的 per-channel override 是否采用三态: inherit / on / off?
- per-channel `requireMention` 不能让 channel owner 扩大外部 agent 的 attention / capability; safe rule 是 agent owner 可 opt into broader delivery, channel owner 只能 reduce / mute / remove。
- 已决: `@Everyone` 允许所有 channel member 使用, 不是 owner / 管理者专用。
- 已决: `@Everyone` 必须有 rate limit 与 loop prevention; agent 不能递归触发 broadcast。
- `@Everyone` fanout 必须 server-authoritative: client 只发 token, server 按 membership / ACL 计算 recipients, 不接受 client-supplied recipient IDs。
- 切换 `requireMention` 后是否回扫历史消息? 默认候选: 不回扫。

### 4.3 Channel authority

- 已决: self-created / owned channel 没有 leave 选项, 也不做 owner transfer; owner 只能通过 channel management 删除。
- 已决: delete 可采用 soft delete; hard delete / archive 不作为本轮默认产品承诺。
- 用户侧 Channel 管理页是 Settings 的集中管理入口, 还是 channel 内设置的索引页?
- 私有 channel 锁角标如何避免跟 unread / fault / presence badge 冲突?

### 4.4 Client truthfulness

- 已决: gh#724 拆优先级。ArtifactComments production mount、ACL forbidden UX、security / permission 相关 AP bundle UI 留在下一版高优先级; RT-3 presence polish 与更广 e2e platform / quality expansion 是 backlog extraction candidates, 除非 reviewers 反对。
- forbidden state 是重定向、全屏提示, 还是 channel 区域内空态?
- Forbidden UI 不是 authorization; ACL 成功前不能泄漏 private channel / artifact / message 的 name 或 body。
- e2e 反向证机制进入 blueprint invariant, 还是只作为 gh#724 实施验收项?

### 4.5 Privacy scope guard

- gh#654 只区分 backend/security controls 与 user-facing privacy/compliance product; 删除既有 privacy/security boundary 不是 gh#654 解释, 必须另开 major proposal + threat model。
- 必须保留的控制: admin 不能读内容、server-side impersonate/admin/capability controls、data minimization、admin/user/agent rail separation。
- 已决: 删除 / 避免用户侧 privacy promise UI、用户侧 admin impact records / audit、用户侧 impersonate authorization UI; 这些是 cleanup / out of scope, 除非用户重新打开。

### 4.6 Sidebar / account IA

- footer 外露入口默认候选: 头像、Agents、Workspace、Settings。
- Invitations、Remote Nodes、logout 分别进入哪里: 设置、Helper 面板、个人面板?
- 移动 IA 入口不等于合并职责: Remote Agent 与 Helper / Host Bridge 仍分开, credentials / grants / enforcement rails 不合并。
- 个人面板 v1 只放 display name / account summary / logout, 还是也开放账号设置?

## 5. 下一步 review flow

1. PM 先确认 source issue cluster 与非目标, 防止 backlog 误带入。
2. Architect 对照 `migration-analysis.md`, 先解决 gh#681 no-sandbox major-trigger / open major decision, 再判断其余集群是否仍按 minor 处理。
3. Security 独立检查 gh#681 / gh#654 / gh#693 / gh#724 的边界, 尤其是 runtime owner、host command、broadcast abuse、隐私 / 安全边界删除。
4. QA 把 open decisions 转成可验收的反向检查, 尤其是 client truthfulness 和 forbidden state。
5. 用户拍板后, Teamlead 决定是否把 `next/` 冻结并切到下一版 `current/`。
