package storage

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS data_imports (
		source TEXT PRIMARY KEY,
		imported_at TEXT NOT NULL,
		source_path TEXT,
		source_mtime TEXT,
		record_count INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE TABLE IF NOT EXISTS receive_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		received_at TEXT NOT NULL,
		remote_addr TEXT NOT NULL,
		bytes INTEGER NOT NULL,
		raw TEXT NOT NULL,
		packet_type TEXT,
		parse_error TEXT,
		src TEXT,
		src_origin TEXT,
		src_type TEXT,
		dst TEXT,
		msg_id TEXT,
		msg TEXT,
		rssi INTEGER,
		snr INTEGER,
		lat REAL,
		lng REAL,
		alt INTEGER,
		hw_id TEXT,
		batt INTEGER,
		temp1 REAL,
		temp2 REAL,
		hum REAL,
		qfe REAL,
		qnh REAL,
		gas REAL,
		co2 REAL
	)`,
	`CREATE INDEX IF NOT EXISTS receive_log_received_at_idx ON receive_log(received_at)`,
	`CREATE INDEX IF NOT EXISTS receive_log_packet_type_time_idx ON receive_log(packet_type, received_at)`,
	`CREATE INDEX IF NOT EXISTS receive_log_src_origin_time_idx ON receive_log(src_origin, received_at)`,
	`CREATE INDEX IF NOT EXISTS receive_log_dst_time_idx ON receive_log(dst, received_at)`,
	`CREATE TABLE IF NOT EXISTS telemetry_samples (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		received_at TEXT NOT NULL,
		src TEXT NOT NULL,
		src_origin TEXT NOT NULL,
		src_type TEXT,
		metric TEXT NOT NULL CHECK(metric IN ('batt', 'temp1', 'temp2', 'hum', 'qfe', 'qnh', 'gas', 'co2')),
		value REAL NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS telemetry_samples_src_time_idx ON telemetry_samples(src_origin, received_at)`,
	`CREATE INDEX IF NOT EXISTS telemetry_samples_metric_time_idx ON telemetry_samples(metric, received_at)`,
	`CREATE INDEX IF NOT EXISTS telemetry_samples_src_metric_time_idx ON telemetry_samples(src_origin, metric, received_at)`,
	`CREATE TABLE IF NOT EXISTS telemetry_direct_signal (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		received_at TEXT NOT NULL,
		src TEXT NOT NULL,
		src_origin TEXT NOT NULL,
		src_type TEXT,
		rssi INTEGER,
		snr INTEGER,
		CHECK(rssi IS NOT NULL OR snr IS NOT NULL)
	)`,
	`CREATE INDEX IF NOT EXISTS telemetry_direct_signal_src_time_idx ON telemetry_direct_signal(src_origin, received_at)`,
	`CREATE TABLE IF NOT EXISTS chats_dm (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		conversation_id TEXT NOT NULL,
		msg_id TEXT,
		sequence_id TEXT,
		received_at TEXT NOT NULL,
		src TEXT,
		src_type TEXT,
		via TEXT CHECK (via IS NULL OR (json_valid(via) AND json_type(via) = 'array')),
		dst TEXT NOT NULL,
		msg TEXT NOT NULL,
		rssi INTEGER,
		snr INTEGER,
		direction TEXT CHECK(direction IS NULL OR direction IN ('outbound')),
		delivery_status TEXT CHECK(delivery_status IS NULL OR delivery_status IN ('failed')),
		ack_status TEXT CHECK(ack_status IS NULL OR ack_status IN ('ack', 'reject')),
		ack_received_at TEXT,
		ack_src TEXT,
		ack_src_type TEXT,
		ack_rssi INTEGER,
		ack_snr INTEGER,
		ack_via TEXT CHECK (ack_via IS NULL OR (json_valid(ack_via) AND json_type(ack_via) = 'array'))
	)`,
	`CREATE INDEX IF NOT EXISTS chats_dm_conversation_time_idx ON chats_dm(conversation_id, received_at)`,
	`CREATE INDEX IF NOT EXISTS chats_dm_msg_id_idx ON chats_dm(msg_id) WHERE msg_id IS NOT NULL AND msg_id != ''`,
	`CREATE INDEX IF NOT EXISTS chats_dm_pending_ack_idx ON chats_dm(sequence_id, received_at) WHERE direction = 'outbound' AND sequence_id IS NOT NULL AND ack_status IS NULL`,
	`CREATE TABLE IF NOT EXISTS chats_public (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		conversation_id TEXT NOT NULL,
		kind TEXT NOT NULL CHECK(kind IN ('broadcast', 'channel')),
		channel TEXT,
		msg_id TEXT,
		received_at TEXT NOT NULL,
		src TEXT,
		src_type TEXT,
		via TEXT CHECK (via IS NULL OR (json_valid(via) AND json_type(via) = 'array')),
		dst TEXT NOT NULL,
		msg TEXT NOT NULL,
		rssi INTEGER,
		snr INTEGER
	)`,
	`CREATE INDEX IF NOT EXISTS chats_public_conversation_time_idx ON chats_public(conversation_id, received_at)`,
	`CREATE INDEX IF NOT EXISTS chats_public_channel_time_idx ON chats_public(channel, received_at) WHERE channel IS NOT NULL`,
	`CREATE INDEX IF NOT EXISTS chats_public_msg_id_idx ON chats_public(msg_id) WHERE msg_id IS NOT NULL AND msg_id != ''`,
	`CREATE TABLE IF NOT EXISTS chat_reads (
		conversation_id TEXT PRIMARY KEY,
		last_read TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS nodes (
		node_id TEXT PRIMARY KEY,
		lat REAL,
		lng REAL,
		alt INTEGER,
		hw_id TEXT,
		firstseen TEXT,
		lastseen TEXT,
		lastdirectseen TEXT,
		rssi INTEGER,
		snr INTEGER,
		via TEXT CHECK (via IS NULL OR (json_valid(via) AND json_type(via) = 'array'))
	)`,
	`CREATE INDEX IF NOT EXISTS nodes_lastseen_idx ON nodes(lastseen)`,
	`CREATE INDEX IF NOT EXISTS nodes_hw_id_idx ON nodes(hw_id)`,
	`CREATE TABLE IF NOT EXISTS stats_hourly (
		hour_unix INTEGER PRIMARY KEY,
		dm INTEGER NOT NULL,
		dm_ack INTEGER NOT NULL,
		public INTEGER NOT NULL,
		telemetry INTEGER NOT NULL,
		position INTEGER NOT NULL,
		errors INTEGER NOT NULL,
		total INTEGER NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS stats_channels (
		hour_unix INTEGER NOT NULL,
		kind TEXT NOT NULL CHECK(kind IN ('broadcast', 'channel', 'dm')),
		target TEXT NOT NULL,
		count INTEGER NOT NULL,
		PRIMARY KEY (hour_unix, kind, target),
		FOREIGN KEY (hour_unix) REFERENCES stats_hourly(hour_unix) ON DELETE CASCADE
	)`,
	`CREATE INDEX IF NOT EXISTS stats_channels_kind_hour_idx ON stats_channels(kind, hour_unix)`,
	`CREATE INDEX IF NOT EXISTS stats_channels_target_hour_idx ON stats_channels(target, hour_unix)`,
	`CREATE TABLE IF NOT EXISTS stats_distance (
		hour_unix INTEGER NOT NULL,
		bucket_start_km INTEGER NOT NULL,
		bucket_end_km INTEGER NOT NULL,
		count INTEGER NOT NULL,
		PRIMARY KEY (hour_unix, bucket_start_km),
		CHECK (bucket_start_km >= 0),
		CHECK (bucket_end_km = bucket_start_km + 5),
		FOREIGN KEY (hour_unix) REFERENCES stats_hourly(hour_unix) ON DELETE CASCADE
	)`,
	`CREATE INDEX IF NOT EXISTS stats_distance_bucket_hour_idx ON stats_distance(bucket_start_km, hour_unix)`,
	`CREATE TABLE IF NOT EXISTS dm_stats (
		callsign TEXT PRIMARY KEY,
		sent INTEGER NOT NULL,
		ack INTEGER NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS channel_show (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		mode TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS channel_show_channels (
		channel TEXT PRIMARY KEY,
		last_message_at TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS channel_show_channels_last_message_at_idx ON channel_show_channels(last_message_at)`,
	`CREATE TABLE IF NOT EXISTS station_identity (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		callsign TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS http_sessions (
		token_hash TEXT PRIMARY KEY,
		expires_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS http_sessions_expires_at_idx ON http_sessions(expires_at)`,
}
