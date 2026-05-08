# RT-3 ⭐ content lock — presence 活物感字面 + DOM data-attr (≤40 行)

> RT-3 ⭐ 升级 client UI 字面 SSOT, 跟 stance §4+§5 byte-identical. **反 typing-indicator 立场延伸 — typing 类同义词全 reject** (thinking 5-pattern 锁链承袭).

## §1 presence 字面锁 (byte-identical)
| 槽位 | 字面 |
|---|---|
| online dot tooltip | `在线` |
| offline dot tooltip | `离线` |
| recently active | `刚刚活跃` |
| last-seen template | `最近活跃 ${N} 分钟前` |
| zero-state | (null, 不渲染) |

## §2 DOM data-attr SSOT
| attr | 取值 |
|---|---|
| `data-rt3-presence-dot` | `online` / `offline` / `recently-active` |
| `data-rt3-last-seen` | unix millis |
| `data-rt3-cursor-user` | user_id |

## §3 反约束 — typing 类同义词全 reject (反向 grep 0 hit)
**英**: `typing` / `composing` / `is_typing` / `user_typing` / `composing_indicator`
**中**: `正在输入` / `正在打字` / `输入中` / `打字中`
反向 grep 真测在 packages/client/ + packages/server-go/ 0 hit.

## §4 thinking 5-pattern 锁链立场延伸 (RT-3 ⭐ 第 N+1 处)
5 禁词 (跟 BPP-3+CV-7+CV-8/9/11/12/13/14+DM-3/4/9/12 承袭): `processing` / `responding` / `thinking` / `analyzing` / `planning` 在 RT-3 路径 0 hit. presence 仅活物感 + 时间戳, 不显语义中间态.

## §5 跨 milestone 字面承袭锁链
- AL-3 #324 `online/offline` 二态 → RT-3 ⭐ 三态加 `recently-active` byte-identical
- RT-1.3 #296 cursor opaque token 字面不动
- thinking 5-pattern 锁链 RT-3 = 第 N+1 处延伸

## §6 真测 grep 锚 (CI / PR 真验)
```
git grep -nE '"在线"|"离线"|"刚刚活跃"|"最近活跃 \$\{N\} 分钟前"' packages/client/   # ≥4 hit
git grep -nE 'data-rt3-(presence-dot|last-seen|cursor-user)' packages/client/   # ≥3 hit
git grep -nE 'typing|composing|isTyping|正在输入|正在打字' packages/   # 0 hit
git grep -nE 'processing|responding|thinking|analyzing|planning' packages/client/src/components/RT3*   # 0 hit
```
