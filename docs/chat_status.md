# Chat Status Tracking

The backend stores read markers in SQLite and derives unread counts and latest-message previews from chat history. Clients receive a `chatstatus.snapshot` event on every new Server-Sent Events connection.

## Conversation IDs

| Conversation | Example | Meaning |
|---|---|---|
| `P_broadcast` | `P_broadcast` | Broadcast (`dst = *`) |
| `P_<digits>` | `P_2` | Numeric public channel |
| `DM_<mycall>_<peer>` | `DM_QQ1ABC-1_QQ2DEF-3` | Direct-message conversation in `mycall` scope |
| `DM_<basecall>_<peer>` | `DM_QQ1ABC_QQ2DEF-3` | Direct-message conversation in `basecall` scope |

Direct-message API requests use `?scope=mycall` by default. `?scope=basecall` aggregates messages for all numeric SSIDs of the active base callsign.

## API

```text
POST /api/chat/{conversation}/read
```

The endpoint updates the read marker and returns `204 No Content`. With `scope=basecall`, it marks every matching SSID-specific direct-message conversation as read. Authentication is required when web authentication is enabled.

`DELETE /api/chat/{conversation}` removes the associated chat history and read state.

## SSE Snapshot

`chatstatus.snapshot` is sent after the initial position snapshot. Entries contain the latest inbound message time, read time, unread count, and an optional latest-message preview:

```json
{
  "P_broadcast": {
    "lastMsgReceived": "2026-05-24T10:00:00Z",
    "lastRead": "2026-05-24T09:55:00Z",
    "unreadCount": 3,
    "lastMsg": "hello everyone"
  }
}
```

Unread counts exclude local echoes. The snapshot is point-in-time data; clients update live state from later packet events and receive a fresh snapshot after reconnecting.
