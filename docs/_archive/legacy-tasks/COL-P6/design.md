# P6 Technical Design: Slash Commands

## 1. Overview

Add slash command support to the chat input. Typing `/` opens a command picker panel; users can filter, navigate with keyboard, and execute built-in commands. The architecture uses a command registry pattern to support future agent-defined custom commands (v2).

## 2. Command Registry

```typescript
// src/commands/registry.ts

interface CommandDefinition {
  name: string;                          // e.g. "help", "invite"
  description: string;                   // shown in picker
  usage: string;                         // e.g. "/invite @user"
  paramType: "none" | "user" | "text";   // determines argument parsing
  execute: (ctx: CommandContext) => Promise<void>;
}

interface CommandContext {
  channelId: string;
  currentUserId: string;
  args: string;                          // raw text after command name
  resolvedUser?: { id: string; username: string }; // for user-param commands
  dispatch: AppDispatch;                 // useReducer dispatch
  api: ApiClient;
}

class CommandRegistry {
  private commands: Map<string, CommandDefinition> = new Map();

  register(cmd: CommandDefinition): void {
    this.commands.set(cmd.name, cmd);
  }

  get(name: string): CommandDefinition | undefined {
    return this.commands.get(name);
  }

  search(prefix: string): CommandDefinition[] {
    return [...this.commands.values()].filter(
      (cmd) => cmd.name.startsWith(prefix)
    );
  }

  all(): CommandDefinition[] {
    return [...this.commands.values()];
  }
}

export const commandRegistry = new CommandRegistry();
```

The registry is a singleton instantiated at app startup. Each built-in command calls `commandRegistry.register(...)` from `src/commands/builtins.ts`. In v2, agent-provided command definitions will call the same `register` method.

## 3. Built-in Commands

### 3.1 /help

| Field | Value |
|-------|-------|
| paramType | `none` |
| API | None |
| Behavior | Insert a local system message listing all registered commands |

```typescript
execute: async ({ channelId, dispatch }) => {
  const lines = commandRegistry.all().map(c => `${c.usage} — ${c.description}`);
  dispatch({
    type: "INSERT_LOCAL_SYSTEM_MESSAGE",
    payload: { channelId, text: lines.join("\n") },
  });
};
```

No network call. The message is rendered client-side only and not persisted.

### 3.2 /invite @user

| Field | Value |
|-------|-------|
| paramType | `user` |
| API | `POST /api/v1/channels/:channelId/members` body: `{ userId }` |
| Behavior | Add resolved user to current channel |

```typescript
execute: async ({ channelId, resolvedUser, api }) => {
  if (!resolvedUser) throw new CommandError("Usage: /invite @user");
  await api.post(`/channels/${channelId}/members`, { userId: resolvedUser.id });
};
```

### 3.3 /leave

| Field | Value |
|-------|-------|
| paramType | `none` |
| API | `DELETE /api/v1/channels/:channelId/members/:currentUserId` |
| Behavior | **Show confirmation dialog** ("确定离开 #channel-name？"), then remove self from channel and navigate to channel list |

```typescript
execute: async ({ channelId, currentUserId, api, dispatch }) => {
  // Show confirm dialog (via dispatch or window.confirm for v1)
  const confirmed = window.confirm('确定离开当前频道？');
  if (!confirmed) return;
  await api.delete(`/channels/${channelId}/members/${currentUserId}`);
  dispatch({ type: "NAVIGATE_AFTER_LEAVE" });
};
```

### 3.4 /topic \<text\>

| Field | Value |
|-------|-------|
| paramType | `text` |
| API | `PUT /api/v1/channels/:channelId` body: `{ topic }` |
| Behavior | Set the channel topic |

Requires a new or extended API endpoint (see Section 6).

```typescript
execute: async ({ channelId, args, api }) => {
  if (!args.trim()) throw new CommandError("Usage: /topic <text>");
  await api.put(`/channels/${channelId}`, { topic: args.trim() });
};
```

### 3.5 /dm @user

| Field | Value |
|-------|-------|
| paramType | `user` |
| API | None (reuses existing `openDm` action) |
| Behavior | Open or create a DM conversation with the resolved user |

```typescript
execute: async ({ resolvedUser, dispatch }) => {
  if (!resolvedUser) throw new CommandError("Usage: /dm @user");
  dispatch({ type: "OPEN_DM", payload: { userId: resolvedUser.id } });
};
```

## 4. SlashCommandPicker Component

Reuses the popup + keyboard navigation pattern from `MentionPicker`.

```
src/components/chat/
  SlashCommandPicker.tsx   — popup UI
  useSlashCommands.ts      — hook: trigger detection, filtering, keyboard nav
```

### 4.1 Trigger Logic (in useSlashCommands)

Integrated into `MessageInput.tsx` alongside the existing mention trigger.

```typescript
function useSlashCommands(inputValue: string, cursorPos: number) {
  // Activate when:
  //   1. Input starts with "/"
  //   2. Cursor is within the command token (before any space)
  // Returns: { isActive, filtered, selectedIndex, handlers }
}
```

