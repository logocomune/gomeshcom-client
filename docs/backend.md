# Go Backend

`gomeshcomd` is the Go service for MeshCom UDP traffic and browser APIs.

## Configuration

Configuration uses `github.com/ardanlabs/conf/v3` with the `GOMESHCOM` prefix. Values can be supplied by environment variables or command-line flags.

| Field                      | Environment                             | Flag                            | Default              |
| -------------------------- | --------------------------------------- | ------------------------------- | -------------------- |
| HTTP address               | `GOMESHCOM_HTTP_ADDR`                   | `--http-addr`                   | `127.0.0.1:8080`     |
| UDP listen address         | `GOMESHCOM_UDP_LISTEN_ADDR`             | `--udp-listen-addr`             | `0.0.0.0:1799`       |
| Node UDP address           | `GOMESHCOM_NODE_ADDR`                   | `--node-addr`                   | _(empty)_            |
| Local callsign             | `GOMESHCOM_MY_CALL`                     | `--my-call`                     | `QQ0XX-1`            |
| Data directory             | `GOMESHCOM_DATA_DIR`                    | `--data-dir`                    | `./data`             |
| Max message length         | `GOMESHCOM_MAX_MESSAGE_LENGTH`          | `--max-message-length`          | `149`                |
| Send dedup TTL             | `GOMESHCOM_SEND_DEDUP_TTL`              | `--send-dedup-ttl`              | `2s`                 |
| Send delay                 | `GOMESHCOM_SEND_DELAY`                  | `--send-delay`                  | `40s`                |
| Disable TX (dry-run)       | `GOMESHCOM_SEND_DISABLE_TX`             | `--send-disable-tx`             | `false`              |
| UDP forward targets        | `GOMESHCOM_FORWARD_TARGETS`             | `--forward-targets`             | _(empty)_            |
| Auth username              | `GOMESHCOM_AUTH_USERNAME`               | `--auth-username`               | _(empty)_            |
| Auth password              | `GOMESHCOM_AUTH_PASSWORD`               | `--auth-password`               | _(empty)_            |
| Auth session TTL           | `GOMESHCOM_AUTH_SESSION_TTL`            | `--auth-session-ttl`            | `24h`                |
| Auth cookie name           | `GOMESHCOM_AUTH_COOKIE_NAME`            | `--auth-cookie-name`            | `meshcom_session`    |
| Receive log enabled        | `GOMESHCOM_RECEIVE_LOG_ENABLED`         | `--receive-log-enabled`         | `true`               |
| Receive log path           | `GOMESHCOM_RECEIVE_LOG_PATH`            | `--receive-log-path`            | `./data/raw`         |
| Receive log retention days | `GOMESHCOM_RECEIVE_LOG_RETENTION_DAYS`  | `--receive-log-retention-days`  | `365`                |
| Receive log replay window  | `GOMESHCOM_RECEIVE_LOG_REPLAY_WINDOW`   | `--receive-log-replay-window`   | `1h`                 |
| Chat log path              | `GOMESHCOM_CHAT_LOG_PATH`               | `--chat-log-path`               | `./data/chat`        |
| Chat history window        | `GOMESHCOM_CHAT_LOG_HISTORY_WINDOW`     | `--chat-log-history-window`     | `24h`                |
| Chat max history window    | `GOMESHCOM_CHAT_LOG_MAX_HISTORY_WINDOW` | `--chat-log-max-history-window` | `720h`               |
| HTTP request log           | `GOMESHCOM_REQUEST_LOG_ENABLED`         | `--request-log-enabled`         | `false`              |
| Log level                  | `GOMESHCOM_LOG_LEVEL`                   | `--log-level`                   | `info`               |

Startup banner shows `NODE     autodetect` in the header row.

Show generated usage:

```sh
go run ./cmd/gomeshcomd --help
```

Enable UDP receive debug logs:

```sh
GOMESHCOM_LOG_LEVEL=debug go run ./cmd/gomeshcomd
```

## Current API

Swagger/OpenAPI contract: [`docs/openapi.yaml`](openapi.yaml).

All `/api/*` responses send `Cache-Control: no-store, no-cache, must-revalidate, max-age=0`, `Pragma: no-cache`, and `Expires: 0` so browsers and proxies do not reuse stale API data. Static assets under `/_app/immutable/` send `Cache-Control: public, max-age=31536000, immutable`; `index.html` sends `Cache-Control: no-cache, must-revalidate` so deploys can publish new asset URLs quickly.

- `GET /api/health`
- `GET /api/positions`
- `POST /api/messages`
- `GET /api/events`
- `GET /api/channel-show`
- `PUT /api/channel-show`
- `GET /api/chat/list`
- `GET /api/chat/{conversation}?hours=N`
- `DELETE /api/chat/{conversation}`
- `POST /api/chat/{conversation}/read`


`GET /api/channel-show` returns the frontend channel visibility preference:

