# Statistics

The Statistics page (`/statistics`) shows aggregated packet traffic heard by the local node.

## Data model

All hourly buckets are stored in a single file:

```
data/stats/stats.json
```

The file is a JSON object keyed by UTC-hour Unix timestamp. Example:

```json
{
  "1748692800": {
    "hour": 1748692800,
    "dm": 3,
    "dm_ack": 2,
    "public": 17,
    "telemetry": 8,
    "position": 5,
    "errors": 0,
    "total": 33,
    "distance_km": { "0-5": 2, "5-10": 1, "45-50": 1 }
  },
  "1748696400": { "hour": 1748696400, "dm": 1, ... }
}
```

| Field | Meaning |
|---|---|
| `hour` | Unix timestamp (UTC), truncated to the hour |
| `dm` | Received DM text messages |
| `dm_ack` | Outgoing DM messages confirmed delivered (echo matched) |
| `public` | Broadcast / channel text messages |
| `telemetry` | Telemetry packets |
| `position` | Position packets |
| `errors` | UDP datagrams that failed to parse |
| `total` | `dm + public + telemetry + position` (errors excluded) |
| `distance_km` | Count of position packets bucketed in 5 km bins from own station |

## Cleanup / retention

On every periodic flush (every minute), buckets older than `GOMESHCOM_STATS_RETENTION_DAYS` (default `30`) are removed from the in-memory map and the file is rewritten. Set to `0` for infinite retention.

## Distance histogram

Computed using the Haversine great-circle distance between the sender's last reported position and the own station's position (callsign `GOMESHCOM_MY_CALL`). If the own station has no known position, the distance field is omitted and the histogram is empty.

Bucket width: 5 km. Bins above 100 km are merged into `100+`.

## API

```
GET /api/stats?hours=N
```

- `hours` — time window (default `24`, max `720`).
- Returns `{ from, to, hours, buckets: Bucket[] }` sorted by hour ascending.

## Configuration

| Env var | Default | Description |
|---|---|---|
| `GOMESHCOM_STATS_ENABLED` | `true` | Enable/disable stats collection |
| `GOMESHCOM_STATS_PATH` | `./data/stats/stats.json` | Path to the stats JSON file |
| `GOMESHCOM_STATS_RETENTION_DAYS` | `30` | Days of hourly buckets to retain |
