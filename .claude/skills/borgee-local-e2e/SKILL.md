---
name: borgee-local-e2e
description: Use when running borgee end-to-end tests locally — covers any feature that touches both the web UI and a Linux VM (signup, channel, message, DM, artifact, helper install, openclaw plugin, configure, channel bridge, etc.). Default runs full e2e; user can scope to one slice.
---

# Borgee 本地 e2e

> e2e = 真扮演用户走整条产品流程. 任何用户不会自己 hardcode 的东西必须从 web UI 拿. 这套是单一统一的本地 e2e 协议, 适用 borgee 任何端到端流程.

## 默认范围

调用这个 skill 时:
- **不指定范围** → 跑**完整 e2e**: 从 fresh server + fresh dev-vm 开始, 注册 / 登录 / channel / 消息 / DM / helper install / openclaw plugin install / openclaw configure / channel bridge — 整条用户旅程一遍.
- **指定范围** (例: "测 helper install", "测 channel 消息发送", "测 openclaw configure") → 跑该 slice. Slice 仍按用户视角操作 (不绕 UI), 但前置步骤可以**通过同样的用户操作**快进 (不准 SQL 插).

---

## 1. 怎么 setup 环境

### 拓扑 — 三条铁律

```
┌──────── 宿主开发机 ────────┐         ┌──── docker (dev-vm) ────┐
│ server-go 进程 (:4900)      │ ◄────► │ systemd + 干净 Ubuntu   │
│ web client 进程 (Vite)      │   网络  │ (空, 没装任何 borgee /  │
│ browser (你 = 用户)         │   桥    │  Node / openclaw)       │
└────────────────────────────┘         └─────────────────────────┘
```

1. **server-go 在宿主跑, 不在容器**. 不准把 server 塞 dev-vm 的 docker network — 真 prod 里 server 和 helper 永远在不同机器, 本地 e2e 要保拓扑一致.
2. **dev-vm 是干净 Ubuntu + systemd**, 不预装任何 borgee / Node / openclaw / systemd unit. 真用户拿到的就是 fresh VM, 所有东西都该走 install-butler.
3. **dev-vm 一次性**. 每个 e2e 跑完 `docker compose down -v` 删干净, 下次重起干净环境.

### Bootstrap 步骤

**先判 scope: 要不要 dev-vm?**

- **纯 browser scope** (signup / channel / 消息 / DM / artifact 等不涉及 helper / runtime 安装的流程) → **不需要 docker**, 走 `packages/e2e/playwright.config.ts` 既有的 `webServer:` 机制 (自动起 server-go + Vite + cleanup), 直接 `pnpm exec playwright test`. 快, 10s 内能跑通 signup.
- **有 helper scope** (helper install / openclaw plugin / configure / channel bridge 等需要真 VM 的流程) → 走下面完整 bootstrap.

完整 bootstrap (有 helper):

```bash
# 一次性 (或者环境变了再来一次)
# server-go 启动前要 env 就绪 — 至少:
#   BORGEE_MANIFEST_SIGNING_KEY    (ed25519 seed)
#   BORGEE_MANIFEST_SIGNING_PUBKEY (对应公钥)
#   BORGEE_DEV_ARTIFACTS_DIR       (真 build 出来的 plugin tarball 目录)
# 生成脚本跟仓库现行约定走 (找 */dev-bootstrap/ 或 scripts/ 下的
# gen-keypair.sh / build-plugin-artifact.sh, 不在了就跟当时的 README)

# 长跑两个终端
cd packages/server-go && make run    # server-go 监听 :4900
pnpm dev                              # web client (Vite)

# e2e 开始前
cd scripts/dev-vm
docker compose up -d
docker exec borgee-vm systemctl is-system-running  # 必须 "running"

# e2e 结束后
cd scripts/dev-vm
docker compose down -v
```

dev-vm 必须能解析宿主 (`host.docker.internal` / docker bridge gateway). 这是 compose 的事, 不是每次 e2e 都要再配 — 解析不到就改 compose, 不是改 e2e spec.

### Troubleshooting

