# Collab → Borgee Task Board

> **BOARD.md 是任务状态的唯一 SOT（Single Source of Truth）**
> **Owner** = 此刻球在谁手上，随状态流转变化。所有任务必须有 Owner。
> 状态：Backlog → Ready → In Progress → In Review → Done → Archive
> Done 的任务移到 [`ARCHIVE.md`](ARCHIVE.md)
> **一任务一分支**：任务开始时创建分支，PRD/设计/开发/测试全程在同一分支，QA 验收 + review 通过后才合并。

## Features

| ID | 任务 | 状态 | Owner | Branch | PR | 文档 | 备注 |
|----|------|------|-------|--------|----|----- |------|
| COL-B01 | 私有频道 E2E 加密 | Backlog | 野马 | — | — | — | 复杂，需建军定方向 |
| COL-B05 | Agent 运行平台 | Backlog | 野马 | — | — | — | |
| COL-B06 | 基础镜像优化 | Backlog | 飞马 | — | — | — | CI build 加速 |
| COL-B09 | 已读回执（✓✓） | Backlog | 野马 | — | — | — | P5 v2 |
| COL-B11 | 画布 + 文档协作 | Backlog | 野马 | — | — | — | |
| COL-B13 | DB 操作改用 ORM | Backlog | 飞马 | — | — | — | 技术债 |
| COL-B26 | 频道拖动排序 | Backlog | 野马 | — | — | — | 侧边栏频道拖动调整顺序 |

## Bugs

| ID | Bug | 状态 | Owner | Branch | PR | 备注 |
|----|-----|------|-------|--------|----|----- |
| COL-BUG-022 | Admin Create User 不应允许创建 agent；user 列表不应显示 agent | Backlog | 战马 | — | — | agent 由用户自己创建，Admin UI 的 Create User 禁止选 agent；user 列表只显示用户不显示 agent |
| COL-BUG-023 | 默认管理员 userid 用整个邮箱，应只取 @ 前面部分 | Backlog | 战马 | — | — | 创建默认 admin 时 userid 取 email 的 local-part（建军 04-26 提） |
| COL-BUG-024 | /api/v1/users 缺 admin 权限校验，member 能看到所有用户 | Backlog | 战马 | — | — | P0 安全问题：admin 接口需完全独立，/users 只允许 admin 访问（建军 04-26 提，关联 COL-B27） |
| COL-BUG-025 | Prod WS 连接断开（“连接已断开”红色横幅） | Backlog | 战马 | — | — | P2 WS race condition，重连成功不影响使用 |
| COL-BUG-027 | 用户创建频道失败 | Backlog | 战马 | — | — | P0 prod 上用户建不了 channel（建军 04-27 发现） |
| COL-BUG-028 | Agent 在线列表消失 | Backlog | 战马 | — | — | P1 侧边栏 agent 在线状态不显示（建军 04-27 发现） |
| COL-BUG-029 | 侧边栏底部用户区域去掉名字，只保留头像 | Backlog | 战马 | — | — | UI 优化（建军 04-27 提） |
