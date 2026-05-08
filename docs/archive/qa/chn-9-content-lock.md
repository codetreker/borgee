# CHN-9 content lock — VisibilityBadge 文案 + DOM (战马D v0)

> 战马D · 2026-04-30 · client SPA VisibilityBadge 三态文案 byte-identical
> 锁. **关联**: spec `chn-9-spec.md` + stance + acceptance.
> **承袭锚**: CHN-7 #550 mute UX (emoji + text 二段) + CHN-8 #552 NotifPref
> 三向锁 + 同义词反向模式.

## §1 VisibilityBadge DOM (byte-identical)

```tsx
const visibilityLabels = {
  creator_only: { emoji: '🔒', text: '仅创建者' },
  private:      { emoji: '👥', text: '成员可见' },
  public:       { emoji: '🌐', text: '组织内可见' },
};

<span
  className="visibility-badge"
  data-visibility={visibility}
  title={`可见性: ${visibilityLabels[visibility].text}`}
>
  {visibilityLabels[visibility].emoji} {visibilityLabels[visibility].text}
</span>
```

**字面锁** (vitest 反向 grep 守):
- `🔒 仅创建者` byte-identical (creator_only)
- `👥 成员可见` byte-identical (private = members)
- `🌐 组织内可见` byte-identical (public = org)
- `data-visibility` ∈ {`creator_only`, `private`, `public`} 三态

## §2 反约束 — 同义词 reject

VisibilityBadge + 任何 visibility 相关 UI 字面反向 reject:
- `secret` (English) — 反 reject
- `exclusive` — 反 reject
- `team-only` — 反 reject
- `external` — 反 reject (跟 org-public 拆死)
- `外部` (Chinese external) — 反 reject
- `外公` (Chinese 外公 ≠ 公开) — 反 reject (字符相似但语义不同)
- `绝密` — 反 reject (跟 creator_only 拆死, 我们用 `仅创建者`)
- `公共` — 反 reject (我们用 `组织内可见`)

## §3 const 三向锁 (server + client + DB byte-identical)

| 端 | 字面 |
|---|---|
| server (Go) | `const VisibilityCreatorOnly = "creator_only"` / `VisibilityMembers = "private"` (alias) / `VisibilityOrgPublic = "public"` (alias) |
| client (TS) | `export const VISIBILITY_CREATOR_ONLY = 'creator_only';` / `VISIBILITY_MEMBERS = 'private';` / `VISIBILITY_ORG_PUBLIC = 'public';` |
| DB | `channels.visibility` TEXT 列字面 ∈ {`'creator_only'`, `'private'`, `'public'`} |

**反约束**:
- 改一处 = 改三处 (server const + client const + DB 字面).
- IsValidVisibility(s) 谓词单源 (server + client byte-identical: s ∈ 三 case).
- existing rows 'public'/'private' 行为 byte-identical 保留 (向后兼容).

## §4 字符串映射 byte-identical (server + client + DB)

| visibility (string) | server const | client const | DB literal | UI 文案 |
|---|---|---|---|---|
| `creator_only` | VisibilityCreatorOnly | VISIBILITY_CREATOR_ONLY | `'creator_only'` | `🔒 仅创建者` |
| `private` | VisibilityMembers | VISIBILITY_MEMBERS | `'private'` (legacy) | `👥 成员可见` |
| `public` | VisibilityOrgPublic | VISIBILITY_ORG_PUBLIC | `'public'` (legacy) | `🌐 组织内可见` |
| 外值 | (反 IsValidVisibility) | (反 isValidVisibility) | (反 PATCH 400) | — |

## §5 toast 文案 byte-identical

| 触发 | toast 文案 |
|---|---|
| 切换成功 | (无 toast — UI badge 视觉反馈) |
| 切换失败 | `可见性切换失败` byte-identical (跟 mute `静音失败` / archive `归档失败` 同模式 — 操作 + 失败 拼接) |

## §6 反 leak 行为锁

creator_only channel 严格 reject 公开列表:
- `GET /api/v1/channels` 非 creator user 0 行匹配
- ListChannelsWithUnread `visibility = 'public'` filter byte-identical
  保留 (creator_only 不命中 org-public preview)
- preview endpoint 走 CHN-1.2 既有 owner-only 立场承袭 (反向 grep
  `creator_only.*public` 0 hit)