- **`/tmp` noexec → `go run` 失败**: 沙箱 / 部分 CI host 把 `/tmp` 挂 noexec, `go run` 输出二进制无法执行. 解: `export GOTMPDIR=$HOME/go-tmp TMPDIR=$HOME/go-tmp` (或任何用户家目录下可执行位置), 再 `make run`.
- **dev-vm `systemctl is-system-running` 长时间 `starting`**: 多等几秒 (10-20s); 仍不行检查 `docker exec borgee-vm systemctl --failed` 看哪个 unit 卡了; 多半是 systemd 在容器里依赖某个 host 资源.

---

## 2. e2e 的定义

**e2e** = 一条流程从**用户能起始的状态**开始, 经过**所有真实进程边界**, 到**用户能验证的产物**, 全程**真按真点真观测**, 不绕过任何环.

判定一个测试是不是 e2e:

| 不是 e2e | 是 e2e |
|---|---|
| 直接 POST 一个 REST API | browser 真按按钮, fetch 是 UI 自己发的 |
| `page.route()` mock server 响应 | 真打 server, 真等真响应 |
| `page.evaluate(fetch(...))` 拿 cookie 直调 | 走可见 UI 元素 (input / button / link) |
| SQL 预插 helper / channel / agent / 任何业务关系 | 通过 UI 操作建出来 |
| docker exec 容器里 echo 一行充当"用户消息" | browser 输入框真打字, Enter 真发 |
| 验证手段 = 只 assert HTTP 200 | 验证手段 = 用户可见结果 (UI 显示 / 文件 / 容器内进程状态 / 真日志) |
| 跨进程的事先用 mock filler 补 | 跨进程真启动, 真传 frame, 真消费 |

e2e 必须**至少跨**这些边界 (按测的功能可省一些, 不能全省):
- browser ↔ web client (Vite/SPA)
- web client ↔ server-go (HTTP / WS)
- server-go ↔ helper daemon (helper 走 WS 长连 server)
- helper daemon ↔ rootd (本地 UDS)
- rootd ↔ install-butler / openclaw / 其他 runtime
- 必要时 ↔ openclaw plugin (BPP frame / poll / 真消息)

---

## 3. 用什么测试

| 工具 | 用途 |
|---|---|
| **Playwright** (`pnpm exec playwright test`) | 浏览器驱动: 真打开 web UI, 真按真填真观察. 这是 e2e 的默认主驱动. spec 必须用真 selector (testid / role / text), 不准 `page.route()` stub. **每次跑必加 timeout** (`--timeout=30000` per-test). |
| **`docker exec borgee-vm <cmd>`** | 验证 VM 内真产物: `systemctl status <unit>` / `journalctl -u <unit> --since '30s ago'` / `cat /var/lib/borgee/...` / `ps aux \| grep <proc>`. 验"helper 装上了 / 进程在跑 / 文件落了"用这个. |
| **server-go 日志 / DB** | 宿主看 server-go stdout 真日志; sqlite DB 文件直接 `sqlite3` 查表观察真数据 (建 channel / 真发消息后 DB 该有的行). 不在 spec 里跑 mock DB. |
| **browser DevTools / Network panel** | 必要时手验, 看 WS frame 真传, server 响应真返. Playwright 跑通后人工再扫一眼是健康习惯. |
| **截图 / 视频** | Playwright `screenshot: 'only-on-failure'` + trace, 失败时定位用. UI 质量评审也用截图. |

**绝不用**: `nock` / `msw` / `page.route` / 任何 HTTP 录回放 / 任何 server-side mock middleware. e2e 就是不带这些.

---

## 4. 关注点

跑 e2e 时心里要盯的事:

### 4.1 真按真点
- 每个 UI 操作都用真 selector 找到真元素, 真触发 (click / fill / press)
- 没找到元素就让 spec 失败, 不绕去 `page.evaluate(...)` 调内部函数
- 输入框真打字, 不直接 `input.value = '...'`

### 4.2 整条链路真闭合
- 用户操作 X → 期望产物 Y → Y 必须在**真物理位置**被观测
  - UI 上 X → 后端 DB 真有行 → helper 容器内 service 真启动 → openclaw 真 load plugin → channel 真收消息
- 每个环节都要有断言或日志验证, 不能只看头尾两端

### 4.3 验证手段是真产物
- DB row, journalctl line, file content, 进程 PID — 这些是真产物
- UI 显示的状态 (Helper status = connected, Job status = succeeded) — 也是真产物
- 不接受: spec 内部计算的"预期值跟自己刚塞的值相等"

