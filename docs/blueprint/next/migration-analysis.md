# Next Blueprint Migration Analysis

> 状态: 下一版蓝图讨论用影响分析。不是冻结结论。

## 0. 版本判断

默认判断: **非反转集群为 minor bump; gh#681 no-sandbox 是 major-trigger / open major decision**。

理由: 多数 issue 只是给已冻结的产品立场补粒度、补页面、补真实 UI surface, 没有默认推翻“Borgee 是 agent 协作平台, 不是 agent 平台”。但 current `host-bridge.md` 把 process sandboxing / sandbox 写入 v1 trust model; gh#681 的“不提供 / 不承诺 sandbox”是该支柱的 stance reversal, 不能无条件归为 minor。

进入 **major bump** 的触发条件有三类:

- Borgee 从接入 / 配置 / Helper 变成 runtime owner, 直接承担 LLM/runtime/shell 执行责任。
- gh#681 no-sandbox 保持为 v1 stance, 且 freeze 时没有同步重写 / 移除 current host-bridge 的 process sandboxing trust pillar。
- 既有 privacy / security 边界被删除, 例如 admin 默认可读内容、跨 org 可替别人 agent 扩权、backend/server-side controls 被移除。

---

## 1. Cluster impact

| 集群 | 触达 current blueprint | 影响判断 | bump |
|---|---|---|---|
| Host bridge | `host-bridge.md`, `plugin-protocol.md`, `agent-lifecycle.md`, `client-shape.md` | 强化 Helper onboarding / autostart; gh#681 no-sandbox 反转 current sandbox trust pillar, freeze 前必须解决 | major-trigger / open |
| Mention routing | `concept-model.md`, `channel-model.md`, `auth-permissions.md`, `realtime.md` | 增加 per-channel 接收策略与 broadcast mention; 需补 abuse / rate-limit / ACL 红线 | minor |
| Channel authority | `channel-model.md`, `concept-model.md`, `client-shape.md`, `data-layer.md` | 补 owner leave / delete / 管理页 / 私有标识规则 | minor |
| Client truthfulness | `client-shape.md`, `canvas-vision.md`, `auth-permissions.md`, `realtime.md` | 要求已实现能力在 production UI 真实可达, 无权访问有明确状态 | minor |
| Privacy scope guard | `admin-model.md`, `host-bridge.md`, `auth-permissions.md`, `concept-model.md` | 不扩 compliance 产品面; 既有安全 / 隐私边界保留 | no expansion |
| Sidebar / account IA | `client-shape.md`, `channel-model.md` | 补 sidebar footer 与 account entry 细节 | minor |

## 2. Host bridge impact

### 当前立场

- Borgee 不带 runtime, 通过 plugin 接 OpenClaw / Hermes / 自建。
- Borgee Helper 在用户机器上管理 runtime 安装 / 配置, 但不直接替 runtime 执行命令; 长生命周期服务不持有 sudo。
- v1 的信任模型来自拆 daemon、签名 / 沙箱 / 用户授权、allowlists、一键卸载、不自动更新、情境化授权; current 明确要求 process sandboxing。

### 下一版增量

- gh#681 把“网页配 OpenClaw”补成 onboarding 主路径: 安装 plugin、创建 agent、配置 channel。
- gh#681 v1 scope guard: OpenClaw only, Mac/Linux only, local-host setup only; 不做 remote-host setup; 直连 / power-user plugin 路径仍合法。
- gh#659 把长生命周期、非 sudo helper / agent service 的常驻语义补成 OS 重启后自动恢复与 crash restart。
- 这些都是 host-bridge 目标态的完成项, 不是让 Borgee 成为 runtime owner。
- Autostart 边界: 只给长生命周期 helper / agent service; 不做 boot-time installer, 不缓存 sudo, 不给短命 `install-butler` 做 supervised restart loop。
- Guardrails: bounded restart / backoff、用户可见 status / logs、卸载必须 disable service、无持久 privileged installer。

### Host trust boundary decision

gh#681 的用户决策可以被下一版记录, 但它反转 current host-bridge 的 v1 process sandbox / sandbox trust pillar。除非 freeze 时同步重写 / 移除 current sandbox 信任支柱, 该变化必须按 major-trigger / open major decision 处理, 不能默认 minor。

已决边界:

- gh#681 host bridge v1 不提供 / 不承诺 sandbox; trust boundary 来自没有 Borgee command channel、显式用户授权、file / network allowlists、非 sudo 长生命周期服务。sandbox 若重开, 属 backlog / security hardening, 不是 v1 acceptance。
- 长生命周期 helper / agent service 不持有 sudo。
- file / network 只走 allowlist, 授权状态由 owner 控制。
- v1 没有 Borgee command channel; shell / 命令执行继续属于 runtime。
- `install-butler` 只做装卸, 不常驻, 不开机自启, 不监督重启。

### Major 风险

- 如果 no-sandbox 作为 v1 acceptance 保留, 但 freeze 不重写 current 的 process sandboxing / sandbox trust pillar, 就是 stance reversal。
- 如果网页配置演变成 Borgee 自己托管 / 调度 runtime, 就越过 current 的平台边界。
- 如果 Borgee 直接暴露 host command 通道, 就越过 `host-bridge.md` v1 “不直接跑命令”的红线。
- 如果 Helper / Host Bridge 与 Remote Agent 在 IA 移动时合并 credentials / grants / enforcement rails, 也是安全边界删除 / 绕开。

## 3. Mention routing impact

### 当前立场

