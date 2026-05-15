# Content Lock

Task 6 adds server-authority unavailable reasons to the read-only Settings channel management action list.

## User-Facing Rule Reasons

- `服务器权限不允许删除频道`
- `服务器权限不允许归档频道`

Task 5 locked the other action IDs and rule reasons. Task 6 only adds permission-state reasons for delete/archive when ownership exists but the current server permission state does not allow the action.

## Reverse Locks

- Settings channel management remains read-only and must not render mutation buttons for `leave`, `delete`, `archive`, or `owner-transfer`.
- Owner transfer remains unavailable.