```json
{
  "mode": "allowlist",
  "channels": ["*", "222", "22201"]
}
```

`mode:"all"` shows every channel. `mode:"allowlist"` shows only `channels`, where `*` means Broadcast and numeric strings are public channels. `PUT /api/channel-show` replaces the preference and also accepts a raw array shorthand such as `["*", "222"]`, treated as `mode:"allowlist"`. The backend does not drop, filter, or hide packets based on this setting; receive logs, chat status, replay, and live SSE delivery remain complete. The normalized preference is kept in memory, persisted atomically to `data/channel_show.json` on the next one-minute save tick, and flushed on shutdown.

Chat history defaults are conversation-aware. `P_broadcast` and `P_<channel>` use `GOMESHCOM_CHAT_LOG_HISTORY_WINDOW` (default `24h`). `DM_<CALLSIGN>` uses a 30-day default window. The optional `hours` query parameter overrides both defaults and is still capped by `GOMESHCOM_CHAT_LOG_MAX_HISTORY_WINDOW` (default `720h`, 30 days).
The web UI stores the last selected chat conversation in `localStorage` and restores it after restart. If the stored conversation is no longer returned by `/api/chat/list`, the UI falls back to Broadcast.

`DELETE /api/chat/{conversation}` deletes the chat history file for the specified conversation and removes its entry from the chat status index. Returns `204` on success.

`POST /api/chat/{conversation}/read` marks a conversation as read (zeroes its unread counter and updates `lastRead` in the chat status index). Returns `204` on success.

`POST /api/messages` enforces an 8 KB body limit. It accepts:

```json
{
  "dst": "*",
  "msg": "hello"
}
```

`POST /api/messages` validates the request, suppresses immediate duplicates within `GOMESHCOM_SEND_DEDUP_TTL` (default `2s`), and transmits the text to the MeshCom node via UDP. It returns `202` on accept, `429` for duplicate suppression (with `Retry-After: 2`), `502` on UDP send failure, and `503` when no bridge is configured.

After a message is accepted and written to UDP, the backend tracks it for 5 seconds. If the same message is not observed returning from the node as a UDP `msg` packet during that window, the backend writes a persistent chat record with `direction:"outbound"` and `delivery_status:"failed"`, then emits a `message.failed` SSE event. Node echo matching accepts the plain text, a numeric node sequence suffix such as `{571}`, and truncated numeric suffixes such as `{571`. The web UI renders failed status as a red `X`; public and channel sends show a green cloud once the local node echo is observed.

When `GOMESHCOM_SEND_DISABLE_TX=true`, `/api/events` includes `txDisabled:true` in the initial `station.identity` event. The web UI shows the dry-run banner and disables the send button, while `/api/messages` still accepts requests and logs the payload instead of sending UDP traffic.

When `GOMESHCOM_REQUEST_LOG_ENABLED=true`, every HTTP request emits one structured `slog` record after the response completes. The record includes method, endpoint path, status code, caller IP, RFC3339 timestamp, and duration. Caller IP prefers `CF-Connecting-IP`, then `X-Forwarded-For`, then `X-Real-IP`, then the socket remote address.

The web map treats `rssi`/`snr` as direct-node metadata. Direct updates preserve previously known signal values when a live packet omits them; indirect updates still refresh `lastSeen` without replacing signal fields on the origin node.
For `pos` packets with a relay chain, the live frontend refreshes `lastSeen` on every hop, keeps `rssi`/`snr` only on the last hop, and leaves origin/intermediate relays without signal fields.
Map marker hover tooltips show station freshness plus `firstSeen` and `lastSeen`; marker clicks do not open a detail card. The local `MyCall` marker hover shows only callsign and device name.
The map toolbar includes a ruler toggle (default off) that draws green lines from `MyCall` to currently direct-heard stations and prints per-line distance labels in kilometers.
The map toolbar also includes a realtime DM tracking toggle that draws dashed hop-by-hop routes for live direct messages (`src -> via -> dst`) and removes each trace automatically after 45 seconds.
Realtime DM route tracking includes ACK/reject `msg` packets as long as they carry a non-channel destination callsign.

`GET /api/positions` returns the persisted node position map loaded at startup and updated from incoming `pos` packets:

```json
{
  "QQ1ABC-1": {
    "lat": 48.1,
    "lng": 16.3,
    "alt": 123,
    "firstseen": "2026-05-15T10:00:00Z",
    "lastseen": "2026-05-15T10:05:00Z",
    "lastdirectseen": "2026-05-15T10:05:00Z",
    "rssi": -90,
    "snr": 8,
    "via": ["QQ5AKT-10", "QQ5PFI-1"]
  }
}
```

`GOMESHCOM_MY_CALL` and `--my-call` accept `IU5PMP` or a `QQ`-prefixed callsign with optional numeric SSID, for example `IU5PMP`, `IU5PMP-1`, `QQ1ABC`, or `QQ1ABC-7`. Lowercase input is normalized to uppercase at startup, and non-`IU5PMP` prefixes are rewritten to `QQ`.

