# MeshCom IoT UDP Simulator

`cmd/iot-simulator` is a small UDP node simulator for local goMeshCom tests.
It binds a UDP socket, logs incoming and outgoing datagrams, emits MeshCom
ExtUDP position packets, sends periodic direct messages, broadcasts, channel
messages, and simulates two DM responders. Automatic timed sends are opt-in via
`-enable-pos1`, `-enable-pos2`, `-enable-dm`, `-enable-broadcast`, and `-enable-chan2`;
without these flags the simulator runs in RX-driven responder mode.

## Usage

Run `gomeshcomd` on the default UDP port:

```sh
go run ./cmd/gomeshcomd --my-call="QQ5SIM-9"
```

Run the simulator in another terminal with the same local callsign:

```sh
go run ./cmd/iot-simulator -my-call QQ5SIM-9
```

Enable all timed sends:

```sh
go run ./cmd/iot-simulator -my-call QQ5SIM-9 -enable-pos1 -enable-pos2 -enable-dm -enable-broadcast -enable-chan2
```

The simulator sends packets to `127.0.0.1:1799` from a local UDP socket bound
to `:1798`.

## Position Packets

The default position is centered near:

```json
{
  "lat": 43.7303,
  "lat_dir": "N",
  "long": 10.3956,
  "long_dir": "E"
}
```

`QQ1TST-1` emits one `pos` packet every minute when `-enable-pos1` is set. `QQ1TST-2` emits one `pos` packet every two minutes when `-enable-pos2` is set. Both use the same base position and apply a random coordinate offset up to `0.0005` decimal degrees on latitude and longitude.

## Direct Messages

With `-enable-dm`, every minute the simulator sends one test DM from `QQ1TST-1` to `-my-call`. With `-enable-broadcast`, every minute it sends one broadcast message to `*`. With `-enable-chan2`, every minute it sends one channel message to `2`.

```json
{"src_type":"lora","type":"msg","src":"QQ1TST-1","dst":"QQ5SIM-9","msg":"DM test 1"}
```

Incoming UDP `msg` packets addressed to `QQ1TST-1` are handled as a direct
node response:

1. The simulator echoes the received message back as a UDP node packet from
   the configured `-target` as a packet toward `QQ1TST-1`, adding `{123` when the message has no sequence.
2. The simulator sends an ACK from `QQ1TST-1` to the same configured target, for example
   `QQ5SIM-9 :ack123`.

Incoming UDP `msg` packets addressed to `QQ1TST-2` are handled as a repeater:

1. The simulator echoes the received message back as a UDP node packet from
   the configured `-target` as a packet toward `QQ1TST-2`, adding `{123` when the message has no sequence.
2. After `-ack-delay` (`5s` by default), it sends an ACK from `QQ1TST-2` to
   the same configured target.
3. It sends a second DM from `QQ1TST-2` to the same configured target with
   `MIRROR: <original message>`.

Messages addressed to `*` or any numeric channel destination (for example `1`, `2`, `7`) are echoed to the configured `-target` with source `-my-call`, the same destination, and the original message text. Other destinations are logged but do not generate a response.

## Logging

Each received and transmitted datagram is logged on stderr in a human-readable
single line:

```text
14:23:05 | TX | msg  | QQ1TST-1 -> QQ5SIM-9 | {"type":"msg",...}
14:23:12 | RX | msg  | - -> QQ1TST-2 | {"type":"msg",...}
```

The fields are local time, direction, packet type, sender/destination, and raw
JSON.

## Flags

```sh
go run ./cmd/iot-simulator \
  -my-call QQ5SIM-9 \
  -target 192.168.1.50:1799 \
  -listen-addr :1798 \
  -interval 1m \
  -pos2-interval 2m \
  -dm-interval 1m \
  -enable-pos1 \
  -enable-pos2 \
  -enable-dm \
  -enable-broadcast \
  -enable-chan2 \
  -ack-delay 5s \
  -lat 43.7303 \
  -long 10.3956 \
  -jitter 0.0005
```

Use `-help` to show all available flags.
