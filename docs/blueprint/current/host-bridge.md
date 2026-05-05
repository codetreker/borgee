# Host Bridge — 在用户机器上的特权进程

> Borgee Helper 是用户授权 Borgee 在自己机器上运行的特权进程，负责安装/管理 agent runtime（OpenClaw 等）以及向 agent 暴露受控的 host 资源。
> 状态：建军 + 飞马 + 野马 对齐（2026-04-27）。前置阅读：[`agent-lifecycle.md`](agent-lifecycle.md)、[`plugin-protocol.md`](plugin-protocol.md)。

## 0. 一句话定义

> **"Borgee Helper" 是 Borgee 在用户机器上的代理人——负责装 runtime、桥接 agent 看 host 文件。它的存在是一次信任赌注，v1 用五件事建立这份信任。**

---

## 1. 目标态（Should-be）— 五条立场

### 1.1 内部双 daemon，UI 合一

**内部拆分**：

| 内部 daemon | 生命周期 | 权限 | 职责 |
|------------|----------|------|------|
| `install-butler` | 短命（任务完即退） | 需 sudo（首次） | 下载/安装/卸载 runtime 二进制 |
| `host-bridge` | 常驻 | 无 sudo，独立 OS user/group | 文件读写 / 网络出站 / （v2）命令执行 |

**威胁模型分离**：

- 单 daemon 一旦被攻陷 = root + 长生命周期，赌注太大
- 拆分后**攻击面减半**——install-butler 短命且只在装/卸时活；host-bridge 长命但无 sudo
- 工程量 +30% 是可控代价

**UI / 品牌层合一**：

- 用户在 Borgee UI 里只看到一个名字：**"Borgee Helper"**
- 一个状态图标，一个安装包，一组日志
- 进程列表里两个进程不可避免——**正面回答**：在 about 页说明"为了安全隔离安装权限和日常文件访问"
- 这是**可信度故事**，讲清楚比藏起来好

### 1.2 安全四件套（v1 必须）

#### 1. 白名单 + 双签

- v1 仅安装 **Borgee 签名 manifest** 内的 runtime
- 每个 runtime 二进制：**SHA256 校验** + **GPG 签名**双校验
- manifest 由 Borgee 服务端分发，定期更新可信 runtime 列表（OpenClaw 是 v1 唯一项）

#### 2. 进程沙箱

- host-bridge 跑在**独立 OS user/group**（首次安装时创建）
- Linux：systemd unit + cgroups 限制
- macOS：launchd unit + `sandbox-exec` profile
- Windows：v2 才支持，需重新设计

#### 3. 更新策略：分类、不自动

- **自动更新 = 反模式**——绝不在 v1 出现
- 分两类:
  - **安全补丁**：启动时**显眼提示** + 用户**一键确认**
  - **功能更新**：藏在设置面板（避免 Adobe 式骚扰）
- 用户随时可拒绝/延后

#### 4. 一键完全卸载（信任底线）

- 必须能一键**完整卸载**：
  - 二进制文件
  - 配置 / 状态文件
  - 安装的 runtime 们
  - Borgee server 端的注册记录
  - OS user/group / launchd / systemd unit
- "**装得上卸得掉**"是信任底线——用户知道随时可撤回授权

### 1.3 情境化授权（4 类，分时机问）

⭐ **关键 UX 守则**：装的时候轻，用的时候才问，问的时候有理由。

#### 装机时授权（只 2 类）

够起 OpenClaw 即可：

- ✅ **安装**（install）：允许 Borgee Helper 装/卸 runtime 二进制
- ✅ **执行**（exec）：允许启动 runtime 进程并管理其生命周期

#### 触发时授权（2 类，按需弹窗）

第一次某个 agent 需要时才问：

- **文件读写**（filesystem）：agent 第一次想读用户目录 → 弹窗
- **网络出站**（network）：agent 第一次想发请求到非 Borgee 域 → 弹窗

#### 弹窗 UX（plain language + 具体上下文）

参考 macOS Sequoia 屏幕录制权限提示风格：

```
┌────────────────────────────────────────────────┐
│  DevAgent 想读你的代码目录 ~/code              │
│                                                │
│  原因：DevAgent 配置中的"代码 review"能力     │
│        需要读取项目文件                        │
│                                                │
│  [✗ 拒绝]    [✓ 仅这一次]    [✓ 始终允许]      │
└────────────────────────────────────────────────┘
```

