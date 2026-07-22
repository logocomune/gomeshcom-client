package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/stats"
)

const statsImportSource = "stats"

func (db *DB) ImportStats(ctx context.Context, path string) error {
	if imported, err := db.importDone(ctx, statsImportSource); err != nil {
		return err
	} else if imported {
		return nil
	}

	buckets, sourceInfo, err := readStats(path)
	if err != nil {
		return err
	}

	return db.WithTx(ctx, func(tx *sql.Tx) error {
		for _, bucket := range buckets {
			if err := insertStatsBucket(ctx, tx, bucket); err != nil {
				return err
			}
		}
		return recordImport(ctx, tx, statsImportSource, sourceInfo, len(buckets))
	})
}

func readStats(path string) (map[int64]*stats.Bucket, importSourceInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[int64]*stats.Bucket{}, importSourceInfo{path: path}, nil
		}
		return nil, importSourceInfo{}, fmt.Errorf("open stats import %s: %w", path, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, importSourceInfo{}, fmt.Errorf("stat stats import %s: %w", path, err)
	}

	var buckets map[int64]*stats.Bucket
	if err := json.NewDecoder(file).Decode(&buckets); err != nil {
		return nil, importSourceInfo{}, fmt.Errorf("decode stats import %s: %w", path, err)
	}
	if buckets == nil {
		buckets = map[int64]*stats.Bucket{}
	}
	for hour, bucket := range buckets {
		if bucket == nil {
			buckets[hour] = &stats.Bucket{HourUnix: hour}
			continue
		}
		if bucket.HourUnix == 0 {
			bucket.HourUnix = hour
		}
	}

	return buckets, importSourceInfo{path: path, mtime: stat.ModTime().UTC().Format(time.RFC3339Nano)}, nil
}

func insertStatsBucket(ctx context.Context, tx *sql.Tx, bucket *stats.Bucket) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO stats_hourly(hour_unix, dm, dm_ack, public, telemetry, position, errors, total)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(hour_unix) DO UPDATE SET
			dm = excluded.dm,
			dm_ack = excluded.dm_ack,
			public = excluded.public,
			telemetry = excluded.telemetry,
			position = excluded.position,
			errors = excluded.errors,
			total = excluded.total
	`, bucket.HourUnix, bucket.DM, bucket.DMAck, bucket.Public, bucket.Telemetry, bucket.Position, bucket.Errors, bucket.Total); err != nil {
		return fmt.Errorf("insert stats hourly %d: %w", bucket.HourUnix, err)
	}
	for key, count := range bucket.Channels {
		kind, target := statsChannelParts(key)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO stats_channels(hour_unix, kind, target, count)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(hour_unix, kind, target) DO UPDATE SET count = excluded.count
		`, bucket.HourUnix, kind, target, count); err != nil {
			return fmt.Errorf("insert stats channel %d/%s: %w", bucket.HourUnix, key, err)
		}
	}
	for label, count := range bucket.DistanceKm {
		start, end, err := statsDistanceRange(label)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO stats_distance(hour_unix, bucket_start_km, bucket_end_km, count)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(hour_unix, bucket_start_km) DO UPDATE SET bucket_end_km = excluded.bucket_end_km, count = excluded.count
		`, bucket.HourUnix, start, end, count); err != nil {
			return fmt.Errorf("insert stats distance %d/%s: %w", bucket.HourUnix, label, err)
		}
	}
	return nil
}

func statsChannelParts(key string) (string, string) {
	switch {
	case key == "broadcast":
		return "broadcast", "*"
	case strings.HasPrefix(key, "ch:"):
		return "channel", strings.TrimPrefix(key, "ch:")
	case strings.HasPrefix(key, "dm:"):
		return "dm", strings.TrimPrefix(key, "dm:")
	default:
		return "channel", key
	}
}

func statsDistanceRange(label string) (int, int, error) {
	if strings.HasSuffix(label, "+") {
		start, err := strconv.Atoi(strings.TrimSuffix(label, "+"))
		if err != nil {
			return 0, 0, fmt.Errorf("parse stats distance label %q: %w", label, err)
		}
		return start, start + 5, nil
	}
	parts := strings.Split(label, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("parse stats distance label %q", label)
	}
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("parse stats distance label %q: %w", label, err)
	}
	end, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("parse stats distance label %q: %w", label, err)
	}
	return start, end, nil
}
