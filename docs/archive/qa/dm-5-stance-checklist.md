# DM-5 立场反查清单 (战马E v0)

> 战马E · 2026-04-29 · 跟 CV-9/10/11/12 client-only 同模式 + CV-7 reaction endpoint 复用.
> **关联**: spec `dm-5-spec.md` (b783c24) + acceptance + content-lock.

## §0 立场总表 (4 立场 + 3 边界)

| # | 立场 | 反约束 |
|---|---|---|
| ① | reaction 走既有 PUT/DELETE/GET `/api/v1/messages/{id}/reactions` 单源, **0 server code** | 反向 grep `dm5.*reaction\|reaction_summary.*PRIMARY\|dm5.*aggregator` 在 internal/ count==0 |
| ② | owner-only ACL byte-identical 15+ 处, admin 不挂 | 反向 grep `admin.*dm.*reaction\|admin.*reaction.*summary` 在 admin*.go count==0 |
| ③ | thinking 5-pattern 锁链不漂 (read-side, 8 处不变) | 反向 grep `dm5.*thinking\|dm5.*subject` 在 client/src 0 hit |
| ④ | chip DOM `data-dm5-reaction-chip` + `data-dm5-reaction-count` + `data-dm5-reaction-mine` 锚 + 文案 `{emoji} {count}` byte-identical | 反向 grep 3 锚 ≥1 hit each |

## §1 立场 ① 0 server code

- [ ] CV-7 既有 reaction endpoint 不动
- [ ] 1 unit 反向断 GET `/messages/{id}/reactions` 在 DM channel 等价
- [ ] git diff packages/server-go/ 0 production 行
- [ ] 反向 grep 3 锚 0 hit

## §2 立场 ② owner-only / admin 不挂

- [ ] DM channel-member 既有 ACL 不动
- [ ] 反向 grep `admin.*dm.*reaction` 0 hit

## §3 立场 ③ thinking 5-pattern 不漂

- [ ] read-side, 不解 thinking
- [ ] CV-7/CV-8 write-side hook 不动
- [ ] 反向 grep `dm5.*thinking|dm5.*subject` 0 hit

## §4 立场 ④ DOM 锚 + 文案

- [ ] `data-dm5-reaction-chip="<emoji>"` 锚 (反向 grep ≥1)
- [ ] `data-dm5-reaction-count="<N>"` 锚 (反向 grep ≥1)
- [ ] `data-dm5-reaction-mine` 锚 (current user reacted highlight, 反向 grep ≥1)
- [ ] 文案 `{emoji} {count}` byte-identical (空格分隔)

## §5 边界 ⑤⑥⑦ — fail-closed / forward-only / 不裂表

- [ ] cross-channel reject 跟 CV-7..CV-12 同源
- [ ] forward-only — reaction 即写即读, 不留 history
- [ ] 不裂表 — 0 schema 改 (message_reactions 既有覆盖)

## §6 退出条件

- §1+§2+§3+§4 全 ✅
- 反向 grep 5 锚: 4 处 0 hit + DOM 3 锚 ≥1 hit
- 1 server unit + 5 vitest + e2e 3 case 全 PASS
- 0 schema 改 + 0 server production code
