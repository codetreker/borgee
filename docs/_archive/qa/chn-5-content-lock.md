# CHN-5 content lock — archive UI 文案 + DOM (战马D v0)

> 战马D · 2026-04-30 · client SPA `已归档` badge / ArchivedChannelsPanel
> 折叠区 / 恢复 button + system DM 互补二式 文案 byte-identical 锁.
> **关联**: spec `chn-5-spec.md` + stance `chn-5-stance-checklist.md` +
> acceptance `acceptance-templates/chn-5.md`.
> **承袭锚**: CHN-1.3 #288 SortableChannelItem `已归档` badge + CHN-1.2
> #265 archive system DM 文案; CHN-5 仅追加 unarchive 互补二式.

## §1 system DM 文案 (互补二式 byte-identical)

| action | body 字面 (server side fanout, RFC3339 ts) |
|---|---|
| archive | `channel #{name} 已被 {owner} 关闭于 {ts}` (CHN-1.2 #265 既有 byte-identical 不动) |
| unarchive | `channel #{name} 已被 {owner} 恢复于 {ts}` (CHN-5 加, 跟 archive 互补字面: 关闭 ↔ 恢复) |

**反约束** (跟 CHN-1.2 既有 fanoutArchiveSystemMessage 同模式):
- ts 走 `time.UnixMilli(ts).UTC().Format(time.RFC3339)` (跟 archive 同源).
- owner = users.DisplayName (具体名), fallback `system` if 空; 不接受 raw UUID.
- channel name 含 `#` 前缀字面 (`channel #foo 已被...`).
- 反向同义词 reject: archive 不使用 `归档于 / 存档于 / 封存于`; unarchive 不使用 `还原于 / 解档于 / 重启于`.

## §2 client ArchivedChannelsPanel DOM (byte-identical)

```tsx
<details className="archived-panel" data-testid="archived-channels-panel">
  <summary className="archived-panel-summary">已归档频道</summary>
  <ul className="archived-channel-list">
    {channels.map(ch => (
      <li key={ch.id} className="archived-channel-item" data-archived="true">
        <span className="channel-name">#{ch.name}</span>
        <span className="archived-badge" title="已归档">已归档</span>
        <button
          className="btn btn-sm btn-restore"
          data-action="restore"
          onClick={() => handleRestore(ch.id)}
        >恢复</button>
      </li>
    ))}
  </ul>
</details>
```

**字面锁** (vitest 反向 grep 守):
- summary 字面 `已归档频道` byte-identical
- badge 字面 `已归档` byte-identical 跟 CHN-1.3 SortableChannelItem 同源
- button 字面 `恢复` byte-identical (跟 ChannelMembersModal `恢复频道` 二字 prefix 不同 — 一字 `恢复` 是 panel 行级紧凑, 二字 `恢复频道` 是 modal 全局 — 立场: 二式同源不漂, 字面分开守)
- data-action="restore" 反向 reject 同义词 (`unarchive` / `restore-channel` / `un-archive`)

## §3 toast 文案 byte-identical

| 触发 | toast 文案 (showToast) |
|---|---|
| 恢复成功 | `频道已恢复` (跟 ChannelMembersModal handleArchive 既有 byte-identical) |
| 恢复失败 | `恢复失败` (互补; archive 既有 `归档失败` 不动) |

## §4 反约束 — 同义词 reject

- archive button 字面 `归档` (1 字 panel) / `归档频道` (2 字 modal); 反向 reject `存档 / 封存 / 退役 / archive`.
- 恢复 button 字面 `恢复` (1 字 panel) / `恢复频道` (2 字 modal); 反向 reject `还原 / 解档 / 重启 / unarchive / restore`.
- vitest 反向 grep 跟 CHN-1.3 #288 同义词反向同模式.
