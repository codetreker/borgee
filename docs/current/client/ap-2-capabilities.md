# AP-2 client — capability-based UI without role names (≤60 行)

> 落地: feat/ap-2 AP2.2 (`lib/capabilities.ts` + `lib/capability-bundles.ts` + `components/PermissionsView.tsx` + `components/BundleSelector.tsx` + 22 vitest + 4 Playwright e2e)
> 关联: server `docs/current/server/ap-2.md` /api/v1/me/permissions response shape

## 1. capability label source of truth — `lib/capabilities.ts`

```ts
export const CAPABILITY_TOKENS = [...] as const; // 14 literals, byte-identical with server auth.ALL
export type CapabilityToken = (typeof CAPABILITY_TOKENS)[number];
export function capabilityLabel(token: string): string;        // 14 中文字面 LABEL_MAP + 未知 forward-compat
export function isKnownCapability(token: string): boolean;     // 反向断言 helper
```

LABEL_MAP has 14 byte-identical literals (with content-lock §1; dot notation after CAPABILITY-DOT):

- channel.read → 查看频道 / channel.write → 在频道发消息 / channel.delete → 删除频道
- artifact.read → 查看产物 / artifact.write → 编辑产物 / artifact.commit → 提交产物
- artifact.iterate → 迭代产物 / artifact.rollback → 回滚产物
- user.mention → 提及用户 / dm.read → 查看私信 / dm.send → 发送私信
- channel.manage_members → 管理频道成员 / channel.invite → 邀请用户 / channel.change_role → 调整成员能力

## 2. component — `components/PermissionsView.tsx`

DOM data attributes are defined in one place (byte-identical with content-lock §2):

- `data-ap2-permissions-view` (root list)
- `data-ap2-capability-row` + `data-ap2-capability-token` + `data-ap2-scope` + `data-ap2-known`
- `data-ap2-capability-label` + `data-ap2-capability-scope`
- 5 态: `data-ap2-empty` / `data-ap2-loading` / `data-ap2-error` + 多行 + wildcard `*` 渲染 `完整能力`

## 3. Negative constraints

- ❌ RBAC role labels must not appear (English admin/editor/viewer/owner + Chinese 管理员/编辑者/查看者) 0 hit
- ❌ capability labels must not be duplicated inline (production calls `capabilityLabel` from one source)
- ❌ admin-only UI stays separate (`capabilityLabel` is not imported under `components/admin/*`)
- ❌ thought-process 5-pattern and typing-indicator wording must stay aligned with RT-3 #616

## 4. bundle source of truth — `lib/capability-bundles.ts` + `components/BundleSelector.tsx`

3 bundle (蓝图 §1.3 A' 快速 bundle 无角色名, byte-identical; CAPABILITY-DOT 后 dot-notation):

- `workspace` (工作能力) → channel.write + artifact.write + artifact.commit (3)
- `reader` (阅读能力) → channel.read + artifact.read + dm.read (3)
- `mention` (提及能力) → user.mention + dm.send (2)

BundleSelector requires an explicit user confirmation: bundle click → expand capability checkboxes (default-all-checked but uncheckable) → confirm → caller sends N AP-1 PUT `/api/v1/permissions` requests (reuse existing endpoint; do not add POST `/api/v1/bundles`).

DOM 出处: `data-ap2-bundle-selector` / `data-ap2-bundle-row` / `data-bundle-name` / `data-ap2-bundle-checkbox` / `data-ap2-bundle-confirm`.

## 5. tests

- `__tests__/ap-2-capabilities.test.ts` 5 vitest
- `__tests__/PermissionsView.test.tsx` 5 vitest
- `__tests__/capability-bundles.test.ts` 5 vitest (跨层锁定 + assertBundlesValid + helpers)
- `__tests__/BundleSelector.test.tsx` 4 vitest (expand + 主权 uncheck + 必显式 confirm + DOM 出处)
- `__tests__/ap-2-reverse-grep.test.ts` 11 vitest (14 const + 反 RBAC 英 4 / 中 3 + admin 独立 + source-of-truth check + PascalCase bundle 名 + role in bundle const + POST /api/v1/bundles + BundleHasCapability/HasBundle 0 hit)
- `packages/e2e/tests/agent-permission-bundle.spec.ts` Playwright 4 case (capability response shape + no bundle endpoint drift + actual UI render with 8 RBAC terms 0 hit in body + admin-only UI kept on a separate path) + screenshot `docs/qa/screenshots/ap-2-bundle-ui.png`
