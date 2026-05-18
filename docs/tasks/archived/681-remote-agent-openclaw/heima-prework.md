# #681 Remote-Agent + openclaw 自动配置 — Security Pre-work

> Issue: gh#681 (p1-high, backlog) "扩展 Remote-Agent 的能力, 支持自动配置 openclaw"
> Author: heima (Security)
> Status: pre-work — 给将来真接手 dev 当输入, 不是 design doc, **不锁实施细节**, 只给 Security 风险清单 + 蓝图引用 + 反向锚
> 蓝图: `blueprint/current/host-bridge.md` (五条立场) + `agent-lifecycle.md §4 安全模型重写`

## §0 为什么这条 Security 风险特别大

#681 把 remote-agent 从 **295 LOC 的文件代理** (当前 `packages/remote-agent/src/{agent,fs-ops,index}.ts`) 升级为 "runtime 安装管家" — 网页配 plugin / 创建 agent / 配 channel.

蓝图 `agent-lifecycle.md §4` 字面写明:

> "remote-agent 从'文件代理'升级为'runtime 安装管家', **安全边界完全不同**"

| 维度 | 旧 remote-agent (现状) | 新 remote-agent (#681 要做) |
|------|----------------------|---------------------------|
| 暴露面 | 受限文件读 (read-only, 白名单 dir) | **下载并执行二进制** (OpenClaw, 未来 Hermes) |
| token 失效后果 | 顶多读不到文件 | **攻击者可远程在用户机器跑任意进程** (RCE) |
| 攻击路径 | 文件读取 | RCE / 持久化 / 横向移动 |

**"上一个版本是文件代理 token 泄了顶多读文件; 新版本 token 泄了 = RCE"** — 这是 #681 的 Security 核心风险.

## §1 蓝图已定的 5 件事 (实施时必须满足)

蓝图 `host-bridge.md §1` 拍了 5 条立场, 实施 design 必须**逐条对账**:

### 1.1 内部双 daemon (install-butler + host-bridge)

- `install-butler`: 短命, 需 sudo, 仅装/卸 runtime 二进制
- `host-bridge`: 常驻, **无 sudo**, 独立 OS user/group
- **威胁模型分离** — 单 daemon 一旦被攻陷 = root + 长生命周期, 赌注太大
- 蓝图原文: "拆分后**攻击面减半**"

**Security pre-work 风险**: 实施时如果**偷懒合一个 daemon** (理由可能是 "工程量 +30% 太麻烦"), 就违反核心立场, 这条 Security 必须**红线 NACK**.

### 1.2 安全四件套

#### A. 白名单 + 双签 manifest

- v1 仅安装 **Borgee 签名 manifest 内的 runtime**
- 每个 runtime 二进制: **SHA256 + GPG 双签**
- manifest 由 Borgee 服务端分发

**Security 风险**:
- ① **签名 key 私钥泄露** = 攻击者可以签恶意 runtime, 全用户 RCE. 需要硬件 HSM / 离线签 / 双人签
- ② **TOCTOU**: 校验 SHA256 后到执行之间二进制被换. 必须 download → verify → exec 在同一文件描述符 / 内存 page (不允许"verify 后丢盘再 exec")
- ③ **manifest server 被劫持** (DNS hijack / cert pinning 缺): 攻击者改 manifest 内容. 必须 manifest 也走 GPG 签 + cert pinning

#### B. 进程沙箱

- Linux: systemd unit + cgroups
- macOS: launchd unit + `sandbox-exec` profile

**Security 风险**:
- 沙箱 profile 写不严 = 用户机器上 host-bridge 看似沙箱, 实际能读 `~/`, 等于无沙箱. 实施 design 必须**附 sandbox-exec profile / systemd unit 字面**, 反向 grep 验"无 deny-by-default 规则" count == 0

#### C. 更新策略 — 分类不自动

- 蓝图明文: "**自动更新 = 反模式, 绝不在 v1 出现**"
- 安全补丁: 显眼提示 + 用户一键确认
- 功能更新: 藏在设置面板

**Security 风险**:
- 实施如果**偷加 silent auto-update** (理由"用户体验更好") = 违反信任底线. Security 必须红线
- 安全补丁的"一键确认"按钮**不能默认 focus 到确认** (跟 #691 RollbackConfirmModal 同款 — 默认 focus 取消, 反误回滚 同思路反误升级)

#### D. 一键完全卸载

- 蓝图: "**装得上卸得掉**是信任底线"
- 二进制 / 配置 / 状态 / 安装的 runtime / server 端注册 / OS user/group / launchd / systemd unit 全清

**Security 风险**:
- ① 卸载脚本**自己也是特权**, 卸载过程是 RCE 路径 (sudo 跑 user-controlled path → 路径注入)
- ② 卸载后**不残留授权 token**: 反向 grep `~/.borgee/`, `/var/lib/borgee-*`, `~/Library/Application Support/Borgee/` 等所有可能位置 count == 0
- ③ server 端注册记录删除走**当前 user 的 cookie**, 不接 client-supplied user_id (跟 #687 同源, by-construction 防 IDOR)

### 1.3 情境化授权 (4 类, 分时机问)

**装机时** (2 类): install / exec
**触发时** (2 类): filesystem / network — 第一次 agent 需要时弹窗 (per-agent subset, 不是 owner 一次给所有)

**Security 风险**:
- ① 弹窗如果走 web UI 渲染 user-controlled 字段 (e.g., `<agent.displayName> 想读你的代码目录`), 必须 JSX 转义 / DOMPurify, 反 stored XSS 跨过弹窗骗用户授权 (类似 #696 #698 标准)
- ② 弹窗的 "始终允许" 选项**必须存到 host-bridge 本地 SQLite**, 不能存到 server (server 端可被攻陷, 本地存才能撑住"server 不可信"的威胁模型). 蓝图 `host-bridge.md §4 不在本轮范围` 已经说"SQLite 持久化授权状态 → 第 10 轮数据层"
- ③ 授权状态文件**权限 0600** + owned by host-bridge OS user, 反其它 process 读取 / 篡改

### 1.4 v1 不在 Borgee 跑命令 (B), v2 推完整 host 桥 (C)

- v1: 命令执行**走 OpenClaw 自带 shell tool** (runtime 沙箱内), Borgee 不直接跑
- v2: daemon 拆分留好接口, 完整 host 命令通道单独立项

**Security 风险**:
- ① **不能在 v1 偷加命令执行通道** (理由可能是 "用户问能不能直接跑 shell" — Security 红线 NACK, 必须坚持蓝图)
- ② v2 设计前**必须重新做 Security 评估** + 单独 milestone (蓝图字面)

### 1.5 v1 release 硬指标

DevAgent 跑测试 demo (用户 @DevAgent → DevAgent 通过 OpenClaw shell tool 执行 pytest → 结果回流) **真验 v1 release 不可少** — 这是产品需求, 不是 Security 红线, 但跟 Security 沾边的是: demo 跑通的 OpenClaw shell tool **必须真在 runtime 沙箱里跑** (不绕过 sandbox), 不允许 demo 时为了过 demo 而禁沙箱.

## §2 12 类 Security checklist 对 #681 的预判

| § | 类目 | #681 风险点 | 实施 design 应锁的反约束 |
|---|------|-----------|---------------------|
| §1 | auth/authz | install-butler 启动 (sudo) 是否绑定 Borgee Helper UI 操作 sender? | install-butler trigger 必须走 user-confirmed Borgee UI 链 + 一次性 token; sudo 需要 OS 原生 prompt (osascript / pkexec), **不能 cache sudo** |
| §1 | cookie domain | host-bridge 本地 daemon 跟 Borgee server 通讯的 token 怎么放? | mTLS / OAuth-like rotation; **不能存在 ~/Library/Preferences/ 等 macOS user-readable 位置**, 必须 keychain (macOS) / secret-service (Linux) |
| §2 | input validation | manifest URL / runtime URL / install path 是否被 user-controlled? | manifest URL hardcode + cert pinning; runtime URL 来自 manifest (server-signed); install path 由 Borgee Helper 内置 enum, 不读 client-supplied path |
| §2 | command injection | install-butler 跑 shell? | **不要 shell exec** — 用 syscall execve 直接传 argv, 反 string concat |
| §2 | path traversal | host-bridge 文件代理白名单 (现有 fs-ops.ts:70 readdirSync) 是否真挡 `..` / symlink 越界? | normalize + realpath + check prefix; 反向 grep `path.join` 不带 prefix-check 0 hit |
| §2 | SSRF | host-bridge 网络出站白名单是否被 agent 字段绕过? | 白名单 enum (从蓝图 manifest 来), reject internal IP / loopback / link-local / metadata service (169.254.169.254 / fd00::); IP literal 也要 reject (反 hostname 绕 DNS resolve 后变内网 IP) |
| §3 | sensitive data | install / exec / fs / net 授权 log 是否记 user PII? | 反向锚: 授权 log 只记 (timestamp, agent_id, capability, allow/deny), 不记 user 文件路径 / 文件内容 |
| §3 | secrets in git | install-butler 签名公钥 + cert pinning fingerprint hardcode 在哪? | 编译期 const, **不放 .env / runtime config**; private key 永不上 git |
| §4 | sessions / token | host-bridge daemon 启动时 token? token 多长? rotate? | 短命 access (≤ 1h) + refresh rotation; **agent 失败超 N 次 token 失效 + 强制重 register** (反 brute-force) |
| §5 | rate limit | install-butler trigger 频率? agent 弹窗骚扰? | install 全机一次 (rate limit by host fingerprint), 弹窗每 capability 每 agent 限频 |
| §6 | dependencies | OpenClaw / Hermes 第三方 runtime 引入新 supply-chain 风险 | 蓝图已说 "v1 仅 OpenClaw" 是收窄; 仍需 OpenClaw 上游 CVE 监控; OpenClaw 安装从 manifest 拉, 不从 OpenClaw 官 server 直拉 (manifest 是 Borgee server 复签的代理) |
| §7 | configuration | host-bridge 配置文件位置 / 权限 | 0600 + owner = host-bridge OS user; sandboxed dir (e.g., `/var/lib/borgee-helper/`) |
| §7 | privileged user | host-bridge 跑 root 还是非特权 user? | 蓝图字面**无 sudo, 独立 OS user/group** — 实施 design 必须 systemd `User=borgee-helper Group=borgee-helper` + macOS launchd `UserName` 字面 |
| §8 | IDOR | server 端 host-bridge 注册 endpoint 是否 scope by user.ID? | host_id ↔ user.ID 1:N, 注册 / 删除 / 列表 endpoint 全 ctx user.ID, 不接 body user_id (跟 #687 同源) |
| §8 | privilege escalation | 弹窗 "始终允许" 后 agent 是否能扩 capability? | 授权 grant 锁 capability list at-grant-time, agent 加新字段不自动继承 |
| §8 | race | 用户拒绝弹窗后 agent 立刻又重试? | 拒绝后**冷却期** (≥ 60s 同 agent 同 capability 不再弹), 反骚扰 |

## §3 反向锚 (实施 design 阶段必查)

实施 dev 写 #681 design doc 时必须 grep 验证:

| Grep | 期望 | 防的事 |
|------|-----|------|
| `child_process.exec\b` 在 install-butler / host-bridge | 0 hit (用 execFile / spawn argv) | 命令注入 |
| `shell: true` 在 spawn() | 0 hit | 命令注入 |
| `https?://[^"]*` hardcode 在二进制源 | 仅 manifest URL 一处 | manifest URL 不被环境变量覆盖 |
| `crypto.verify` / GPG 签验 | 至少 2 处 (manifest + runtime) | 双签实施 |
| `process.setuid` / `seteuid` | 仅在 install-butler 短命 setup | host-bridge 不掉 sudo |
| `~/.borgee` / `Application Support/Borgee` | 文件权限测试 0600 | 授权状态不可被其它 process 读 |
| `169\.254\.169\.254\|metadata\|localhost\|127\.\|::1\|fd00:` 在 SSRF blacklist | 至少 6 类 IP literal 都 reject | SSRF 防 cloud metadata |
| `auto.{0,3}update\|silentUpdate` | 0 hit | v1 不允许 auto-update |
| `--admin\|disable-ruleset` | 0 hit | 跟 Borgee server 标准一致, 不允许 bypass |

## §4 跟既有 Security memory 对账

Borgee Security memory anchors 跟 #681 相关项:

- **`testing_admin_credentials`** — testing admin/testing-e2e-2026 是 testing only, **不能在 remote-agent / host-bridge 实施时被引用** (硬编码到 dev tool 都不行)
- **`deploy_config_env_file`** — host-bridge 配置走文件不走 yaml heredoc; 跟 server `.env` 模式一致
- **`no_admin_merge_bypass`** — Security review 实施 PR 不允许 `gh pr merge --admin`
- **`pr_merge_wait_liema_real_test`** — host-bridge / install-butler 任何写 (装 / 卸 / 启动) 都必须 liema 真验, 不允许 page.evaluate 假绿

## §5 实施 design 进入前 Security 必看清单

接手 dev 起 design doc 时, **以下问题必须在 design 里答清楚** (Security review 时这些没答 = 直接 NACK):

1. **install-butler 怎么 invoke sudo?** (osascript prompt / pkexec / 别的)
2. **host-bridge OS user/group 创建是 install-butler 第一次跑时? 卸载时一起删?**
3. **manifest URL 是哪个? cert pinning fingerprint 是什么? 编译期 hardcode 还是运行时配?**
4. **runtime 二进制 SHA256 + GPG 双签的 GPG public key 在哪? 怎么 rotate?**
5. **host-bridge 跟 Borgee server 的 token: 类型 / 生命周期 / 存储位置 / rotation 策略?**
6. **沙箱 profile (sandbox-exec / systemd unit) 字面是什么? deny-by-default 还是 allow-by-default?**
7. **授权状态 SQLite 在哪? 文件权限? 加密?** (蓝图 §4 说"第 10 轮数据层", 但 #681 要落地必须先答)
8. **网络出站白名单怎么生成? 是不是从 manifest 来?**
9. **弹窗 UX 渲染 user-controlled 字段 (agent.displayName) 走 sanitize 路径?**
10. **卸载脚本路径? 测试用例覆盖一键完全卸载 + 反向 grep 残留 0 hit?**
11. **DevAgent 跑测试 demo 真验时沙箱真生效 (反"为了 demo 禁沙箱")?**

## §6 不在 Security pre-work 范围 (留给后续)

- **具体 Architecture 设计** — feima 飞马负责 (install-butler / host-bridge 之间 IPC 协议 / 状态机 / 故障路径)
- **具体 PM 体验** — yema 野马负责 (弹窗文案 / 信任 banner / 卸载确认流程)
- **具体 QA 测试** — liema 烈马负责 (单元 / 集成 / e2e / OS 真验)
- **具体 Dev 实施** — zhanma 战马负责 (Go / TS 代码)
- **蓝图深化** — 蓝图 `host-bridge.md §4` 已经说 "BPP 协议 install-butler 握手 → plugin-protocol.md §2", "SQLite 持久化 → 第 10 轮", 这些是蓝图侧深化, **不在 Security pre-work 范围**

## §7 总结 (一句话给真接手 dev)

> "**#681 的核心不是写代码, 是把蓝图 `host-bridge.md §1` 五条立场 + 安全四件套全部落地, 任何一条偷工减料 = 信任赌注崩盘.** Security 角度最关注: ① 双 daemon 真拆 (install-butler 短命 + host-bridge 无 sudo) ② manifest 双签 (SHA256 + GPG, key 不能放 git) ③ 授权状态本地 SQLite 0600 ④ 一键完全卸载残留 0 hit ⑤ v1 绝不偷加命令通道. 蓝图字面已经写得很细, 实施 design 必须逐条引蓝图行号, 不允许 design 跟蓝图漂."

---

**Security pre-work 输出归档**: 当 #681 进入实施时, dev 起 design doc 应**显式引本文 §1-§5**, design Security 签字时 review 直接 cite 本文锚点, 反 review 漂.
