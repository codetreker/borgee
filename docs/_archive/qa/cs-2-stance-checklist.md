# CS-2 立场反查表 (故障三态 + 四层 UX 呈现)

> **状态**: v0 (野马 / 飞马, 2026-04-30)
> **目的**: CS-2 实施 PR 直接吃此表为 acceptance — 战马D 实施 + 烈马 acceptance template + 飞马 spec brief 反查立场漂移; 一句话立场 + §X.Y 锚 + 反约束 + v0/v1.
> **关联**: 蓝图 `client-shape.md` §1.3 (故障 UX 分层呈现 + 三态枚举 + plain language + inline 修复); AL-1b #462 5-state PresenceDot (busy/idle 跟故障三态拆分立场); AL-4 #417 runtime status 6-reason 字典; reasons.IsValid #496 SSOT 包.
> **依赖**: 0 server / 0 schema (Wrapper 选项 C 同 CS-1 模式承袭).

---

## 1. CS-2 立场反查表 (3 立场)

| # | 立场锚 | 一句话立场 | 反约束 (X 是, Y 不是) | v0 / v1 |
|---|--------|----------|----------------------|---------|
| ① | client-shape §1.3 三态枚举表 + AL-1b §2.3 BPP progress frame | **故障三态 byte-identical 跟蓝图 — `online` / `failed` / `offline`** (野马 push back 收敛锁) | **是** `FAILURE_TRI_STATE = ['online', 'error', 'offline']` SSOT 单源 + `IsFailureState(s)` helper (跟 reasons.IsValid #496 同模式); **不是** 第 4 态 `busy` / `idle` 漂入 (那是 AL-1b BPP progress frame 真实施时的活, CS-2 三态拆死锁 byte-identical 跟蓝图 §1.3 "工作中/空闲 BPP progress 真实施再加第四态" 字面承袭); **不是** `standby` / `connecting` / `paused` 同义词漂入; 反向 grep `'busy'\|'idle'\|'standby'` 在 cs-2-* 0 hit | v0/v1 永久锁 — 三态收敛是产品定位红线 |
| ② | client-shape §1.3 故障 UX 四层呈现表 + plain language 字面承袭 | **4 层 UX 呈现 byte-identical (头像角标 + 浮层 + banner + 故障中心), inline 修复不跳设置页** | **是** 4 层组件落地 (`PresenceDot variant="failure"` 红点 + `FailurePopover.tsx` 浮层 3 button + `FailureBanner.tsx` 阈值触发全屏宽 + `FailureCenter.tsx` 团队栏聚合) byte-identical 跟蓝图表; popover 3 inline button (`重连` / `重填 API key` / `查日志`) 文案 byte-identical 跟蓝图字面; **不是** 5 层漂 (toast / modal / inline-error 第 5 层; 反向 grep `toast.*failure\|FailureModal\|FailureInlineError` count==0); **不是** 浮层跳设置页 (蓝图字面 "inline 修复, 不跳设置页"; 反向 grep `navigate.*\/settings\|history\.push.*settings` 在 Failure*.tsx count==0); **不是** 浮层只显示 raw error code (必经 plain language 映射) | v0: 4 层 UI 占位 + repair hook stub; v1: repair 真路径接 plugin SDK |
| ③ | client-shape §1.3 plain language + reasons.IsValid #496 SSOT + 0-server-prod 选项 C | **plain language 文案映射 6-dict byte-identical 跟 AL-4 reason 字典 + 0 server / 0 schema** | **是** `FAILURE_REASON_LABELS` 6 dict (`api_key_invalid` / `quota_exceeded` / `network_unreachable` / `runtime_crashed` / `runtime_timeout` / `unknown`) byte-identical 跟 reasons.IsValid #496 + AL-4 #417 字面承袭, 改 = 改三处 (server reasons.go + client cs2-failure-labels.ts + content-lock §1); 文案 byte-identical 跟蓝图字面对比 (`"DevAgent 跟 OpenClaw 失联"` 替 `connection refused: openclaw://localhost:9100` 等); **不是** server 改 (`git diff origin/main -- packages/server-go/` count==0); **不是** schema 改 (反向 grep `migrations/cs_2\|cs2.*api\|cs2.*server` count==0); **不是** 同义词漂 (`故障了` / `挂了` / `不可用` / `服务异常` 在 user-visible text count==0) | v0/v1 永久锁 — 0-server-prod + plain language byte-identical 是 wrapper 立场 |

---

## 2. 黑名单 grep — CS-2 实施 PR merge 后跑, 全部预期 0 命中

```bash
# 立场 ① — busy/idle 不漂入三态枚举 (AL-1b 拆死)
git grep -nE "'busy'|'idle'|'standby'" packages/client/src/lib/cs2-failure-*  # 0 hit
# 立场 ② — 4 层不漂 (不另起 5 层)
git grep -nE 'toast.*failure|FailureModal|FailureInlineError' packages/client/src/  # 0 hit
# 立场 ② — inline 修复不跳设置页 (蓝图字面)
git grep -nE 'navigate.*\/settings|history\.push.*settings' packages/client/src/components/Failure*.tsx  # 0 hit
# 立场 ③ — 0 server 改 (Wrapper 选项 C)
git diff origin/main -- packages/server-go/ | grep -c '^\+'  # 0 production lines
# 立场 ③ — 0 schema 改
git grep -nE 'migrations/cs_2|cs2.*api|cs2.*server' packages/server-go/internal/  # 0 hit
# 立场 ③ — 同义词反向 (plain language byte-identical 跟蓝图)
git grep -nE '故障了|挂了|不可用|服务异常' packages/client/src/lib/cs2-failure-labels.ts  # 0 hit
# 立场 ③ — admin god-mode 不挂 (ADM-0 §1.3 红线)
git grep -nE 'admin.*failure-ux|admin.*FailureCenter' packages/client/src/  # 0 hit
```

---

## 3. 不在 CS-2 范围 (避免 PR 膨胀, 跟 spec §3 同源)

- ❌ 第 4 态 `busy` / `idle` (留 AL-1b §2.3 BPP progress frame 真实施)
- ❌ inline 修复真路径 (重连 RPC / 重填 key form / 查日志 page) — 留 plugin SDK + AL-2a / HB-3 真路径
- ❌ IndexedDB 乐观缓存 (蓝图 §1.4 — 留 **CS-4**, 跟 CS-1 spec §3 留账 byte-identical)
- ❌ Tauri 壳 / host-bridge daemon (留 HB-2, 依赖 HB-1)
- ❌ PWA install + Web Push (留 CS-3, 依赖 DL-4 #485 ✅ merged)
- ❌ admin god-mode 故障 UX (永久不挂, ADM-0 §1.3 红线)
- ❌ 桌面通知 / 故障声音 (留 DL-4 push gateway 接 Web Notifications API)

---

## 4. 验收挂钩

- CS-2.1 PR: 立场 ①③ — `FAILURE_TRI_STATE` 三态 byte-identical + `FAILURE_REASON_LABELS` 6-dict byte-identical 跟 AL-4 + 8 vitest (TestCS21_*)
- CS-2.2 PR: 立场 ② — 4 组件 + 3 inline button + repair hook stub + 12 vitest (TestCS22_*)
- CS-2.3 entry 闸: 立场 ①-③ 全锚 + §2 黑名单 grep 全 0 + 跨 milestone byte-identical (AL-1b PresenceDot variant + reasons.IsValid #496 + AL-4 #417 + ADM-0 §1.3 红线) + REG-CS2-001..006 全 🟢 + e2e 4 case PASS

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-30 | 野马 / 飞马 | v0, 3 立场 (三态拆死 / 4 层 UX byte-identical 不漂 5 层 / plain language 6-dict + 0-server-prod 选项 C) 承袭蓝图 §1.3 故障 UX 字面 + AL-1b 拆分立场 + reasons.IsValid #496 SSOT + AL-4 #417 reason 字典. 7 行反向 grep (含 admin god-mode 反向第 7 锚) + 7 项不在范围 + 验收挂钩三段对齐. 命名澄清: CS-2 = §1.3 (CS-3=PWA / CS-4=IndexedDB), 跟 CS-1 spec §3 留账 byte-identical. 0 server / 0 schema wrapper 模式同 CS-1 / CV-9..14 / DM-5..6 / DM-9. |
