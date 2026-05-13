# 716 立场 — e2e 真 UI 验证铁律

> Issue: gh#716 (P0 / current-iteration)

## 立场 ① 真 UI 是 e2e 唯一定义

e2e 必须走浏览器, `page.goto` / `page.click` / `page.fill` / `getByRole` / `locator()` 是真用户路径. 没浏览器交互的 spec 不是 e2e.

## 立场 ② 反 4 类假证模式

- F1 — 不允许 `fs.existsSync` / `fs.stat` / `fs.readFileSync` / `fs.readdirSync` 检查 git 内文件 (那是 lint, 不是 e2e)
- F2 — 不允许 `page.evaluate(() => fetch())` 走 cookie 直调后端 (那是后端 contract test)
- F3 — 不允许 `apiRequest.newContext` 只打 API 不开浏览器 (那是 integration test). seed precondition 用 REST 允许, 测试主体必须走 UI.
- F4 — 不允许源码字符串 grep 假装锁
- F5 — 不允许 noop `expect(true).toBe(true)` 占位

## 立场 ③ 反向证: 关 backend 后必 fail

每个 e2e 绿色 case 关掉 backend 后必须 fail. 这是 issue §验收第 4 条.

第一轮 (本 PR) 仅消除假 e2e (F1-F5 = 0 hit). 反向证机制 (kill backend 跑全 fail) 由 liema 立基础设施, 留 followup, 不在本 PR.

## 立场 ④ 文件名按功能不按 milestone

文件名禁用 milestone 前缀 (`al-3-3-` / `cv-7-` / `chn-4-` / `gh-698-` / `g2.4-` 等). 改成功能名 (`agent-list-presence-indicator` / `comment-edit-delete-permission` / `channel-collab-tabs` / `agent-config-form-layout` / `demo-screenshot-archive`). 跟 memory `file_naming_no_milestone_prefix` 一致.

## 立场 ⑤ 描述自然语言不黑话

spec 文件头部注释禁用 "立场承袭" / "byte-identical" / "锚" / "audit真删" / "反向 grep" / "✅✅" 等黑话. 改自然语言 ("跟 X 一致" / "字面相等" / "关联" / "已删" / "查找"). 跟 memory `no_jargon_strict` 一致.

## 立场 ⑥ 一 PR 一锅

DELETE 3 + PASS 24 + PASS+fix 3 + REWRITE 16 一个 PR 全做完. 跟用户铁律 `strict_one_milestone_one_pr` 一致, 跟 issue §"P0 一次做干净, 不允许留着以后改" 一致. 不拆 docs/test-rename/REWRITE 三 PR.

## 反 X 段 (by-construction)

- 反 F1: PR 合前 `grep -rE "fs\.(existsSync|stat|readFileSync|readdirSync|statSync)" packages/e2e/tests/*.spec.ts` 必须 0 hit
- 反 F2: `grep -rE "page\.evaluate\([^)]*=>[^)]*fetch" packages/e2e/tests/*.spec.ts` 必须 0 hit
- 反 F5: `grep -rE "expect\(true\)\.toBe\(true\)" packages/e2e/tests/*.spec.ts` 必须 0 hit
- 反命名违规: `find packages/e2e/tests -name "[a-z][a-z]-[0-9]*.spec.ts" -o -name "gh-*.spec.ts" -o -name "g[0-9]*.spec.ts"` 必须 0 hit (空)
- 反假 UI 凑数: REWRITE 后 client 没真 UI 入口的 case 留 `test.skip` + todo 注释 + follow-up issue, 不降级回 REST
- 反偷懒 admin god-mode 跑测: REWRITE seed 仍用现有 e2e-admin / register user 模式, 不引入新凭据
