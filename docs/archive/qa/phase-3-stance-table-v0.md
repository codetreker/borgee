# Phase 3 业主感知反查表 v0 — BPP 协议骨架 5 立场

> **状态**: v0 (野马, 2026-04-28) — Phase 3 启动前置, R4 review anchor
> **配套**: PR #234 vision §1 Phase 3 主线 + PR #241 §2 第一周派活 (业主感知 ⑥ 反查表) + PR #225 §2 业主感知 5 条 (Phase 2) 外延
> **目的**: Phase 3 BPP 协议骨架 (战马A 第一周) 落地时业主感知锚, 战马A PR 直接吃此表为 acceptance; 飞马 / 烈马 review 拿此表反查立场漂移; Phase 3 闸 4 demo 野马签字硬条件。
> **关联**: `plugin-protocol.md` + `host-bridge.md §1.3` ("装时轻, 用时问") + `agent-lifecycle.md §2.1` + `realtime.md §2.3` + R3-4 (frame schema = /ws byte-identical) + `phase-3-4-vision.md` §3 ⑥-⑩。

---

## 1. 5 条业主感知立场 (Phase 3 BPP 主线)

| # | 业主感知锚 | 蓝图 § | 实施落点 (BPP-1 应有) | 反向断言 (用户感知红线) |
|---|----------|-------|---------------------|------------------------|
| **⑥** **agent 创建后真上线** (不是数据库 fake online) | host-bridge §1.3 + agent-lifecycle §2.1 | agent 创建 → host-bridge 启进程 → BPP handshake `agent_online` frame → sidebar online 灯**真**亮 (不是 INSERT 后立即标 online) | sidebar online 灯亮的瞬间 = host-bridge 进程 PID 存在 + BPP ping 双向通过 (≤ 3s); 杀进程 → 灯 ≤ 5s 灭 |
| **⑦** **agent 离线时收 system DM** (不靠你刷新发现) | concept-model §4.1 + realtime §2.3 | host-bridge 心跳超时 (≥ 30s no ping) → server 主动 emit system DM "你的 agent {name} 离线了, 上次心跳 {ts}"; 不可关闭 | DB system DM 表必有新行, body 含 agent **name** 非 raw UUID (跟 §1.1 + bug-029 同根); push frame 走 R3-4 /ws schema |
| **⑧** **邀请通知 ≤ 3s 推送** (BPP 替换 stub) | realtime §2.3 + R3-4 | RT-0 server 现走 stub /ws push, BPP-1 落地后 frame schema **byte-identical** 替换 stub (CI lint 强制); 业主感知 latency 不变 (≤ 3s) | INFRA-2 stopwatch fixture 测 BPP push ≤ 3s + frame diff CI ws/ ↔ bpp/ schema 0 字节漂移 (R3-4 锁) |
| **⑨** **agent 状态字面准确** (online/offline/error 三态非二态糊弄) | agent-lifecycle §2.3 (R3-5 砍 busy 出 Phase 2) | sidebar 显示 agent online (绿点) / offline (灰点) / error (红点 + 错误提示); 不出现"已连接" 这种含糊词 | sidebar DOM 含 `data-state="online"` / `="offline"` / `="error"` 三选一; error 态必有 hover tooltip 含 last error code |
| **⑩** **host-bridge "装时轻"** (业主装 host-bridge 不需要装 SDK / Python / docker) | host-bridge §1.3 第一句 | host-bridge 是单二进制下载 → 启动后所有 agent 进程由 host-bridge 内部跑 (业主不需要装 Python / docker); BPP 协议自描述 (业主不需要懂协议) | host-bridge --help 输出无依赖提示; 安装包大小 ≤ 50 MB; 业主端 setup 文档 ≤ 5 步 (锚 onboarding-journey §3 配套) |

---

## 2. 黑名单 grep — Phase 3 BPP-1 红线闭合

```bash
# BPP frame schema 必须与 /ws push 字节一致 (R3-4 锁)
diff <(jq -S . packages/server-go/internal/ws/frames.json) <(jq -S . packages/server-go/internal/bpp/frames.json)
# 预期 0 行差异 (CI lint 强制, 任意一边漂移 → CI 红)

# agent online 状态不能是数据库直标 (必须经 BPP handshake)
grep -rn "UPDATE agents SET status = 'online'" packages/server-go/internal/store/ | grep -v _test.go
# 预期 0 命中 (status 必须由 BPP `agent_online` frame 接收后才 UPDATE)

# system DM 不可静默 (agent offline 检测必须发 DM)
grep -rn "agent.*offline.*system_message\|EmitOfflineDM" packages/server-go/internal/api/agents.go
# 预期 ≥ 1 命中 (BPP-1 PR 必含)
```

---

## 3. 反向断言锁 (BPP-1 PR 必含测试)

| 反向断言 | 测试位置 | 锁点 |
|---------|---------|------|
| BPP frame schema = /ws schema byte-identical | `internal/bpp/schema_test.go::TestFramesMirror` (跟 R3-4 R3-8 测试同根) | jq diff exit code 0 |
| agent 创建后 host-bridge 启进程, online 灯延迟 ≤ 3s | `e2e/bpp-agent-online.spec.ts` + INFRA-2 stopwatch | latency ≤ 3000ms |
| 杀 agent 进程 → sidebar offline ≤ 5s + system DM 落库 | `e2e/bpp-agent-offline-dm.spec.ts` | latency ≤ 5000ms + DB row count +1 |
| sidebar 状态 DOM 含三态字面 (online/offline/error) | `client/src/__tests__/sidebar-agent-state.test.tsx` | data-state attr 反查 |
| host-bridge 单二进制 ≤ 50 MB (无外部依赖) | CI release pipeline `.github/workflows/host-bridge-build.yml` | binary size 反查 |

---

## 4. 不在 Phase 3 BPP-1 范围 (避免 PR 膨胀, 留给 BPP-2..N)

- ❌ busy / idle 三态 — Phase 4 AL-1b (R3-5 已锁砍)
- ❌ agent 配置热更新 — Phase 4 AL-2 (PR #234 vision §2)
- ❌ agent 退役流程 — Phase 4 AL-4
- ❌ presence 多 device 同步 — Phase 4 AL-3
- ❌ host-bridge 自动升级 — v1+ (本期单二进制下载即可)
- ❌ 跨 org agent 协作 — multi-org v1+

---

## 5. 验收挂钩

- BPP-1 PR (战马A): §1 5 条立场实施落点全在 + §2 黑名单 grep 命中 0 + §3 反向断言 5 项全绿
- Phase 3 闸 4 demo 野马签字: 5 条业主感知 5 张截屏 (类似 G2.4 5+1=6 模式), 落 `docs/qa/signoffs/phase-3-bpp1-yema-signoff.md`
- Phase 3 退出 gate 联签: ≥ 4/5 截屏 ✅ (与 Phase 2 G2.4 ≥ 4/6 同模式; 砍 ⑩ host-bridge 安装大小 demo 给 R5 跑)

---

## 6. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-28 | 野马 | v0, 5 条立场 + 3 黑名单 grep + 5 反向断言 + 不在范围 6 条; Phase 3 BPP-1 战马A 直接吃 |
