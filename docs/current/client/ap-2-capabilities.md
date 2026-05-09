# AP-2 client — capability 透明 UI 无角色名 (≤60 行)

> 落地: feat/ap-2 AP2.2 (`lib/capabilities.ts` + `lib/capability-bundles.ts` + `components/PermissionsView.tsx` + `components/BundleSelector.tsx` + 22 vitest + 4 Playwright e2e)
> 关联: server `docs/current/server/ap-2.md` /api/v1/me/permissions response shape

## 1. capability label SSOT — `lib/capabilities.ts`

```ts
export const CAPABILITY_TOKENS = [...] as const; // 14 字面 byte-identical 跟 server auth.ALL
export type CapabilityToken = (typeof CAPABILITY_TOKENS)[number];
export function capabilityLabel(token: string): string;        // 14 中文字面 LABEL_MAP + 未知 forward-compat
export function isKnownCapability(token: string): boolean;     // 反向断言 helper
```

LABEL_MAP 14 字面 byte-identical (跟 content-lock §1; CAPABILITY-DOT 后 dot-notation):
- channel.read → 查看频道 / channel.write → 在频道发消息 / channel.delete → 删除频道
- artifact.read → 查看产物 / artifact.write → 编辑产物 / artifact.commit → 提交产物
- artifact.iterate → 迭代产物 / artifact.rollback → 回滚产物
- user.mention → 提及用户 / dm.read → 查看私信 / dm.send → 发送私信
- channel.manage_members → 管理频道成员 / channel.invite → 邀请用户 / channel.change_role → 调整成员能力

## 2. component — `components/PermissionsView.tsx`

DOM data-attr SSOT (跟 content-lock §2 byte-identical):
- `data-ap2-permissions-view` (root list)
- `data-ap2-capability-row` + `data-ap2-capability-token` + `data-ap2-scope` + `data-ap2-known`
- `data-ap2-capability-label` + `data-ap2-capability-scope`
- 5 态: `data-ap2-empty` / `data-ap2-loading` / `data-ap2-error` + 多行 + wildcard `*` 渲染 `完整能力`

## 3. 反约束

- ❌ RBAC 角色名漂入 (英 admin/editor/viewer/owner + 中 管理员/编辑者/查看者) 0 hit
- ❌ inline 字面散落 (capabilityLabel SSOT 单源 production 仅 1 hit)
- ❌ admin god-mode UI 永久独立 (capabilityLabel 不挂 components/admin/* 路径)
- ❌ thought-process 5-pattern + typing-indicator 漂入 (跟 RT-3 #616 一致)

## 4. bundle SSOT — `lib/capability-bundles.ts` + `components/BundleSelector.tsx`

3 bundle (蓝图 §1.3 A' 快速 bundle 无角色名, byte-identical; CAPABILITY-DOT 后 dot-notation):
- `workspace` (工作能力) → channel.write + artifact.write + artifact.commit (3)
- `reader` (阅读能力) → channel.read + artifact.read + dm.read (3)
- `mention` (提及能力) → user.mention + dm.send (2)

BundleSelector 主权 UI: bundle click → 展开 capability checkbox (default-all-checked but uncheckable) → 用户必显式 confirm → caller 派 N 次 AP-1 PUT /api/v1/permissions (复用既有 endpoint, 反 POST /api/v1/bundles 旁路).

DOM 锚: `data-ap2-bundle-selector` / `data-ap2-bundle-row` / `data-bundle-name` / `data-ap2-bundle-checkbox` / `data-ap2-bundle-confirm`.

## 5. tests

- `__tests__/ap-2-capabilities.test.ts` 5 vitest
- `__tests__/PermissionsView.test.tsx` 5 vitest
- `__tests__/capability-bundles.test.ts` 5 vitest (跨层锁 + assertBundlesValid + helpers)
- `__tests__/BundleSelector.test.tsx` 4 vitest (expand + 主权 uncheck + 必显式 confirm + DOM 锚)
- `__tests__/ap-2-reverse-grep.test.ts` 11 vitest (14 const + 反 RBAC 英 4 / 中 3 + admin 独立 + SSOT 单源 + PascalCase bundle 名 + role in bundle const + POST /api/v1/bundles + BundleHasCapability/HasBundle 0 hit)
- `packages/e2e/tests/ap-2-bundle.spec.ts` Playwright 4 case (capability response shape + 反 bundle endpoint 漂 + UI 真渲染反 RBAC 8 词 0 hit body + admin god-mode UI 独立路径) + screenshot `docs/qa/screenshots/ap-2-bundle-ui.png`