- Agent 在 channel 里代表自己, mention 不展开到 owner。
- 离线 agent 的 owner 只收到“有人找过它”的 system 提示, 不转发原文。
- 跨 org 协作可以, 扩权不行。

### 下一版增量

- gh#674 增加 per-channel `requireMention` override, 用 channel 语境调节 agent 是否主动接收消息。
- gh#693 增加 `@Everyone` broadcast mention; 所有 channel member 都可使用。
- 需要补清楚 rate limit、agent 递归触发禁止、ACL fanout 过滤。

### Safe rule

- per-channel `requireMention` 不能让 channel owner 扩大外部 agent 的 attention / capability。
- agent owner 可以 opt into broader delivery; channel owner 只能 reduce / mute / remove。
- `@Everyone` fanout server-authoritative: client 只发 token, server 按 membership / ACL 计算 recipients。
- server 不接受 client-supplied recipient IDs; 任何 fanout 都必须重新检查 channel membership / access。
- `@Everyone` 需要 rate limits 与 loop prevention; agent 不能递归触发 broadcast。

### Major 风险

- 如果 `@Everyone` 允许跨 channel / 跨 org 越权 fanout, 是 security boundary 删除 / 绕开。
- 如果离线 fallback 开始转发原消息给 owner, 是 privacy boundary 删除 / 绕开。

## 4. Channel authority impact

### 当前立场

- Channel 是协作场, 可跨 org 共享。
- Channel 创建者所在 org 拥有 channel。
- 作者定义 group 结构, 用户只做个人折叠 / 排序。

### 下一版增量

- gh#688 补 owner leave 规则: self-created / owned channel 没有 leave 选项, 不做 owner transfer; owner 只能通过 channel management 删除。
- gh#685 补用户侧 Channel 管理页, 让“我加入 / 我创建 / 我能退出什么”集中可见。
- gh#690 补私有 channel 的视觉标识, 避免锁图标占据过大信息权重。
- delete 可采用 soft delete; hard delete / archive 不作为本轮默认产品承诺。

### 需要决策

- Channel 管理页是否管理通知 / 折叠 / 排序, 还是只做 membership 清单?

## 5. Client truthfulness impact

### 当前立场

- Client 是主要协作界面, 不应显示无法兑现的假状态。
- “沉默胜于假 loading”已经是 realtime 与 client 的共同红线。

### 下一版增量

- gh#724 高优先级是 ArtifactComments production mount、ACL forbidden UX、security / permission 相关 AP bundle UI。
- RT-3 presence polish 与更广 e2e platform / quality expansion 是 backlog extraction candidates, 除非 reviewers 反对。
- 无权访问 channel / DM / artifact 时, client 需要明确 forbidden state, 不能只空白或 loader。
- Forbidden UI 不是 authorization; server ACL 仍是唯一授权判断。ACL 成功前, forbidden / empty / redirect state 不能泄漏 private channel / artifact / message 的 name 或 body。
- e2e 需要能证明自己在测产品, 但本轮不自动吸收 gh#707 / gh#697 的全部 quality gate 范围。

### 需要决策

- “真实可达 UI surface”是否写成 client blueprint invariant。
- forbidden state 是 local state、redirect state, 还是全局 banner。

## 6. Privacy scope guard impact

### gh#654 精确定义

gh#654 不应解释为“撤销隐私 / 安全边界”。它只应解释为: **现阶段不建设额外 user-facing privacy/compliance 产品面**。

用户侧反复 privacy/compliance promise 会制造噪音; 隐私留在内部安全和后端边界里处理。

删除既有 privacy/security boundary 不是 gh#654 的合法解释; 这需要单独 major proposal + threat model。

不进入下一版范围:

- GDPR / DPA / 合规导出 / 删除请求工作流。
- 新的隐私仪表盘或合规中心。
- 把所有权限解释文案扩成法律协议层。
- user-facing privacy promise UI。
- 用户侧 admin impact records / audit 展示。
- 用户侧 impersonate authorization UI。

必须保留:

- Admin 默认不可读消息 / DM / artifact / 文件内容。
- server-side impersonate / admin / capability controls。
- data minimization 与 admin / user / agent rail separation。
- Host 权限继续最小化; 若保留用户侧授权, 仍遵守“装时轻、用时问、问时有理由”。
- Agent capability 继续 owner-only; 跨 org 只能减权, 不能加权。
- Backend audit / enforcement logs 作为内部安全控制保留; 不默认承诺用户侧 audit / system message 产品面。

### bump 结论

若只是把 user-facing compliance / privacy 产品面排除在 v1 外, 不需要 bump。若用 gh#654 删除上述 backend/security 边界, 就是 major, 且需要用户明确拍板 + threat model。

## 7. Sidebar / account IA impact

### 当前立场

- Client 三栏 + 顶部团队栏 + sidebar 是主协作界面。
- 蓝图没有细化 sidebar footer 的按钮预算和 account entry。

### 下一版增量

- gh#669 将 footer 外露入口压到少数关键项。
- gh#670 把头像升级为 account entry, logout 移入个人面板。
- 默认讨论起点: 头像 / Agents / Workspace / Settings 留外; Invitations / Remote Nodes / logout 收进二级入口。
- Remote Nodes / Helper / Host Bridge 入口可以重新摆放, 但 IA 移动不合并职责: credentials / grants / enforcement rails 仍分别建模。

### 需要决策

- account panel v1 是否只读 + logout, 还是包含账号设置动作。
- Remote Nodes 是 Helper / runtime 管理入口, 还是 Settings 子页。
- sidebar footer 调整是否与 gh#690 私有锁角标共用一套 sidebar IA 规范。