`GET /api/events` emits the configured station identity once as `station.identity` immediately after the initial heartbeat. It also emits the position map once as a `positions.snapshot` SSE event, chat unread state as `chatstatus.snapshot`, and frontend channel visibility as `channelshow.snapshot` before replayed or live packet events. A `heartbeat` event is sent every 15 seconds to keep the connection alive.

`chatstatus.snapshot` data is a map keyed by conversation ID. Each entry contains:

```json
{
  "P_broadcast": {
    "lastMsgReceived": "2026-05-26T10:00:00Z",
    "lastRead": "2026-05-26T09:55:00Z",
    "unreadCount": 3,
    "lastMsg": "hello"
  }
}
```

The chat status index is persisted atomically to `data/chat/msg_idx.json` on the next one-minute save tick and flushed on shutdown. Incoming messages from stations other than `GOMESHCOM_MY_CALL` increment the unread counter; `POST /api/chat/{conversation}/read` resets it.

When authentication is enabled (`GOMESHCOM_AUTH_USERNAME` and `GOMESHCOM_AUTH_PASSWORD` both set), the server requires a session cookie obtained via `POST /api/session`. Sessions expire after `GOMESHCOM_AUTH_SESSION_TTL` (default `24h`) and the cookie name is controlled by `GOMESHCOM_AUTH_COOKIE_NAME` (default `meshcom_session`). Expired sessions are automatically evicted every 5 minutes. The login endpoint enforces a 1 KB body limit.

`GET /api/session` returns `{"required": bool, "authenticated": bool}` — always accessible without a session, returning `401` when auth is required but the caller is not authenticated. `DELETE /api/session` invalidates the current session and clears the cookie; always returns `204`.

`GOMESHCOM_FORWARD_TARGETS` accepts a comma-separated list of `host:port` UDP addresses. Every received datagram is mirrored unmodified to each target immediately after local processing. Duplicate addresses are silently deduplicated.

The UDP receive JSONL log is enabled by default and keeps daily raw packet logs for `GOMESHCOM_RECEIVE_LOG_RETENTION_DAYS` (default `365`). On each SSE connection, the server replays valid `packet.received` events from the last `GOMESHCOM_RECEIVE_LOG_REPLAY_WINDOW` (default `1h`) so the UI can repopulate recent messages after reconnecting.

## Build Pipeline

`./build.sh` and the GoReleaser pre-release hook build the SvelteKit frontend with `npm install` followed by `npm run build`.
`npm ci` is intentionally avoided here because the frontend lockfile can drift from transient dependency metadata during release builds.

## Data Directory

Runtime files live under `data/` by default:

```text
data/
  channel_show.json
  nodes/
    positions.json
  raw/
    received.20260516.jsonl
  chat/
    msg_idx.json
    P_broadcast.jsonl
    DM_QQ1ABC-1.jsonl
```

Only `.gitkeep` placeholders are tracked. Runtime content is ignored by Git.
On startup, `gomeshcomd` creates `data/raw`, `data/nodes`, and `data/chat` unconditionally.

## Receive Log

JSONL logging is enabled by default for every received UDP datagram:

```sh
go run ./cmd/gomeshcomd
```

Default file:

```text
data/raw/received.YYYYMMDD.jsonl
```

Each line contains:

```json
{
  "received_at": "2026-05-14T19:00:00Z",
  "remote_addr": "192.168.0.2:1799",
  "bytes": 36,
  "raw": "{\"type\":\"msg\",\"dst\":\"*\",\"msg\":\"hello\"}",
  "packet_type": "msg"
}
```

Incoming datagrams are appended to one file per UTC day, for example `data/raw/received.20260516.jsonl`. `ReadSince` scans the daily files from the cutoff date through today. Daily files outside `GOMESHCOM_RECEIVE_LOG_RETENTION_DAYS` are pruned on append.

## Chat Log

Incoming `msg` packets are appended to per-conversation JSONL files under `GOMESHCOM_CHAT_LOG_PATH` (default `./data/chat`). DM messages are filtered: only messages where `src` or `dst` matches `GOMESHCOM_MY_CALL` are stored.

Outbound messages that do not echo back from the local node within the send tracking window are stored in the same per-conversation files with `delivery_status:"failed"`. The `src` field is the callsign configured when the send happened, so failed history remains stable even if `GOMESHCOM_MY_CALL` changes later.

Conversation file naming:

| Destination       | File                  |
| ----------------- | --------------------- |
| `*` or empty      | `P_broadcast.jsonl`   |
| Numeric (channel) | `P_<number>.jsonl`    |
| Callsign (DM)     | `DM_<CALLSIGN>.jsonl` |

## Packet Parsing

`internal/meshcom` parses incoming JSON datagrams into typed packets:

- `msg`
- `pos`
- `tele`

Unknown fields are preserved in the parsed envelope for future firmware changes and hardware captures.
