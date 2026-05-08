# 716 — e2e 真 UI 审计

> Issue: https://github.com/codetreker/borgee/issues/716
> 优先级: P0 / current-iteration

## 要做什么

全量审计 `packages/e2e/tests/**/*.spec.ts`, 每个 case 必须满足:

1. 走真 UI — Playwright `page.goto` / `page.click` / `page.fill` 等真用户交互
2. 断言真行为 — DOM 状态变化 / 网络请求结果 / URL 变化 / 可见文案变化
3. 不允许这几种假 e2e:
   - `fs.existsSync` / `fs.stat` / `fs.readFileSync` 检查 git 内文件 (那是 lint, 不是 e2e)
   - `page.evaluate(() => fetch(...))` 走 cookie 直调后端 (那是后端 contract test)
   - 用 `apiRequest` 只打 API 不开浏览器 (那是 integration test)
   - 源码字符串 grep 假装锁

## 不做什么

- 不删任何 e2e case (DELETE 标记单独走 follow-up issue)
- 不改 e2e 框架本身 (Playwright config / fixture / helper 不动)

## 边界

- 文件名都改成有意义的真实功能名, 不要 milestone 名 (al-3-* / cv-7-* 这种)
- 文件里所有描述文字全部 refine, 不允许黑话/简写, 全部自然语言

## 已知反例 (PR #715 暴露)

- `packages/e2e/tests/hb-1b-installer.spec.ts` 6 case 全是 `fs.existsSync` PNG 文件存在性检查
- `packages/e2e/tests/chn-4-screenshots-followup.spec.ts` §4 §5 是 `fs.stat` 截图存在性
