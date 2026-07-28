# Chat Text Actions

Chat messages render radio-amateur callsigns and web links without interpreting any other message text as HTML.

## Callsigns

Recognized callsigns are underlined in both public channels and direct messages. A callsign without a numeric SSID, such as `@IU5PMP`, has no action menu. A callsign with a numeric SSID, such as `@IU5PMP-12`, opens a menu with:

- **Chat**: opens a direct-message conversation for that callsign.
- **Map**: appears only when current node data contains coordinates; it opens Map and centers that node.

## Web links

Recognized `http://` and `https://` links are underlined. Selecting one opens a menu with **Follow link**. This action opens the target in a separate page with `noopener,noreferrer` protections.

## Broadcast time beacons

Broadcast hides `{CET}` network-time beacons by default. Enable **Show time beacons** in the Broadcast header to display them. The filter does not affect direct messages or other public channels.
