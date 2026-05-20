---
version: v1.1
accepted: 2026-05-18
prev: v1.0
---

# Host Bridge — Helper / OpenClaw bounded remote actuator

> Borgee Helper 是用户授权 Borgee 在自己机器上运行的特权进程，负责安装/管理 agent runtime（OpenClaw 等）以及在 enrollment 之后作为 **bounded remote actuator** 执行预授权的 typed host-management jobs。
> 状态：v1.1 promotion (2026-05-18)。前置阅读：[`agent-lifecycle.md`](agent-lifecycle.md)、[`plugin-protocol.md`](plugin-protocol.md)。

## 0. 一句话定义

> **"Borgee Helper" 是 Borgee 在用户机器上的代理人——负责安装 runtime，并在用户一次本地 enrollment 后，让 Web 端通过 bounded, pre-authorized typed jobs 完成 Configure OpenClaw / 配置 / 启停服务等动作。它不是 Borgee command channel，也不是 runtime owner；它的存在需要用户授予高度信任，v1.1 用六件事建立这份信任。**

---

## 1. 目标态（Should-be）— 六条约定

### 1.1 内部双 daemon，UI 合一

**内部拆分**：

| 内部 daemon | 生命周期 | 权限 | 职责 |
|------------|----------|------|------|
| `install-butler` | 短命（任务完成即退出） | 需 sudo（首次） | 下载/安装/卸载 runtime 二进制；可见、不缓存 sudo |
| `host-bridge` | 常驻 | **非 sudo**，独立 OS user/group | enrollment / typed job pull / 本地策略校验 / 文件读写 / 网络出站 |

**威胁模型分离**：单 daemon 一旦被攻陷 = root + 长生命周期；拆分让安装权限和日常 actuator 操作各自最小化。

**UI / 品牌合一**：用户在 Borgee UI 里只看到 "Borgee Helper" — 一个状态图标、一个安装包、一组日志。

### 1.2 Bounded remote actuator 产品立场（HB-RA-1A）

> 一次本地 enrollment 之后，Web 端 Configure OpenClaw 可以**通过预授权的 typed jobs** 让 Helper 执行 install plugin / create or update agent config / configure Borgee plugin connection / channel binding / start-stop-restart 已声明的 enrolled 服务。不要求用户再 SSH 一次。

- Enrollment-time delegation **不是 blanket preauthorization**；只覆盖 closed v1 typed-job taxonomy（OpenClaw / Helper lifecycle + config）。其余 file / network / resource 仍走 owner-controlled allowlists + revocation。
- Helper 走 **outbound WebSocket 持久连接** (`wss://`); server 在已建立的 WS 上 push job 给 helper, 永远不向 host 主动 dial 新连. 连接 idle 时走 ping/pong 探活 (替代 5min POST /status freshness 模型).
- Helper 是**两进程 privilege separation**: 主 daemon (`borgee daemon`) 跑 `borgee` 系统用户, 持有 WS 长连 + 跑无 root 的 executor; root daemon (`borgee rootd`) 跑 root, 只监听本地 UDS, 接受预定义窄命令白名单 (install_plugin / service_lifecycle / delegation_revoke), 主 daemon 通过 IPC 转发 root 请求. 攻陷主 daemon 不直接得 root.
- Server 入队鉴权 + Helper 本地策略**双侧都要**校验 owner / org / enrollment / delegation / job type / manifest 或 artifact / paths-domains / 已声明 service IDs / revocation。
- Web 端只发 schema-bound typed jobs；**不接受** arbitrary shell / argv / executable path / script / 任意 service unit。
- 长生命周期 Helper / OpenClaw 服务**保持非 sudo**；`install-butler` 只在需要时短期出现，绝不缓存 sudo。
- Revoke / uninstall 阻挡未来 jobs、确定性结算队列内 / leased jobs、失效 helper 凭证、停掉 in-scope services；UI 必须能看到 revoked / uninstalled。
- 状态和日志 bounded + redacted；失败 job **不能**显示成成功，也不能无限 spin。
- Helper UI 位置可以挪，但 **Remote Agent 和 Helper 的 credentials / grants / 强制执行 rail 一直分开**（PS-1）。

### 1.3 安全四件套（v1 必须，v1.1 保留）

#### 1. 签名 manifest + 双校验

- 只安装 Borgee 签名 manifest 内列出的 runtime；每个二进制走 SHA256 + GPG 双校验。
- manifest 由 Borgee 服务端分发，定期更新可信 runtime 列表（v1 仍只有 OpenClaw）。
- typed job 若涉及 install / config / service IDs，envelope **必须**带 `manifest_digest` 把 job 绑定到具体已签 artifact。

#### 2. 进程沙箱

- `host-bridge` 跑在独立 OS user/group（首次安装时创建）。
- Linux：systemd unit + cgroups 限制；macOS：launchd unit + `sandbox-exec` profile；Windows：v2 才支持。
- v1.1 沙箱**必须**允许已声明的 typed jobs（声明过的 paths / domains / service IDs / outbound WS 长连），不允许任意 host 控制。`HB-RA-1B` 的 sandbox profile 细节由实施层 task design 决定。

#### 3. 更新策略：分类、不自动

- 自动更新仍是反模式。安全补丁启动时显眼提示 + 用户一键确认；功能更新只在设置面板提示。

#### 4. 一键完全卸载

- 一键卸载必须清除：二进制 / 配置 / 状态 / 已安装的 runtime / server 端注册记录 / OS user-group / launchd 或 systemd unit。

### 1.4 情境化授权（"装时轻，用时问，问时有理由"）

