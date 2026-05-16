# MeshCom UDP Message Format

## Overview

MeshCom nodes use two independent UDP channels:

| Channel | Port | Direction | Purpose |
|---------|------|-----------|---------|
| **MeshCom** | 1990 | node ↔ server | LoRa packet relay + keepalive |
| **ExtUDP** | 1799 | node ↔ external system | JSON integration interface |

Max UDP payload: **255 bytes** (`UDP_TX_BUF_SIZE`).  
Packets with more than 6 consecutive `0x00` bytes are rejected (`MAX_ZEROS = 6`).

Server addresses (auto-selected):

| Network | OE server | DL server |
|---------|-----------|-----------|
| Internet | `meshcom.oevsv.at` | `192.68.17.26` |
| Hamnet | `44.143.8.143` | `meshcom.hamnet.cloud` |

---

## Channel 1 – MeshCom Protocol (Port 1990)

Every packet starts with a 4-byte ASCII **indicator** that identifies the message type.

### Node → Server

#### `KEEP` – Registration / Heartbeat

Sent periodically to keep the server connection alive and announce the node.

```
KEEP<GW_ID_8HEX><CALL_9><VER_4><VERSUB_1>[GRC_IDS]
```

| Field | Len | Format | Description |
|-------|-----|--------|-------------|
| `KEEP` | 4 | ASCII | Indicator |
| GW_ID | 8 | hex uppercase | Gateway ID (`_GW_ID`, 32-bit, e.g. `FFFFFFFF`) |
| CALL | 9 | left-justified, space-padded | Node callsign (e.g. `QQ1KBC-12`) |
| VER | 4 | left-justified | Firmware version (e.g. `4.35`) |
| VERSUB | 1 | char | Firmware sub-version (e.g. `p`) |
| GRC_IDS | var | `N;N;...` | Semicolon-separated group chat IDs (optional) |

Example:
```
KEEPFFFFFFFFQQ1KBC-12 4.35p20;232;
```

#### `DATA` – LoRa Packet Relay

Sent when a LoRa packet is received and must be forwarded to the server.

```
DATA<GW_ID_8HEX><CALL_9><VER_4><VERSUB_1><RSSI_4><SNR_4><MOD_2><APRS_PAYLOAD>
```