**Activation rules:**
- `/` at position 0 of the input → open picker
- Characters after `/` filter the command list (e.g. `/in` → show `/invite`)
- Space after command name → close picker, enter argument mode
- Esc → close picker, keep text
- **No match** → show "没有找到命令" empty state in picker

### 4.2 Keyboard Navigation

| Key | Action |
|-----|--------|
| ArrowUp / ArrowDown | Move selection |
| Tab / Enter | Select command, insert into input |
| Esc | Close picker |

Identical behavior to MentionPicker. Both hooks feed into a shared `onKeyDown` handler in MessageInput that delegates to whichever picker is active (slash commands take priority when input starts with `/`).

### 4.3 Argument Phase

After a command is selected:

- **`paramType: "user"`** — Show MentionPicker for `@user` resolution. The existing mention logic handles search and selection. On selection, `resolvedUser` is captured.
- **`paramType: "text"`** — Show an inline placeholder hint (e.g. "Enter channel topic..."). No special picker.
- **`paramType: "none"`** — **Execute immediately on selection** (Tab/Enter in picker). No argument phase needed.

Submission (Enter without picker open) triggers command execution.

### 4.4 Component Tree

```
MessageInput
├── SlashCommandPicker   (visible when / active, no space yet)
├── MentionPicker        (visible when @ active OR slash arg needs user)
└── <textarea>
```

## 5. Command Execution Flow

```
User types "/invite @alice" + Enter
  │
  ├─ MessageInput.handleSubmit()
  │    ├─ Detect leading "/"
  │    ├─ Parse: commandName="invite", rawArgs="@alice"
  │    ├─ Lookup: commandRegistry.get("invite")
  │    ├─ Resolve params based on paramType
  │    │    └─ "user" → resolvedUser from mention selection state
  │    ├─ Build CommandContext
  │    └─ Call command.execute(ctx)
  │         ├─ Success → clear input, optional toast
  │         └─ Error → show inline error message
  └─ (input is NOT sent as a chat message)
```

Key point: lines starting with `/` matching a registered command are **never** sent as regular messages.

## 6. Backend: Channel Topic API

Extend the existing channel update endpoint or add a new one:

```
PUT /api/v1/channels/:id
Body: { topic: string }
Response: 200 { channel }
```

**Changes required:**
- Channel model: ensure `topic` field exists (add migration if missing)
- Channel update route handler: accept and validate `topic` field
- Authorization: only channel members (or admins) can set topic
- WebSocket broadcast: emit `channel:updated` event so other clients update the topic in real-time

## 7. Error Handling

| Scenario | Behavior |
|----------|----------|
| Unknown command (e.g. `/foo`) | Send as regular message (no interception) |
| Missing required param | Inline error: "Usage: /invite @user" |
| API failure (invite, leave, topic) | Inline error with message from server |
| Not a channel member (topic/leave) | Server returns 403 → show error |

Errors are displayed as transient inline messages below the input (same pattern as message send failures).

## 8. State Changes

### AppContext additions

```typescript
// New action types
| { type: "INSERT_LOCAL_SYSTEM_MESSAGE"; payload: { channelId: string; text: string } }
| { type: "NAVIGATE_AFTER_LEAVE" }

// Reducer handles INSERT_LOCAL_SYSTEM_MESSAGE by appending to
// channel message list with { type: "system", persisted: false }
```

No new top-level state. The slash command picker state (open/closed, selection index, resolved args) lives in `useSlashCommands` hook local state.

## 9. Task Breakdown

### Phase 1: Core Infrastructure
1. **Command Registry** — `CommandRegistry` class + singleton export (`src/commands/registry.ts`)
2. **Built-in command definitions** — register all 5 commands (`src/commands/builtins.ts`)
3. **`useSlashCommands` hook** — trigger detection, filtering, keyboard navigation
4. **`SlashCommandPicker` component** — popup UI rendering filtered commands

### Phase 2: MessageInput Integration
5. **Wire SlashCommandPicker into MessageInput** — trigger on `/`, coordinate with MentionPicker
6. **Command execution in handleSubmit** — parse, resolve params, execute, prevent message send
7. **Argument phase for user-param commands** — reuse MentionPicker after command selection
8. **Argument phase for text-param commands** — placeholder hint UX

### Phase 3: Backend + Individual Commands
9. **Channel topic API** — `PUT /channels/:id` with topic field, migration if needed, WebSocket broadcast
10. **Implement /help execute** — `INSERT_LOCAL_SYSTEM_MESSAGE` action + reducer handling
11. **Implement /invite execute** — call existing members API
12. **Implement /leave execute** — call existing delete-member API + navigation
13. **Implement /topic execute** — call new topic API
14. **Implement /dm execute** — call existing `openDm` dispatch

### Phase 4: Polish
15. **Error handling** — inline errors for missing params, API failures, unknown commands pass-through
16. **Tests** — registry unit tests, hook tests, integration tests for each command
