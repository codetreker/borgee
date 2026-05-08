# CHN-11 content lock — MemberList + AddMemberModal + KickConfirmModal (战马D v0)

战马D · 2026-04-30 · client SPA 三组件 文案 byte-identical 锁.

## §1 MemberList DOM (byte-identical)

```tsx
<div className="member-list" data-testid="member-list">
  <header className="member-list-header">
    <h3>成员</h3>
    <button
      type="button"
      data-testid="member-list-add"
      onClick={onAdd}
    >
      添加成员
    </button>
  </header>
  <ul className="member-list-rows">
    {members.map((m) => (
      <li
        key={m.user_id}
        className="member-list-row"
        data-member-user-id={m.user_id}
      >
        <span>{m.display_name}</span>
        {canManage && (
          <button
            type="button"
            data-testid={`member-remove-${m.user_id}`}
            onClick={() => onRemove(m)}
          >
            移除
          </button>
        )}
      </li>
    ))}
  </ul>
</div>
```

**字面锁**:
- title `成员` 2 字 byte-identical
- add button `添加成员` 4 字 byte-identical
- remove button `移除` 2 字 byte-identical
- `data-testid="member-list"` byte-identical
- `data-testid="member-list-add"` byte-identical
- `data-testid="member-remove-{userId}"` 行级锚 byte-identical
- `data-member-user-id` 行级锚 byte-identical
- 空 members → return null (整个 list 不渲染)

## §2 AddMemberModal DOM

```tsx
<div className="add-member-modal" data-testid="add-member-modal" role="dialog">
  <h3>添加成员</h3>
  <input
    type="text"
    data-testid="add-member-input"
    placeholder="用户邮箱或 ID"
    value={value}
    onChange={(e) => setValue(e.target.value)}
  />
  <button data-testid="add-member-submit" onClick={handleSubmit}>添加</button>
  <button data-testid="add-member-cancel" onClick={onCancel}>取消</button>
</div>
```

**字面锁**:
- title `添加成员` byte-identical
- placeholder `用户邮箱或 ID` byte-identical
- submit `添加` 1 字 byte-identical
- cancel `取消` 2 字 byte-identical

## §3 KickConfirmModal DOM (byte-identical)

```tsx
<div className="kick-confirm-modal" data-testid="kick-confirm-modal" role="dialog">
  <h3>确认移除 {user.display_name}?</h3>
  <button data-testid="kick-confirm-yes" onClick={handleConfirm}>确认</button>
  <button data-testid="kick-confirm-no" onClick={onCancel}>取消</button>
</div>
```

**字面锁**:
- title `确认移除 {user.display_name}?` byte-identical (user 占位 + ?
  半角 byte-identical)
- confirm `确认` 2 字 byte-identical
- cancel `取消` 2 字 byte-identical

## §4 反约束 — 同义词 reject

MemberList + AddMemberModal + KickConfirmModal 任何 user-visible 文本反向
reject:
- `invite` (English) — 反 reject (data-testid + className 例外)
- `kick` — 反 reject
- `remove` (English) — 反 reject
- `expel` — 反 reject
- `逐出` — 反 reject (我们用 `移除`)
- `踢出` — 反 reject
- `邀请` — 反 reject (我们用 `添加`)

## §5 toast 文案 byte-identical

| 触发 | toast 文案 |
|---|---|
| 添加成功 | (无 toast — modal 关闭即视觉反馈) |
| 添加失败 | `添加成员失败` byte-identical |
| 移除成功 | (无 toast) |
| 移除失败 | `移除成员失败` byte-identical |
