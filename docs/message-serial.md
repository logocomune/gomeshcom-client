# MeshCom Serial Message Format

## Overview

MeshCom uses two distinct serial interfaces:

| Interface | Feature flag | Hardware | Purpose |
|-----------|-------------|----------|---------|
| **SoftSerial** | `ENABLE_SOFTSER` | ESP32: SoftwareSerial; nRF52: Serial1 | External sensor / data logger integration |
| **ExtUDP Serial fallback** | `bEXTUDP = false` | USB/debug `Serial` | JSON output to serial when ExtUDP socket inactive |

---

## SoftSerial – External Sensor Interface

### Configuration

| Setting | Description |
|---------|-------------|
| `bSOFTSERON` | Enable flag |
| `node_ss_baud` | Baud rate (e.g. 9600, 115200) |
| `node_ss_rx_pin` | RX GPIO pin (ESP32 only) |
| `node_ss_tx_pin` | TX GPIO pin (ESP32 only) |
| Frame format | 8N1 |
| Timeout | 50 ms per read; 6000 ms max per query |

On nRF52, uses `Serial1` directly (no pin config).

Currently supported application: `SOFTSER_APP_ID = 1` (water level sensor, e.g. OTT netDL 500).

---

### Protocol: Node → Sensor (Query)

Commands are plain ASCII terminated with `\r`.  
Before every command, a bare `\r` is sent with a 500 ms delay to wake the device.

```
\r
<command>\r
```

| Command | Description |
|---------|-------------|
| `/cl/time/get` | Request current time from device |
| `/cl/data/get/<YYYYMMDDHHmmss>` | Request measurement data for given timestamp |

Query flow:

```
step 0:  send  /cl/time/get\r
step 1:  recv  <0x02>YYYYMMDDHHmmss<0x03>   → parse time, then send:
         send  /cl/data/get/YYYYMMDDHHmmss\r
step 2:  recv  <StationDataList>...</StationDataList>  → decode XML → inject telemetry
```

---

### Protocol: Sensor → Node (Response)

Responses are raw bytes read until `0x00` (null terminator) or 6000 ms timeout.  
Non-printable bytes (except `\r` `\n`) are escaped as `<0xNN>`.

#### Time response

```
<0x02>YYYYMMDDHHmmss<0x03>
```

| Part | Value |
|------|-------|
| `<0x02>` | STX (start of text) |
| 14 digits | `YYYYMMDDHHmmss` timestamp |
| `<0x03>` | ETX (end of text) |

Example: `<0x02>20250422130000<0x03>`

#### Data response – XML (OTT netDL format)

```xml
<StationDataList>
  <StationData stationId="0077234567" name="DemoStationNetDL500" timezone="+01:00">
    <StationInfo
      time="2025-04-22T13:04:19"
      firmware="V3080"
      batteryVoltage="13.34"
      temperature="28.62"
      deviceType="OTT netDL 500"
    />
    <ChannelData channelId="0060" name="Wasserstand" unit="cm"
                 samplingInterval="300" storageInterval="300">
      <Values>
        <VT t="2025-04-22T12:55:00">28.5</VT>
        <VT t="2025-04-22T13:00:00">30.1</VT>
      </Values>
    </ChannelData>
    <ChannelData channelId="0065" name="Wassertemperatur" unit="°C" ...>
      <Values>
        <VT t="2025-04-22T13:00:00">22.7</VT>
      </Values>
    </ChannelData>
    <ChannelData channelId="0050" name="Batteriespannung" unit="V" ...>
      <Values>
        <VT t="2025-04-22T13:00:00">12.9</VT>
      </Values>
    </ChannelData>
  </StationData>
</StationDataList>
```

**Parsed fields:**

| XML attribute / element | Stored in | Used for |
|------------------------|-----------|---------|
| `StationData@stationId` | `strSOFTSERAPP_ID` | APRS source callsign (last 9 chars) |
| `StationData@name` | `strSOFTSERAPP_NAME` | BITS station name |
| `StationData@timezone` | `strTELE_UTCOFF` | UTC offset (currently forced to 0.0) |
| `ChannelData@channelId` | `strTELE_CH_ID` | Channel IDs in T# message |
| `ChannelData@name` | `strTELE_PARM` | PARM parameter names |
| `ChannelData@unit` | `strTELE_UNIT` | UNIT strings |
| `VT` last value | `strTELE_VALUES` | Values in T# message |
| `VT@t` first timestamp | `strTELE_DATETIME` | Timestamp in T# message |