- **装机时**只授安装 + 执行两类。
- **运行时**第一次需要 filesystem / network 时再弹窗问，附原因。
- Per-agent subset：每个 agent 只拿自己实际需要的子集——这跟 [`concept-model.md` §1.2](concept-model.md) "agent 默认最小化"对齐。

### 1.5 ⭐ v1 release 前置验收（保留）

- 端到端场景："Dev agent 在 channel 里收到执行测试请求"——用户 @DevAgent → DevAgent 通过 OpenClaw shell tool 执行 `pytest` → 结果回流 channel + workspace artifact。
- OpenClaw shell tool 是 runtime 沙箱内的执行通道；Borgee 平台层不直接执行 host 命令，命令执行仍由 runtime 负责。**Bounded typed jobs 只覆盖 helper / OpenClaw lifecycle + config，不是 runtime 的 shell 通道。**

### 1.6 Bounded service lifecycle 与 boot/crash（HB-RA-1B 关联）

- 已声明的长生命周期 Helper / OpenClaw services 必须能跨 OS 重启 / 进程崩溃自恢复（boot + crash restart）；这是 Configure OpenClaw value path 的硬要求。
- `install-butler` 例外：短命，不开机自启，不监督重启。
- 服务 ID 必须来自 signed manifest 或 enrollment state；不接受 client-supplied unit names。

---

## 2. 信任赌注的六条支柱（v1.1）

1. **拆分 daemon**（最小权限）
2. **签名 + 沙箱 + 用户授权**（防御深度）
3. **可逆卸载**（信任可撤回 — "装得上卸得掉"）
4. **Bounded typed actuator + 不自动更新**（不滥用信任 — 没有 arbitrary command channel，只有 closed v1 taxonomy + schema-bound jobs）
5. **情境化授权**（使用相关能力时再请求）
6. **Helper / Remote Agent 强 rail 分离**（credentials / grants / enforcement 不并轨，PS-1）

少一条都不行——这六条互相支撑，构成 v1.1 信任模型的最小集。

---

## 3. Bounded remote actuator 执行契约锚（HB-RA-1B）

> 执行契约 shape 在本节锁定到 Phase / Milestone 规划粒度，具体 schema / migration / 端点 / 测试由 task-level Dev design 决定。详细规划范围已在 v1.1 promotion 之前由 `docs/tasks/archived/phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator` 内 12 个 task PR 落地（见各 task `progress.md`）。

### 3.1 Job envelope（典型字段）

| 字段 | 含义 |
|---|---|
| `job_id` | 全局唯一 |
| `enrollment_id` | 指向已授权 helper enrollment |
| `job_type` | 必须属于 closed v1 taxonomy |
| `schema_version` | 每个 job_type 固定 schema |
| `payload` | schema-bound；拒绝 unknown / extra fields |
| `manifest_digest` | 涉及 install / config / service IDs 时必填，绑 signed artifact |
| `status` | queued / leased / running / succeeded / failed / cancelled / expired |
| `lease` | helper 领取后的 lease + expiry，防重复执行 |
| `result` | terminal status / failure reason / bounded log refs / audit refs |
| `ttl`, `retry_backoff`, `cancellation`, `idempotency_key` | 过期、限重试、可取消、可幂等 |

### 3.2 Closed v1 typed jobs

| job_type | 允许动作 | 关键约束 |
|---|---|---|
| `openclaw.install_from_manifest` | 从 signed manifest 安装或补齐 OpenClaw plugin | manifest_digest 必填；只走 approved paths / domains |
| `openclaw.configure_agent` | create / update OpenClaw agent config | 固定 schema；只走 approved config paths |
| `borgee_plugin.configure_connection` | create / update Borgee plugin connection / channel binding | owner / org / channel 授权 |
| `service.lifecycle` | start / stop / restart 已声明的 helper / OpenClaw service | 只允许 signed manifest / enrollment state 声明过的 service IDs |
| `state.write` | 写 approved helper / OpenClaw config 或 state path | 不允许任意路径、不允许 dump 私有内容 |
| `status.collect` | 收 helper / OpenClaw status + bounded log | redacted、bounded、不含 token / secret |
| `delegation.revoke` | 本地禁用 delegation | 必须确定性结算队列 / leased jobs |
| `helper.uninstall` | 卸载 / 禁用 in-scope helper / plugin 资产 | 关 autostart / service；不删 out-of-scope 数据 |

Rejected by design：unknown job types、extra fields、arbitrary argv、arbitrary executable path、client-supplied script、client-supplied unit name、arbitrary local service restart、arbitrary shell、allowlists 之外的 path / domain。

### 3.3 Revoke / uninstall race

- Future jobs：server enqueue gate 立即拦。
- Queued jobs：server 标 denied / cancelled，helper 不执行。
- Leased-before-action：helper 在 local action 前重新校验 revocation，按 cancelled / revoked 结算。
- Running：helper 在每个 bounded action boundary 重新校验；revoke 命中就停在下一个安全边界，记 deterministic terminal status。
- Helper auth：persistent helper credential 立即失效；下一次 WS 重连或 in-flight job 命中失效凭证, server 返回 revoked / stale。
- UI：显示 revoked / uninstalled，不能只显示 offline。

---

## 4. 不在 v1.1 范围

- Borgee 平台层任意 host command channel / arbitrary shell（仍 v2+ 单独立项；v1.1 只有 closed typed jobs，不是 command channel）
- Windows 支持（v2）
- agent 配置中 "哪些 host 资源该问" 的字段定义（见 [`auth-permissions.md`](auth-permissions.md)）
- host-bridge 的 SQLite 持久化（授权状态）（见 [`data-layer.md`](data-layer.md)）
- 用户侧 privacy / compliance 产品面（PS-1：不扩；后端 admin / privacy / security / 数据最小化 / Helper-Remote Agent rail 分离一律保留）
