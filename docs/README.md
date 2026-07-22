# Documentation

Documentation describes current, supported behavior. Design notes and unimplemented proposals do not belong here.

## Getting Started

- [First Setup](first-setup.md): configure a MeshCom node to send EXT UDP traffic to `gomeshcomd`.
- [Backend and Configuration](backend.md): configuration precedence, persistence, HTTP API, and operational behavior.
- [OpenAPI Contract](openapi.yaml): machine-readable HTTP API specification.

## Features

- [Graph View](graph.md): graph page and Map graph overlay behavior.
- [Statistics](statistics.md): hourly traffic aggregates and retention.
- [Chat Status Tracking](chat_status.md): unread state and conversation IDs.
- [IoT UDP Simulator](iot-simulator.md): local packet simulator flags and behavior.

## MeshCom Reference

- [UDP Message Format](message-udp.md)
- [Serial Message Format](message-serial.md)
- [Public Groups](groups.md)
- [Hardware IDs](hardware-ids.md)

External protocol details are documented only when they describe functionality this project currently supports. Refer to the upstream MeshCom firmware for device configuration and protocol authority.
