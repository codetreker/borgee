# Implementation · Host Bridge (Borgee Helper)

> 蓝图: [`../../blueprint/current/host-bridge.md`](../../blueprint/current/host-bridge.md)
> 现状: 当前有 remote-agent (Tauri shell), 但只是窗口壳, 没有"信任五支柱", 没有 install-butler / host-bridge daemon 概念
> 阶段: ⚡ v0
> 所属 Phase: Phase 4

## 1. 现状 → 目标 概览

**现状**: remote-agent 是 Tauri 窗口壳, 没真正的 daemon; 安装 plugin / 调用 host 命令的能力都没有。
**目标**: blueprint 五条立场 — Borgee Helper = Tauri shell + install-butler daemon + host-bridge daemon (内部双 daemon UI 合一); 安全四件套 + 信任五支柱; 情境化授权 4 类; v1 不在 Borgee 跑命令, v2 推完整 host 桥。
**主要差距**: 几乎全新, 唯一已有的是 Tauri shell。

## 2. Milestones

### HB-1: install-butler daemon (plugin 安装管家)

- **目标**: blueprint §1.1 — 内部双 daemon, install-butler 负责 plugin 下载/校验/安装。
- **Owner**: 飞马 (review 安全模型) / 战马 / 野马 / 烈马
- **范围**: daemon 启动 + IPC 接口; 从 server 拉 plugin manifest (依赖 server 端 manifest API, 见 data-layer **DL-4 server-side-services**); 签名校验; 安装到本地路径
- **不在范围**: plugin 自动更新 (v1+); 多源 registry ❌; **server 端 manifest API → DL-4**
- **依赖**: **DL-4 (server-side-services 提供 plugin manifest API)**
- **预估**: ⚡ v0 1-2 周
- **Acceptance**: E2E (用户点装 plugin → daemon 下载装好可启动)

### HB-2: host-bridge daemon (host 命令通道, 仅查询)

- **目标**: blueprint §1.4 — v1 不在 Borgee 跑命令, 但要有桥接接口供未来 v2 推。
- **Owner**: 飞马 / 战马 / 野马 / 烈马
- **范围**: daemon 接 plugin 的"读类"请求 (列文件 / 读 env), 不做写; 所有读都走情境化授权
- **不在范围**: 写命令 / shell exec ❌ (v2)
- **依赖**: HB-1
- **预估**: ⚡ v0 1 周
- **Acceptance**: 行为不变量 (任何写类调用拒绝, 单测) + 数据契约 (IPC schema)

### HB-3: 情境化授权 4 类

- **目标**: blueprint §1.3 — 4 类授权分时机问 (一次 / 会话 / 30 天 / 永久)。
- **Owner**: 飞马 / 战马 / 野马 (UX 立场) / 烈马
- **范围**: 授权弹窗 + 持久化 (`host_grants(scope, ttl, granted_at)`); 不同 scope 不同 TTL
- **依赖**: HB-2
- **预估**: ⚡ v0 1 周
- **Acceptance**: E2E + 行为不变量 (TTL 过期回到询问)

### HB-4: 信任五支柱可见 ⭐

- **目标**: blueprint §2 — 五支柱 (开源 / 签名 / 可审计日志 / 可吊销 / 限定能力) 在 UI 可被用户读到; **同时收口蓝图 §1.5 v1 release 硬指标** (见下表)。
- **Owner**: 野马 (主, demo 签字) / 战马 / 飞马 / 烈马
- **范围**: Helper 设置页可看每条支柱状态; 审计日志可导出; v1 release gate 数字化指标全过
- **预估**: ⚡ v0 4-5 天 + 硬指标补 3-4 天
- **Acceptance** (⭐ 标志性, 4.1+4.2 双挂):
  - 行为不变量 4.1: 五支柱状态 API 返回结构稳定 (合约测试) + 审计日志格式 schema 锁定
  - 行为不变量 4.1 + 数据契约 (Helper v1 release gate, 见下)
  - 用户感知签字 4.2 (野马: "作为用户我敢装这个 daemon", 截 3 张: 五支柱状态页 / 情境授权弹窗 / 撤销后行为)

#### Helper v1 release gate (蓝图 §1.5 硬指标)

> 烈马要求: "硬指标"必须数字化, 否则只能 4.2 主观签字, 不算可重复。
> **测量基准锁定 (烈马 R2)**: 启动时间在 CI runner (GitHub Actions ubuntu-latest, 4 vCPU 16 GB) 上测量; 本机数字仅参考, 以 CI 为准。

| 指标 | 阈值 | 测量方式 | 基准环境 | Owner |
|------|------|---------|---------|-------|
| Helper 启动时间 (冷启动) | < 800 ms | benchmark 单测 (CI) | GitHub Actions ubuntu-latest 4vCPU 16GB | 战马 / 烈马 |
| Helper 崩溃率 (内部 dogfood 1 周) | < 0.1% (1 千次会话最多 1 次) | 崩溃报告统计 | 内部 dogfood (建军/飞马/野马/烈马 4 人 1 周) | 烈马 |
| 签名校验失败率 (合法 plugin) | 0% | 合约测试 (CI) | CI | 烈马 |
| 审计日志格式 | 锁定 JSON schema (含 actor / action / target / when / scope) | schema 文件 + 校验单测 | CI | 飞马 / 烈马 |
| 撤销 grant → daemon 立即拒绝 | < 100 ms | 行为不变量单测 | CI | 烈马 |
| 任何写类 IPC 调用 | 一律拒绝 (v1 仅读) | 单测覆盖每种写法 | CI | 烈马 |

**HB-4 关闭条件**: 上述 6 行指标全过 + 野马签字 + 截屏。任意一行不达标 → ⭐ milestone 不能关。

## 3. 不在 host-bridge 范围

- 写命令 / 完整 host 桥 → v2
- BPP 协议本身 → BPP 模块

## 4. Blueprint 反查表

| Milestone | §X.Y | 立场一句话 |
|-----------|------|-----------|
| HB-1 | host-bridge §1.1 | install-butler 负责 plugin 安装 |
| HB-2 | host-bridge §1.4 | v1 仅读, 不跑命令, 留接口 |
| HB-3 | host-bridge §1.3 | 4 类授权分时机问 |
| HB-4 | host-bridge §2 | 五支柱可见, 用户敢装 |
