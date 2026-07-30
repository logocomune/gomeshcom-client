# Backend and Configuration

`gomeshcomd` receives MeshCom ExtUDP-compatible traffic over UDP or serial, stores runtime data in SQLite, and serves the browser UI and HTTP API.

## Configuration

Configuration precedence, from lowest to highest, is:

1. built-in defaults;
2. `data/configs/gomeshcomd.toml`;
3. `GOMESHCOM_*` environment variables;
4. command-line flags.

On first start, the daemon writes a commented TOML template. Set `GOMESHCOM_DATA_DIR` before startup to place that file and all runtime data elsewhere. Environment values and flags use Go duration syntax (`720h`); TOML also accepts day values such as `30d`.

Use `--help` to list every available flag:

```sh
./gomeshcomd --help
```

| Setting | Default | Purpose |
|---|---:|---|
| `GOMESHCOM_TRANSPORT_MODE` | `udp` | Node transport; `udp` or `serial` |
| `GOMESHCOM_HTTP_ADDR` | `127.0.0.1:8080` | HTTP listen address |
| `GOMESHCOM_UDP_LISTEN_ADDR` | `0.0.0.0:1799` | ExtUDP listen address |
| `GOMESHCOM_NODE_ADDR` | empty | Fixed node address; empty enables source auto-detection |
| `GOMESHCOM_MY_CALL` | `QQ0XX-1` | Startup callsign default |
| `GOMESHCOM_DATA_DIR` | `./data` | Runtime data directory |
| `GOMESHCOM_DEMO_MODE` | `false` | Disables transmit and configuration writes |
| `GOMESHCOM_FORWARD_TARGETS` | empty | Comma-separated UDP forwarding targets |
| `GOMESHCOM_AUTH_USERNAME` / `GOMESHCOM_AUTH_PASSWORD` | empty | Enables web authentication when both values are set |
| `GOMESHCOM_STORAGE_SQLITE_PATH` | `./data/gomeshcom.db` | SQLite database path |
| `GOMESHCOM_STORAGE_PURGE_INTERVAL` | `4h` | SQLite maintenance interval |
| `GOMESHCOM_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error` |

Serial mode requires firmware 4.35+ and an explicit
`GOMESHCOM_SERIAL_DEVICE`. Default framing is 115200 8N1 with no flow control.
See [Serial Transport](serial.md) for DTR/RTS, reconnect, terminal, platform,
and Docker details.

The configuration API exposes effective values, environment overrides, and restart requirements through `GET /api/config`. It accepts writable changes through `PUT /api/config`. Environment-managed values remain read-only.

## Runtime Data

SQLite is the runtime store. It uses WAL mode, foreign keys, one pooled connection, and a five-second busy timeout. It stores received packets, chat history and read markers, node positions, hourly statistics, telemetry, sessions, channel visibility, station identity, and DM statistics.

At first database creation, compatible JSON and JSONL files are imported once. They remain migration input only; live state is no longer written to them. SQLite maintenance removes expired receive-log, public-chat, node, and telemetry rows according to the `GOMESHCOM_STORAGE_*_RETENTION` settings.

## Callsign

`GOMESHCOM_MY_CALL` sets the startup default. A persisted station identity takes precedence after the first runtime change. Update the active callsign without restart with:

```text
PUT /api/adm/configs/my-call
```

The update is validated, applied to long-lived services, persisted in SQLite, and announced through the `station.identity` SSE event.

## HTTP API and Events

The machine-readable contract is [OpenAPI](openapi.yaml). Common endpoints include:

- `GET /api/health`
- `GET /api/config`, `PUT /api/config`
- `GET /api/positions`
- `POST /api/messages`
- `GET /api/chat/list`, `GET /api/chat/{conversation}`, `POST /api/chat/{conversation}/read`
- `GET /api/stats`, `GET /api/stats/dm`
- `GET /api/events`

`/api/events` is a Server-Sent Events stream. Initial events include station identity, position data, chat status, and channel visibility, followed by received packet events and a heartbeat every 15 seconds. API responses are not cacheable; immutable frontend assets receive long-lived cache headers. Gzip compression applies to eligible API and asset responses, never to SSE.

When authentication is enabled, log in through `POST /api/session`; successful sessions are stored in SQLite and use an HTTP-only cookie. `GET /api/session` reports authentication status and `DELETE /api/session` logs out.

## Sending and Forwarding

`POST /api/messages` validates messages, limits outgoing text to `GOMESHCOM_MAX_MESSAGE_LENGTH` UTF-8 characters, and suppresses immediate duplicates for `GOMESHCOM_SEND_DEDUP_TTL` (default `2s`). In demo mode it does not transmit.

Each received UDP datagram can be mirrored byte-for-byte to the comma-separated targets in `GOMESHCOM_FORWARD_TARGETS`. In serial mode, each extracted ExtUDP JSON object is forwarded instead. Duplicate targets are ignored.