### 4.4 错误路径不能省
- happy path 跑通是底线
- 至少补一条错误路径: token 错 / 权限不足 / 服务下线 / 重复操作 / revoke 后再用
- 错误路径同样真按真点真观测, 不是 mock 一个错误响应

### 4.5 跑完真删干净
- dev-vm `docker compose down -v` 一定要跑, 不留 volume
- server-go 跑前 / 跑后 DB 状态可重复 (要么 fresh DB, 要么测前清表)
- 一台 dev-vm 不允许跨多个 e2e 跑次 — 一跑一删

### 4.6 用户视角验真
- 自问: "我作为用户能不能 ____?" — 能, 才算这一步通了
- 例: "我作为用户能不能看到 helper 上线" → 能 (UI 显示 connected) → 通
- 例: "我作为用户能不能在 channel 里收到 plugin 的回应" → 看 channel UI 真有新 message → 通
- 不能用 "我作为开发者能不能 grep 到日志" 代替 "用户能不能看到"

### 4.7 UI 合理性 + 正确性
功能跑通 ≠ UI OK. 真跑 e2e 时**眼睛要在 browser 上**, 不能只盯 spec 输出. 检查清单:

- **状态反馈正确**: 点了按钮有 loading 态? 完成有成功反馈? 失败有可读错误 (不是 stack trace)?
- **空 / 部分 / 满载状态**: 列表为空时有 empty state? 数据多时分页 / 滚动正常? 边界值 (字符串过长, 数字超限, 时间格式) 真显示对?
- **跨页面状态一致**: 在 A 页改了东西, B 页打开后真看到更新? 不是 stale cache?
- **realtime 更新真到**: 另一个 tab / dev-vm 触发的变化, 当前 tab 真自动刷新? 不需要手动 reload?
- **文案合理**: 按钮 / 状态 / 错误 / 提示 — 中文英文不混乱, 不出现 placeholder ("TODO" / `{{name}}` / "undefined") / 不出现 dev-only debug 字串.
- **布局不错乱**: 不同 viewport (起码 1280 + 375) 没溢出 / 重叠 / 元素被遮挡.
- **没 console 红字**: browser DevTools console 必须**零 error** (warning 允许但要看一眼). 红字 = UI 真有 bug, 不管功能是否跑通.
- **键盘可达**: 关键操作 Tab 能聚焦, Enter 能触发, Esc 能关弹窗. 不强求 WCAG 全套, 但破到完全用不了键盘 = ❌.
- **a11y 最小集**: form input 有 label, modal 有 aria-modal / focus trap, 链接有可读 text — 用 Playwright `getByRole` 找得到 = 基本合格.

UI 合理性问题跟功能 bug 同等严重 — 不准用"功能通了, UI 小问题之后修"的借口把 UI 退化吞下.

### 4.8 反模式自检 (每次跑前过一遍)
- [ ] server-go 在不在容器里? 在 → ❌, 必须宿主
- [ ] dev-vm Dockerfile 里有没有预装任何 borgee / Node / openclaw / systemd unit? 有 → ❌, 干净 Ubuntu
- [ ] e2e spec 里有没有 hardcode `--server-origin=...` / `--enrollment-token=...` / api_key? 有 → ❌, 从 UI 拿
- [ ] Playwright spec 里有没有 `page.route()` / `page.evaluate(fetch)` 绕 UI? 有 → ❌, 真按真点
- [ ] 跑前是不是 SQL 插了业务数据? 是 → ❌, 全程从 UI 操作
- [ ] dev-vm 是不是上一次跑剩下的? 是 → ❌, down -v 重起
- [ ] 测试只 assert HTTP 200 没看真产物? 是 → ❌, 看真产物
- [ ] 只跑了 happy path? 是 → ⚠️, 至少补一条错误路径
- [ ] 跑过程中眼睛有没有真在 browser 上 (不只盯 spec 输出)? 没 → ⚠️, 4.7 那些 UI 维度必扫
- [ ] browser DevTools console 有没有红字 error? 有 → ❌ (功能通了 UI 有 bug 一样不能算过)
- [ ] 关键文案有没有 placeholder / debug 字串 / 中英乱串? 有 → ❌
