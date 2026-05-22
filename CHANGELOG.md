# Changelog

All notable changes to this project are documented in this file.

## [Unreleased]

### Added

- **IoT simulator granular auto-send flags**: `cmd/iot-simulator` now exposes `-enable-pos1`, `-enable-pos2`, `-enable-dm`, `-enable-broadcast`, and `-enable-chan2` so each timed send stream can be enabled independently while DM responders remain active. All responder transmissions now use configured `-target` UDP endpoint.
- **UDP stream replay cursor**: `/api/events` accepts `from=<RFC3339 timestamp>` and the web UDP stream clear action stores that cursor in `localStorage`, clears visible packets, and reconnects SSE from that point.
- **Map ruler overlay**: default map now has a disabled-by-default ruler button that draws green `MyCall -> direct station` lines and prints distance labels along each line in kilometers.
- **Realtime DM route tracking**: map toolbar now includes a toggle button that draws dashed hop-by-hop DM routes (`src -> via -> dst`) for live direct messages and automatically removes each route after 45 seconds.

### Changed

- **Chat message cards**: removed the raw JSON button from public and direct chat message cards.
- **DM ACK details**: direct-message chat cards now show every ACK source with its own RTT and relay path details instead of only the preferred ACK summary.

- **Event stream replay cursor capping**: `/api/events` now caps the `from` parameter to the configured `ReplayWindow` if `from` is further back in time.
- **IoT simulator command README**: documented local usage, responder behavior, common run modes, flags, and log output for `cmd/iot-simulator`.
- **Web UI helper refactoring**: extracted `ChatPanel`, `UdpStreamPanel`, and pure chat record/UDP stream presentation helpers from the monolithic `+page.svelte`, added unit coverage for those helpers, and documented the next component extraction slices.

### Fixed

- **Goroutine/subscription leak in HTTP server**: watch goroutines in the server now correctly unsubscribe and terminate on Close/Shutdown, resolving resource leaks in runtime and tests.
- **Realtime DM trace for ACK packets**: map live tracking now keeps `msg` ACK/reject packets in route tracing, so packets like `src=IU5RTR-02,IZ5CND-10` and `dst=IU5PMP-1` render both hop segments for 45 seconds.
- **Sanitized amateur radio callsigns**: audited and updated all mock/example/placeholder amateur radio callsigns to use compliant "QQ" prefix format across simulator commands, frontend Svelte pages, test files, and API docs.

- **DM ACK scoping**: ACK and reject indicators now match the sent message destination and local callsign, preventing ACKs for different messages with the same sequence number from appearing on the wrong chat card.
- **Replay packet filtering for chat/ACK UI**: frontend ACK indexing now ignores `packet.received` SSE events with `replay:true`, so replay bursts are not counted as extra ACKs on latest chat messages.

- **ACK timing**: packet SSE events and chat JSONL records now share the same backend `received_at` timestamp, and the web client uses backend time for chat and ACK RTT instead of browser arrival time.

- **Position signal freshness**: direct `msg`, `tele`, and `pos` packets without `rssi`/`snr` now preserve existing node signal values instead of overwriting them with `0`.

- **HTTP response caching**: all `/api/*` responses now send no-store cache headers, `/_app/immutable/*` assets use one-year immutable caching, and `index.html` requires revalidation.

- **Broadcast clear backend deletion**: the web UI now always sends the delete request when clearing the Broadcast chat so backend chat log files are removed even if local history state is empty.

- **DM send echo matching**: pending outbound DM records are now removed when the node echo appends a truncated sequence suffix such as `{42`, preventing duplicate spinner records.

---

## [0.5.0] - 2026-05-18

### Added

