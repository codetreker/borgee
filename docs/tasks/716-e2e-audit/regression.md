# 716 回归项

| 检查项 | 怎么验 | 谁负责 | 状态 |
|---|---|---|---|
| F1: 不允许 `fs.existsSync` / `fs.stat` / `fs.readFileSync` 在 e2e spec | `grep -rE "fs\.(existsSync\|stat\|readFileSync\|readdirSync)" packages/e2e/tests/*.spec.ts` 期望 0 hit | liema | ✅ DELETE hb-1b-installer + clean chn-4-screenshots §4§5 注释 后 0 hit |
| F2: 不允许 `page.evaluate(() => fetch())` 走 cookie 直调 (主体路径) | `grep -rE "page\.evaluate\([^)]*=>[^)]*fetch" packages/e2e/tests/*.spec.ts` 期望 0 hit (REWRITE-NAV 显式允许 server gate sanity F2 例外, 必带注释标记) | liema | ✅ 主体 0 hit, REWRITE-NAV server gate sanity 例外明文标 |
| F3-1: 主体路径只走 REST (`.goto=0 + .click=0 + apiRequest>0`) | `for f in packages/e2e/tests/*.spec.ts; do goto=$(grep -cE '\.goto\(' $f); click=$(grep -cE '\.click\(' $f); api=$(grep -cE 'apiRequest\.newContext' $f); [ "$goto" -eq 0 ] && [ "$click" -eq 0 ] && [ "$api" -gt 0 ] && echo "PURE_REST: $f"; done` 期望 0 hit (SKIP+followup 不算, test.describe.skip 包整 describe) | liema | ✅ 16 REWRITE 全完工后 0 PURE_REST hit |
| F3-2: 主体路径 apiRequest 仅在 seed (admin login + invite + register) | grep `apiRequest.newContext` 全部出现在 seed helper function 内 | yema/heima | ✅ 全 spec apiRequest 用法 limit 到 admin/mintInvite/registerUser helper |
| F4: 不允许源码字符串 grep 假装锁 (`fs.readFileSync` 读 .go/.ts 文件) | `grep -rE "fs\.readFileSync.*\.go\|fs\.readFileSync.*\.ts" packages/e2e/tests/*.spec.ts` 期望 0 hit | liema | ✅ DELETE hb-1b-installer 后 0 hit |
| F5: 不允许 noop `expect(true).toBe(true)` 占位 | `grep -rE "expect\(true\)\.toBe\(true\)" packages/e2e/tests/*.spec.ts` 期望 0 hit | liema | ✅ DELETE 2 noop spec 后 0 hit |
| 文件名: 反 milestone 前缀 / gh-XXX- 前缀 | `ls packages/e2e/tests/ \| grep -E "^[a-z]{2}-[0-9]\|^gh-[0-9]+\|^g[0-9]"` 期望 0 文件 | yema | ✅ 28 PASS rename + 6 REWRITE rename + 9 SKIP rename 后全功能名 |
| 反向证: e2e 绿色 case 关 backend 必 fail | testing 环境停 server container + 跑全套 e2e + 断 ≥90% fail | liema | ⏳ gh#724 §3 follow-up (CI job 工作量超 #716, liema Q2 拍 follow-up) |
| client UI 缺失 3 步判断 (反"看着没就 skip") | grep production mount + testing 真浏览器手验 + 1+2 都无才 SKIP | zhanma-c | ✅ 9 SKIP+followup 严格依 3 步 (含 2026-05-11 二次复核 ap-2 + rt-3) |
