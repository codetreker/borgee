# RT-4 content lock — ChannelPresenceList (战马D v0)

战马D · 2026-04-30 · client SPA ChannelPresenceList.tsx 文案 byte-identical 锁.

## §1 ChannelPresenceList DOM (byte-identical)

```tsx
{onlineUserIds.length > 0 && (
  <div
    className="channel-presence-list"
    data-testid="channel-presence-list"
  >
    <span className="channel-presence-count">
      当前在线 {onlineUserIds.length} 人
    </span>
    <ul className="channel-presence-avatars">
      {onlineUserIds.slice(0, 5).map((id) => (
        <li
          key={id}
          className="channel-presence-avatar"
          data-presence-user-id={id}
        >
          <span className="channel-presence-dot" aria-hidden="true">●</span>
        </li>
      ))}
      {onlineUserIds.length > 5 && (
        <li
          className="channel-presence-overflow"
          data-testid="channel-presence-overflow"
        >
          +{onlineUserIds.length - 5}
        </li>
      )}
    </ul>
  </div>
)}
```

**字面锁**:
- `当前在线 N 人` byte-identical (N 动态)
- `+N` overflow byte-identical (N = length - 5)
- `data-testid="channel-presence-list"` byte-identical
- `data-testid="channel-presence-overflow"` byte-identical
- `data-presence-user-id` 行级锚 byte-identical
- 空 onlineUserIds → 整个 list 不渲染 (return null)
- ≤5 显示 / >5 显示前 5 + overflow chip

## §2 反约束 — 同义词 reject (user-visible Chinese only)

- `在线状态` — 反 reject (跟 user-visible 拆死, 我们用 `当前在线`)
- `上线` — 反 reject
- `在线人员` — 反 reject
- `在线列表` — 反 reject

(English `presence/online/typing/composing` 在 className/data-testid 例外
— 反向 grep 跟 user-visible Chinese 拆死.)

## §3 const 单源

- PRESENCE_AVATAR_LIMIT = 5 (client only — overflow threshold).

## §4 既有 RT-2 typing 路径不动

TypingIndicator.tsx 既有 (5s timeout 自动消失) byte-identical 不动 — RT-4
**不**改 TypingIndicator 文案 / DOM / 行为 (反向 grep `rt_4` / `rt4` 在
TypingIndicator.tsx 0 hit).