#### Per-agent subset

- 每个 agent 只拿它**实际需要的子集**——不是 owner 一次给所有 agent
- 对齐 [concept-model §2](concept-model.md) "agent 默认最小化"原则

### 1.4 v1 不在 Borgee 跑命令（B），v2 推完整 host 桥（C）

#### v1 行为

| 资源 | v1 是否暴露 | 备注 |
|------|------------|------|
| 文件读写 | ✅ 白名单目录 | 现有 remote-agent 路径校验扩展 |
| 网络出站 | ✅ 白名单域 | host-bridge 加 outbound 控制 |
| **命令执行** | ❌ Borgee 不直接做 | **走 OpenClaw 自带 shell tool**（runtime 沙箱内） |
| 进程查看 | ❌ | v2+ |
| 屏幕/键盘 | ❌ | 不在路线图 |

#### 关键产品立场

> "**Borgee 平台不直接执行你机器上的命令；命令执行是 runtime（OpenClaw）的事，沙箱也由 runtime 负责。这是平台与执行层的责任划分。**"

把这条**讲成立场，不是欠缺**——它是 [agent-lifecycle §1](agent-lifecycle.md) "Borgee 是协作平台不是 agent 平台"在 host 维度的延伸。

#### v2 路径

- daemon 拆分为这一步**留好接口**——host-bridge 已经常驻，加命令通道是新接口
- 完整 host 命令通道单独立项，需重新做安全评估
- 早期 v2 仅放给 power user / 显式 opt-in

### 1.5 ⭐ v1 release 硬指标

> 早期采用者就是开发者，"Dev agent 跑测试"是高频期待。如果 OpenClaw shell tool 这条 fallback 不好用，v1 体验会被骂。

**v1 上线前必须验证**：

- 端到端 demo："**Dev agent 在 channel 里被要求跑测试**"
  - 流程：用户 @DevAgent → DevAgent 通过 OpenClaw shell tool 执行 `pytest` → 结果回流到 channel + workspace artifact
  - 验证：OpenClaw shell tool **真的**能调起命令、捕获输出、传回 Borgee
- 这是 v1 release 的**硬指标**，不是 nice-to-have

---

## 2. 信任赌注的五条支柱

> Borgee 在用户机器装特权进程是一次信任赌注。v1 把信任建立在**五件事**上：

1. **拆分 daemon**（最小权限）
2. **签名 + 沙箱 + 用户授权**（防御深度）
3. **可逆卸载**（信任可撤回——"装得上卸得掉"）
4. **不自动更新 + 不做完整 host 桥**（不滥用信任）
5. **情境化授权**（用的时候才问，不在装机时吓跑用户）

少一条都不行——这五条互相支撑，构成 v1 信任模型的最小集。

---

## 3. 与现状的差距

代码现状里 `remote-agent` 是简单的单 daemon 文件代理，跟目标态差距巨大：

| 目标态 | 现状 | 差距 |
|--------|------|------|
| 拆两个 daemon (install-butler + host-bridge) | 一个 daemon 只做文件代理 | **重写**——install-butler 完全新建，host-bridge 由现 remote-agent 演化 |
| Borgee Helper 单一品牌 UX | remote-agent CLI、用户感知是个 "agent" | UI/包装层重做，统一名字与状态 |
| 签名 manifest + 双校验 | 无 | 新建 manifest 系统 + 签名工具链 + 服务端分发 |
| 4 类情境化授权 | 启动参数 `--dirs` 一次给 | UI + 持久化授权状态 + **触发式弹窗组件** + per-agent subset |
| 一键完全卸载 | 只能手动 SIGINT 后 rm | 卸载脚本 + 服务端注销逻辑 + OS 资源清理 |
| 安全补丁 banner | 无 | 版本检查 + 启动时 banner UX |
| **v1 release 硬指标**：DevAgent 跑测试 demo | 没做过 | 端到端验证 OpenClaw shell tool 可用作 v1 fallback |

---

## 4. 不在本轮范围

- BPP 协议中 install-butler 的握手 / runtime schema → [`plugin-protocol.md` §2](plugin-protocol.md)
- 命令执行通道（v2） → 单独立项
- Windows 支持 → v2
- agent 配置中"哪些 host 资源该问"的字段定义 → 第 8 轮"Auth & 权限"
- host-bridge 的 SQLite 持久化（授权状态） → 第 10 轮"数据层"
