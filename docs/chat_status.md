# Chat Status Tracking

## Overview

The backend tracks per-conversation read/unread state for every chat thread (broadcast channels, numeric channels, DMs). This allows clients to display unread badges and "last read" indicators without maintaining their own counters.

## Data Model

Each conversation thread is identified by its **conversation ID** — the same identifier used throughout the REST API:

| Pattern | Example | Description |
|---------|---------|-------------|
| `P_broadcast` | `P_broadcast` | Broadcast channel (`dst = *`) |
| `P_<digits>` | `P_2` | Numeric channel |
| `DM_<CALLSIGN>` | `DM_QQ1ABC-7` | Direct message thread |

Each thread has three fields:

| Field | Type | Description |
|-------|------|-------------|
| `lastMsgReceived` | RFC3339 timestamp | Timestamp of the most recent inbound message (self-echoes excluded). Zero if no message ever received. |
| `lastRead` | RFC3339 timestamp | Timestamp of the last `POST /api/chat/{id}/read` call. Zero if never marked read. |
| `unreadCount` | integer ≥ 0 | Inbound messages since the last `MarkRead`. Zeroed by `POST /api/chat/{id}/read`. |
| `lastMsg` | string (optional) | Raw text of the most recent inbound message. Omitted when no message has been received. |

## Persistence

State is serialised to `<ChatLog.Path>/msg_idx.json` (default: `./data/chat/msg_idx.json`).

- Written **atomically** (write to `.tmp` + `os.Rename`) every minute when there are pending changes.
- A final flush happens on graceful shutdown (SIGTERM / SIGINT).
- Loaded at startup; missing file is treated as empty state.
- Leftover `.tmp` files from previous crashes are deleted on load.

### Example `msg_idx.json`

```json
{
  "P_broadcast": {
    "lastMsgReceived": "2026-05-24T10:00:00Z",
    "lastRead": "2026-05-24T09:55:00Z",
    "unreadCount": 3,
    "lastMsg": "hello everyone"
  },
  "DM_QQ1ABC-7": {
    "lastMsgReceived": "2026-05-24T08:30:00Z",
    "lastRead": "2026-05-24T08:30:00Z",
    "unreadCount": 0
  }
}
```

## API

### Mark conversation as read

```
POST /api/chat/{conversation}/read
```

- **Auth**: required when authentication is enabled (session cookie).
- **Path param** `conversation`: must match `P_broadcast`, `P_<digits>`, or `DM_<CALLSIGN>`.
- **Effect**: sets `unreadCount = 0` and `lastRead = now()` for the conversation.
- **Returns**: `204 No Content` on success; `400` for invalid ID; `401` if unauthenticated.
- **Idempotent**: safe to call multiple times.

```sh
curl -X POST http://localhost:8080/api/chat/P_broadcast/read
```

## SSE Event: `chatstatus.snapshot`

On every new `/api/events` SSE connection the server injects a `chatstatus.snapshot` event **after** `positions.snapshot` and **before** `channelshow.snapshot` and the replay window. The payload is the full status map at that instant.

```
event: chatstatus.snapshot
data: {"P_broadcast":{"lastMsgReceived":"2026-05-24T10:00:00Z","lastRead":"2026-05-24T09:55:00Z","unreadCount":3}}
```

The snapshot is a point-in-time copy. Subsequent inbound messages increment `unreadCount` in memory but do not push live updates over SSE — clients can derive increments from the existing `packet.received` events. The updated state is available in the next `chatstatus.snapshot` on reconnect, or can be fetched by issuing a new SSE connection.

## Conversation Deletion

When `DELETE /api/chat/{conversation}` is called:

1. The `.jsonl` history file is removed from disk (existing behaviour).
2. The corresponding entry is removed from the in-memory status store.
3. `msg_idx.json` is immediately persisted so the deletion survives a restart.

This ensures that a deleted conversation does not reappear with stale unread counts after the backend restarts.

## Implementation Notes

- **Self-echo exclusion**: when a message is received with `Source == MyCall` (the node echoing our own outgoing message), `unreadCount` is **not** incremented.
- **Via-path**: the origin call is extracted as `strings.SplitN(Source, ",", 2)[0]` before comparison with `MyCall`, so relayed self-echoes are also excluded.
- **Concurrency**: the in-memory store is protected by `sync.Mutex`; all reads and writes are safe for concurrent access.
