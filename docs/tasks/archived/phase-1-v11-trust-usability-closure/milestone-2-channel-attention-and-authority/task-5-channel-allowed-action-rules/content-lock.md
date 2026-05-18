# Content Lock

Task 5 introduces short action labels and rule reasons inside Settings channel management.

## Locked Action IDs

- `leave`
- `delete`
- `archive`
- `owner-transfer`

These action IDs appear in `data-action` attributes and are the handoff contract for Task 6.

## User-Facing Rule Reasons

- `创建者不能退出自己创建的频道`
- `当前用户未知，不能退出频道`
- `默认频道不能退出`
- `未加入频道不能退出`
- `可退出已加入频道`
- `仅创建者可删除频道`
- `默认频道不能删除`
- `创建者可删除频道`
- `仅创建者可归档频道`
- `默认频道不能归档`
- `创建者可归档频道`
- `本轮不支持所有权转让`

## Reverse Locks

- No Settings action item should be a `button` in this task.
- No Settings channel-management component should import `leaveChannel`, `deleteChannel`, or `archiveChannel` in this task.
