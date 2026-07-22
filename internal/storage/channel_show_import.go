package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/channelshow"
)

const channelShowImportSource = "channel_show"

func (db *DB) ImportChannelShow(ctx context.Context, path string) error {
	if imported, err := db.importDone(ctx, channelShowImportSource); err != nil {
		return err
	} else if imported {
		return nil
	}

	config, sourceInfo, err := readChannelShow(path)
	if err != nil {
		return err
	}

	return db.WithTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO channel_show(id, mode) VALUES (1, ?) ON CONFLICT(id) DO UPDATE SET mode = excluded.mode`, config.Mode); err != nil {
			return fmt.Errorf("insert channel show mode: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM channel_show_channels`); err != nil {
			return fmt.Errorf("clear channel show channels: %w", err)
		}
		for _, channel := range config.Channels {
			lastMessageAt, err := channelLastMessageAt(ctx, tx, channel)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO channel_show_channels(channel, last_message_at) VALUES (?, ?)`, channel, lastMessageAt); err != nil {
				return fmt.Errorf("insert channel show channel %s: %w", channel, err)
			}
		}
		return recordImport(ctx, tx, channelShowImportSource, sourceInfo, len(config.Channels))
	})
}

func readChannelShow(path string) (channelshow.Config, importSourceInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return channelshow.DefaultConfig(), importSourceInfo{path: path}, nil
		}
		return channelshow.Config{}, importSourceInfo{}, fmt.Errorf("open channel show import %s: %w", path, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return channelshow.Config{}, importSourceInfo{}, fmt.Errorf("stat channel show import %s: %w", path, err)
	}
	var config channelshow.Config
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return channelshow.Config{}, importSourceInfo{}, fmt.Errorf("decode channel show import %s: %w", path, err)
	}
	normalized, err := channelshow.Normalize(config)
	if err != nil {
		return channelshow.Config{}, importSourceInfo{}, fmt.Errorf("validate channel show import %s: %w", path, err)
	}
	return normalized, importSourceInfo{path: path, mtime: stat.ModTime().UTC().Format(time.RFC3339Nano)}, nil
}

func channelLastMessageAt(ctx context.Context, tx *sql.Tx, channel string) (any, error) {
	var value sql.NullString
	var err error
	if channel == "*" {
		err = tx.QueryRowContext(ctx, `SELECT MAX(received_at) FROM chats_public WHERE kind = 'broadcast'`).Scan(&value)
	} else {
		err = tx.QueryRowContext(ctx, `SELECT MAX(received_at) FROM chats_public WHERE kind = 'channel' AND channel = ?`, channel).Scan(&value)
	}
	if err != nil {
		return nil, fmt.Errorf("query channel show last message %s: %w", channel, err)
	}
	if !value.Valid {
		return nil, nil
	}
	return value.String, nil
}