| Field | Len | Format | Description |
|-------|-----|--------|-------------|
| `DATA` | 4 | ASCII | Indicator |
| GW_ID | 8 | hex uppercase | Gateway ID |
| CALL | 9 | left-justified, space-padded | Node callsign |
| VER | 4 | left-justified | Firmware version |
| VERSUB | 1 | char | Firmware sub-version |
| RSSI | 4 | decimal, space-padded | Signal strength in dBm (e.g. `-123`) |
| SNR | 4 | decimal, space-padded | Signal-to-noise ratio (e.g. `  -5`) |
| MOD | 2 | decimal | LoRa modulation index (`03` = LongSlow) |
| APRS payload | var | binary | See [APRS Binary Payload](#aprs-binary-payload) |

Example header (36 bytes) + binary payload:
```
DATAFFFFFFFFQQ1KBC-12 4.35p-123  -503<binary>
```

---

### Server → Node

#### `GATE` – LoRa Packet to Transmit

The server sends this when a packet must be broadcast over LoRa by the node.

```
GATE<APRS_PAYLOAD>
```

| Field | Len | Description |
|-------|-----|-------------|
| `GATE` | 4 | Indicator (0x47 0x41 0x54 0x45) |
| APRS payload | var | Binary APRS payload (see below) |

The first byte of the payload determines the message type:

| Byte | ASCII | Type |
|------|-------|------|
| `0x3A` | `:` | Text message |
| `0x21` | `!` | Position |
| `0x40` | `@` | Hey / ping |

#### `BEAT` – Server Heartbeat

Sent by the server to confirm connectivity.

```
BEAT<data>
```

| Field | Len | Description |
|-------|-----|-------------|
| `BEAT` | 4 | Indicator (0x42 0x45 0x41 0x54) |
| data | var | Server info (callsign, version – informational only) |

Example raw bytes:
```
42 45 41 54 00 09 4F 45 31 4B 46 52 2D 47 57 01 05 4B 46 52 36 35
B  E  A  T  ..     O  E  1  K  F  R  -  G  W  ..    K  F  R  6  5
```

---

## APRS Binary Payload

Used inside both `GATE` (incoming) and `DATA` (outgoing) messages.  
Minimum valid size: **16 bytes**.

### Header (6 bytes, fixed)

| Offset | Size | Description |
|--------|------|-------------|
| 0 | 1 | **Payload type** – `0x3A` text / `0x21` position / `0x40` hey |
| 1–4 | 4 | **Message ID** – uint32, little-endian |
| 5 | 1 | **Flags + max_hop** – bit field (see below) |

**Byte 5 bit field:**

| Bit | Mask | Meaning |
|-----|------|---------|
| 7 | `0x80` | `msg_server` – originated from server |
| 6 | `0x40` | `msg_track` – tracking enabled |
| 5 | `0x20` | `msg_app_offline` – app offline mode |
| 4 | `0x10` | `msg_mesh` – mesh routing |
| 3–0 | `0x0F` | `max_hop` – remaining hop count |

### Variable-length middle section

| Field | Terminator | Description |
|-------|-----------|-------------|
| **Source path** | `>` (0x3E) | Callsign + relay path, comma-separated (e.g. `QQ1KBC-12,QQ3CGG`) |
| **Destination path** | `payload_type` byte | Callsign or `*` for broadcast (e.g. `QQ5BYE-1`) |
| **Payload** | `0x00` | ASCII content (see payload formats below) |

### Trailer (fixed, appended after payload `0x00`)

| Offset from payload end | Size | Description |
|------------------------|------|-------------|
| +0 | 1 | `source_hw` – sender hardware ID |
| +1 | 1 | `source_mod` – bits [7:4] = country, bits [3:0] = modulation |
| +2 | 1 | FCS high byte (sum of all bytes [0..+1] >> 8) |
| +3 | 1 | FCS low byte (sum & 0xFF) |
| +4 | 1 | `fw_version` – firmware version (numeric, short) |
| +5 | 1 | `last_hw` – last-heard hardware ID (bit 7 set = last-heard flag) |
| +6 | 1 | `fw_sub_version` – firmware sub-version char |
| +7 | 1 | `0x7E` – end marker |

Full packet structure diagram:
```
[0]      payload_type
[1..4]   msg_id (LE uint32)
[5]      flags | max_hop
[6..n]   source_path ... '>'
[n+1..m] destination_path ... payload_type
[m+1..p] payload ... 0x00
[p+1]    source_hw
[p+2]    source_mod
[p+3]    FCS_HIGH
[p+4]    FCS_LOW
[p+5]    fw_version
[p+6]    last_hw
[p+7]    fw_sub_version
[p+8]    0x7E
```

---

## Payload Formats by Type

### Type `0x3A` – Text Message

ASCII string, one of:

| Pattern | Meaning |
|---------|---------|
| `:{*}<text>` | Broadcast to all nodes |
| `:{<callsign>}<text>` | Direct message |
| `:{<callsign>}<text>{<seq_num>}` | DM with sequence number (used for ACK) |
| `:ack<id>` | Acknowledge message with numeric ID |
| `:rej<id>` | Reject (NACK) message with numeric ID |
| `:{SET}<json>` | Node configuration command |
| `:{CET}<json>` | Node configuration response |

Special destination `100001` indicates telemetry (suppressed in ExtUDP output).

### Type `0x21` – Position

ASCII string, fields are positional or key-value:

```
<lat_ddmm.mm><N|S><aprs_group><lon_dddmm.mm><E|W><aprs_symbol>[<atxt>][/B=<bat>][/A=<alt>][/P=<press>][/H=<hum>][/T=<temp1>][/O=<temp2>][/F=<qfe>][/Q=<qnh>]
```

| Field | Key | Unit |
|-------|-----|------|
| Latitude | positional | ddmm.mm + N/S |
| APRS group | positional | char (e.g. `/`) |
| Longitude | positional | dddmm.mm + E/W |
| APRS symbol | positional | char (e.g. `&`) |
| Additional text | positional after symbol | free text up to 25 chars |
| Battery | `/B=` | % |
| Altitude | `/A=` | meters |
| Pressure (station) | `/P=` | hPa |
| Humidity | `/H=` | % |
| Temperature 1 | `/T=` | °C |
| Temperature 2 | `/O=` | °C |
| Pressure (sea level) | `/Q=` | hPa |
| QFE | `/F=` | hPa (integer) |

Example:
```
4807.01N/01619.20E&MeshCom Node/B=095/A=000161/P=1004.9/H=40.2/T=23.4/Q=1005.4
```

### Type `0x40` – Hey / Ping

Same structure as text message; payload is a short greeting or ping string.

---

## Channel 2 – ExtUDP JSON Interface (Port 1799)

Independent UDP socket, enabled via `bEXTUDP` flag. Allows external systems (e.g. Home Assistant, Node-RED) to send and receive MeshCom messages in JSON.

### Incoming (External → Node)

```json
{"type": "msg", "dst": "QQ5BYE-1", "msg": "Hello"}
```

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| `dst` | string | 1–9 chars | Destination callsign or `"*"` for broadcast |
| `msg` | string | 1–150 chars | Message text |

The node injects this as a local LoRa message (same path as a BLE/serial message).

### Outgoing (Node → External)

Sent for every LoRa or UDP packet the node processes. One or two UDP packets per event.

#### Text message (`type: "msg"`)

```json
{
  "src_type": "lora",
  "type":     "msg",
  "src":      "QQ1KBC-12,QQ3CGG",
  "dst":      "QQ5BYE-1",
  "msg":      "Hello World",
  "msg_id":   "AABBCCDD",
  "firmware": 35,
  "fw_sub":   "p",
  "rssi":     -123,
  "snr":      -5
}
```

| Field | Description |
|-------|-------------|
| `src_type` | Source: `"lora"` (received via LoRa), `"udp"` (received via server), `"node"` (own node) |
| `src` | Full source path (callsign + relays) |
| `dst` | Destination callsign or `"*"` |
| `msg_id` | 8-char hex message ID |
| `firmware` | Sender firmware version (numeric) |
| `fw_sub` | Firmware sub-version char |
| `rssi` | Received signal strength (dBm), 0 if `src_type="node"` |
| `snr` | Signal-to-noise ratio, 0 if `src_type="node"` |

#### Position (`type: "pos"`)

```json
{
  "src_type":          "lora",
  "type":              "pos",
  "src":               "QQ1KBC-12",
  "msg":               "",
  "lat":               48.1168,
  "lat_dir":           "N",
  "long":              16.3206,
  "long_dir":          "E",
  "aprs_symbol":       "&",
  "aprs_symbol_group": "/",
  "hw_id":             4,
  "msg_id":            "AABBCCDD",
  "alt":               161,
  "batt":              95,
  "firmware":          35,
  "fw_sub":            "p",
  "rssi":              -123,
  "snr":               -5
}
```

#### Telemetry (`type: "tele"`)

Sent as a second UDP packet immediately after a position packet when `src_type` is `"lora"` or `"node"`.

```json
{
  "src_type": "lora",
  "type":     "tele",
  "src":      "QQ1KBC-12",
  "batt":     95,
  "temp1":    23.4,
  "temp2":    0.0,
  "hum":      40.2,
  "qfe":      1004,
  "qnh":      1005.4,
  "gas":      50000.0,
  "co2":      420.0
}
```

| Field | Unit | Source (`node`) | Source (`lora`) |
|-------|------|-----------------|-----------------|
| `batt` | % | `meshcom_settings.node_proz` | from APRS position payload `/B=` |
| `temp1` | °C | `node_temp` | `/T=` |
| `temp2` | °C | `node_temp2` | `/O=` |
| `hum` | % | `node_hum` | `/H=` |
| `qfe` | hPa | `node_press` | `/F=` |
| `qnh` | hPa | `node_press_asl` | `/Q=` |
| `gas` | Ω | `node_gas_res` | gas resistance |
| `co2` | ppm | `node_co2` | CO₂ sensor |

---

## Source Files

| File | Role |
|------|------|
| `src/udp_functions.cpp` | MeshCom protocol (port 1990): RX, TX, KEEP, DATA, GATE, BEAT |
| `src/extudp_functions.cpp` | ExtUDP JSON interface (port 1799): send/receive |
| `src/aprs_functions.cpp` | APRS binary encode/decode (`encodeAPRS`, `decodeAPRS`) |
| `src/aprs_structures.h` | `aprsMessage`, `aprsPosition` struct definitions |
| `src/configuration_global.h` | Constants: ports, buffer sizes, limits |