- **Chat message filter**: the web chat header now includes a filter field beside the delete/clear action so operators can search visible messages by text, source, destination, or message type.
- **About page reference repository**: the web About page now links the upstream reference repository and shows the `github.com` domain alongside existing GitHub issue reporting.
- **Persistent failed send status**: outbound chat messages appear immediately in the web chat with a pending spinner. After the accepted message is written to UDP, the backend waits up to 5 seconds for the node echo. If no echo arrives, it persists the message with `delivery_status:"failed"` and emits a `message.failed` event so the web chat shows a red `X` that survives reloads.
- **TX dry-run mode** (`GOMESHCOM_SEND_DISABLE_TX=true`): suppresses all outbound UDP writes. Each message that would have been sent is logged at `WARN` level with its JSON payload. The web UI shows a persistent amber banner and disables the send composer so operators immediately see that TX is disabled. Useful for monitoring-only deployments.
- **Responsive mobile layout**: the web UI adapts to narrow viewports (< 768px). On phones, Chat, Map, and UDP stream panels stack vertically. Drag handles are hidden on mobile, status indicators collapse to compact variants, chat typography shrinks slightly, and UDP stream rows hide secondary fields.
- **Chat sidebar collapse**: the chat channels column now has a header button that shrinks it into a narrow left rail so the message pane gets more horizontal space. The collapsed state persists in `localStorage`.
- **Collapsed `New DM` button**: when the chat sidebar is collapsed, the `New DM` action shortens to `DM +` to save space in the narrow rail.
- **Mobile collapsed chat rail**: when the chat sidebar is collapsed on small screens, the rail stays on the left of the message pane instead of stacking above it.
- **Configurable HTTP request logging**: `GOMESHCOM_REQUEST_LOG_ENABLED=true` logs structured request records with endpoint, status, caller IP, timestamp, and duration. Caller IP prefers `CF-Connecting-IP`, then `X-Real-IP`.
- **Remember last chat**: the web UI stores the last selected chat in `localStorage` and restores it on restart. If that conversation no longer exists, it opens Broadcast.
- **UDP RX forwarder** (`GOMESHCOM_FORWARD_TARGETS=host:port,...`): mirrors every received UDP datagram byte-for-byte to one or more downstream `gomeshcomd` instances. Enables multi-instance topologies where a single node feeds several processing nodes. Forwarding is best-effort (per-target buffered channel, drop-oldest on overflow) and happens before parsing so even malformed packets are mirrored.
- **`udpsend` tool** (`cmd/udpsend`): CLI utility to send a single UDP datagram to a `host:port`. Accepts payload as UTF-8 string (`-payload`) or hex string (`-hex`). Useful for manual testing and scripting.
- **`udprecv` tool** (`cmd/udprecv`): CLI utility to listen on a UDP address and print each received datagram with RFC3339Nano timestamp, source address, and byte count. Output is either quoted string (default) or hex dump (`-hex` flag). Configurable receive buffer via `-buf`.

### Changed

- Map marker clicks no longer open the station detail card; station `firstSeen` and `lastSeen` now appear directly in the hover tooltip.
- The local `MyCall` map marker hover now shows only callsign and device name, since station freshness metadata is not useful for the own marker.
- Web public/channel chat history now requests 7 days, while `DM_<CALLSIGN>` leaves the window unset so the backend's DM default applies.

### Fixed

- Outgoing message echo matching now accepts truncated numeric node sequence suffixes such as `{571`, preventing valid node echoes from being marked as failed.
- Public and channel chat sends now show a green cloud after the local node echo is observed, matching the existing failed-send indicator behavior.
- Web DM history requests no longer send a public/channel history window, allowing the backend's DM default window to apply.
- Direct-node hover and details keep showing `rssi`/`snr` after live `msg`/`pos` updates that omit those fields. Live freshness merges now preserve existing signal values instead of clearing them with `undefined`.
- Indirect `pos` packets now refresh `lastSeen` on every hop in the `via` chain. Signal values stay attached to direct packets and the last relay hop only.

---

## [0.4.2] - 2026-05-16

### Added

- First setup guide for MeshCom LAN deployment, including node IP discovery, ExtUDP destination configuration, firewall requirements for UDP `1799`, restart note, and startup examples.

### Fixed

- **Map — tooltip missing on standalone nodes**: hovering over a single node (not part of a cluster bubble) showed no tooltip. The `pointermove` handler only handled cluster features (which carry a `features[]` array); raw marker features (with a direct `position` property) were silently ignored. Both the hover tooltip and the click-to-select panel are now fixed to handle both feature types.

### Changed

- README quick start now links to the dedicated first setup guide and keeps the top-level setup overview concise.
- First setup guide now notes that public-IP deployments are possible but require extra routing and firewall care, and it clarifies when to bind the web UI to `0.0.0.0:8080` or a specific host IP.
- First setup guide now states that the MeshCom node must be connected to Wi-Fi before reading its IP or applying ExtUDP settings.

---

## [0.4.1] - 2026-05-16

### Fixed

- `crypto.randomUUID` not available in non-secure contexts (plain HTTP): SSE event ID generation now falls back to a `Date.now` + `Math.random` based ID when the Web Crypto API is unavailable.

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
