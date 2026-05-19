# Emulator Changelog

All notable emulator and simulator changes are documented in this file.

## [Unreleased]

### Added

- **MeshCom IoT UDP simulator** (`cmd/iot-simulator`): local ExtUDP simulator that listens on `-listen-addr` (`:1798` by default) and sends generated packets to `-target` (`127.0.0.1:1799` by default). `-my-call` is required and is used as the local station callsign for responder flows.
- **Granular automatic send flags**: timed transmissions are disabled by default and can be enabled independently with `-enable-pos1`, `-enable-pos2`, `-enable-dm`, `-enable-broadcast`, and `-enable-chan2`.
- **Position simulation**: `QQ1TST-1` and `QQ1TST-2` can emit `pos` packets on separate intervals (`-interval`, `-pos2-interval`) using configurable base coordinates (`-lat`, `-long`), jitter (`-jitter`), altitude (`-alt`), battery (`-batt`), hardware ID (`-hw-id`), firmware (`-firmware`), and random seed (`-seed`).
- **Message simulation**: when enabled, the simulator sends periodic DMs to `-my-call`, broadcasts to `*`, and channel messages to destination `2` using `-dm-interval`.
- **Responder routing**: incoming messages to `QQ1TST-1` echo the original message with a sequence suffix and ACK the sender; incoming messages to `QQ1TST-2` act as repeater traffic, echo immediately, then ACK and send `MIRROR: <message>` after `-ack-delay`.
- **Public/channel responder**: incoming messages with destination `*` or numeric channel destinations are forwarded back to `-target` with source `-my-call` and the same destination.
- **Readable simulator logs**: every TX/RX datagram logs local time, direction, packet type, sender, destination, byte count, target address, and raw JSON.
