package telemetry

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/meshcom"
)

type Store struct {
	db *sql.DB
}

func NewSQLite(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Append(ctx context.Context, packet meshcom.Telemetry, receivedAt time.Time) error {
	if s == nil || s.db == nil {
		return nil
	}
	source := strings.TrimSpace(packet.Source)
	if source == "" {
		return nil
	}
	origin, via := meshcom.SplitSourcePath(source)
	receivedAtText := receivedAt.UTC().Format(time.RFC3339Nano)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin telemetry transaction: %w", err)
	}
	if err := s.appendSamples(ctx, tx, packet, source, origin, receivedAtText); err != nil {
		_ = tx.Rollback()
		return err
	}
	if len(via) == 0 {
		if err := s.appendDirectSignal(ctx, tx, packet, source, origin, receivedAtText); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit telemetry transaction: %w", err)
	}
	return nil
}

func (s *Store) appendSamples(ctx context.Context, tx *sql.Tx, packet meshcom.Telemetry, source string, origin string, receivedAt string) error {
	for _, sample := range telemetrySamples(packet) {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO telemetry_samples(received_at, src, src_origin, src_type, metric, value)
			VALUES (?, ?, ?, ?, ?, ?)
		`, receivedAt, source, origin, nullString(packet.SourceType), sample.metric, sample.value)
		if err != nil {
			return fmt.Errorf("insert telemetry sample %s: %w", sample.metric, err)
		}
	}
	return nil
}

func (s *Store) appendDirectSignal(ctx context.Context, tx *sql.Tx, packet meshcom.Telemetry, source string, origin string, receivedAt string) error {
	if packet.RSSI == nil && packet.SNR == nil {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO telemetry_direct_signal(received_at, src, src_origin, src_type, rssi, snr)
		VALUES (?, ?, ?, ?, ?, ?)
	`, receivedAt, source, origin, nullString(packet.SourceType), nullableInt(packet.RSSI), nullableInt(packet.SNR))
	if err != nil {
		return fmt.Errorf("insert telemetry direct signal: %w", err)
	}
	return nil
}

type sample struct {
	metric string
	value  float64
}

func telemetrySamples(packet meshcom.Telemetry) []sample {
	samples := make([]sample, 0, 8)
	samples = appendIntSample(samples, "batt", packet.Battery)
	samples = appendFloatSample(samples, "temp1", packet.Temp1)
	samples = appendFloatSample(samples, "temp2", packet.Temp2)
	samples = appendFloatSample(samples, "hum", packet.Humidity)
	samples = appendFloatSample(samples, "qfe", packet.QFE)
	samples = appendFloatSample(samples, "qnh", packet.QNH)
	samples = appendFloatSample(samples, "gas", packet.Gas)
	samples = appendFloatSample(samples, "co2", packet.CO2)
	return samples
}

func appendFloatSample(samples []sample, metric string, value *float64) []sample {
	if value == nil {
		return samples
	}
	return append(samples, sample{metric: metric, value: *value})
}

func appendIntSample(samples []sample, metric string, value *int) []sample {
	if value == nil {
		return samples
	}
	return append(samples, sample{metric: metric, value: float64(*value)})
}

func nullString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}
