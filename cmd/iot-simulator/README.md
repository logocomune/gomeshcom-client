# MeshCom IoT Simulator

`iot-simulator` emits MeshCom ExtUDP packets and responds to UDP messages for
local `gomeshcomd` testing. It is useful when no physical node is available or
when repeatable traffic is needed for chat, map, and UDP stream checks.

The simulator binds one UDP socket, sends packets to a target endpoint, logs RX
and TX datagrams to stderr, and can act as two test stations:

- `QQ1TST-1`: direct node, position sender, DM sender, DM responder.
- `QQ1TST-2`: repeater-like node, position sender, delayed ACK and mirror responder.

## Quick Start

Start `gomeshcomd` with a local callsign:

```sh
go run ./cmd/gomeshcomd --my-call QQ5SIM-9
```

Run the simulator in another terminal:

```sh
go run ./cmd/iot-simulator -my-call QQ5SIM-9
```

By default, the simulator listens on `:1798`, sends to `127.0.0.1:1799`, and
waits for incoming messages. Automatic sends are disabled until explicitly
enabled.

Enable all periodic traffic:

```sh
go run ./cmd/iot-simulator \
  -my-call QQ5SIM-9 \
  -enable-pos1 \
  -enable-pos2 \
  -enable-dm \
  -enable-broadcast \
  -enable-chan2
```

## Common Runs

Send only position packets from `QQ1TST-1`:

```sh
go run ./cmd/iot-simulator -my-call QQ5SIM-9 -enable-pos1
```

Send positions from both simulated stations:

```sh
go run ./cmd/iot-simulator \
  -my-call QQ5SIM-9 \
  -enable-pos1 \
  -enable-pos2 \
  -interval 30s \
  -pos2-interval 45s
```

Send DM, broadcast, and channel traffic:

```sh
go run ./cmd/iot-simulator \
  -my-call QQ5SIM-9 \
  -enable-dm \
  -enable-broadcast \
  -enable-chan2 \
  -dm-interval 15s
```

Target a remote `gomeshcomd` UDP listener:

```sh
go run ./cmd/iot-simulator \
  -my-call QQ5SIM-9 \
  -listen-addr :1798 \
  -target 192.168.1.50:1799
```

## Responder Behavior

The simulator always listens for incoming UDP packets while running.

Messages addressed to `QQ1TST-1` generate:

1. Echo from the original sender to `QQ1TST-1`.
2. ACK from `QQ1TST-1` to the original sender.

Messages addressed to `QQ1TST-2` generate:

1. Echo from the original sender to `QQ1TST-2`.
2. ACK from `QQ1TST-2` after `-ack-delay`.
3. Mirror DM from `QQ1TST-2` with `MIRROR: <original message>`.

Messages addressed to `*` or a numeric channel such as `1`, `2`, or `7` are
echoed from `-my-call` to the same destination. Other destinations are ignored.

If an incoming message has no trailing MeshCom sequence such as `{123`, the
simulator adds `{123` before sending echo and ACK responses.

## Flags

| Flag | Default | Description |
| --- | --- | --- |
| `-my-call` | required | Local callsign used as DM destination and broadcast/channel echo source. |
| `-listen-addr` | `:1798` | Local UDP address for simulator RX and TX socket. |
| `-target` | `127.0.0.1:1799` | UDP destination for generated packets. |
| `-enable-pos1` | `false` | Enable periodic `QQ1TST-1` position packets. |
| `-enable-pos2` | `false` | Enable periodic `QQ1TST-2` position packets. |
| `-enable-dm` | `false` | Enable periodic DM packets from `QQ1TST-1` to `-my-call`. |
| `-enable-broadcast` | `false` | Enable periodic broadcast packets to `*`. |
| `-enable-chan2` | `false` | Enable periodic channel packets to `2`. |
| `-interval` | `1m` | `QQ1TST-1` position interval. |
| `-pos2-interval` | `2m` | `QQ1TST-2` position interval. |
| `-dm-interval` | `1m` | DM, broadcast, and channel send interval. |
| `-ack-delay` | `5s` | Delay before `QQ1TST-2` ACK and mirror response. |
| `-lat` | `43.7303` | Base latitude for position packets. |
| `-long` | `10.3956` | Base longitude for position packets. |
| `-jitter` | `0.0005` | Maximum random coordinate offset in decimal degrees. |
| `-alt` | `12` | Simulated altitude in meters. |
| `-batt` | `95` | Simulated battery percentage. |
| `-hw-id` | `IOT-SIM` | Simulated hardware identifier. |
| `-firmware` | `sim` | Simulated firmware version. |
| `-seed` | current time | Random seed for deterministic positions and message IDs. |

Run `go run ./cmd/iot-simulator -help` for the current flag list.

## Logs

The simulator writes startup, send, receive, and parsed packet logs to stderr:

```text
iot-simulator listening on [::]:1798, target 127.0.0.1:1799, my-call QQ5SIM-9
sent pos 238 bytes to 127.0.0.1:1799 src=QQ1TST-1 lat=43.730218 long=10.395919 msg_id=7F3A91D2
14:23:05 | TX | pos  | QQ1TST-1 -> - | {"type":"pos",...}
14:23:12 | RX | msg  | QQ5SIM-9 -> QQ1TST-1 | {"type":"msg",...}
```

## Related Documentation

See [docs/iot-simulator.md](../../docs/iot-simulator.md) for higher-level
workflow notes.
