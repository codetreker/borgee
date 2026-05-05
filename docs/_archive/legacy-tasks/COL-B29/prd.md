# COL-B29 身份与权限体系 v2 — PRD

日期：2026-04-27 | 状态：Draft

## 背景

随着 Borgee 持续迭代（Go server 重写、Admin 分离），系统的身份和权限模型已经与最初设计完全不同。当前代码中 admin/user/agent 的权限边界模糊，导致多个 bug（member 建不了频道、agent 在线列表消失等）。需要从系统设计层面重新梳理三种身份的定位和权限逻辑。

## 目标用户

- **Admin**：系统管理员（部署/运维人员）
- **User**：人类用户（团队成员）
- **Agent**：由用户创建的 bot

## 核心需求

### 需求 1: 三种身份定义清晰化

- 用户故事：作为系统设计者，我需要三种身份各自有明确的定位和边界，避免身份混淆
- 关键决策：
  - **Admin** 是系统管理员，不是用户。独立身份，环境变量配置，只能做系统管理，不能聊天
  - **User** 是人类用户，默认拥有所有用户级别权限，所有 user 完全平等
  - **Agent** 由 user 创建，归属于 owner（user），权限受控
- 验收标准：
  - [ ] Admin 无法发消息、加入频道、出现在用户列表
  - [ ] User 登录后能使用所有用户功能，无需额外授权
  - [ ] Agent 只能在被授予的权限范围内操作

### 需求 2: User 权限简化

- 用户故事：作为普通用户，我登录后应该能立即使用所有功能，不需要等 admin 授权
- 关键决策：
  - User 默认拥有所有用户权限（等同 `*`）
  - User 之间没有权限差异，所有 user 完全平等
  - 不需要为 user 维护权限表
- 验收标准：
  - [ ] User 登录后可以创建频道、发消息、创建 agent 等所有用户操作
  - [ ] 不存在"user 缺权限"的情况

### 需求 3: Agent 权限控制

- 用户故事：作为用户，我创建 agent 后能控制它能做什么、不能做什么
- 关键决策：
  - Agent 默认只有发消息权限
  - Owner 可以 grant/revoke 其他权限（频道操作、reaction、文件上传等）
  - Agent 可被授予的权限 = 当前系统所有用户权限
  - 权限表只为 agent 服务
  - Agent 不能创建其他 agent
- 验收标准：
  - [ ] 新创建的 agent 默认只能发消息
  - [ ] Owner 能给 agent 添加/移除权限
  - [ ] Agent 尝试未授权操作时被拒绝

### 需求 4: User 生命周期

- 用户故事：作为管理员，我需要能管理用户的状态（启用/禁用），但不能永久删除
- 关键决策：
  - User 不能被删除，只能被 disable
  - User 被 disable 后，其创建的 agent 也跟着失效
- 验收标准：
  - [ ] Admin 后台只有 disable 按钮，没有删除按钮
  - [ ] Disable 后用户无法登录，其 agent 无法连接

### 需求 5: Agent 归属与可见性

- 用户故事：作为用户，我只能看到和管理自己创建的 agent
- 关键决策：
  - Agent 归属于创建它的 user（owner）
  - User 在用户界面只能看到和管理自己的 agent
  - Admin 后台可以查看所有用户的 agent（只读）
- 验收标准：
  - [ ] 用户在 agent 管理页只看到自己的 agent
  - [ ] Admin 后台 User Detail 页能查看该用户的 agent 列表（不能操作）

## 不在 v1 范围

- 用户组/团队概念（所有 user 平等）
- 频道级别的细粒度权限（如"某用户只能在某频道发消息"）
- Agent 之间的权限继承

## 验收标准（全局）

- [ ] 系统中不存在 `admin` role（用户表只有 `member` 和 `agent`）
- [ ] User 登录后所有功能立即可用
- [ ] Agent 权限受控，默认只能发消息和读消息
- [ ] User disable 后其 agent 失效（中间件检查 owner 状态）
- [ ] Admin 和用户系统权限完全隔离
- [ ] 现有 user_permissions 表数据清除

## 已确认的决策

1. Agent 默认权限：message.send + message.read（建军 + 飞马确认）
2. User disable 后：中间件检查 owner 状态，直接拒绝 agent 请求（建军确认）
3. 现有 user_permissions 表数据直接清掉（建军确认）