Only the **last** `<VT>` element per channel is used as the current value.

---

### APRS Telemetry Message Sequence

After decoding the XML, the node sends a **cyclic sequence** of APRS text messages  
(type `0x3A`, destination `100001` = telemetry sink, not relayed to end users).

`iNextTelemetry` cycles: 0 → 1 → 2 → 3 → 4+ (repeating values).

| Step | `iNextTelemetry` | Payload format | Example |
|------|-----------------|---------------|---------|
| 0 | 0 | `<id_9>:PARM.<ch_id> <name>[,...]` | `077234567:PARM.60 Wasserstand,65 Wassertemperatur,50 Batteriespannung` |
| 1 | 1 | `<id_9>:UNIT.<unit>[,...]` | `077234567:UNIT.cm,C,V` |
| 2 | 2 | `<id_9>:EQNS.<a,b,c x5>` | `077234567:EQNS.0,1,0,0,1,0,0,1,0,0,1,0,0,1,0` |
| 3 | 3 | `<id_9>:BITS.00000000<station_name>` | `077234567:BITS.00000000DemoStationNetDL500` |
| 4+ | ≥4 | `<id_9>:T#<seq>,<v1>,<v2>,<v3>,0,0,00000000,<datetime>,<ch_id1>,<ch_id2>,<ch_id3>` | `077234567:T#249,28.5,25.0,13.4,0,0,00000000,2025-04-22T13:00:00,60,65,50` |

**T# value fields:**

| Position | Content |
|----------|---------|
| `T#NNN` | 3-digit sequence number (`node_msgid`) |
| `v1..v3` | Up to 5 channel values (float, 1 decimal) |
| `0,0` | Two unused channels (always zero) |
| `00000000` | Digital bits (unused) |
| datetime | ISO8601 timestamp from last `<VT>` |
| ch_id1..N | Channel IDs from `<ChannelData@channelId>` (trimmed leading zeros) |

These messages are wrapped in the standard APRS binary payload (type `0x3A`) and forwarded over LoRa and UDP. See `message-udp.md` for binary payload structure.

**EQNS fields** — 5 triples `a,b,c` where `value = a*x² + b*x + c`.  
Default (no scaling): `0,1,0` per channel → value passes through unchanged.  
Override via `meshcom_settings.node_eqns`.

---

### Debug Serial Output (displaySOFTSER)

When the node receives a T# telemetry message from the network, it echoes a compact XML to the debug serial:

```xml
<SD Id="077234567" name="DemoStationNetDL500">
  <CD id="0060" name="Wasserstand" unit="cm"><VT t="2025-04-22T13:00:00">30.1</VT></CD>
  <CD id="0065" name="Wassertemperatur" unit="°C"><VT t="2025-04-22T13:00:00">22.7</VT></CD>
  <CD id="0050" name="Batteriespannung" unit="V"><VT t="2025-04-22T13:00:00">12.9</VT></CD>
</SD>
```

This is diagnostic output only (USB/debug serial), not transmitted over any network.

---

## ExtUDP Serial Fallback

When `bEXTUDP = false` (ExtUDP socket not started), `sendExtern()` falls back to printing the same JSON to the debug serial (`Serial.printf`) instead of sending a UDP packet.

Output is identical to the ExtUDP JSON format documented in `message-udp.md`:

```
[EXT] Out: {"src_type":"lora","type":"msg","src":"QQ1KBC-12","dst":"*","msg":"Hello","msg_id":"AABBCCDD",...} Len: 120
[EXT] Tele-Out: {"src_type":"lora","type":"tele","src":"QQ1KBC-12","batt":95,...} Len: 90
```

For long JSON strings, the log line is split at the midpoint to avoid serial buffer overflow:

```
[EXT] Out: <first half><second half> Len: <total>
```

---

## Source Files

| File | Role |
|------|------|
| `src/softser_functions.cpp` | Query/receive loop, XML decode trigger, debug output |
| `src/tinyxml_functions.cpp` | OTT netDL XML parser → fills `node_parm_*`, `node_unit`, `node_values` |
| `src/loop_functions.cpp` | `sendTelemetry()` — builds and injects APRS telemetry sequence |
| `src/extudp_functions.cpp` | `sendExtern()` — serial fallback when `bEXTUDP=false` |
| `src/configuration_global.h` | `SOFTSER_APP_ID`, pin/baud settings |
