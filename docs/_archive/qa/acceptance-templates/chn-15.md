# Acceptance Template — CHN-15: channel readonly toggle

> Spec: `docs/implementation/modules/chn-15-spec.md` (战马C v0)
> 立场: `docs/qa/chn-15-stance-checklist.md` (3 + 3 边界)
> Content lock: `docs/qa/chn-15-content-lock.md` (3 文案 + DOM)
> 关联: CHN-3.1 #410 user_channel_layout schema + CHN-7 #550 mute (bit 1) + CHN-1.2 channel.created_by gate + ADM-0 §1.3 admin 红线
> 前置: CHN-7 #550 SetMuteBit Store helper ✅ + CHN-9 #561 visibility 三态 ✅

## 验收清单

### CHN-15.1 server bit 4 + Store helper (0 schema)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 0 schema 改 反向断言 (filepath.Walk migrations/ 反向 0 hit) | grep | 战马C / 烈马 | `internal/api/chn_15_readonly_test.go::TestCHN151_NoSchemaChange` |
| 1.2 ReadonlyBit=16 const + IsReadonly 谓词单源 + bitmap 4 位互不影响 | unit | 战马C / 烈马 | `TestCHN151_ReadonlyBit_ByteIdentical` (ReadonlyBit==16 + IsReadonly truth table) |
| 1.3 Store.GetChannelReadonly + SetChannelReadonly 走 creator 单行 SSOT | unit | 战马C / 烈马 | `internal/store/chn_15_readonly_test.go` |

### CHN-15.2 server endpoints + send gate

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 PUT owner-only ACL (channel.CreatedBy == user.ID; non-creator 403) | unit | 战马C / 烈马 | `TestCHN152_SetReadonly_OwnerOnly_HappyPath` + `_NonOwner_403` |
| 2.2 DELETE idempotent unset | unit | 战马C / 烈马 | `TestCHN152_UnsetReadonly_Idempotent` |
| 2.3 readonly=true 时 non-creator POST /messages → 403 `channel.readonly_no_send` 字面 byte-identical | unit | 战马C / 烈马 | `TestCHN152_SendBlockedForNonCreator_WhenReadonly` |
| 2.4 readonly=true 时 creator 自己仍可 send 200 | unit | 战马C / 烈马 | `TestCHN152_SendAllowedForCreator_WhenReadonly` |
| 2.5 admin-rail 0 endpoint 反向断言 + admin god-mode send 不挂 | grep + unit | 战马C / 烈马 | `TestCHN152_NoAdminReadonlyPath` (filepath.Walk 反向 grep 0 hit) |
| 2.6 错码字面单源 byte-identical | unit | 战马C / 烈马 | `TestCHN152_ErrCode_ByteIdentical` |

### CHN-15.3 client + closure

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 `lib/readonly.ts::READONLY_BIT=16` 双向锁 + setChannelReadonly/unsetChannelReadonly API wrappers | vitest | 战马C | `__tests__/readonly-content-lock.test.ts` |
| 3.2 ReadonlyToggle.tsx data-testid + data-readonly enum + 文案 byte-identical + click toggle | vitest | 战马C | `__tests__/ReadonlyToggle.test.tsx` |
| 3.3 ReadonlyBadge.tsx (`只读` 标签) + 同义词反向 grep | vitest | 战马C / 烈马 | `__tests__/ReadonlyBadge.test.tsx` |
| 3.4 closure: REG-CHN15-001..006 + acceptance + PROGRESS [x] | docs | 战马C / 烈马 | registry + PROGRESS + 4 件套全闭 |

## 不在本轮范围 (spec §4)

- v2 schedule readonly / message edit / archive 联动 / audit log row / agent runtime override

## 退出条件

- CHN-15.1 1.1-1.3 (0 schema + ReadonlyBit byte-identical + Store helper) ✅
- CHN-15.2 2.1-2.6 (PUT/DELETE owner-only + send gate non-creator/creator + admin not mounted + 错码 byte-identical) ✅
- CHN-15.3 3.1-3.4 (READONLY_BIT 双向锁 + ReadonlyToggle/ReadonlyBadge + 同义词反向 + closure) ✅
- 5 反向 grep count==0 + REG-CHN15-001..006 + 4 件套全闭
