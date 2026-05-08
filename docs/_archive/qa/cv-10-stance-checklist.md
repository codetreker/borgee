# CV-10 立场反查清单 (战马E v0)

> 战马E · 2026-04-29 · 立场 review checklist (跟 CV-9 #539 client-only 同模式 + DM-4 既有 draft key namespace 承袭)
> **关联**: spec `docs/implementation/modules/cv-10-spec.md` (aa743bc) + acceptance `docs/qa/acceptance-templates/cv-10.md` + content-lock `docs/qa/cv-10-content-lock.md`.

## §0 立场总表 (4 立场 + 3 边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | 草稿走 localStorage 单源 — key `borgee.cv10.comment-draft:<artifactId>`; **0 server production code + 0 schema 改 + 0 新 endpoint** | CV-9 #539 client-only + DM-4 既有 draft 同模式 | 反向 grep `comment_drafts.*PRIMARY\|/comments/.*/draft\|/artifacts/.*/draft` 在 internal/ count==0 |
| ② | submit 后清 + page leave 警告 (浏览器原生 beforeunload, 不挂自定义 modal) | UX 不变量 + 复用浏览器原生 | 反向 grep `cv10.*confirm.*leave\|cv10.*custom.*modal` count==0 |
| ③ | owner-only ACL byte-identical — 草稿仅本地 per-browser-profile, 无跨 user 漏 | 隐私 §13 + ADM-0 §1.3 红线 | 反向 grep `admin.*cv10.*draft\|admin.*comment.*draft` 在 admin*.go count==0 |
| ④ | textarea + restore toast DOM 锚 + 文案 byte-identical | content-lock 必锁 | 反向 grep `data-cv10-draft-textarea\|data-cv10-restore-toast` ≥2 hit; "已恢复未保存的草稿" + "草稿已清除" 各 ≥1 hit |

## §1 立场 ① localStorage 单源 + 0 server code

- [ ] hook 用 localStorage (反约束 sessionStorage 0 hit `cv10.*sessionStorage`)
- [ ] key namespace `borgee.cv10.comment-draft:<artifactId>` byte-identical (反向 grep ≥1 hit)
- [ ] 0 server code — git diff packages/server-go/ 0 production 行
- [ ] 反向 grep `comment_drafts.*PRIMARY\|/comments/.*/draft\|/artifacts/.*/draft` 0 hit

## §2 立场 ② submit 清 + leave 警告

- [ ] submit 成功 → localStorage.removeItem(key) (vitest 反向断 getItem returns null)
- [ ] beforeunload 走浏览器原生 (event.preventDefault() + returnValue=''), 不挂自定义 modal
- [ ] 反向 grep `cv10.*confirm.*leave` 0 hit

## §3 立场 ③ owner-only / privacy

- [ ] localStorage 是 per-browser-profile, server 不持有任何 draft (admin god-mode 不能看)
- [ ] 反向 grep `admin.*cv10.*draft\|admin.*comment.*draft` 在 admin*.go 0 hit

## §4 立场 ④ DOM + 文案 byte-identical

- [ ] textarea 渲染 `data-cv10-draft-textarea="<artifactId>"` (反向 grep ≥1)
- [ ] restore toast 渲染 `data-cv10-restore-toast` (反向 grep ≥1)
- [ ] 文案 "已恢复未保存的草稿" byte-identical (反向 grep ≥1)
- [ ] 文案 "草稿已清除" byte-identical (反向 grep ≥1)

## §5 边界 ⑤⑥⑦ — fail-closed / forward-only / 不裂表

- [ ] 跨 user 不漏 — logout 清 cv10 keys (复用既有 cleanup path)
- [ ] forward-only — submit 即清, 不留 history; reload restore 是同一个 draft
- [ ] 不裂表 — 0 schema 改 + localStorage only

## §6 退出条件

- §1 (4) + §2 (3) + §3 (2) + §4 (4) + §5 (3) 全 ✅
- 反向 grep 6 锚: 4 处 0 hit + DOM/文案 ≥1 hit
- vitest 4 case + e2e 3 case 全 PASS
- 0 schema 改 + 0 server production code
