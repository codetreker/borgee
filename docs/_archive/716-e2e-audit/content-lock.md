# 716 文案锁 — e2e spec 描述自然语言规范

> Issue: gh#716 (P0)

此 PR 不动产品文案. 文案锁仅锁 e2e spec 文件头部注释的写法规范.

## ① 禁用黑话词列表 (字面替换)

| 黑话 | 改自然语言 |
|---|---|
| 立场承袭 / 立场反查 / 立场承袭 byte-identical | 跟 X 一致 / 跟 docs/blueprint/X.md §X.Y 一致 |
| byte-identical | 字面相等 / 一字一字对得上 |
| 锚 / 锚点 / 反向 grep 锚 | 关联 / 链接 / 引用 / 查找模式 |
| audit真删 | 已删 (audit 后) / 删除原因: ... |
| 反约束 sanity | 反向检查 / 兜底验证 |
| 字面验 byte-identical | 字面相等检查 |
| ⭐⭐⭐ / 🔴🔴 / 三联签 | 不用强调符, 写完整句子 |
| 守源头 / 锁源头 byte-identical 守 | 由 X 测试覆盖 / 由 X 锁定 |
| 真挂 / 真测 / 真路径 | 真实路径 / 真实测试 |
| #480 byte-identical / 跟 #530 同模式 | 跟 PR #480 一致 / 跟 PR #530 同款 |
| 反 IDOR / 反越权 / 反假装真值 | 防越权 / 防 IDOR / 防伪造 |
| follow-up | 后续任务 / 待办 |
| acceptance §X 闭环 | 完成 acceptance §X / 满足 acceptance §X |

## ② 文件头部注释模板 (REWRITE 后)

```typescript
// tests/<feature-name>.spec.ts — <一句话描述这 spec 测什么>
//
// 测试范围:
//   - <case 1 一句话>
//   - <case 2 一句话>
//   - <case 3 一句话>
//
// 关联文档:
//   - 蓝图: docs/blueprint/current/<file>.md (如有)
//   - 验收: docs/tasks/<owner>/acceptance.md §X (如有)
//
// 实施约束:
//   - 真 UI 走浏览器 (page.goto + page.click + DOM 断)
//   - seed 用 REST (admin login + invite + register), 测试主体必须走 UI
//   - 不允许 fs.* / page.evaluate(fetch) / 只打 API / noop
```

## ③ test/describe 文字规范

- describe 写一句话, 描述这一组测什么 (例: `'agent 配置 form 在不同 viewport 下排版正确'`)
- test 名写一句话, 描述这 case 测什么场景 (例: `'1280 viewport 下 6 个 field 各占独立行'`)
- 不在 describe / test 名里塞 §X.Y 编号 (写 acceptance.md 引用就够). 如必须引, 写 `(acceptance §X.Y)` 末尾.

## ④ DELETE spec 不留替身

DELETE 的 3 个 spec 不写 "已删, 由 X 替代" 占位. 直接 git rm. 删除原因写在 audit.md 表格内.

## ⑤ 反向 grep 守卫

PR 合前 (review 阶段必查):

```bash
# 黑话残余
grep -rnE "立场承袭|byte-identical|锚点|audit真删|反约束 sanity|守源头" packages/e2e/tests/*.spec.ts
# 期望: 0 hit

# 强调符堆叠
grep -rE "🔴🔴|⭐⭐⭐" packages/e2e/tests/*.spec.ts
# 期望: 0 hit
```
