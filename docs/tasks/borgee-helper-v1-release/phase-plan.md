# Borgee Helper v1 release wave

> Issue: gh#681 (p1-high) "扩展 Remote-Agent 支持自动配 OpenClaw" — 实质为蓝图 `host-bridge.md §3 与现状的差距` 7 行差距表全重写.
> Author: feima (Architect), 跟 heima Security pre-work `heima-prework.md` §1-§5 字面对账.
> 范围: 拆 8 milestone (HB-7 ~ HB-14), 给 team-lead / yema / liema / zhanma 4 角色入口, 不锁实施细节, 不开 PR.
> 前置阅读: `docs/blueprint/current/host-bridge.md` 全文 + `docs/blueprint/current/agent-lifecycle.md §2.2 + §4` + `docs/tasks/borgee-helper-v1-release/heima-prework.md` 全文.

## §0 关键约束 (实施时必须满足)

蓝图 `host-bridge.md §1` 拍 5 条立场, heima pre-work 对应 12 类 Security checklist + 11 个 design 必答问题. 任何一条偷工减料 = 信任赌注崩盘 (heima §7).

实施 design 阶段 4 角色必须**逐条引下面 5 条立场 + heima §1.1 ~ §1.5 + §5 11 问**:

1. **内部双 daemon** (蓝图 §1.1 + heima §1.1) — install-butler 短命特权 + host-bridge 常驻无 sudo. 现状: HB-2 host-bridge daemon 已落 (PR #617, `packages/borgee-helper/`); install-butler **binary 真实施未落** (HB-1 #491 仅 server endpoint, 5 ⚪ pending).
2. **安全四件套** (蓝图 §1.2 + heima §1.2) — A 白名单 + 双签 / B 进程沙箱 / C 不自动更新 / D 一键完全卸载.
3. **情境化授权 4 类** (蓝图 §1.3 + heima §1.3) — install / exec 装机时, filesystem / network 触发时. 现状: HB-3 schema + REST endpoint 已落 (PR #507 / #520); **UI 弹窗 + per-agent SQLite 持久化未落**.
4. **v1 不挂命令通道** (蓝图 §1.4 + heima §1.4) — 命令执行走 OpenClaw shell tool, Borgee 不直接跑.
5. **release 硬指标** (蓝图 §1.5 + heima §1.5) — DevAgent 跑测试 demo 端到端 + 沙箱真生效. 现状: HB-4.2 demo deferred.

## §1 milestone 拆段 (8 个)

每个 milestone 在 wave 容器内一个子目录; 真起 milestone 时由 4 角色按 `blueprintflow:blueprintflow-milestone-fourpiece` 替换 placeholder spec.md 为完整 4-piece (spec / stance / acceptance / content-lock).

| # | milestone | 子目录 | 蓝图 §X.Y 锚 | 入口 (开工前必须有) | 出口 (合 PR 前必须有) |
|---|-----------|--------|------|------|------|
| **HB-7** | install-butler binary 真实施 | [`HB-7-install-butler/`](HB-7-install-butler/) | `host-bridge.md §1.1` 双 daemon + `§1.2 A` 双签 + heima §1.1 + §1.2A | HB-1 server endpoint 已落 (#491); HB-2 host-bridge daemon 已落 (#617); GPG signing pipeline (HB-8) 设计已 freeze | 短命 binary 跑 install / verify / uninstall 三动作; 反向 grep `child_process.exec\|shell:\s*true` 0 hit; manifest 双签 (SHA256 + GPG) 真验通过; 反向锚 heima §3 9 条 grep 表全过 |
| **HB-8** | manifest signing toolchain | [`HB-8-manifest-signing/`](HB-8-manifest-signing/) | `host-bridge.md §1.2 A` + heima §1.2A 三风险 (key 泄露 / TOCTOU / DNS 劫持) | HB-1 server endpoint (#491) 已落; HSM / 离线签流程 yema + heima 联签 | server 端 GPG 私钥 HSM-only; cert pinning fingerprint 编译期 const; manifest URL hardcode 反 env 覆盖; key rotation runbook 落 `docs/current/server/api/host-grants.md` 同模式 |
| **HB-9** | 网页配 OpenClaw 触发链 | [`HB-9-web-trigger-chain/`](HB-9-web-trigger-chain/) | `agent-lifecycle.md §2.2 默认路径` + heima §1.1 (sudo 走 OS 原生 prompt 不 cache) | HB-7 install-butler binary 已合; HB-3 host_grants endpoint 已落 (#507 / #520) | 网页"添加 agent" → server 下发一次性 token → install-butler 走 osascript / pkexec OS prompt → host-bridge 注册到 server, 真跑通; install 全机 1 次 rate limit |
| **HB-10** | 4 类授权弹窗 UI + per-agent SQLite | [`HB-10-authorization-ui/`](HB-10-authorization-ui/) | `host-bridge.md §1.3` + heima §1.3 (XSS / SQLite 0600 / 冷却期 60s) | HB-3 schema + REST 已落; HB-7 host-bridge daemon SQLite 启动路径已开 | 装机 2 类 (install / exec) + 触发 2 类 (filesystem / network) UI 弹窗; per-agent 授权写 host-bridge 本地 SQLite (file 0600); JSX 转义 user-controlled 字段; 拒绝 60s 冷却; 默认 focus 取消按钮 |
| **HB-11** | 一键完全卸载完整链 | [`HB-11-uninstall-flow/`](HB-11-uninstall-flow/) | `host-bridge.md §1.2 D` + heima §1.2D 三风险 (路径注入 / 残留 token / IDOR) | HB-7 + HB-9 + HB-10 已落 (卸载需要清的全集才齐) | 二进制 / 配置 / runtime / server 注册 / OS user-group / launchd / systemd unit 全清; 反向 grep `~/.borgee\|/var/lib/borgee-*\|Application Support/Borgee` 残留 0 hit; server 注销走当前 user cookie 不接 client-supplied user_id |
| **HB-12** | 安全补丁 banner UX | [`HB-12-security-patch-banner/`](HB-12-security-patch-banner/) | `host-bridge.md §1.2 C` + heima §1.2C ("一键确认"默认 focus 取消) | HB-7 binary version 检查路径已开 | 启动时 banner 显眼提示安全补丁 + 一键确认 (默认 focus 取消, 反误升级); 功能更新藏在设置面板; 反向 grep `auto.{0,3}update\|silentUpdate` 0 hit |
| **HB-13** | 创建 agent + 配 channel 网页流程 | [`HB-13-agent-channel-web-config/`](HB-13-agent-channel-web-config/) | `agent-lifecycle.md §2.1 用户自填` + §2.2 默认路径 + #681 issue body 第 2 / 3 项 | HB-9 plugin 配好链 已合 | 网页"添加 agent"填名字 → 选 runtime (v1 仅 OpenClaw) → plugin connection 自动注册; 给已有 agent 配 channel 走 owner-only 鉴权; agent 列表 4 态 + 故障原因码字面跟 AL-1a #249 byte-identical |
| **HB-14** | DevAgent v1 release demo 真跑通 (closure) | [`HB-14-devagent-demo/`](HB-14-devagent-demo/) | `host-bridge.md §1.5` 硬指标 + HB-4.2 deferred | HB-7 ~ HB-13 全合 | 用户 @DevAgent → DevAgent 通过 OpenClaw shell tool 执行 pytest → 结果回流到 channel + workspace artifact; 沙箱 (Linux landlock + macOS Seatbelt) 真生效, 反 "为了 demo 禁沙箱"; liema e2e 真 UI 走 Playwright 验, 反 page.evaluate 假绿 |

## §2 依赖关系图

```
HB-7 (install-butler binary)  ←─ depends on HB-1 server endpoint #491 + HB-2 daemon #617 (已落)
   ↑
HB-8 (signing toolchain)  ←─ 跟 HB-7 强依赖, 串行起 HB-8 (设计 freeze) → HB-7 (消费)
   ↑
HB-9 (网页触发链) ←─ 依赖 HB-7 + HB-3 #507 / #520 (已落)
   ↑
HB-10 (4 类授权 UI + SQLite) ←─ 依赖 HB-7 (daemon SQLite 入口) + HB-3 (schema)
   ↑
HB-11 (一键完全卸载) ←─ 依赖 HB-7 + HB-9 + HB-10 (清单全集才齐)
   ↑
HB-12 (banner UX) ←─ 依赖 HB-7 (version 检查路径); 跟 HB-9 ~ HB-11 可并行
   ↑
HB-13 (agent + channel 网页配) ←─ 依赖 HB-9 (plugin 配好)
   ↑
HB-14 (DevAgent demo) ←─ wave closure gate, 等 HB-7 ~ HB-13 全合
```

**并行机会** (跟 memory `parallel_default_protocol` 一致):

- HB-8 (signing toolchain server 端) 跟 HB-7 (install-butler binary) 可起 2 战马同时, 战马 H 写 server signing + 战马 I 写 binary, HB-8 freeze 后战马 I 接进 HB-7
- HB-10 (4 类授权 UI) 跟 HB-12 (banner UX) 跟 HB-13 (agent 配 channel UI) 三个 client UI milestone, 可派 3 战马并行
- HB-11 (卸载) 独立模块, 可在 HB-9 + HB-10 落地后立刻起

## §3 Wave closure gate

HB-14 (DevAgent v1 release demo) 是 closure milestone. 走普通 milestone PR 流程 (`blueprintflow:blueprintflow-milestone-fourpiece` + `blueprintflow:blueprintflow-pr-review-flow`), 4 角色联签:

- **Dev**: 实施层确认 8 milestone acceptance 真到位, 没漏
- **PM**: 产品立场 + onboarding 体验签字
- **QA**: e2e 真 UI 真跑通 (Linux + macOS 双平台真验, 反 `page.evaluate` 假绿)
- **Security**: 12 类 checklist 全过, 11 design 必答全答

**Wave closure 真行为指标** (非 grep, 必真验):

1. 用户网页"添加 agent" → 一键真装 OpenClaw 二进制 (macOS .pkg + Linux .deb 双平台真验)
2. install-butler binary 跑 sudo prompt → host-bridge 起在独立 OS user (无 sudo)
3. manifest 双签 (SHA256 + GPG) 真验通过, 试装 unsigned binary 必拒
4. 4 类授权弹窗 UI 真点 (装机 install / exec + 触发 filesystem / network), per-agent SQLite 持久化
5. 一键完全卸载真跑通, 反向 grep 残留 0 hit
6. DevAgent demo 端到端: 用户 @DevAgent → DevAgent 通过 OpenClaw shell 执行 pytest → 结果回 channel + workspace artifact, 沙箱真生效

**不做**:

- 不立新 Phase (蓝图 `host-bridge.md §1 ~ §5` 没变, 只是把 §3 差距表填上, 蓝图合同不变 → wave 不是 Phase, 按 v1.4.1 `blueprintflow:blueprintflow-phase-plan` skill "When to start a new Phase vs add a wave")
- 不动 `_archive/implementation/00-foundation/execution-plan.md` (Phase 0 ~ 4 历史快照, archive 不动)

## §4 反向 grep 检查 (heima §3 9 条原表 + feima 补 3 条)

实施 dev 写 design doc + 合 PR 前必 grep 验, 任何一条 hit 数不对 = 直接 NACK:

| Grep | 期望 hit 数 | 守的事 |
|------|----|------|
| `child_process.exec\b` 在 install-butler / host-bridge | 0 | 命令注入 (heima §3) |
| `shell:\s*true` 在 spawn() | 0 | 命令注入 (heima §3) |
| `https?://[^"]*` hardcode 在 binary 源 | 仅 manifest URL 1 处 | manifest URL 不被 env 覆盖 (heima §3) |
| `crypto.verify\|GPG.*verify` | ≥ 2 处 (manifest + runtime) | 双签实施 (heima §3) |
| `process.setuid\|seteuid` | 仅 install-butler 短命 setup | host-bridge 不掉 sudo (heima §3) |
| `~/.borgee\|Application Support/Borgee` 文件权限测试 | 0600 | 授权状态不被其它 process 读 (heima §3) |
| `169\.254\.169\.254\|metadata\|localhost\|127\.\|::1\|fd00:` 在 SSRF blacklist | ≥ 6 类 IP literal 都 reject | SSRF 防 cloud metadata (heima §3) |
| `auto.{0,3}update\|silentUpdate` | 0 | v1 不准 auto-update (heima §3) |
| `--admin\|disable-ruleset` | 0 | server 标准一致, 不准 bypass (heima §3) |
| `admin.*host_grant\|host_grant.*admin` (feima 补) | 0 | admin god-mode 不挂用户授权 (跟 HB-3 #507 一致, ADM-0 §1.3 红线) |
| `pwa\|appmanifest` 在 install-butler / host_manifest.go (feima 补) | 0 | 跟 DL-4 #485 PWA manifest endpoint 拆死锚 (host_manifest.go §28-§35 既有) |
| `page\.evaluate\|fetch\(.*api` 在 e2e/ HB-* (feima 补) | 0 | e2e 真 UI 不 cURL bypass (memory `e2e_no_curl_only_ui`) |

## §5 不在 phase-plan 范围 (留给后续角色)

- **每个 milestone 的 4 件套** — yema (PM brief) + heima (security pre-work 已写) + liema (acceptance template) + zhanma (implementation design) 各角色按 `blueprintflow:blueprintflow-milestone-fourpiece` skill 各自起, 替换子目录里的 placeholder spec.md.
- **install-butler ↔ host-bridge IPC 协议字面** — feima 在 HB-7 design 阶段起.
- **GPG signing key 管理 runbook** — heima 在 HB-8 design 阶段起 (HSM / 离线签 / 双人签).
- **UI 弹窗具体文案** — yema 在 HB-10 + HB-12 + HB-13 design 阶段起.
- **e2e 测试矩阵** — liema 在 HB-14 design 阶段起 (Linux .deb + macOS .pkg 双平台).
- **DevAgent demo 脚本** — yema (产品脚本) + zhanma (技术实现) 联手, HB-14 出口前定.

---

**phase-plan 输出归档**: 当 wave 进入实施时, 4 角色起 design doc 应**显式引本文 §0 ~ §4 + heima-prework.md §1 ~ §5**, 不允许 design 跟蓝图 / heima pre-work / phase-plan 漂.
