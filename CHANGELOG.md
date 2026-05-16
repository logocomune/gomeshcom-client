# Changelog

All notable changes to this project are documented in this file.

---

## [0.4.0] - 2026-05-16

### Added

- **Map — node clustering**: stations closer than 30 px are grouped into a bubble showing the count; individual markers still visible for groups of 3 or fewer. Toggle button in the map controls; state persists across reloads.
- **Map — own callsign marker**: the local station (`MyCall`) is displayed as a red marker rendered above all others.
- **Map — label toggle**: button to show or hide callsign labels on markers; state persists across reloads.
- **Optional HTTP auth**: when `GOMESHCOM_AUTH_USERNAME` and `GOMESHCOM_AUTH_PASSWORD` are set, protected API and SSE endpoints require login and the web UI presents a sign-in modal. Successful login creates an HTTP-only session cookie.
- **NodeAddr auto-detection**: when `GOMESHCOM_NODE_ADDR` is not configured, the node address is inferred from the source of the first valid incoming UDP packet. Explicit configuration always takes priority and is never overridden.
- `POST /api/messages` returns `503 Service Unavailable` with `{"error":"node not yet detected"}` when no node address is configured and no UDP traffic has been received yet.

### Changed

- `GOMESHCOM_NODE_ADDR` defaults to empty — auto-detection is now the default behaviour.
- Maidenhead grid overlay defaults to off.
- Map: day/night zone overlay removed.

### Fixed

- DM conversations are now keyed on the interlocutor's callsign so both the sent and received sides of a thread appear as a single conversation. Previously, incoming messages (`dst=MyCall`) and outgoing messages (`src=MyCall`) landed in separate entries, one of which was labelled with the local callsign.
- The chat sidebar no longer creates DM entries for conversations between other stations that do not involve the local callsign.
- Duplicate chat records sharing the same message ID are suppressed both at read time (backend) and on live SSE updates (frontend).

---

## [0.3.0] - 2025-05-01

### Added

- **Web dashboard**: real-time map of heard stations with freshness colour coding — green (direct, ≤30 min), blue (relayed or direct >30 min, ≤1 h), gray (1–48 h); nodes silent for more than 48 h are hidden.
- **Map tooltips**: callsign, relative age, RSSI, SNR, altitude, battery, coordinates, and hardware device name (e.g. "T-Beam", "Heltec V3") when available.
- **Map controls**: Maidenhead grid overlay, marker label toggle, clustering toggle; all states persist in `localStorage`.
- **Chat panel**: broadcast channel, group channels, and direct messages. Per-conversation history loaded from disk on switch. Unread indicators (green dot + bold label) cleared on visit; read timestamps persisted in `localStorage`.
- **Message send**: send to broadcast, a channel, or a callsign with a loading indicator. Inline error banner on failure; duplicate-message notice on `429`.
- **ACK indicators**: LoRa acknowledgement (`✓✓`) and gateway acknowledgement (`☁️`) shown on outgoing messages, including group channel fan-out.
- **Delete / clear**: trash icon in the chat header deletes a channel or DM conversation (`DELETE /api/chat/{id}`); for broadcast it clears messages while keeping the entry. Modal confirmation prevents accidental deletes.
- **Persistent chat logs**: per-conversation JSONL files under `data/chat/` (`P_broadcast.jsonl`, `P_<channel>.jsonl`, `DM_<callsign>.jsonl`). Configurable history window (default 24 h, max 720 h).
- **Position store**: incoming `pos` packets are persisted to `data/nodes/positions.json` with relay-path (`via`) tracking. Freshness attribution propagated to the last relay hop for relayed packets.
- **SSE stream** (`GET /api/events`): snapshot on connect, configurable replay of recent packets (default 6 h), live events.
- **REST API**: `GET /api/chat/list`, `GET /api/chat/{id}?hours=N`, `DELETE /api/chat/{id}`, `GET /api/positions`, `GET /api/health`, `POST /api/messages`.
- **Single-binary deployment**: SvelteKit frontend embedded in the Go binary via `embed.FS`; SPA client-side routing fallback included.
- **Docker image**: multi-stage, distroless, multi-platform (linux/amd64, linux/arm64, linux/arm/v6); `/data` volume for runtime state.
- **Release pipeline**: GoReleaser producing binaries for Linux (amd64 / arm64 / armv6), macOS, Raspberry Pi, and Windows.

### Fixed

- MeshCom packet parsing handles `firmware`, `hw_id`, and `batt` fields sent as JSON numbers instead of strings.
- SSE `packet.received` events carry the `type` field so the frontend correctly routes `msg`, `pos`, and `tele` packets.
