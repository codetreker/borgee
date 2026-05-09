# 716 回归项

| 检查项 | 怎么验 | 谁负责 |
|---|---|---|
| 不允许 `fs.existsSync` / `fs.stat` 在 e2e spec 里 | grep `packages/e2e/tests/**` | liema |
| 不允许 `page.evaluate(() => fetch` 走 cookie 直调 | grep `page.evaluate` 看上下文 | liema |
| 每个 e2e 绿色 case 关 backend 后必 fail | testing 环境停服跑 e2e | liema |
