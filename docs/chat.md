# Chat Text Actions

Chat messages render radio-amateur callsigns and web links without interpreting any other message text as HTML.

## Callsigns

Recognized callsigns are underlined in both public channels and direct messages. A callsign without a numeric SSID, such as `@IU5PMP`, has no action menu. A callsign with a numeric SSID, such as `@IU5PMP-12`, opens a menu with:

- **Chat**: opens a direct-message conversation for that callsign.
- **Map**: appears only when current node data contains coordinates; it opens Map and centers that node.

## Web links

Recognized `http://` and `https://` links are underlined. Selecting one opens a menu with **Follow link**. This action opens the target in a separate page with `noopener,noreferrer` protections.

## Control frames

MeshCom carries protocol frames inside ordinary text payloads. goMeshCom classifies them and never renders them as chat messages:

| Frame | Payload | Handling |
|-------|---------|----------|
| ACK | `:ackNNN`, `ackNNN`, `CALL :ackNNN` | Delivery tracking only |
| Reject | `:rejNNN`, `rejNNN` | Delivery tracking only |
| Custody notice | Nine-byte addressee followed by `:stoNNN <dst>` | Node freshness only |

A custody notice is sent by a store-and-forward node to the original sender of a message it holds. The backend records the datagram in the receive log and refreshes the node's last-heard state, but it does not write it to the chat history, so it opens no conversation, updates no preview, and raises no unread badge, sound, or toast. The traffic view still lists it, labelled **Custody notice**.

The tag must sit exactly at UTF-8 byte 9 followed by three ASCII digits. Both backend and frontend count bytes, including for Unicode text. For example, `QQ1OWN-1 :sto156 QQ1PEER-2` is a notice; text mentioning `:sto` at another byte offset stays a normal message. Payloads containing `{`, `:ack`, or `:rej` are not custody notices.

Custody notices already stored by an earlier release remain in SQLite. They are hidden from messages and previews and excluded from conversation discovery, activity timestamps, record counts, and unread status. A conversation containing only custody notices no longer appears in the conversation list. Existing read markers are preserved; no data migration or deletion is needed.

## Broadcast time beacons

Broadcast hides `{CET}` network-time beacons by default. Enable **Show time beacons** in the Broadcast header to display them. The filter does not affect direct messages or other public channels.
