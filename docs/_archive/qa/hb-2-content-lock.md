# HB-2 v0(D) content lock — daemon UI / status 5 支柱字面 (≤40 行)

> daemon UI / status 字面 SSOT 跟 stance §1+§7 byte-identical. **5 支柱字面跨 HB-4 release-gate + content-lock + UI 三处对锁**. 反 typing/loading 类禁词 (跟 RT-3 ⭐ + thinking 5-pattern 锁链承袭).

## §1 5 支柱状态字面 (HB-4 release-gate + UI byte-identical)
| 支柱 | 字面 |
|---|---|
| 启动 (< 800ms) | `已启动` (绿) / `启动中` (灰, 仅 < 800ms 内) / `启动失败` (红) |
| 崩溃率 (< 0.1%) | `稳定运行` (绿) / `近 24h 崩溃 ${N} 次` (黄) |
| 签名校验 (0% fail) | `签名校验通过` (绿) / `签名校验失败` (红) |
| audit 写状态 | `审计日志正常` (绿) / `审计日志异常` (红) |
| 撤销 (< 100ms) | `授权可即时撤销` (绿固定文案) |

## §2 daemon 命名字面 (跟 #599 PM 必修 #1 承袭)
- daemon 进程显示名: `Borgee Helper` (byte-identical 跨 about 页 + 进程列表 + setting)
- 反 `borgee-daemon` / `borgee-bridge` / `host-bridge-helper` 同义词漂

## §3 反约束 — typing / loading 类禁词 (反向 grep 0 hit)
**英**: `typing` / `composing` / `loading` / `please wait` / `spinner`
**中**: `正在输入` / `正在加载` / `请稍候` / `加载中` / `处理中`
反向 grep 真测在 packages/client/ + daemon UI 路径 0 hit (跟 RT-3 ⭐ presence 活物感同精神).

## §4 thinking 5-pattern 锁链立场延伸 (HB-2 v0(D) 第 N+2 处)
5 禁词 (跟 BPP-3+CV-7+CV-8/9/11/12/13/14+DM-3/4/9/12+RT-3 承袭): `processing` / `responding` / `thinking` / `analyzing` / `planning` 在 HB-2 daemon UI 路径 0 hit. 5 支柱仅状态 + 数字, 不显语义中间态.

## §5 DOM data-attr SSOT (daemon status UI)
| attr | 取值 |
|---|---|
| `data-hb2-pillar` | `startup` / `crash` / `signature` / `audit` / `revocation` |
| `data-hb2-pillar-state` | `green` / `yellow` / `red` |
| `data-hb2-helper-name` | `Borgee Helper` (字面单源锚) |

## §6 真测 grep 锚 (CI / PR 真验)
```
git grep -nE '"已启动"|"稳定运行"|"签名校验通过"|"审计日志正常"|"授权可即时撤销"' packages/   # ≥5 hit
git grep -nE 'data-hb2-(pillar|pillar-state|helper-name)' packages/client/   # ≥3 hit
git grep -nE 'typing|composing|loading|spinner|正在加载|加载中|请稍候' packages/client/src/components/HB2*   # 0 hit
git grep -nE 'processing|responding|thinking|analyzing|planning' packages/client/src/components/HB2*   # 0 hit
```
